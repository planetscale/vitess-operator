/*
Copyright 2019 PlanetScale.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package vitessshardreplication

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"vitess.io/vitess/go/mysql"

	"vitess.io/vitess/go/sqltypes"
	"vitess.io/vitess/go/vt/topo"

	topodatapb "vitess.io/vitess/go/vt/proto/topodata"
	"vitess.io/vitess/go/vt/topo/topoproto"
	"vitess.io/vitess/go/vt/wrangler"

	// register grpc tabletmanager client
	_ "vitess.io/vitess/go/vt/vttablet/grpctmclient"
	"vitess.io/vitess/go/vt/vttablet/tmclient"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/results"
)

const (
	repairReplicationTimeout = 15 * time.Second
)

/*
repairReplication check if any replica tablets have broken replication,
and tries to fix them if it's safe to do so automatically.

For example, one scenario this handles is if the replica has the wrong master
address, which could happen if it missed the message the last time the master
changed. This periodic repair ensures such missed messages are eventually
corrected. This also handles when a master restarts and comes up read-only,
which is a safety mechanism to wait for confirmation that it's still the master,
which we provide here.

If any repair is attempted, it will be done while holding the shard lock, so it
won't happen concurrently with other Vitess replication management operations.
*/
func (r *ReconcileVitessShard) repairReplication(ctx context.Context, vts *planetscalev2.VitessShard, wr *wrangler.Wrangler) (reconcile.Result, error) {
	// TODO(enisoc): Upstream changes to make Vitess capable of fixing this on its own.
	/*
		Currently, Vitess only tries to fix the case when a tablet has both its
		SQL and IO threads stopped, which usually means mysqld has restarted.
		This repair mechanism in Vitess is implemented inside each tablet (in
		the replication_reporter), and as a result, it lacks context on whether
		other tablets are also broken. To avoid a thundering herd of tablets
		trying to acquire the shard lock and perform reparent operations all at
		once (such as would happen when all tablets are broken at once due to a
		master going down) requires a "global" component that polls all tablets.
		Vitess does not yet have such a component, so for now we do this in the
		Kubernetes operator.
	*/

	keyspaceName := vts.Labels[planetscalev2.KeyspaceLabel]
	resultBuilder := &results.Builder{}

	// If using external datastore then replication is handled for us.
	if vts.UsingExternalDatastore() {
		return resultBuilder.Result()
	}

	// Don't hold our slot in the reconcile work queue for too long.
	ctx, cancel := context.WithTimeout(ctx, repairReplicationTimeout)
	defer cancel()

	canRepair, err := r.canRepairReplication(ctx, vts, wr)
	if err != nil {
		r.recorder.Eventf(vts, corev1.EventTypeWarning, "RepairCheckFailed", "failed to check whether replication repair is needed: %v", err)
		return resultBuilder.Result()
	}
	if !canRepair {
		// There's nothing wrong that we know how to fix.
		return resultBuilder.Result()
	}

	// If we get here, we found something we think we can fix. Acquire the shard
	// lock so we don't fight with other replication management operations.
	var actionErr error
	ctx, unlock, lockErr := wr.TopoServer().LockShard(ctx, keyspaceName, vts.Spec.Name, "RepairReplication")
	if lockErr != nil {
		return resultBuilder.Error(lockErr)
	}
	defer unlock(&actionErr)

	actionErr = r.repairReplicationLocked(ctx, vts, wr)
	if actionErr != nil {
		r.recorder.Eventf(vts, corev1.EventTypeWarning, "RepairReplicationFailed", "failed to repair replication: %v", actionErr)
		return resultBuilder.Error(actionErr)
	}
	return resultBuilder.Result()
}

/*
canRepairReplication checks for any type of breakage that we know how to fix.

This is done without any locks, and should be quick enough to poll frequently,
especially in the "happy path" when replication is healthy. If any repair is
needed, we will re-check everything after acquiring the shard lock.
*/
func (r *ReconcileVitessShard) canRepairReplication(ctx context.Context, vts *planetscalev2.VitessShard, wr *wrangler.Wrangler) (bool, error) {
	// Check who the master is according to the shard record.
	if vts.Status.MasterAlias == "" {
		// The shard has no master, or we weren't able to read topo.
		// Either way, there's nothing we can do.
		return false, fmt.Errorf("no master for shard")
	}
	masterAlias, err := topoproto.ParseTabletAlias(vts.Status.MasterAlias)
	if err != nil {
		// This should never happen. There's no point retrying.
		return false, fmt.Errorf("invalid master alias: %v", err)
	}
	masterTabletInfo, err := wr.TopoServer().GetTablet(ctx, masterAlias)
	if err != nil {
		return false, fmt.Errorf("failed to get record for master tablet %v: %v", vts.Status.MasterAlias, err)
	}
	if masterTabletInfo.Type != topodatapb.TabletType_MASTER {
		// The shard record says this is the master, but the tablet doesn't agree.
		// We don't know how to recover this automatically.
		return false, fmt.Errorf("shard record has tablet %v as the master, but the tablet is not of type master", vts.Status.MasterAlias)
	}

	// Check if the master is read-only. This could happen if it restarted and
	// is waiting for confirmation that it's still the master.
	masterReadOnly, err := isTabletReadOnly(ctx, wr.TabletManagerClient(), masterTabletInfo.Tablet)
	if err != nil {
		return false, fmt.Errorf("failed to execute query against master tablet %v: %v", vts.Status.MasterAlias, err)
	}
	if masterReadOnly {
		return true, nil
	}

	// List all tablets for the shard. A partial result is ok. We might miss
	// some tablets, but it's still better to check the tablets we can see.
	// The missing tablets might be in a cell whose local topo is unavailable.
	// We'll try again to contact them on the next poll.
	keyspaceName := vts.Labels[planetscalev2.KeyspaceLabel]
	tablets, err := wr.TopoServer().GetTabletMapForShard(ctx, keyspaceName, vts.Spec.Name)
	if err != nil && !topo.IsErrType(err, topo.PartialResult) {
		return false, fmt.Errorf("failed to get tablet map for shard: %v", err)
	}
	checkCtx, cancelCheck := context.WithCancel(ctx)
	defer cancelCheck()
	results := make(chan bool, len(tablets))
	for tabletAlias, tabletInfo := range tablets {
		go func(tabletAlias string, tabletInfo *topo.TabletInfo) {
			results <- canRepairTablet(checkCtx, tabletAlias, tabletInfo, masterTabletInfo, wr)
		}(tabletAlias, tabletInfo)
	}
	// No one ever closes the results chan, but we know how many to expect.
	var foundFixableTablet bool
	for range tablets {
		canFix := <-results
		if canFix && !foundFixableTablet {
			// If any tablet has a fixable problem, we short-circuit and return true.
			// Cancel the context to tell all the other goroutines to give up.
			// We'll keep looping to wait for them, so we know they've all stopped
			// using the Wrangler before we return.
			foundFixableTablet = true
			cancelCheck()
		}
	}
	return foundFixableTablet, nil
}

func canRepairTablet(ctx context.Context, tabletAlias string, tabletInfo, masterTabletInfo *topo.TabletInfo, wr *wrangler.Wrangler) bool {
	if !shouldCheckTablet(tabletInfo, masterTabletInfo) {
		return false
	}
	// Get the replication status of the tablet.
	status, err := wr.TabletManagerClient().SlaveStatus(ctx, tabletInfo.Tablet)
	if err != nil {
		// We don't know how to fix this.
		return false
	}
	// Check if the master address needs to be fixed.
	if status.MasterHost != topoproto.MysqlHostname(masterTabletInfo.Tablet) ||
		status.MasterPort != topoproto.MysqlPort(masterTabletInfo.Tablet) {
		return true
	}
	// We didn't find any problems we know how to fix.
	return false
}

func (r *ReconcileVitessShard) repairReplicationLocked(ctx context.Context, vts *planetscalev2.VitessShard, wr *wrangler.Wrangler) error {
	keyspaceName := vts.Labels[planetscalev2.KeyspaceLabel]
	shardName := vts.Spec.Name

	// Re-read the shard record and tablet records now that we have the lock,
	// and look for any problems that we know how to fix.
	shardInfo, err := wr.TopoServer().GetShard(ctx, keyspaceName, shardName)
	if err != nil {
		return err
	}
	if topoproto.TabletAliasIsZero(shardInfo.MasterAlias) {
		return fmt.Errorf("shard has no master")
	}
	masterAlias := topoproto.TabletAliasString(shardInfo.MasterAlias)

	// Get the master tablet record.
	masterTabletInfo, err := wr.TopoServer().GetTablet(ctx, shardInfo.MasterAlias)
	if err != nil {
		return fmt.Errorf("failed to get tablet record for master %v", masterAlias)
	}
	if masterTabletInfo.Type != topodatapb.TabletType_MASTER {
		// The shard record says this is the master, but the tablet doesn't agree.
		// We don't know how to recover this automatically.
		return fmt.Errorf("shard record has tablet %v as the master, but the tablet is not of type master", masterAlias)
	}

	// Check if the master's mysqld is read-only. This could happen if it
	// restarted and is waiting for confirmation that it's still the master.
	masterReadOnly, err := isTabletReadOnly(ctx, wr.TabletManagerClient(), masterTabletInfo.Tablet)
	if err != nil {
		return fmt.Errorf("failed to execute query against master tablet %v: %v", masterAlias, err)
	}
	if masterReadOnly {
		if err := r.recoverRestartedMasterLocked(ctx, vts, wr, masterTabletInfo.Tablet, masterAlias); err != nil {
			return fmt.Errorf("failed to recover restarted master: %v", err)
		}
	}

	// Get all tablet records for the shard. A partial result is ok. We might miss
	// some tablets, but it's still better to check the tablets we can see.
	// The missing tablets might be in a cell whose local topo is unavailable.
	// We'll try again to contact them on the next poll.
	tablets, err := wr.TopoServer().GetTabletMapForShard(ctx, keyspaceName, shardName)
	if err != nil && !topo.IsErrType(err, topo.PartialResult) {
		return fmt.Errorf("failed to get tablet map for shard: %v", err)
	}

	// Try to fix any replica/rdonly tablets that have the wrong master address.
	wg := &sync.WaitGroup{}
	for tabletAlias, tablet := range tablets {
		if !shouldCheckTablet(tablet, masterTabletInfo) {
			continue
		}

		wg.Add(1)
		go func(tabletAlias string, tablet *topodatapb.Tablet) {
			defer wg.Done()

			// Check replication status of the tablet. If we can't fetch status,
			// or if replication is not configured at all, we rely on the tablet
			// itself to fix that because we don't know whether it's ready.
			status, err := wr.TabletManagerClient().SlaveStatus(ctx, tablet)
			if err != nil {
				return
			}
			if status.MasterHost == topoproto.MysqlHostname(masterTabletInfo.Tablet) &&
				status.MasterPort == topoproto.MysqlPort(masterTabletInfo.Tablet) {
				// The master address is already correct.
				return
			}
			// Try to fix the master address.
			// Only force slave start on replicas, not rdonly.
			// A rdonly might be stopped on purpose for a diff.
			forceSlaveStart := tablet.Type == topodatapb.TabletType_REPLICA
			err = wr.TabletManagerClient().SetMaster(ctx, tablet, masterTabletInfo.Alias, 0 /* don't try to wait for a reparent journal entry */, "" /* don't wait for any position */, forceSlaveStart)
			reparentTabletCount.WithLabelValues(metricLabels(vts, err)...).Inc()
			if err != nil {
				// Just log the error instead of failing the process, because fixing replicas is best-effort.
				log.Warningf("failed to reparent tablet %v to master %v: %v", tabletAlias, masterAlias, err)
			}
			// Note this is still a Warning. A tablet with the wrong master address is not Normal.
			r.recorder.Eventf(vts, corev1.EventTypeWarning, "ReparentTablet", "reparented tablet %v to current master %v", tabletAlias, masterAlias)
		}(tabletAlias, tablet.Tablet)
	}
	wg.Wait()

	return nil
}

func (r *ReconcileVitessShard) recoverRestartedMasterLocked(ctx context.Context, vts *planetscalev2.VitessShard, wr *wrangler.Wrangler, masterTablet *topodatapb.Tablet, masterAlias string) error {
	keyspaceName := vts.Labels[planetscalev2.KeyspaceLabel]
	shardName := vts.Spec.Name

	// Get all tablets. When recovering a restarted master, we currently require
	// that all tablets are visible, so we don't allow partial results.
	tablets, err := wr.TopoServer().GetTabletMapForShard(ctx, keyspaceName, shardName)
	if err != nil {
		return fmt.Errorf("failed to get tablet map for shard: %v", err)
	}

	// Check that this is the only potential master.
	// We already checked that the global shard record and the master
	// tablet record agree that this tablet is the master, and we currently
	// hold the shard lock so no one else is allowed to change the master.
	// Make sure none of the other tablets we can see claim to be master.
	for tabletAlias, tabletInfo := range tablets {
		if tabletInfo.Type == topodatapb.TabletType_MASTER && tabletAlias != masterAlias {
			// Another tablet also claims to be master. We don't know how to
			// repair this automatically.
			return fmt.Errorf("tablet %v also claims to be master", tabletAlias)
		}
	}

	// Check that no other replicas are ahead of this master.
	posStr, err := wr.TabletManagerClient().MasterPosition(ctx, masterTablet)
	if err != nil {
		return fmt.Errorf("can't get master position: %v", err)
	}
	masterPos, err := mysql.DecodePosition(posStr)
	if err != nil {
		return err
	}
	if err := checkReplicaPositions(ctx, wr.TabletManagerClient(), tablets, masterPos); err != nil {
		return err
	}

	// Recheck that we still have the distributed lock.
	if err := topo.CheckShardLocked(ctx, keyspaceName, shardName); err != nil {
		return fmt.Errorf("lost topology lock, aborting: %v", err)
	}
	// Set the master read-write.
	err = wr.TabletManagerClient().SetReadWrite(ctx, masterTablet)
	recoverRestartedMasterCount.WithLabelValues(metricLabels(vts, err)...).Inc()
	if err != nil {
		return fmt.Errorf("failed to set master read-write: %v", err)
	}
	// Note this is still a Warning. A tablet that restarted while it was still master is not Normal.
	r.recorder.Eventf(vts, corev1.EventTypeWarning, "RecoverMaster", "recovered restarted master tablet %v", masterAlias)
	return nil
}

const isReadOnlyQuery = "SHOW VARIABLES LIKE 'read_only'"

func isTabletReadOnly(ctx context.Context, tmc tmclient.TabletManagerClient, tablet *topodatapb.Tablet) (bool, error) {
	// Check the read_only variable.
	// Note that even if the tablet sets super_read_only to ON,
	// that will also set read_only to ON.
	qrproto, err := tmc.ExecuteFetchAsDba(ctx, tablet, true /*usePool*/, []byte(isReadOnlyQuery), 1 /*maxRows*/, false /*disableBinlogs*/, false /*reloadSchema*/)
	if err != nil {
		return false, err
	}
	qr := sqltypes.Proto3ToResult(qrproto)
	if len(qr.Rows) != 1 {
		return false, errors.New("no read_only variable in mysql")
	}
	return qr.Rows[0][1].ToString() == "ON", nil
}

func tabletTypeRepairable(tabletType topodatapb.TabletType) bool {
	// We only try to repair REPLICA and RDONLY tablets for now.
	// TODO(enisoc): Does it make sense to try to reparent any other tablet types?
	switch tabletType {
	case topodatapb.TabletType_REPLICA, topodatapb.TabletType_RDONLY:
		return true
	}
	return false
}

func shouldCheckTablet(tabletInfo, masterTabletInfo *topo.TabletInfo) bool {
	// We only try to repair certain types of tablets.
	if !tabletTypeRepairable(tabletInfo.GetType()) {
		return false
	}
	// We can't repair a replica tablet if it's still listed as the shard master.
	// We would end up trying to reparent the tablet to itself.
	// If the shard master is updated later, we'll try again then.
	if topoproto.TabletAliasEqual(tabletInfo.GetAlias(), masterTabletInfo.GetAlias()) {
		return false
	}
	return true
}

// checkReplicaPositions returns success only if all replicas are equal to or
// behind the given master position.
func checkReplicaPositions(ctx context.Context, tmc tmclient.TabletManagerClient, tablets map[string]*topo.TabletInfo, masterPos mysql.Position) error {
	checkCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	resultChan := make(chan error)
	expectedResultCount := 0
	for tabletAlias, tablet := range tablets {
		// TODO(enisoc): Does it make sense to try to reparent any other tablet types?
		if tablet.Type != topodatapb.TabletType_REPLICA && tablet.Type != topodatapb.TabletType_RDONLY {
			continue
		}

		expectedResultCount++
		go func(tabletAlias string, tablet *topodatapb.Tablet) (err error) {
			defer func() {
				resultChan <- err
			}()

			// We use the poorly-named MasterPosition RPC to get the current
			// position independent of whether replication is configured.
			replicaPosStr, err := tmc.MasterPosition(checkCtx, tablet)
			if err != nil {
				return fmt.Errorf("can't check replica position: %v", err)
			}
			replicaPos, err := mysql.DecodePosition(replicaPosStr)
			if err != nil {
				return fmt.Errorf("can't decode replica position: %v", err)
			}
			// Check that the master is equal to or ahead of this replica.
			if !masterPos.AtLeast(replicaPos) {
				return fmt.Errorf("replica %v position (%v) is ahead of master position (%v)", tabletAlias, replicaPos, masterPos)
			}

			return nil
		}(tabletAlias, tablet.Tablet)
	}

	var firstErr error
	for i := 0; i < expectedResultCount; i++ {
		err := <-resultChan
		if err != nil && firstErr == nil {
			// Remember the first error and tell everyone else nevermind.
			// Continue draining the channel so we know they've all quit.
			firstErr = err
			cancel()
		}
	}
	return firstErr
}
