/*
Copyright 2019 PlanetScale Inc.

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
	"vitess.io/vitess/go/netutil"

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

For example, one scenario this handles is if the replica has the wrong primary
address, which could happen if it missed the message the last time the primary
changed. This periodic repair ensures such missed messages are eventually
corrected. This also handles when a primary restarts and comes up read-only,
which is a safety mechanism to wait for confirmation that it's still the primary,
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
		primary going down) requires a "global" component that polls all tablets.
		Vitess does not yet have such a component, so for now we do this in the
		Kubernetes operator.
	*/

	keyspaceName := vts.Labels[planetscalev2.KeyspaceLabel]
	resultBuilder := &results.Builder{}

	// If using external datastore then replication is handled for us.
	if vts.Spec.UsingExternalDatastore() {
		return resultBuilder.Result()
	}

	// If we have configured the operator to ignore replication repair, bail.
	if !*vts.Spec.Replication.RecoverRestartedMaster {
		return resultBuilder.Result()
	}

	// If the current primary is in a cell that this VitessCluster doesn't
	// manage, skip replication repair and assume another instance will do it.
	if vts.Status.HasMaster != corev1.ConditionTrue {
		return resultBuilder.Result()
	}
	primaryAlias, err := topoproto.ParseTabletAlias(vts.Status.MasterAlias)
	if err != nil {
		return resultBuilder.Result()
	}
	if !vts.Spec.CellInCluster(primaryAlias.GetCell()) {
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
	// Check who the primary is according to the shard record.
	if vts.Status.MasterAlias == "" {
		// The shard has no primary, or we weren't able to read topo.
		// Either way, there's nothing we can do.
		return false, fmt.Errorf("no primary for shard")
	}
	primaryAlias, err := topoproto.ParseTabletAlias(vts.Status.MasterAlias)
	if err != nil {
		// This should never happen. There's no point retrying.
		return false, fmt.Errorf("invalid primary alias: %v", err)
	}
	primaryTabletInfo, err := wr.TopoServer().GetTablet(ctx, primaryAlias)
	if err != nil {
		return false, fmt.Errorf("failed to get record for primary tablet %v: %v", vts.Status.MasterAlias, err)
	}
	if primaryTabletInfo.Type != topodatapb.TabletType_PRIMARY {
		// The shard record says this is the primary, but the tablet doesn't agree.
		// We don't know how to recover this automatically.
		return false, fmt.Errorf("shard record has tablet %v as the primary, but the tablet is not of type primary", vts.Status.MasterAlias)
	}

	// Check if the primary is read-only. This could happen if it restarted and
	// is waiting for confirmation that it's still the primary.
	primaryReadOnly, err := isTabletReadOnly(ctx, wr.TabletManagerClient(), primaryTabletInfo.Tablet)
	if err != nil {
		return false, fmt.Errorf("failed to execute query against primary tablet %v: %v", vts.Status.MasterAlias, err)
	}
	if primaryReadOnly {
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
			results <- canRepairTablet(checkCtx, tabletAlias, tabletInfo, primaryTabletInfo, wr)
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

func canRepairTablet(ctx context.Context, tabletAlias string, tabletInfo, primaryTabletInfo *topo.TabletInfo, wr *wrangler.Wrangler) bool {
	if !shouldCheckTablet(tabletInfo, primaryTabletInfo) {
		return false
	}
	// Get the replication status of the tablet.
	status, err := wr.TabletManagerClient().ReplicationStatus(ctx, tabletInfo.Tablet)
	if err != nil {
		// We don't know how to fix this.
		return false
	}
	// Check if the primary address needs to be fixed.
	if netutil.JoinHostPort(status.SourceHost, status.SourcePort) != topoproto.MysqlAddr(primaryTabletInfo.Tablet) {
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
	if topoproto.TabletAliasIsZero(shardInfo.PrimaryAlias) {
		return fmt.Errorf("shard has no primary")
	}
	primaryAlias := topoproto.TabletAliasString(shardInfo.PrimaryAlias)

	// Get the primary tablet record.
	primaryTabletInfo, err := wr.TopoServer().GetTablet(ctx, shardInfo.PrimaryAlias)
	if err != nil {
		return fmt.Errorf("failed to get tablet record for primary %v", primaryAlias)
	}
	if primaryTabletInfo.Type != topodatapb.TabletType_PRIMARY {
		// The shard record says this is the primary, but the tablet doesn't agree.
		// We don't know how to recover this automatically.
		return fmt.Errorf("shard record has tablet %v as the primary, but the tablet is not of type primary", primaryAlias)
	}

	// Check if the primary's mysqld is read-only. This could happen if it
	// restarted and is waiting for confirmation that it's still the primary.
	primaryReadOnly, err := isTabletReadOnly(ctx, wr.TabletManagerClient(), primaryTabletInfo.Tablet)
	if err != nil {
		return fmt.Errorf("failed to execute query against primary tablet %v: %v", primaryAlias, err)
	}
	if primaryReadOnly {
		if err := r.recoverRestartedPrimaryLocked(ctx, vts, wr, primaryTabletInfo.Tablet, primaryAlias); err != nil {
			return fmt.Errorf("failed to recover restarted primary: %v", err)
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

	// Try to fix any replica/rdonly tablets that have the wrong primary address.
	wg := &sync.WaitGroup{}
	for tabletAlias, tablet := range tablets {
		if !shouldCheckTablet(tablet, primaryTabletInfo) {
			continue
		}

		wg.Add(1)
		go func(tabletAlias string, tablet *topodatapb.Tablet) {
			defer wg.Done()

			// Check replication status of the tablet. If we can't fetch status,
			// or if replication is not configured at all, we rely on the tablet
			// itself to fix that because we don't know whether it's ready.
			status, err := wr.TabletManagerClient().ReplicationStatus(ctx, tablet)
			if err != nil {
				return
			}
			if netutil.JoinHostPort(status.SourceHost, status.SourcePort) == topoproto.MysqlAddr(primaryTabletInfo.Tablet) {
				// The primary address is already correct.
				return
			}
			// Try to fix the primary address.
			// Only force start replication on replicas, not rdonly.
			// A rdonly might be stopped on purpose for a diff.
			forceStartReplication := tablet.Type == topodatapb.TabletType_REPLICA
			err = wr.TabletManagerClient().SetReplicationSource(ctx, tablet, primaryTabletInfo.Alias, 0 /* don't try to wait for a reparent journal entry */, "" /* don't wait for any position */, forceStartReplication)
			reparentTabletCount.WithLabelValues(metricLabels(vts, err)...).Inc()
			if err != nil {
				// Just log the error instead of failing the process, because fixing replicas is best-effort.
				log.Warningf("failed to reparent tablet %v to primary %v: %v", tabletAlias, primaryAlias, err)
			}
			// Note this is still a Warning. A tablet with the wrong primary address is not Normal.
			r.recorder.Eventf(vts, corev1.EventTypeWarning, "ReparentTablet", "reparented tablet %v to current primary %v", tabletAlias, primaryAlias)
		}(tabletAlias, tablet.Tablet)
	}
	wg.Wait()

	return nil
}

func (r *ReconcileVitessShard) recoverRestartedPrimaryLocked(ctx context.Context, vts *planetscalev2.VitessShard, wr *wrangler.Wrangler, primaryTablet *topodatapb.Tablet, primaryAlias string) error {
	keyspaceName := vts.Labels[planetscalev2.KeyspaceLabel]
	shardName := vts.Spec.Name

	// Get all tablets. When recovering a restarted primary, we currently require
	// that all tablets are visible, so we don't allow partial results.
	tablets, err := wr.TopoServer().GetTabletMapForShard(ctx, keyspaceName, shardName)
	if err != nil {
		return fmt.Errorf("failed to get tablet map for shard: %v", err)
	}

	// Check that this is the only potential primary.
	// We already checked that the global shard record and the primary
	// tablet record agree that this tablet is the primary, and we currently
	// hold the shard lock so no one else is allowed to change the primary.
	// Make sure none of the other tablets we can see claim to be primary.
	for tabletAlias, tabletInfo := range tablets {
		if tabletInfo.Type == topodatapb.TabletType_PRIMARY && tabletAlias != primaryAlias {
			// Another tablet also claims to be primary. We don't know how to
			// repair this automatically.
			return fmt.Errorf("tablet %v also claims to be primary", tabletAlias)
		}
	}

	// Check that no other replicas are ahead of this primary.
	posStr, err := wr.TabletManagerClient().PrimaryPosition(ctx, primaryTablet)
	if err != nil {
		return fmt.Errorf("can't get primary position: %v", err)
	}
	primaryPos, err := mysql.DecodePosition(posStr)
	if err != nil {
		return err
	}
	if err := checkReplicaPositions(ctx, wr.TabletManagerClient(), tablets, primaryPos); err != nil {
		return err
	}

	// Recheck that we still have the distributed lock.
	if err := topo.CheckShardLocked(ctx, keyspaceName, shardName); err != nil {
		return fmt.Errorf("lost topology lock, aborting: %v", err)
	}
	// Set the primary read-write.
	err = wr.TabletManagerClient().SetReadWrite(ctx, primaryTablet)
	recoverRestartedMasterCount.WithLabelValues(metricLabels(vts, err)...).Inc()
	if err != nil {
		return fmt.Errorf("failed to set primary read-write: %v", err)
	}
	// Note this is still a Warning. A tablet that restarted while it was still primary is not Normal.
	r.recorder.Eventf(vts, corev1.EventTypeWarning, "RecoverPrimary", "recovered restarted primary tablet %v", primaryAlias)
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

func shouldCheckTablet(tabletInfo, primaryTabletInfo *topo.TabletInfo) bool {
	// We only try to repair certain types of tablets.
	if !tabletTypeRepairable(tabletInfo.GetType()) {
		return false
	}
	// We can't repair a replica tablet if it's still listed as the shard primary.
	// We would end up trying to reparent the tablet to itself.
	// If the shard primary is updated later, we'll try again then.
	if topoproto.TabletAliasEqual(tabletInfo.GetAlias(), primaryTabletInfo.GetAlias()) {
		return false
	}
	return true
}

// checkReplicaPositions returns success only if all replicas are equal to or
// behind the given primary position.
func checkReplicaPositions(ctx context.Context, tmc tmclient.TabletManagerClient, tablets map[string]*topo.TabletInfo, primaryPos mysql.Position) error {
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

			// We use the poorly-named PrimaryPosition RPC to get the current
			// position independent of whether replication is configured.
			replicaPosStr, err := tmc.PrimaryPosition(checkCtx, tablet)
			if err != nil {
				return fmt.Errorf("can't check replica position: %v", err)
			}
			replicaPos, err := mysql.DecodePosition(replicaPosStr)
			if err != nil {
				return fmt.Errorf("can't decode replica position: %v", err)
			}
			// Check that the primary is equal to or ahead of this replica.
			if !primaryPos.AtLeast(replicaPos) {
				return fmt.Errorf("replica %v position (%v) is ahead of primary position (%v)", tabletAlias, replicaPos, primaryPos)
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
