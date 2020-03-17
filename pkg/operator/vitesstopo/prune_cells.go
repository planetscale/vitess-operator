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
	OrphanedCells map[string]*planetscalev2.OrphanStatus
}

func PruneCells(ctx context.Context, p PruneCellsParams) (reconcile.Result, error) {
	resultBuilder := &results.Builder{}

	// Get list of cells in topo.
	cellNames, err := p.TopoServer.GetCellInfoNames(ctx)
	if err != nil {
		p.Recorder.Eventf(p.EventObj, corev1.EventTypeWarning, "TopoListFailed", "failed to list cells in topology: %v", err)
		return resultBuilder.RequeueAfter(topoRequeueDelay)
	}

	// Clean up cells that exist but shouldn't.
	for _, name := range cellNames {
		if p.DesiredCells[name] == nil && p.OrphanedCells[name] == nil {
			// The cell exists in topo, but not in the VT spec.
			// It's also not being kept around by a blocked turn-down.
			// See if we can delete it. This will fail if the cell is not empty.
			if err := p.TopoServer.DeleteCellInfo(ctx, name); err != nil {
				p.Recorder.Eventf(p.EventObj, corev1.EventTypeWarning, "TopoCleanupFailed", "unable to remove cell %s from topology: %v", name, err)
				resultBuilder.RequeueAfter(topoRequeueDelay)
			} else {
				p.Recorder.Eventf(p.EventObj, corev1.EventTypeNormal, "TopoCleanup", "removed unwanted cell %s from topology", name)
			}
		}
	}
	return resultBuilder.Result()
}
