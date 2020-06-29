/*
Copyright 2020 PlanetScale Inc.

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

package vitesstopo

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"vitess.io/vitess/go/vt/topo"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/results"
)

type PruneCellsParams struct {
	// EventObj holds the object type that the recorder will use when writing events.
	EventObj   runtime.Object
	TopoServer *topo.Server
	Recorder   record.EventRecorder
	// DesiredCells is a map of cell names to their lockserver specs.
	DesiredCells map[string]*planetscalev2.LockserverSpec
	// OrphanedCells is a list of unwanted cells that could not be turned down.
	OrphanedCells map[string]planetscalev2.OrphanStatus
}

// PruneCells will prune cells that exist but shouldn't anymore.
func PruneCells(ctx context.Context, p PruneCellsParams) (reconcile.Result, error) {
	resultBuilder := &results.Builder{}

	// Get list of cells in topo.
	cellNames, err := p.TopoServer.GetCellInfoNames(ctx)
	if err != nil {
		p.Recorder.Eventf(p.EventObj, corev1.EventTypeWarning, "TopoListFailed", "failed to list cells in topology: %v", err)
		return resultBuilder.RequeueAfter(topoRequeueDelay)
	}

	candidates := CellsToPrune(cellNames, p.DesiredCells, p.OrphanedCells)

	result, err := DeleteCells(ctx, p.TopoServer, p.Recorder, p.EventObj, candidates)
	resultBuilder.Merge(result, err)

	return resultBuilder.Result()
}

// CellsToPrune returns a list of cell candidates for pruning, based on a provided list of cells to consider.
func CellsToPrune(cellNames []string, desiredCells map[string]*planetscalev2.LockserverSpec, orphanedCells map[string]planetscalev2.OrphanStatus) []string {
	var candidates []string

	for _, name := range cellNames {
		_, orphaned := orphanedCells[name]
		if desiredCells[name] == nil && !orphaned {
			// The cell exists in topo, but not in the VT spec.
			// It's also not being kept around by a blocked turn-down.
			// We should add it to the list of candidates to prune.
			candidates = append(candidates, name)
		}
	}

	return candidates
}

// DeleteCells takes in a list of cell names and deletes their CellInfo records from topology.
func DeleteCells(ctx context.Context, ts *topo.Server, recorder record.EventRecorder, eventObj runtime.Object, cellNames []string) (reconcile.Result, error) {
	resultBuilder := &results.Builder{}

	for _, cellName := range cellNames {
		// topo.NoNode is the error type returned if we can't find the cell when deleting. This ensures that this operation is idempotent.
		if err := ts.DeleteCellInfo(ctx, cellName, true); err != nil && !topo.IsErrType(err, topo.NoNode) {
			recorder.Eventf(eventObj, corev1.EventTypeWarning, "TopoCleanupFailed", "unable to remove cell %s from topology: %v", cellName, err)
			resultBuilder.RequeueAfter(topoRequeueDelay)
		} else if err == nil {
			recorder.Eventf(eventObj, corev1.EventTypeNormal, "TopoCleanup", "removed unwanted cell %s from topology", cellName)
		}
	}

	return resultBuilder.Result()
}
