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
	"fmt"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"vitess.io/vitess/go/mysql"
	topodatapb "vitess.io/vitess/go/vt/proto/topodata"
	"vitess.io/vitess/go/vt/topo"
	"vitess.io/vitess/go/vt/topo/topoproto"
	"vitess.io/vitess/go/vt/wrangler"

	// register grpc tabletmanager client
	_ "vitess.io/vitess/go/vt/vttablet/grpctmclient"
	"vitess.io/vitess/go/vt/vttablet/tmclient"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/results"
)

const (
	initRestoredShardTimeout = 15 * time.Second
	tabletStatusCheckTimeout = 5 * time.Second
)

/*
initRestoredShard starts replication on a shard that's just been restored
from a cold backup. That is, all the replicas were created just now and all
restored from the same backup. We need to elect an initial primary and start
replication.

When a shard is first bootstrapped, if backups are enabled, we will use vtbackup
to seed an initial backup, which makes that bootstrap process just a special
case of handling a shard that's restored from backup.
*/
func (r *ReconcileVitessShard) initRestoredShard(ctx context.Context, vts *planetscalev2.VitessShard, wr *wrangler.Wrangler) (reconcile.Result, error) {
	keyspaceName := vts.Labels[planetscalev2.KeyspaceLabel]
	shardName := vts.Spec.Name
	resultBuilder := &results.Builder{}

	// If backups are disabled, the shard can't have been restored from backup.
	if !vts.Spec.BackupsEnabled() || vts.Spec.UsingExternalDatastore() {
		return resultBuilder.Result()
	}

	// Check if the shard has a primary.
	switch vts.Status.HasMaster {
	case corev1.ConditionTrue:
		// The shard already has a primary. Nothing to do.
		return resultBuilder.Result()
	case corev1.ConditionUnknown:
		// We don't know the topo status, so it's not safe to try. Check again later.
		return resultBuilder.RequeueAfter(replicationRequeueDelay)
	}

	// Wait until the initial backup has been seeded. This will also be true if
	// we ran the initial vtbackup job and it found that a backup already
	// existed (the cold restore case), since vtbackup's "initial backup" mode
	// is idempotent.
	if vts.Status.HasInitialBackup != corev1.ConditionTrue {
		r.recorder.Eventf(vts, corev1.EventTypeNormal, "InitShardWaiting", "can't initialize shard: waiting for initial backup to complete")
		return resultBuilder.Result()
	}

	// Now wait for at least one primary-eligible tablet to finish restoring.
	// For now, we just elect whoever finishes restoring first as the primary.
	foundCandidatePrimary := false
	for _, tablet := range vts.Status.Tablets {
		// Check if the Pod is Running.
		// Note that we don't expect it to be Ready because they can't be
		// healthy before the shard has been initialized anyway.
		if tablet.Running != corev1.ConditionTrue {
			continue
		}
		// Check if the tablet has type "replica", meaning it's primary-eligible
		// and is not in the middle of a restore, or type "primary", meaning we
		// already promoted a primary, but failed to update the shard record.
		if tablet.Type == "replica" || tablet.Type == "master" {
			foundCandidatePrimary = true
			break
		}
	}
	if !foundCandidatePrimary {
		// Requeue to check if any tablets are done restoring yet.
		r.recorder.Eventf(vts, corev1.EventTypeNormal, "InitShardWaiting", "can't initialize shard: no primary-eligible replica tablet is ready to become primary")
		return resultBuilder.RequeueAfter(replicationRequeueDelay)
	}

	// Now we start talking to topo and directly to tablets.
	// Don't hold our slot in the reconcile work queue for too long.
	ctx, cancel := context.WithTimeout(ctx, initRestoredShardTimeout)
	defer cancel()

	// If we get here, there should in theory be at least one candidate primary
	// that's done restoring, but we might have just caught it claiming to be a
	// replica before it started the restore process. We'll check for sure while
	// holding the shard lock, so just go ahead and try the election.
	if primaryAlias, err := electInitialShardPrimary(ctx, keyspaceName, shardName, wr); err != nil {
		r.recorder.Eventf(vts, corev1.EventTypeWarning, "InitShardFailed", "failed to initialize shard: %v", err)
		resultBuilder.RequeueAfter(replicationRequeueDelay)
	} else {
		r.recorder.Eventf(vts, corev1.EventTypeNormal, "InitShardPrimary", "initialized shard replication with primary tablet %v", topoproto.TabletAliasString(primaryAlias))
	}

	return resultBuilder.Result()
}

// electInitialShardPrimary picks a replica in the shard and promotes it to
// primary, without trying to initialize the database. It assumes all replicas
// already have synchronized replication positions and an initialized database
// because they all restored from the same backup.
func electInitialShardPrimary(ctx context.Context, keyspaceName, shardName string, wr *wrangler.Wrangler) (primaryAlias *topodatapb.TabletAlias, finalErr error) {
	// Lock the shard to avoid running concurrently with other replication commands.
	ctx, unlock, lockErr := wr.TopoServer().LockShard(ctx, keyspaceName, shardName, "electShardPrimary")
	if lockErr != nil {
		return nil, lockErr
	}
	defer unlock(&finalErr)

	// Now that we have the lock, verify the state is as we expect.
	// There should be no shard primary.
	shard, err := wr.TopoServer().GetShard(ctx, keyspaceName, shardName)
	if err != nil {
		return nil, err
	}
	if !topoproto.TabletAliasIsZero(shard.PrimaryAlias) {
		return nil, fmt.Errorf("can't elect primary: shard already has a primary: %v", topoproto.TabletAliasString(shard.PrimaryAlias))
	}

	// Check if any tablet has already been promoted to primary.
	tablets, err := wr.TopoServer().GetTabletMapForShard(ctx, keyspaceName, shardName)
	if err != nil {
		return nil, fmt.Errorf("can't get tablets for shard: %v", err)
	}
	var existingPrimary *topo.TabletInfo
	for tabletName, tablet := range tablets {
		if tablet.GetType() == topodatapb.TabletType_PRIMARY {
			if existingPrimary != nil {
				// We found more than one existing primary. That shouldn't happen.
				return nil, fmt.Errorf("can't elect primary: shard has multiple tablets that claim to be primary: %v, %v", tabletName, existingPrimary.AliasString())
			}
			existingPrimary = tablet
		}
	}
	if existingPrimary != nil {
		// Check we still have the topology lock.
		if err := topo.CheckShardLocked(ctx, keyspaceName, shardName); err != nil {
			return nil, fmt.Errorf("lost topology lock; aborting: %v", err)
		}
		// A tablet has already been promoted to primary, but the shard record is
		// stale. Make the shard record consistent.
		_, err := wr.TopoServer().UpdateShardFields(ctx, keyspaceName, shardName, func(shard *topo.ShardInfo) error {
			shard.PrimaryAlias = existingPrimary.Alias
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("failed to fix shard record for already-promoted primary %v: %v", existingPrimary.AliasString(), err)
		}
		return existingPrimary.Alias, nil
	}

	// Check status of all tablets.
	statusChan := make(chan *tabletStatus, len(tablets))
	statusCheckCtx, statusCheckCancel := context.WithTimeout(ctx, tabletStatusCheckTimeout)
	defer statusCheckCancel()
	for tabletName, tablet := range tablets {
		go func(tabletName string, tablet *topo.TabletInfo) {
			statusChan <- getTabletStatus(statusCheckCtx, wr.TabletManagerClient(), tabletName, tablet)
		}(tabletName, tablet)
	}

	// There should be at least one primary-eligible replica that's done
	// restoring. For now, just pick the first one we find.
	// TODO(enisoc): Allow configuration of which cell(s) to prefer to put primarys in.
	var candidatePrimary *tabletStatus
	restoredReplicas := []*tabletStatus{}

	// No one ever closes the statusChan, but we know how many to expect.
	for range tablets {
		status := <-statusChan
		if status.err != nil {
			// We weren't able to check this tablet. Ignore the status values.
			continue
		}
		if !status.databaseExists {
			// Ignore tablets that are not done restoring.
			continue
		}
		switch status.tablet.GetType() {
		case topodatapb.TabletType_REPLICA:
			restoredReplicas = append(restoredReplicas, status)

			// Set this as the candidate primary, if we haven't found one yet,
			// or if this one is farther ahead.
			if candidatePrimary == nil || !candidatePrimary.replicationPosition.AtLeast(status.replicationPosition) {
				candidatePrimary = status
			}
		case topodatapb.TabletType_RDONLY:
			restoredReplicas = append(restoredReplicas, status)
		}
	}

	if candidatePrimary == nil {
		return nil, fmt.Errorf("can't elect primary: didn't find any valid candidate")
	}

	// Check we still have the topology lock.
	if err := topo.CheckShardLocked(ctx, keyspaceName, shardName); err != nil {
		return nil, fmt.Errorf("lost topology lock; aborting: %v", err)
	}
	// Promote the candidate to primary.
	_, err = wr.TabletManagerClient().PromoteReplica(ctx, candidatePrimary.tablet.Tablet)
	if err != nil {
		return nil, fmt.Errorf("failed to promote tablet %v to primary: %v", candidatePrimary.tablet.AliasString(), err)
	}
	// Update the shard record.
	_, err = wr.TopoServer().UpdateShardFields(ctx, keyspaceName, shardName, func(shard *topo.ShardInfo) error {
		shard.PrimaryAlias = candidatePrimary.tablet.Alias
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update shard record: %v", err)
	}

	// Try to reparent other replicas that are done restoring. The rest will see
	// the primary in the shard record and configure replication automatically.
	// Even for the replicas we do try, this is best-effort. If it fails, we'll
	// try again later in the usual replication repair path.
	wg := &sync.WaitGroup{}
	for _, replicaStatus := range restoredReplicas {
		if topoproto.TabletAliasEqual(replicaStatus.tablet.Alias, candidatePrimary.tablet.Alias) {
			// Skip the one we promoted to primary.
			continue
		}
		wg.Add(1)
		go func(tablet *topo.TabletInfo) {
			defer wg.Done()
			err := wr.TabletManagerClient().SetReplicationSource(ctx, tablet.Tablet, candidatePrimary.tablet.Alias, 0 /* don't try to wait for a reparent journal entry */, "" /* don't wait for any position */, true /* forceStartReplication */)
			if err != nil {
				log.Warningf("best-effort configuration of replication for tablet %v failed: %v", tablet.AliasString(), err)
			}
		}(replicaStatus.tablet)
	}
	wg.Wait()

	return candidatePrimary.tablet.Alias, nil
}

type tabletStatus struct {
	replicationConfigured bool
	replicationPosition   mysql.Position
	databaseExists        bool
	tablet                *topo.TabletInfo
	err                   error
}

func getTabletStatus(ctx context.Context, tmc tmclient.TabletManagerClient, tabletName string, tablet *topo.TabletInfo) *tabletStatus {
	status := &tabletStatus{
		tablet: tablet,
	}

	// Get the replication status for each tablet.
	_, err := tmc.ReplicationStatus(ctx, tablet.Tablet)
	if err == nil {
		// We got a real replication status, which means the tablet was already replicating at some point.
		status.replicationConfigured = true
	} else if !isErrNotReplica(err) {
		// We expect the error ErrNotReplica, which means "SHOW SLAVE STATUS" returned
		// zero rows (replication is not configured at all).
		// If ReplicationStatus() failed for the wrong reason, we don't know
		// whether replication is configured.
		status.err = fmt.Errorf("couldn't determine whether tablet %v has replication configured: %v", tabletName, err)
		return status
	}

	// Get the current position of each tablet.
	positionStr, err := tmc.PrimaryPosition(ctx, tablet.Tablet)
	if err != nil {
		status.err = fmt.Errorf("couldn't get replicaiton position for tablet %v: %v", tabletName, err)
		return status
	}
	status.replicationPosition, err = mysql.DecodePosition(positionStr)
	if err != nil {
		status.err = fmt.Errorf("couldn't get replicaiton position for tablet %v: %v", tabletName, err)
		return status
	}

	// Check if the main keyspace database exists.
	status.databaseExists, err = tabletDatabaseExists(ctx, tmc, tablet.Tablet)
	if err != nil {
		status.err = fmt.Errorf("couldn't determine whether tablet %v database exists: %v", tabletName, err)
		return status
	}

	return status
}

// isErrNotReplica returns true if the given error is recognized as one of the
// errors Vitess sends to indicate that it successfully checked replication
// status, but the answer is that replication is in a disabled state.
// Replication could be in a disabled state either because the tablet is a
// replica that has not been configured yet, or because it's acting as a primary.
//
// Unfortunately, the Vitess RPC to fetch replication status does not offer any
// officially-supported way to distinguish between this case (replication disabled)
// and other errors (e.g. MySQL didn't respond). The best we can do for now is
// inspect the text of the error.
//
// In July 2020, Vitess changed the text of the error we're looking for.
// To remain compatible with older versions, we check for these historical
// values as well as the latest known value pulled from our build dependency.
//
// TODO: Add an officially-supported signal in the Vitess RPC to recognize this
//       important state programmatically.
func isErrNotReplica(err error) bool {
	errString := err.Error()

	// Check for the latest known value from the version of Vitess we import.
	if strings.Contains(errString, mysql.ErrNotReplica.Error()) {
		return true
	}

	// Check for historically known values since Vitess does not treat this
	// error string as part of its public API and hence the value may change.
	for _, search := range errNotReplicaStrings {
		if strings.Contains(errString, search) {
			return true
		}
	}

	return false
}

// errNotReplicaStrings are historically known error strings that indicate
// a tablet has replication disabled.
var errNotReplicaStrings = []string{
	"no slave status",
	"no replication status",
}
