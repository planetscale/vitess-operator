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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/lockserver"
	"planetscale.dev/vitess-operator/pkg/operator/reconciler"
	"planetscale.dev/vitess-operator/pkg/operator/rollout"
	"planetscale.dev/vitess-operator/pkg/operator/update"
	"planetscale.dev/vitess-operator/pkg/operator/vitesscell"
)

func (r *ReconcileVitessCluster) reconcileCells(ctx context.Context, vt *planetscalev2.VitessCluster) error {
	labels := map[string]string{
		planetscalev2.ClusterLabel: vt.Name,
	}

	// Generate keys (object names) for all desired cells.
	// Keep a map back from generated names to the cell specs.
	keys := make([]client.ObjectKey, 0, len(vt.Spec.Cells))
	cellMap := make(map[client.ObjectKey]*planetscalev2.VitessCellTemplate, len(vt.Spec.Cells))
	for i := range vt.Spec.Cells {
		cell := &vt.Spec.Cells[i]
		key := client.ObjectKey{Namespace: vt.Namespace, Name: vitesscell.Name(vt.Name, cell.Name)}
		keys = append(keys, key)
		cellMap[key] = cell

		// Initialize a status entry for every desired cell, so it will be
		// listed even if we end up not having anything to report about it.
		vt.Status.Cells[cell.Name] = planetscalev2.NewVitessClusterCellStatus()
	}

	return r.reconciler.ReconcileObjectSet(ctx, vt, keys, labels, reconciler.Strategy{
		Kind: &planetscalev2.VitessCell{},

		New: func(key client.ObjectKey) runtime.Object {
			return newVitessCell(key, vt, labels, cellMap[key])
		},
		UpdateInPlace: func(key client.ObjectKey, obj runtime.Object) {
			newObj := obj.(*planetscalev2.VitessCell)
			if *vt.Spec.UpdateStrategy.Type == planetscalev2.ImmediateVitessClusterUpdateStrategyType {
				updateVitessCell(key, newObj, vt, labels, cellMap[key])
				return
			}
			updateVitessCellInPlace(key, newObj, vt, labels, cellMap[key])
		},
		UpdateRollingInPlace: func(key client.ObjectKey, obj runtime.Object) {
			newObj := obj.(*planetscalev2.VitessCell)
			if *vt.Spec.UpdateStrategy.Type == planetscalev2.ImmediateVitessClusterUpdateStrategyType {
				// In this case we should use UpdateInPlace for all updates.
				return
			}
			updateVitessCell(key, newObj, vt, labels, cellMap[key])
		},
		Status: func(key client.ObjectKey, obj runtime.Object) {
			curObj := obj.(*planetscalev2.VitessCell)

			status := vt.Status.Cells[curObj.Spec.Name]
			status.PendingChanges = curObj.Annotations[rollout.ScheduledAnnotation]
			status.GatewayAvailable = curObj.Status.Gateway.Available
			vt.Status.Cells[curObj.Spec.Name] = status
		},
		OrphanStatus: func(key client.ObjectKey, obj runtime.Object, orphanStatus *planetscalev2.OrphanStatus) {
			curObj := obj.(*planetscalev2.VitessCell)
			vt.Status.OrphanedCells[curObj.Spec.Name] = *orphanStatus
		},
		PrepareForTurndown: func(key client.ObjectKey, obj runtime.Object) *planetscalev2.OrphanStatus {
			// Make sure it's ok to delete this cell.
			// We err on the safe side since losing a cell accidentally is very disruptive.
			curObj := obj.(*planetscalev2.VitessCell)
			if curObj.Status.Idle == corev1.ConditionTrue {
				// The cell has no keyspaces deployed in it.
				return nil
			}
			// The cell is either not idle (Idle=False),
			// or we can't be sure whether it's idle (Idle=Unknown).
			return planetscalev2.NewOrphanStatus("NotIdle", "The cell can't be turned down because it's not idle. You must remove all keyspaces from the cell first before removing the cell.")
		},
	})
}

// newVitessCell expands a complete VitessCell from a VitessCellTemplate.
//
// A VitessCell consists of both user-configured parts, which come from VitessCellTemplate,
// plus auto-filled data that we propagate into each VitessCell from here.
// This allows VitessCell to do its job without looking at any other objects,
// and also lets us control when global changes roll out to each cell.
func newVitessCell(key client.ObjectKey, vt *planetscalev2.VitessCluster, parentLabels map[string]string, cell *planetscalev2.VitessCellTemplate) *planetscalev2.VitessCell {
	template := cell.DeepCopy()

	images := planetscalev2.VitessCellImages{}
	planetscalev2.DefaultVitessCellImages(&images, &vt.Spec.Images)

	// Tell each cell what other cells there are.
	allCells := make([]string, 0, len(vt.Spec.Cells))
	for i := range vt.Spec.Cells {
		allCells = append(allCells, vt.Spec.Cells[i].Name)
	}

	// Copy parent labels map and add cell-specific label.
	labels := make(map[string]string, len(parentLabels)+1)
	for k, v := range parentLabels {
		labels[k] = v
	}
	labels[planetscalev2.CellLabel] = cell.Name

	return &planetscalev2.VitessCell{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: key.Namespace,
			Name:      key.Name,
			Labels:    labels,
		},
		Spec: planetscalev2.VitessCellSpec{
			VitessCellTemplate:     *template,
			GlobalLockserver:       *lockserver.GlobalConnectionParams(&vt.Spec.GlobalLockserver, vt.Namespace, vt.Name),
			AllCells:               allCells,
			Images:                 images,
			ImagePullPolicies:      vt.Spec.ImagePullPolicies,
			ImagePullSecrets:       vt.Spec.ImagePullSecrets,
			ExtraVitessFlags:       vt.Spec.ExtraVitessFlags,
			TopologyReconciliation: vt.Spec.TopologyReconciliation,
		},
	}
}

func updateVitessCellInPlace(key client.ObjectKey, vtc *planetscalev2.VitessCell, vt *planetscalev2.VitessCluster, parentLabels map[string]string, cell *planetscalev2.VitessCellTemplate) {
	// Only update things that we don't need to roll out slowly.
	newCell := newVitessCell(key, vt, parentLabels, cell)

	// Update labels, but ignore existing ones we don't set.
	update.Labels(&vtc.Labels, newCell.Labels)

	// Only update replicas if autoscaling is disabled.
	if vtc.Spec.Gateway.Autoscaler != nil && vtc.Spec.Gateway.Autoscaler.MaxReplicas != nil {
		// We allow immediate update of replica counts for stateless workloads,
		// like Deployment does.
		vtc.Spec.Gateway.Replicas = newCell.Spec.Gateway.Replicas
	}
}

func updateVitessCell(key client.ObjectKey, vtc *planetscalev2.VitessCell, vt *planetscalev2.VitessCluster, parentLabels map[string]string, cell *planetscalev2.VitessCellTemplate) {
	newCell := newVitessCell(key, vt, parentLabels, cell)

	// Update labels, but ignore existing ones we don't set.
	update.Labels(&vtc.Labels, newCell.Labels)

	// For now, everything in Spec is safe to update.
	vtc.Spec = newCell.Spec
}
