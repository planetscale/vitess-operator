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
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"vitess.io/vitess/go/mysql"
	"vitess.io/vitess/go/sqltypes"
	topodatapb "vitess.io/vitess/go/vt/proto/topodata"
	"vitess.io/vitess/go/vt/topo"
	"vitess.io/vitess/go/vt/topo/topoproto"
	"vitess.io/vitess/go/vt/vttablet/tmclient"
	"vitess.io/vitess/go/vt/wrangler"

	// register grpc tabletmanager client
	_ "vitess.io/vitess/go/vt/vttablet/grpctmclient"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/results"
)

const (
	initShardMasterTimeout = 15 * time.Second
)

func (r *ReconcileVitessShard) initShardMaster(ctx context.Context, vts *planetscalev2.VitessShard, wr *wrangler.Wrangler) (reconcile.Result, error) {
	// TODO(enisoc): Upstream changes to make an idempotent InitShardMaster and use that here instead.

	keyspaceName := vts.Labels[planetscalev2.KeyspaceLabel]
	resultBuilder := &results.Builder{}

	// If backups are enabled for the shard, don't use the real InitShardMaster.
	// Instead, we rely on the initial backup created with vtbackup, and treat
	// this like a shard that's been restored from a cold backup
	// (see initRestoredShard).
	//
	// If we're using external mysql then we should not ever call initShardMaster,
	// but should instead try to use tabletExternallyReparented.
	if vts.UsingExternalDatastore() || vts.Spec.BackupsEnabled() {
		return resultBuilder.Result()
	}

	ready := r.readyForMaster(vts, resultBuilder)
	if !ready {
		return resultBuilder.Result()
	}

	// Everything we can check through k8s and topology looks good to proceed.
	// Now we start talking to the actual vttablet processes.
	// Don't hold our slot in the reconcile work queue for too long.
	ctx, cancel := context.WithTimeout(ctx, initShardMasterTimeout)
	defer cancel()

	// Check that all desired tablets are ready to initialize replication.
	var masterCandidate *topodatapb.TabletAlias
	errs := make(chan error, len(vts.Status.Tablets))
	for name, tablet := range vts.Status.Tablets {
		tabletAlias, err := topoproto.ParseTabletAlias(name)
		if err != nil {
			r.recorder.Eventf(vts, corev1.EventTypeWarning, "InternalError", "can't parse tablet alias %q: %v", name, err)
			// Return success since there's no point retrying.
			return resultBuilder.Result()
		}

		// Is this tablet eligible to be a master?
		if masterCandidate == nil && tablet.Type == "replica" {
			masterCandidate = tabletAlias
		}

		go func(name string, tabletAlias *topodatapb.TabletAlias) {
			errs <- readyForShardInit(ctx, wr.TopoServer(), wr.TabletManagerClient(), name, tabletAlias)
		}(name, tabletAlias)
	}
	// No one ever closes the errs chan, but we know how many to expect.
	var firstErr error
	for range vts.Status.Tablets {
		err := <-errs
		if err != nil && firstErr == nil {
			firstErr = err
			// We need all the tablets to be ready, so we bail out on the first error.
			// Cancel the context to tell all the other goroutines to give up.
			// We'll keep looping to wait for them, so we know they're all stopped
			// using the Wrangler before we return.
			cancel()
		}
	}
	if firstErr != nil {
		r.recorder.Eventf(vts, corev1.EventTypeWarning, "InitShardBlocked", "can't initialize shard: %v", firstErr)
		return resultBuilder.RequeueAfter(replicationRequeueDelay)
	}

	// Now we know all the tablets are ready to be initialized.
	// See if we have a candidate for master.
	// TODO(enisoc): Allow configuration of which cell(s) to prefer to put masters in.
	if masterCandidate == nil {
		// We didn't find any "replica" (master-eligible) tablets.
		// Return success because there's no point retrying this until someone adds the replicas.
		r.recorder.Eventf(vts, corev1.EventTypeWarning, "InitShardBlocked", "can't initialize shard: no master-eligible tablets (type 'replica') deployed")
		return resultBuilder.Result()
	}

	// All checks passed. Do InitShardMaster.
	if err := wr.InitShardMaster(ctx, keyspaceName, vts.Spec.Name, masterCandidate, true /* force */, initShardMasterTimeout); err != nil {
		r.recorder.Eventf(vts, corev1.EventTypeWarning, "InitShardFailed", "failed to initialize shard: %v", err)
		resultBuilder.RequeueAfter(replicationRequeueDelay)
	} else {
		r.recorder.Eventf(vts, corev1.EventTypeNormal, "InitShardMaster", "initialized shard replication with master tablet %v", topoproto.TabletAliasString(masterCandidate))
	}

	return resultBuilder.Result()
}

func (r *ReconcileVitessShard) readyForMaster(vts *planetscalev2.VitessShard, resultBuilder *results.Builder) bool {
	switch vts.Status.HasMaster {
	case corev1.ConditionTrue:
		// The shard already has a master. Nothing to do.
		return false
	case corev1.ConditionUnknown:
		// We don't know the topo status, so it's not safe to try. Check again later.
		resultBuilder.RequeueAfter(replicationRequeueDelay)
		return false
	}
	// The shard doesn't have a master yet.
	// Are all the desired tablets ready to be initialized?
	if len(vts.Status.Tablets) == 0 {
		// It's not populated yet, or there are no tablets anyway.
		return false
	}
	for name, tablet := range vts.Status.Tablets {
		// Check if the Pod is Running.
		// Note that we don't expect it to be Ready because they can't be
		// healthy before the shard has been initialized anyway.
		if tablet.Running != corev1.ConditionTrue {
			r.recorder.Eventf(vts, corev1.EventTypeNormal, "InitShardWaiting", "can't initialize shard: tablet %v is not running", name)
			// We don't need to poll to recheck this because k8s will tell us if any Pod status changes.
			return false
		}
		// Check if the tablet has registered in topology.
		if tablet.Type == "master" {
			// One of the tablets is already claiming to be master,
			// even though the shard record has no master listed.
			// This is a bad state, so it's not safe for us to try anything.
			r.recorder.Eventf(vts, corev1.EventTypeWarning, "InitShardBlocked", "can't initialize shard: tablet %v is already claiming to be master", name)
			return false
		}
		if tablet.Type == "" || tablet.Type == "unknown" {
			r.recorder.Eventf(vts, corev1.EventTypeNormal, "InitShardWaiting", "can't initialize shard: tablet %v not registered in topology", name)
			// This info comes from topology (not k8s), so we need to re-poll after some delay.
			resultBuilder.RequeueAfter(replicationRequeueDelay)
			return false
		}
		if tablet.Type == "restore" {
			// If any tablet is restoring from backup, we need to wait until
			// it's done so we can check the final MySQL state afterward.
			r.recorder.Eventf(vts, corev1.EventTypeNormal, "InitShardWaiting", "can't initialize shard: tablet %v is restoring", name)
			// The restore could take a while, so just let the periodic resync check later.
			return false
		}
	}

	return true
}

func readyForShardInit(ctx context.Context, ts *topo.Server, tmc tmclient.TabletManagerClient, name string, tabletAlias *topodatapb.TabletAlias) error {
	// Get the tablet record from topology.
	ti, err := ts.GetTablet(ctx, tabletAlias)
	if err != nil {
		return fmt.Errorf("failed to get topology record for tablet %v: %v", name, err)
	}

	// Get the slave status for each tablet.
	_, err = tmc.SlaveStatus(ctx, ti.Tablet)
	if err == nil {
		// We got a real slave status, which means the tablet was already replicating at some point.
		return fmt.Errorf("replication was previously configured on tablet %v", name)
	}
	// We expect the error ErrNotSlave, which means "SHOW SLAVE STATUS" returned
	// zero rows (replication is not configured at all).
	if !strings.Contains(err.Error(), mysql.ErrNotSlave.Error()) {
		// SlaveStatus() failed for the wrong reason.
		return fmt.Errorf("failed to get slave status for tablet %v: %v", name, err)
	}

	// Now we know replication is not configured.
	// Make sure the main database for the keyspace does *not* exist,
	// as evidence that the shard was never initialized before.
	//
	// TODO(enisoc): Upstream a feature that would let us confirm that,
	//   if a restore from backup is going to happen, it happened already.
	//   For now, there's a small chance of a race causing a false negative,
	//   but it's still better to check than to not check.
	dbExists, err := tabletDatabaseExists(ctx, tmc, ti.Tablet)
	if err != nil {
		return fmt.Errorf("couldn't determine whether tablet %v database exists: %v", name, err)
	}
	if dbExists {
		// The shard was already initialized at some point before.
		// This could happen if the tablet has restored from a backup.
		// We must not do InitShardMaster again, because that would reset
		// replication positions (erase GTID history), which would make all
		// existing backups for the shard invalid.
		return fmt.Errorf("the database for keyspace %v was already created on tablet %v; not safe to assume shard is uninitialized", ti.Keyspace, name)
	}

	return nil
}

func tabletDatabaseExists(ctx context.Context, tmc tmclient.TabletManagerClient, tablet *topodatapb.Tablet) (bool, error) {
	dbName := topoproto.TabletDbName(tablet)
	if dbName == "" {
		return true, errors.New("couldn't determine database name")
	}

	// Get a list of all databases.
	qrproto, err := tmc.ExecuteFetchAsDba(ctx, tablet, true /*usePool*/, []byte("SHOW DATABASES"), 10000 /*maxRows*/, false /*disableBinlogs*/, false /*reloadSchema*/)
	if err != nil {
		return false, err
	}
	qr := sqltypes.Proto3ToResult(qrproto)

	// Look for the main database name.
	for _, row := range qr.Rows {
		if row[0].ToString() == dbName {
			return true, nil
		}
	}
	return false, nil
}
