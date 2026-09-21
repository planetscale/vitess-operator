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

package vitessshard

import (
	"context"
	"strings"
	"time"

	"vitess.io/vitess/go/vt/topo/topoproto"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"vitess.io/vitess/go/vt/logutil"
	"vitess.io/vitess/go/vt/topo"
	"vitess.io/vitess/go/vt/wrangler"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/environment"
	"planetscale.dev/vitess-operator/pkg/operator/k8s"
	"planetscale.dev/vitess-operator/pkg/operator/results"
	"planetscale.dev/vitess-operator/pkg/operator/toposerver"

	topodatapb "vitess.io/vitess/go/vt/proto/topodata"
	vtctldatapb "vitess.io/vitess/go/vt/proto/vtctldata"
)

const (
	topoReconcileTimeout = 20 * time.Second

	// topoRequeueDelay is how long to wait before retrying when a topology
	// server call failed. We typically return success with a requeue delay
	// instead of returning an error, because it's unlikely that retrying
	// immediately will be worthwhile.
	topoRequeueDelay = 5 * time.Second
)

func (r *ReconcileVitessShard) reconcileTopology(ctx context.Context, vts *planetscalev2.VitessShard) (reconcile.Result, error) {
	resultBuilder := &results.Builder{}

	// Don't hold our slot in the reconcile work queue for too long.
	ctx, cancel := context.WithTimeout(ctx, topoReconcileTimeout)
	defer cancel()

	ts, err := toposerver.Open(ctx, vts.Spec.GlobalLockserver)
	if err != nil {
		r.recorder.Eventf(vts, corev1.EventTypeWarning, "TopoConnectFailed", "failed to connect to global lockserver: %v", err)
		// Give the lockserver some time to come up.
		return resultBuilder.RequeueAfter(topoRequeueDelay)
	}
	defer ts.Close()

	return r.reconcileTopologyWithServer(ctx, vts, ts.Server)
}

// reconcileTopologyWithServer does the topology reconciliation against an
// already opened topo server. It's split out so it can be exercised in tests
// with an in-memory topo.
func (r *ReconcileVitessShard) reconcileTopologyWithServer(ctx context.Context, vts *planetscalev2.VitessShard, ts *topo.Server) (reconcile.Result, error) {
	keyspaceName := vts.Labels[planetscalev2.KeyspaceLabel]
	resultBuilder := &results.Builder{}

	vtEnv, err := environment.VtEnvironment()
	if err != nil {
		return resultBuilder.Error(err)
	}
	wr := wrangler.New(vtEnv, logutil.NewConsoleLogger(), ts, nil)

	// Get the shard record.
	shardRecordExists := true
	if shard, err := ts.GetShard(ctx, keyspaceName, vts.Spec.Name); err == nil {
		vts.Status.HasMaster = k8s.ConditionStatus(shard.HasPrimary())
		if shard.PrimaryAlias != nil {
			vts.Status.MasterAlias = topoproto.TabletAliasString(shard.PrimaryAlias)
		}
		vts.Status.ServingWrites = k8s.ConditionStatus(shard.IsPrimaryServing)

		// Is the shard in the serving partition for any cell or tablet type?
		if servingCells, err := ts.GetShardServingCells(ctx, shard); err == nil {
			vts.Status.Idle = k8s.ConditionStatus(len(servingCells) == 0)

			if *vts.Spec.TopologyReconciliation.PruneShardCells {
				result, err := r.pruneShardCells(ctx, vts, keyspaceName, servingCells, wr)
				resultBuilder.Merge(result, err)
			}
		} else {
			r.recorder.Eventf(vts, corev1.EventTypeWarning, "TopoGetFailed", "failed to get shard serving cells: %v", err)
			resultBuilder.RequeueAfter(topoRequeueDelay)
		}
	} else if topo.IsErrType(err, topo.NoNode) {
		// The global shard record is gone. This happens legitimately when
		// `Reshard complete` deletes the source shards before the old
		// partitioning is removed from the VitessKeyspace spec. The shard may
		// still be referenced from a cell's SrvKeyspace, so Idle still has to
		// be computed from the serving partitions. That check only needs the
		// keyspace and shard name, not the record itself.
		//
		// HasMaster and ServingWrites are deliberately left Unknown: the
		// vitessshardreplication controller treats HasMaster=False as "no
		// primary yet, go initialize one", which must not happen for a shard
		// whose record has just been deleted, nor for a new shard whose first
		// tablet hasn't created the record yet. Idle is all the keyspace
		// controller needs to decide on a turndown.
		shardRecordExists = false
		shard := topo.NewShardInfo(keyspaceName, vts.Spec.Name, &topodatapb.Shard{}, nil)
		if servingCells, err := ts.GetShardServingCells(ctx, shard); err == nil {
			vts.Status.Idle = k8s.ConditionStatus(len(servingCells) == 0)
		} else {
			// Leave Idle as Unknown: an incomplete view of the cells must
			// never be treated as permission to turn the shard down.
			r.recorder.Eventf(vts, corev1.EventTypeWarning, "TopoGetFailed", "shard record does not exist and failed to get shard serving cells: %v", err)
			resultBuilder.RequeueAfter(topoRequeueDelay)
		}
	} else {
		r.recorder.Eventf(vts, corev1.EventTypeWarning, "TopoGetFailed", "failed to get shard info: %v", err)
		resultBuilder.RequeueAfter(topoRequeueDelay)
	}

	// Get all the tablet records for this shard. Without a shard record this
	// lookup can only fail (it needs the record to find the cells), so skip it
	// rather than requeueing until the shard is turned down.
	if !shardRecordExists {
		return resultBuilder.Result()
	}
	if tablets, err := ts.GetTabletMapForShard(ctx, keyspaceName, vts.Spec.Name); err == nil {
		// Update status for desired tablets.
		for name, status := range vts.Status.Tablets {
			tablet := tablets[name]
			if tablet == nil {
				continue
			}
			status.Type = strings.ToLower(tablet.GetType().String())
			vts.Status.Tablets[name] = status
		}

		if *vts.Spec.TopologyReconciliation.PruneTablets {
			result, err := r.pruneTablets(ctx, vts, tablets, wr)
			resultBuilder.Merge(result, err)
		}
	} else {
		r.recorder.Eventf(vts, corev1.EventTypeWarning, "TopoGetFailed", "failed to get tablet records: %v", err)
		resultBuilder.RequeueAfter(topoRequeueDelay)
	}

	return resultBuilder.Result()
}

func (r *ReconcileVitessShard) pruneTablets(ctx context.Context, vts *planetscalev2.VitessShard, tablets map[string]*topo.TabletInfo, wr *wrangler.Wrangler) (reconcile.Result, error) {
	resultBuilder := &results.Builder{}

	// Clean up tablets that exist but shouldn't.
	for name, tabletInfo := range tablets {
		if !vts.Spec.CellInCluster(tabletInfo.Alias.GetCell()) {
			// Skip tablets whose cell is not defined in the VitessCluster.
			// We should only operate on cells we've been told to manage,
			// since others might be externally managed.
			continue
		}

		_, desired := vts.Status.Tablets[name]
		_, orphaned := vts.Status.OrphanedTablets[name]
		if !desired && !orphaned {
			// The tablet exists in topo, but not in the VitessShard spec.
			// It's also not being kept around by a blocked turn-down.
			// We use the Vitess wrangler (multi-step command executor) to delete the tablet.
			// This is equivalent to `vtctl DeleteTablet`.
			if err := wr.DeleteTablet(ctx, tabletInfo.Alias, false /* allowPrimary */); err != nil {
				r.recorder.Eventf(vts, corev1.EventTypeWarning, "TopoCleanupFailed", "unable to remove tablet %s from topology: %v", name, err)
				resultBuilder.RequeueAfter(topoRequeueDelay)
			} else {
				r.recorder.Eventf(vts, corev1.EventTypeNormal, "TopoCleanup", "removed unwanted tablet %s from topology", name)
			}
		}
	}

	return resultBuilder.Result()
}

func (r *ReconcileVitessShard) pruneShardCells(ctx context.Context, vts *planetscalev2.VitessShard, keyspaceName string, servingCells []string, wr *wrangler.Wrangler) (reconcile.Result, error) {
	resultBuilder := &results.Builder{}

	// Clean up cells from the shard record that we don't deploy to anymore.
	for _, cellName := range servingCells {
		if !vts.Spec.CellInCluster(cellName) {
			// Skip cells that are not even present in the VitessCluster.
			// We should only operate on cells that we've been told to manage,
			// since the others might be externally managed.
			continue
		}
		if topo.InCellList(cellName, vts.Status.Cells) {
			// We still have tablets here. Don't prune this cell.
			continue
		}

		// The cell is listed in topo, but we don't deploy there anymore.
		// We use the Vitess wrangler (multi-step command executor) to remove the cell from that shard.
		// This is equivalent to `vtctl RemoveShardCell`.
		if _, err := wr.VtctldServer().RemoveShardCell(ctx, &vtctldatapb.RemoveShardCellRequest{
			Keyspace:  keyspaceName,
			ShardName: vts.Spec.Name,
			Cell:      cellName,
			Force:     false, /* force */
			Recursive: false, /* recursive */
		}); err != nil {
			r.recorder.Eventf(vts, corev1.EventTypeWarning, "TopoCleanupFailed", "unable to remove cell %s from shard: %v", cellName, err)
			resultBuilder.RequeueAfter(topoRequeueDelay)
		} else {
			r.recorder.Eventf(vts, corev1.EventTypeNormal, "TopoCleanup", "removed unwanted cell %s from shard", cellName)
		}
	}

	return resultBuilder.Result()
}
