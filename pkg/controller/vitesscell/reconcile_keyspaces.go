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

package vitesscell

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apilabels "k8s.io/apimachinery/pkg/labels"
	apitypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/k8s"
	"planetscale.dev/vitess-operator/pkg/operator/results"
	"planetscale.dev/vitess-operator/pkg/operator/vitesscell"
)

type keyspaceCellsMapper struct{}

// Map maps a VitessKeyspace to a list of requests for VitessCells
// in which the keyspace is deployed.
func (*keyspaceCellsMapper) Map(obj handler.MapObject) []reconcile.Request {
	vtk := obj.Object.(*planetscalev2.VitessKeyspace)

	// Request reconciliation for all the VitessCells to which this VitessKeyspace is deployed.
	var requests []reconcile.Request
	for _, cellName := range vtk.Spec.CellNames() {
		// Compute the full VitessCell object name from the cell name.
		clusterName := vtk.Labels[planetscalev2.ClusterLabel]
		requests = append(requests, reconcile.Request{
			NamespacedName: apitypes.NamespacedName{
				Namespace: vtk.Namespace,
				Name:      vitesscell.Name(clusterName, cellName),
			},
		})
	}
	return requests
}

func (r *ReconcileVitessCell) reconcileKeyspaces(ctx context.Context, vtc *planetscalev2.VitessCell) (reconcile.Result, error) {
	resultBuilder := &results.Builder{}

	// List all keyspaces in the same cluster.
	// Note that this is cheap because it comes from the local cache.
	labels := map[string]string{
		planetscalev2.ClusterLabel: vtc.Labels[planetscalev2.ClusterLabel],
	}
	opts := &client.ListOptions{
		Namespace:     vtc.Namespace,
		LabelSelector: apilabels.SelectorFromSet(apilabels.Set(labels)),
	}
	list := &planetscalev2.VitessKeyspaceList{}
	if err := r.client.List(ctx, opts, list); err != nil {
		r.recorder.Eventf(vtc, corev1.EventTypeWarning, "ListFailed", "failed to list VitessKeyspace objects: %v", err)
		return resultBuilder.Error(err)
	}

	// Find the keyspaces deployed in this cell.
	var keyspaces []*planetscalev2.VitessKeyspace
	for vtkIndex := range list.Items {
		vtk := &list.Items[vtkIndex]
		// Is the keyspace deployed in this cell?
		for _, cellName := range vtk.Spec.CellNames() {
			if cellName == vtc.Spec.Name {
				// Yes, it's deployed here.
				keyspaces = append(keyspaces, vtk)
				break
			}
		}
	}

	// Record status for the keyspaces deployed in this cell.
	for _, vtk := range keyspaces {
		// TODO(enisoc): Fill in status fields when VitessKeyspace has status.
		vtc.Status.Keyspaces[vtk.Spec.Name] = planetscalev2.VitessCellKeyspaceStatus{}
	}

	// If we found any keyspaces targeting this cell, we already know we can't call the cell empty.
	if len(keyspaces) > 0 {
		vtc.Status.Idle = corev1.ConditionFalse
	}

	// We have the list of keyspaces that should exist in this cell.
	// See if we need to clean up topology.
	topoEntries, err := r.reconcileTopology(ctx, vtc, keyspaces)
	if err != nil {
		return resultBuilder.RequeueAfter(topoRequeueDelay)
	}

	if topoEntries != nil {
		// We successfully listed topo, so know we know the whole picture.
		// We're idle if both topo and our list of VitessKeyspaces are confirmed empty.
		vtc.Status.Idle = k8s.ConditionStatus(len(topoEntries) == 0 && len(keyspaces) == 0)
	}

	return resultBuilder.Result()
}
