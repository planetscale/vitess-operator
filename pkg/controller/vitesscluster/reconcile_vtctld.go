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
	"planetscale.dev/vitess-operator/pkg/operator/reconciler"
	"planetscale.dev/vitess-operator/pkg/operator/results"
	"planetscale.dev/vitess-operator/pkg/operator/vtctld"
)

func (r *ReconcileVitessCluster) reconcileVtctld(ctx context.Context, vt *planetscalev2.VitessCluster) (reconcile.Result, error) {
	resultBuilder := results.Builder{}
	key := client.ObjectKey{Namespace: vt.Namespace, Name: vtctld.ServiceName(vt.Name)}
	labels := map[string]string{
		planetscalev2.ClusterLabel:   vt.Name,
		planetscalev2.ComponentLabel: planetscalev2.VtctldComponentName,
	}

	// Reconcile vtctld Service.
	err := r.reconciler.ReconcileObject(ctx, vt, key, labels, true, reconciler.Strategy{
		Kind: &corev1.Service{},

		New: func(key client.ObjectKey) runtime.Object {
			svc := vtctld.NewService(key, labels)
			update.ServiceOverrides(svc, vt.Spec.VitessDashboard.Service)
			return svc
		},
		UpdateInPlace: func(key client.ObjectKey, obj runtime.Object) {
			svc := obj.(*corev1.Service)
			vtctld.UpdateService(svc, labels)
			update.InPlaceServiceOverrides(svc, vt.Spec.VitessDashboard.Service)
		},
		Status: func(key client.ObjectKey, obj runtime.Object) {
			svc := obj.(*corev1.Service)
			vt.Status.VitessDashboard.ServiceName = svc.Name
		},
	})
	if err != nil {
		// Record error but continue.
		resultBuilder.Error(err)
	}

	// Reconcile vtctld Deployments.
	specs := r.vtctldSpecs(vt, labels)

	// Generate keys (object names) for all desired vtctld Deployments.
	// Keep a map back from generated names to the vtctld specs.
	keys := make([]client.ObjectKey, 0, len(specs))
	specMap := make(map[client.ObjectKey]*vtctld.Spec, len(specs))
	for _, spec := range specs {
		key := client.ObjectKey{Namespace: vt.Namespace, Name: vtctld.DeploymentName(vt.Name, spec.Cell.Name)}
		keys = append(keys, key)
		specMap[key] = spec
	}

	err = r.reconciler.ReconcileObjectSet(ctx, vt, keys, labels, reconciler.Strategy{
		Kind: &appsv1.Deployment{},

		New: func(key client.ObjectKey) runtime.Object {
			return vtctld.NewDeployment(key, specMap[key], vt.Spec.Images.Mysqld.Image())
		},
		UpdateInPlace: func(key client.ObjectKey, obj runtime.Object) {
			newObj := obj.(*appsv1.Deployment)
			if *vt.Spec.UpdateStrategy.Type == planetscalev2.ImmediateVitessClusterUpdateStrategyType {
				vtctld.UpdateDeployment(newObj, specMap[key], vt.Spec.Images.Mysqld.Image())
				return
			}
			vtctld.UpdateDeploymentImmediate(newObj, specMap[key])
		},
		UpdateRollingInPlace: func(key client.ObjectKey, obj runtime.Object) {
			newObj := obj.(*appsv1.Deployment)
			vtctld.UpdateDeployment(newObj, specMap[key], vt.Spec.Images.Mysqld.Image())
		},
		Status: func(key client.ObjectKey, obj runtime.Object) {
			// This function will get called once for each Deployment.
			// Aggregate as we go to build an overall status for vtctld.
			curObj := obj.(*appsv1.Deployment)

			// We'll say vtctld is Available overall if any of the Deployments is available.
			// The important thing is that somebody will answer when a client hits the Service.
			if available := conditions.Deployment(curObj.Status.Conditions, appsv1.DeploymentAvailable); available != nil {
				// Update the overall status if either we found one that's True, or we previously knew nothing at all (Unknown).
				if available.Status == corev1.ConditionTrue || vt.Status.VitessDashboard.Available == corev1.ConditionUnknown {
					vt.Status.VitessDashboard.Available = available.Status
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

func (r *ReconcileVitessCluster) vtctldSpecs(vt *planetscalev2.VitessCluster, parentLabels map[string]string) []*vtctld.Spec {
	var cells []*planetscalev2.VitessCellTemplate
	if len(vt.Spec.VitessDashboard.Cells) != 0 {
		// Deploy only to the specified cells.
		for _, cellName := range vt.Spec.VitessDashboard.Cells {
			cell := vt.Spec.Cell(cellName)
			if cell == nil {
				r.recorder.Eventf(vt, corev1.EventTypeWarning, "InvalidSpec", "ignoring non-existent cell %q in spec.vtctld.cells", cellName)
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

	glsParams := lockserver.GlobalConnectionParams(&vt.Spec.GlobalLockserver, vt.Namespace, vt.Name)

	// Make a vtctld Deployment spec for each cell.
	specs := make([]*vtctld.Spec, 0, len(cells))
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
		update.StringMap(&extraFlags, vt.Spec.VitessDashboard.ExtraFlags)

		var backupLocation *planetscalev2.VitessBackupLocation
		var backupEngine planetscalev2.VitessBackupEngine
		if vt.Spec.Backup != nil {
			// We don't know which location to use for vtctld.
			// If there are more than one locations configured, we can only choose one.
			// Here we opt to take the first one.
			if len(vt.Spec.Backup.Locations) != 0 {
				backupLocation = &vt.Spec.Backup.Locations[0]
			}
			backupEngine = vt.Spec.Backup.Engine
		}

		specs = append(specs, &vtctld.Spec{
			GlobalLockserver:  glsParams,
			Image:             vt.Spec.Images.Vtctld,
			ImagePullPolicy:   vt.Spec.ImagePullPolicies.Vtctld,
			ImagePullSecrets:  vt.Spec.ImagePullSecrets,
			Cell:              cell,
			Labels:            labels,
			Replicas:          *vt.Spec.VitessDashboard.Replicas,
			Resources:         vt.Spec.VitessDashboard.Resources,
			Affinity:          vt.Spec.VitessDashboard.Affinity,
			ExtraFlags:        extraFlags,
			ExtraEnv:          vt.Spec.VitessDashboard.ExtraEnv,
			ExtraVolumes:      vt.Spec.VitessDashboard.ExtraVolumes,
			ExtraVolumeMounts: vt.Spec.VitessDashboard.ExtraVolumeMounts,
			InitContainers:    vt.Spec.VitessDashboard.InitContainers,
			SidecarContainers: vt.Spec.VitessDashboard.SidecarContainers,
			Annotations:       vt.Spec.VitessDashboard.Annotations,
			ExtraLabels:       vt.Spec.VitessDashboard.ExtraLabels,
			Tolerations:       vt.Spec.VitessDashboard.Tolerations,
			BackupEngine:      backupEngine,
			BackupLocation:    backupLocation,
		})

	}
	return specs
}
