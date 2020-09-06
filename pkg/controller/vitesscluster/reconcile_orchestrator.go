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

	"planetscale.dev/vitess-operator/pkg/operator/update"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/conditions"
	"planetscale.dev/vitess-operator/pkg/operator/lockserver"
	"planetscale.dev/vitess-operator/pkg/operator/orchestrator"
	"planetscale.dev/vitess-operator/pkg/operator/reconciler"
	"planetscale.dev/vitess-operator/pkg/operator/results"
)

func (r *ReconcileVitessCluster) reconcileOrchestrator(ctx context.Context, vt *planetscalev2.VitessCluster) (reconcile.Result, error) {
	key := client.ObjectKey{Namespace: vt.Namespace, Name: orchestrator.ServiceName(vt.Name)}
	labels := map[string]string{
		planetscalev2.ClusterLabel:   vt.Name,
		planetscalev2.ComponentLabel: planetscalev2.OrcComponentName,
	}
	resultBuilder := results.Builder{}

	// Reconcile orchestrator Service.
	err := r.reconciler.ReconcileObject(ctx, vt, key, labels, true, reconciler.Strategy{
		Kind: &corev1.Service{},

		New: func(key client.ObjectKey) runtime.Object {
			svc := orchestrator.NewService(key, labels)
			update.ServiceOverrides(svc, vt.Spec.VitessOrchestrator.Service)
			return svc
		},
		UpdateInPlace: func(key client.ObjectKey, obj runtime.Object) {
			svc := obj.(*corev1.Service)
			orchestrator.UpdateService(svc, labels)
			update.InPlaceServiceOverrides(svc, vt.Spec.VitessOrchestrator.Service)
		},
		Status: func(key client.ObjectKey, obj runtime.Object) {
			svc := obj.(*corev1.Service)
			vt.Status.VitessOrchestrator.ServiceName = svc.Name
		},
	})
	if err != nil {
		// Record error but continue.
		resultBuilder.Error(err)
	}

	// Reconcile orchestrator Deployments.
	specs := r.orchestratorSpecs(vt, labels)

	// Generate keys (object names) for all desired orchestrator Deployments.
	// Keep a map back from generated names to the orchestrator specs.
	keys := make([]client.ObjectKey, 0, len(specs))
	specMap := make(map[client.ObjectKey]*orchestrator.Spec, len(specs))
	for _, spec := range specs {
		key := client.ObjectKey{Namespace: vt.Namespace, Name: orchestrator.DeploymentName(vt.Name, spec.Cell.Name)}
		keys = append(keys, key)
		specMap[key] = spec
	}

	err = r.reconciler.ReconcileObjectSet(ctx, vt, keys, labels, reconciler.Strategy{
		Kind: &appsv1.Deployment{},

		New: func(key client.ObjectKey) runtime.Object {
			return orchestrator.NewDeployment(key, specMap[key])
		},
		UpdateInPlace: func(key client.ObjectKey, obj runtime.Object) {
			newObj := obj.(*appsv1.Deployment)
			if *vt.Spec.UpdateStrategy.Type == planetscalev2.ImmediateVitessClusterUpdateStrategyType {
				orchestrator.UpdateDeployment(newObj, specMap[key])
				return
			}
			orchestrator.UpdateDeploymentImmediate(newObj, specMap[key])
		},
		UpdateRollingInPlace: func(key client.ObjectKey, obj runtime.Object) {
			newObj := obj.(*appsv1.Deployment)
			orchestrator.UpdateDeployment(newObj, specMap[key])
		},
		Status: func(key client.ObjectKey, obj runtime.Object) {
			// This function will get called once for each Deployment.
			// Aggregate as we go to build an overall status for orchestrator.
			curObj := obj.(*appsv1.Deployment)

			// We'll say orchestrator is Available overall if any of the Deployments is available.
			// The important thing is that somebody will answer when a client hits the Service.
			if available := conditions.Deployment(curObj.Status.Conditions, appsv1.DeploymentAvailable); available != nil {
				// Update the overall status if either we found one that's True, or we previously knew nothing at all (Unknown).
				if available.Status == corev1.ConditionTrue || vt.Status.VitessOrchestrator.Available == corev1.ConditionUnknown {
					vt.Status.VitessOrchestrator.Available = available.Status
				}
			}

			// TODO(enisoc): Aggregate other important parts of status besides conditions.
		},
	})
	if err != nil {
		resultBuilder.Error(err)
	}

	return resultBuilder.Result()
}

func (r *ReconcileVitessCluster) orchestratorSpecs(vt *planetscalev2.VitessCluster, parentLabels map[string]string) []*orchestrator.Spec {
	var cells []*planetscalev2.VitessCellTemplate
	if len(vt.Spec.VitessOrchestrator.Cells) != 0 {
		// Deploy only to the specified cells.
		for _, cellName := range vt.Spec.VitessOrchestrator.Cells {
			cell := vt.Spec.Cell(cellName)
			if cell == nil {
				r.recorder.Eventf(vt, corev1.EventTypeWarning, "InvalidSpec", "ignoring non-existent cell %q in spec.orchestrator.cells", cellName)
				continue
			}
			cells = append(cells, cell)
		}
	} else {
		// Deploy to all cells.
		for i := range vt.Spec.Cells {
			cells = append(cells, &vt.Spec.Cells[i])
		}
	}

	glsParams := lockserver.GlobalConnectionParams(&vt.Spec.GlobalLockserver, vt.Name)

	// Make a orchestrator Deployment spec for each cell.
	specs := make([]*orchestrator.Spec, 0, len(cells))
	for _, cell := range cells {
		// Copy parent labels map and add cell-specific label.
		labels := make(map[string]string, len(parentLabels)+1)
		for k, v := range parentLabels {
			labels[k] = v
		}
		labels[planetscalev2.CellLabel] = cell.Name

		// Merge ExtraVitessFlags and ExtraFlags into a new map.
		extraFlags := make(map[string]string)
		update.StringMap(&extraFlags, vt.Spec.ExtraVitessFlags)
		update.StringMap(&extraFlags, vt.Spec.VitessOrchestrator.ExtraFlags)

		specs = append(specs, &orchestrator.Spec{
			GlobalLockserver:  glsParams,
			Image:             vt.Spec.Images.Orchestrator,
			ImagePullPolicy:   vt.Spec.ImagePullPolicies.Orchestrator,
			ImagePullSecrets:  vt.Spec.ImagePullSecrets,
			Labels:            labels,
			Replicas:          *vt.Spec.VitessOrchestrator.Replicas,
			Resources:         vt.Spec.VitessOrchestrator.Resources,
			Affinity:          vt.Spec.VitessOrchestrator.Affinity,
			ExtraFlags:        extraFlags,
			ExtraEnv:          vt.Spec.VitessOrchestrator.ExtraEnv,
			ExtraVolumes:      vt.Spec.VitessOrchestrator.ExtraVolumes,
			ExtraVolumeMounts: vt.Spec.VitessOrchestrator.ExtraVolumeMounts,
			InitContainers:    vt.Spec.VitessOrchestrator.InitContainers,
			SidecarContainers: vt.Spec.VitessOrchestrator.SidecarContainers,
			Annotations:       vt.Spec.VitessOrchestrator.Annotations,
			ExtraLabels:       vt.Spec.VitessOrchestrator.ExtraLabels,
		})
	}
	return specs
}
