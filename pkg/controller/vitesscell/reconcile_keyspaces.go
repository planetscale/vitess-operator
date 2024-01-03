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

package vitesscell

import (
	"context"

	"planetscale.dev/vitess-operator/pkg/operator/toposerver"

	corev1 "k8s.io/api/core/v1"
	apilabels "k8s.io/apimachinery/pkg/labels"
	apitypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/k8s"
	"planetscale.dev/vitess-operator/pkg/operator/results"
	"planetscale.dev/vitess-operator/pkg/operator/vitesscell"
)

// Map maps a VitessKeyspace to a list of requests for VitessCells
// in which the keyspace is deployed.
func keyspaceCellsMapper(ctx context.Context, obj client.Object) []reconcile.Request {
	vtk := obj.(*planetscalev2.VitessKeyspace)

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
	if err := r.client.List(ctx, list, opts); err != nil {
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

	if vtc.Status.Lockserver.Etcd != nil {
		// We know something about local etcd status.
		// We can use that to avoid trying to connect when we know it won't work.
		if vtc.Status.Lockserver.Etcd.Available != corev1.ConditionTrue {
			r.recorder.Event(vtc, corev1.EventTypeNormal, "TopoWaiting", "waiting for local etcd to become Available")
			// Return success. We don't need to requeue because we'll get queued any time the EtcdCluster status changes.
			return resultBuilder.Result()
		}
	}

	// We actually know the address of the local lockserver already,
	// but for now we'll follow the same rule as all Vitess components,
	// which is to use the global lockserver to find the local ones.
	ts, err := toposerver.Open(ctx, vtc.Spec.GlobalLockserver)
	if err != nil {
		r.recorder.Eventf(vtc, corev1.EventTypeWarning, "TopoConnectFailed", "failed to connect to global lockserver: %v", err)
		return resultBuilder.RequeueAfter(topoRequeueDelay)
	}
	defer ts.Close()

	// We have the list of keyspaces that should exist in this cell.
	// See if we need to clean up topology.
	result, err := r.reconcileTopology(ctx, vtc, ts, keyspaces)
	resultBuilder.Merge(result, err)

	// Don't hold our slot in the reconcile work queue for too long.
	ctx, cancel := context.WithTimeout(ctx, topoReconcileTimeout)
	defer cancel()

	// Get the list of keyspaces deployed (served) in this cell.
	srvKeyspaceNames, err := ts.GetSrvKeyspaceNames(ctx, vtc.Spec.Name)
	if err != nil {
		r.recorder.Eventf(vtc, corev1.EventTypeWarning, "TopoListFailed", "failed to list keyspaces in cell-local lockserver: %v", err)
		return resultBuilder.RequeueAfter(topoRequeueDelay)
	}

	// We successfully listed topo, so know we know the whole picture.
	// We're idle if both topo and our list of VitessKeyspaces are confirmed empty.
	vtc.Status.Idle = k8s.ConditionStatus(len(srvKeyspaceNames) == 0 && len(keyspaces) == 0)

	return resultBuilder.Result()
}
