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

package vitesscluster

import (
	"context"
	"time"

	"vitess.io/vitess/go/vt/logutil"
	topodatapb "vitess.io/vitess/go/vt/proto/topodata"
	"vitess.io/vitess/go/vt/topo"
	"vitess.io/vitess/go/vt/wrangler"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/lockserver"
	"planetscale.dev/vitess-operator/pkg/operator/results"
	"planetscale.dev/vitess-operator/pkg/operator/stringsets"
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

func (r *ReconcileVitessCluster) reconcileTopology(ctx context.Context, vt *planetscalev2.VitessCluster) (reconcile.Result, error) {
	resultBuilder := &results.Builder{}

	// Don't hold our slot in the reconcile work queue for too long.
	ctx, cancel := context.WithTimeout(ctx, topoReconcileTimeout)
	defer cancel()

	// Connect to the global lockserver.
	globalParams := lockserver.GlobalConnectionParams(vt)
	if globalParams == nil {
		// This is an invalid config. There's no reason to request a retry. Just wait for the next mutation to trigger us.
		r.recorder.Event(vt, corev1.EventTypeWarning, "TopoInvalid", "no global lockserver is defined")
		return resultBuilder.Result()
	}
	ts, err := toposerver.Open(ctx, *globalParams)
	if err != nil {
		r.recorder.Eventf(vt, corev1.EventTypeWarning, "TopoConnectFailed", "failed to connect to global lockserver: %v", err)
		// Give the lockserver some time to come up.
		return resultBuilder.RequeueAfter(topoRequeueDelay)
	}
	defer ts.Close()

	cellResult, err := r.reconcileCellTopology(ctx, vt, ts.Server, globalParams.Implementation)
	resultBuilder.Merge(cellResult, err)

	keyspaceResult, err := r.reconcileKeyspaceTopology(ctx, vt, ts.Server)
	resultBuilder.Merge(keyspaceResult, err)

	return resultBuilder.Result()
}

func (r *ReconcileVitessCluster) reconcileCellTopology(ctx context.Context, vt *planetscalev2.VitessCluster, ts *topo.Server, globalTopoImpl string) (reconcile.Result, error) {
	resultBuilder := &results.Builder{}

	// Make a map from cell name (as Vitess calls them) back to the cell spec.
	desiredCells := make(map[string]*planetscalev2.VitessCellTemplate, len(vt.Spec.Cells))
	for i := range vt.Spec.Cells {
		cell := &vt.Spec.Cells[i]
		desiredCells[cell.Name] = cell
	}

	if *vt.Spec.TopologyReconciliation.RegisterCellsAliases {
		err := r.registerCellsAliases(ctx, vt, ts, desiredCells)
		if err != nil {
			return resultBuilder.Error(err)
		}
	}

	if *vt.Spec.TopologyReconciliation.RegisterCells {
		result, err := r.registerCells(ctx, vt, ts, globalTopoImpl, desiredCells)
		resultBuilder.Merge(result, err)
	}

	if *vt.Spec.TopologyReconciliation.PruneCells {
		result, err := r.pruneCells(ctx, vt, ts, desiredCells)
		if err != nil {
			return resultBuilder.RequeueAfter(topoRequeueDelay)
		}
		resultBuilder.Merge(result, err)
	}

	return resultBuilder.Result()
}

func (r *ReconcileVitessCluster) pruneCells(ctx context.Context, vt *planetscalev2.VitessCluster, ts *topo.Server, desiredCells map[string]*planetscalev2.VitessCellTemplate) (reconcile.Result, error) {
	resultBuilder := &results.Builder{}

	// Get list of cells in topo.
	cellNames, err := ts.GetCellInfoNames(ctx)
	if err != nil {
		r.recorder.Eventf(vt, corev1.EventTypeWarning, "TopoListFailed", "failed to list cells in topology: %v", err)
		return resultBuilder.Error(err)
	}

	// Clean up cells that exist but shouldn't.
	for _, name := range cellNames {
		if desiredCells[name] == nil && vt.Status.OrphanedCells[name] == nil {
			// The cell exists in topo, but not in the VT spec.
			// It's also not being kept around by a blocked turn-down.
			// See if we can delete it. This will fail if the cell is not empty.
			if err := ts.DeleteCellInfo(ctx, name); err != nil {
				r.recorder.Eventf(vt, corev1.EventTypeWarning, "TopoCleanupFailed", "unable to remove cell %s from topology: %v", name, err)
				resultBuilder.RequeueAfter(topoRequeueDelay)
			} else {
				r.recorder.Eventf(vt, corev1.EventTypeNormal, "TopoCleanup", "removed unwanted cell %s from topology", name)
			}
		}
	}
	return resultBuilder.Result()
}

func (r *ReconcileVitessCluster) registerCells(ctx context.Context, vt *planetscalev2.VitessCluster, ts *topo.Server, globalTopoImpl string, desiredCells map[string]*planetscalev2.VitessCellTemplate) (reconcile.Result, error) {
	resultBuilder := &results.Builder{}

	// Create or update cell info for cells that should exist.
	for name, cell := range desiredCells {
		params := lockserver.LocalConnectionParams(vt, cell)
		if params == nil {
			r.recorder.Eventf(vt, corev1.EventTypeWarning, "TopoInvalid", "no local lockserver is defined for cell %v", name)
			continue
		}
		if params.Implementation != globalTopoImpl {
			r.recorder.Eventf(vt, corev1.EventTypeWarning, "TopoInvalid", "local lockserver implementation for cell %v doesn't match global topo implementation", name)
			continue
		}
		updated := false
		err := ts.UpdateCellInfoFields(ctx, name, func(cellInfo *topodatapb.CellInfo) error {
			// Skip the update if it already matches.
			if cellInfo.ServerAddress == params.Address && cellInfo.Root == params.RootPath {
				return topo.NewError(topo.NoUpdateNeeded, "")
			}
			cellInfo.ServerAddress = params.Address
			cellInfo.Root = params.RootPath
			updated = true
			return nil
		})
		if err != nil {
			// Record the error and continue trying other cells.
			r.recorder.Eventf(vt, corev1.EventTypeWarning, "TopoUpdateFailed", "failed to update lockserver address for cell %v", name)
			resultBuilder.RequeueAfter(topoRequeueDelay)
		}
		if updated {
			r.recorder.Eventf(vt, corev1.EventTypeNormal, "TopoUpdated", "updated lockserver addess for cell %v", name)
		}
	}
	return resultBuilder.Result()
}

func (r *ReconcileVitessCluster) registerCellsAliases(ctx context.Context, vt *planetscalev2.VitessCluster, ts *topo.Server, desiredCells map[string]*planetscalev2.VitessCellTemplate) error {
	// We need to add an alias for all the cells in each region so that vtgate
	// knows that it can route traffic between them.
	// Currently, only one region is supported so we allow routing anywhere.
	//
	// We also need to create the aliases before we create the cells because we
	// don't want any vtgates to start after the cells are created but before
	// the alias exists.
	desiredCellsAliases := buildCellsAliases(desiredCells)
	currentCellsAliases, err := ts.GetCellsAliases(ctx, true)
	if err != nil {
		r.recorder.Eventf(vt, corev1.EventTypeWarning, "TopoCellAlias",
			"Failed to get current cell aliases: %v", err)
		return err
	}
	for alias, desiredCellsAlias := range desiredCellsAliases {
		// If this alias already exists and matches what we are trying to update
		// it to, skip it.
		if _, ok := currentCellsAliases[alias]; ok {
			if stringsets.Equal(desiredCellsAlias.Cells, currentCellsAliases[alias].Cells) {
				continue
			}
		}
		// Create or update cells alias.
		err = ts.UpdateCellsAlias(ctx, alias, func(ca *topodatapb.CellsAlias) error {
			// Even if we're creating, 'ca' will already be non-nil.
			ca.Cells = desiredCellsAlias.Cells
			return nil
		})
		if err != nil {
			r.recorder.Eventf(vt, corev1.EventTypeWarning, "TopoCellAlias",
				"Failed to create or update cell alias: %s: %v", alias, err)
			return err
		}
		r.recorder.Eventf(vt, corev1.EventTypeNormal, "TopoCellAlias",
			"Created or updated cells alias: %s -> %v", alias,
			desiredCellsAlias.Cells)
	}
	return nil
}

func (r *ReconcileVitessCluster) reconcileKeyspaceTopology(ctx context.Context, vt *planetscalev2.VitessCluster, ts *topo.Server) (reconcile.Result, error) {
	resultBuilder := &results.Builder{}

	// Make a map from keyspace name (as Vitess calls them) back to the keyspace spec.
	desiredKeyspaces := make(map[string]*planetscalev2.VitessKeyspaceTemplate, len(vt.Spec.Keyspaces))
	for i := range vt.Spec.Keyspaces {
		keyspace := &vt.Spec.Keyspaces[i]
		desiredKeyspaces[keyspace.Name] = keyspace
	}

	if *vt.Spec.TopologyReconciliation.PruneKeyspaces == true {
		result, err := r.pruneKeyspaces(ctx, vt, ts, desiredKeyspaces)
		resultBuilder.Merge(result, err)
	}

	return resultBuilder.Result()
}

func (r *ReconcileVitessCluster) pruneKeyspaces(ctx context.Context, vt *planetscalev2.VitessCluster, ts *topo.Server, desiredKeyspaces map[string]*planetscalev2.VitessKeyspaceTemplate) (reconcile.Result, error) {
	resultBuilder := &results.Builder{}

	// Get list of keyspaces in topo.
	keyspaceNames, err := ts.GetKeyspaces(ctx)
	if err != nil {
		r.recorder.Eventf(vt, corev1.EventTypeWarning, "TopoListFailed", "failed to list keyspaces in topology: %v", err)
		return resultBuilder.RequeueAfter(topoRequeueDelay)
	}

	// Clean up keyspaces that exist but shouldn't.
	for _, name := range keyspaceNames {
		if desiredKeyspaces[name] == nil && vt.Status.OrphanedKeyspaces[name] == nil {
			// The keyspace exists in topo, but not in the VT spec.
			// It's also not being kept around by a blocked turn-down.
			// We use the Vitess wrangler (multi-step command executor) to recursively delete the keyspace.
			// This is equivalent to `vtctl DeleteKeyspace -recursive`.
			wr := wrangler.New(logutil.NewConsoleLogger(), ts, nil)
			if err := wr.DeleteKeyspace(ctx, name, true); err != nil {
				r.recorder.Eventf(vt, corev1.EventTypeWarning, "TopoCleanupFailed", "unable to remove keyspace %s from topology: %v", name, err)
				resultBuilder.RequeueAfter(topoRequeueDelay)
			} else {
				r.recorder.Eventf(vt, corev1.EventTypeNormal, "TopoCleanup", "removed unwanted keyspace %s from topology", name)
			}
		}
	}
	return resultBuilder.Result()
}
