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
	"planetscale.dev/vitess-operator/pkg/operator/k8s"
	"planetscale.dev/vitess-operator/pkg/operator/results"
	"planetscale.dev/vitess-operator/pkg/operator/toposerver"
)

const (
	topoReconcileTimeout = 5 * time.Second

	// topoRequeueDelay is how long to wait before retrying when a topology
	// server call failed. We typically return success with a requeue delay
	// instead of returning an error, because it's unlikely that retrying
	// immediately will be worthwhile.
	topoRequeueDelay = 5 * time.Second
)

func (r *ReconcileVitessShard) reconcileTopology(ctx context.Context, vts *planetscalev2.VitessShard) (reconcile.Result, error) {
	keyspaceName := vts.Labels[planetscalev2.KeyspaceLabel]
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
	wr := wrangler.New(logutil.NewConsoleLogger(), ts.Server, nil)

	// Get the shard record.
	if shard, err := ts.GetShard(ctx, keyspaceName, vts.Spec.Name); err == nil {
		vts.Status.HasMaster = k8s.ConditionStatus(shard.HasMaster())
		if shard.MasterAlias != nil {
			vts.Status.MasterAlias = topoproto.TabletAliasString(shard.MasterAlias)
		}

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
	} else {
		r.recorder.Eventf(vts, corev1.EventTypeWarning, "TopoGetFailed", "failed to get shard info: %v", err)
		resultBuilder.RequeueAfter(topoRequeueDelay)
	}

	// Get all the tablet records for this shard.
	if tablets, err := ts.GetTabletMapForShard(ctx, keyspaceName, vts.Spec.Name); err == nil {
		// Update status for desired tablets.
		for name, status := range vts.Status.Tablets {
			tablet := tablets[name]
			if tablet == nil {
				continue
			}
			status.Type = strings.ToLower(tablet.GetType().String())
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
		if vts.Status.Tablets[name] == nil && vts.Status.OrphanedTablets[name] == nil {
			// The tablet exists in topo, but not in the VitessShard spec.
			// It's also not being kept around by a blocked turn-down.
			// We use the Vitess wrangler (multi-step command executor) to delete the tablet.
			// This is equivalent to `vtctl DeleteTablet`.
			if err := wr.DeleteTablet(ctx, tabletInfo.Alias, false /* allowMaster */); err != nil {
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
		if err := wr.RemoveShardCell(ctx, keyspaceName, vts.Spec.Name, cellName, false /* force*/, false /* recursive */); err != nil {
			r.recorder.Eventf(vts, corev1.EventTypeWarning, "TopoCleanupFailed", "unable to remove cell %s from shard: %v", cellName, err)
			resultBuilder.RequeueAfter(topoRequeueDelay)
		} else {
			r.recorder.Eventf(vts, corev1.EventTypeNormal, "TopoCleanup", "removed unwanted cell %s from shard", cellName)
		}
	}

	return resultBuilder.Result()
}
