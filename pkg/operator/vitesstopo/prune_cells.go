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
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	topodatapb "vitess.io/vitess/go/vt/proto/topodata"
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
	OrphanedCells map[string]*planetscalev2.OrphanStatus
	// CellsAliasFilterSet is an optional field that can be used to limit responsibility
	// for pruning to a set of CellsAlias string names.
	CellsAliasFilterSet sets.String
}

// PruneCells will prune cells that exist but shouldn't anymore.
func PruneCells(ctx context.Context, p PruneCellsParams) (reconcile.Result, error) {
	resultBuilder := &results.Builder{}

	candidates, result, err := CellCandidatesForPruning(ctx, p)
	resultBuilder.Merge(result, err)

	for _, cellName := range candidates {
		if err := p.TopoServer.DeleteCellInfo(ctx, cellName); err != nil {
			p.Recorder.Eventf(p.EventObj, corev1.EventTypeWarning, "TopoCleanupFailed", "unable to remove cell %s from topology: %v", cellName, err)
			resultBuilder.RequeueAfter(topoRequeueDelay)
		} else {
			p.Recorder.Eventf(p.EventObj, corev1.EventTypeNormal, "TopoCleanup", "removed unwanted cell %s from topology", cellName)
		}
	}

	return resultBuilder.Result()
}

// CandidatesFromCellsList returns a list of cell candidates for pruning, based on a provided list of cells to consider.
func CandidatesFromCellsList(cellNames []string, desiredCells map[string]*planetscalev2.LockserverSpec, orphanedCells map[string]*planetscalev2.OrphanStatus) []string {
	var candidates []string

	for _, name := range cellNames {
		if desiredCells[name] == nil && orphanedCells[name] == nil {
			// The cell exists in topo, but not in the VT spec.
			// It's also not being kept around by a blocked turn-down.
			// We should add it to the list of candidates to prune.
			candidates = append(candidates, name)
		}
	}

	return candidates
}

// CellCandidatesForPruning returns a list of candidates that are optionally filtered by a list of cells aliases.
func CellCandidatesForPruning(ctx context.Context, p PruneCellsParams) ([]string, reconcile.Result, error) {
	resultBuilder := &results.Builder{}

	// Get list of cells in topology derives from cells aliases so we can optionally filter
	// by a supplied list of cells aliases.
	aliases, err := p.TopoServer.GetCellsAliases(ctx, true)
	if err != nil {
		p.Recorder.Eventf(p.EventObj, corev1.EventTypeWarning, "TopoListFailed", "failed to list cells in topology: %v", err)
		result, err := resultBuilder.RequeueAfter(topoRequeueDelay)
		return nil, result, err
	}

	filteredAliases := filterAliasesByAliasSet(aliases, p.CellsAliasFilterSet)
	cellNames := cellNamesFromCellsAliases(filteredAliases)
	candidates := CandidatesFromCellsList(cellNames, p.DesiredCells, p.OrphanedCells)

	result, err := resultBuilder.Result()
	return candidates, result, err
}

// filterAliasesByAliasSet takes a list of topodatapb.CellsAlias and filters them by a set of cellsalias names.
func filterAliasesByAliasSet(aliases map[string]*topodatapb.CellsAlias, filterList sets.String) map[string]*topodatapb.CellsAlias {
	if filterList == nil {
		return aliases
	}

	out := make(map[string]*topodatapb.CellsAlias, len(filterList))

	for aliasName, alias := range aliases {
		if filterList.Has(aliasName) {
			out[aliasName] = alias
		}
	}

	return out
}

// cellNamesFromCellsAliases unpacks all the cells based on the topodatapb.CellsAlias supplied.
func cellNamesFromCellsAliases(aliases map[string]*topodatapb.CellsAlias) []string {
	var cells []string

	for _, alias := range aliases {
		cells = append(cells, alias.Cells...)
	}

	return cells
}
