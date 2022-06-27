/*
Copyright 2022 PlanetScale Inc.

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
	"fmt"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/conditions"
	"planetscale.dev/vitess-operator/pkg/operator/reconciler"
	"planetscale.dev/vitess-operator/pkg/operator/results"
	"planetscale.dev/vitess-operator/pkg/operator/update"
	"planetscale.dev/vitess-operator/pkg/operator/vitesscell"
	"planetscale.dev/vitess-operator/pkg/operator/vtadmin"
	"planetscale.dev/vitess-operator/pkg/operator/vtctld"
	"planetscale.dev/vitess-operator/pkg/operator/vtgate"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (r *ReconcileVitessCluster) reconcileVtadmin(ctx context.Context, vt *planetscalev2.VitessCluster) (reconcile.Result, error) {
	resultBuilder := results.Builder{}
	// Do not deploy vtadmin if not configured
	if vt.Spec.VtAdmin == nil {
		return resultBuilder.Result()
	}

	// Some checks to validate user input
	if len(vt.Spec.Images.Vtadmin) == 0 {
		log.Error("Not deploying vtadmin since image is unspecified")
		return resultBuilder.Result()
	}

	if len(vt.Spec.VtAdmin.APIAddresses) == 0 {
		log.Errorf("Not deploying vtadmin since api addresses field is not specified. Atleast 1 value is required")
	}

	if len(vt.Spec.VtAdmin.APIAddresses) != 1 && len(vt.Spec.VtAdmin.APIAddresses) != len(vt.Spec.VtAdmin.Cells) {
		log.Errorf("Not deploying vtadmin since api addresses field doesn't align with cells field")
	}

	key := client.ObjectKey{Namespace: vt.Namespace, Name: vtadmin.ServiceName(vt.Name)}
	labels := map[string]string{
		planetscalev2.ClusterLabel:   vt.Name,
		planetscalev2.ComponentLabel: planetscalev2.VtadminComponentName,
	}

	// Reconcile vtadmin Service.
	err := r.reconciler.ReconcileObject(ctx, vt, key, labels, true, reconciler.Strategy{
		Kind: &corev1.Service{},

		New: func(key client.ObjectKey) runtime.Object {
			svc := vtadmin.NewService(key, labels)
			update.ServiceOverrides(svc, vt.Spec.VtAdmin.Service)
			return svc
		},
		UpdateInPlace: func(key client.ObjectKey, obj runtime.Object) {
			svc := obj.(*corev1.Service)
			vtadmin.UpdateService(svc, labels)
			update.InPlaceServiceOverrides(svc, vt.Spec.VtAdmin.Service)
		},
		Status: func(key client.ObjectKey, obj runtime.Object) {
			svc := obj.(*corev1.Service)
			vt.Status.Vtadmin.ServiceName = svc.Name
		},
	})
	if err != nil {
		// Record error but continue.
		resultBuilder.Error(err)
	}

	// Reconcile vtadmin Deployments.
	specs, err := r.vtadminSpecs(ctx, vt, labels)
	if err != nil {
		// Record error and stop.
		resultBuilder.Error(err)
		return resultBuilder.Result()
	}

	// Generate keys (object names) for all desired vtadmin Deployments.
	// Keep a map back from generated names to the vtadmin specs.
	keys := make([]client.ObjectKey, 0, len(specs))
	specMap := make(map[client.ObjectKey]*vtadmin.Spec, len(specs))
	for _, spec := range specs {
		key := client.ObjectKey{Namespace: vt.Namespace, Name: vtadmin.DeploymentName(vt.Name, spec.Cell.Name)}
		keys = append(keys, key)
		specMap[key] = spec
	}

	err = r.reconciler.ReconcileObjectSet(ctx, vt, keys, labels, reconciler.Strategy{
		Kind: &appsv1.Deployment{},

		New: func(key client.ObjectKey) runtime.Object {
			return vtadmin.NewDeployment(key, specMap[key])
		},
		UpdateInPlace: func(key client.ObjectKey, obj runtime.Object) {
			newObj := obj.(*appsv1.Deployment)
			if *vt.Spec.UpdateStrategy.Type == planetscalev2.ImmediateVitessClusterUpdateStrategyType {
				vtadmin.UpdateDeployment(newObj, specMap[key])
				return
			}
			vtadmin.UpdateDeploymentImmediate(newObj, specMap[key])
		},
		UpdateRollingInPlace: func(key client.ObjectKey, obj runtime.Object) {
			newObj := obj.(*appsv1.Deployment)
			vtadmin.UpdateDeployment(newObj, specMap[key])
		},
		Status: func(key client.ObjectKey, obj runtime.Object) {
			// This function will get called once for each Deployment.
			// Aggregate as we go to build an overall status for vtadmin.
			curObj := obj.(*appsv1.Deployment)

			// We'll say vtadmin is Available overall if any of the Deployments is available.
			// The important thing is that somebody will answer when a client hits the Service.
			if available := conditions.Deployment(curObj.Status.Conditions, appsv1.DeploymentAvailable); available != nil {
				// Update the overall status if either we found one that's True, or we previously knew nothing at all (Unknown).
				if available.Status == corev1.ConditionTrue || vt.Status.Vtadmin.Available == corev1.ConditionUnknown {
					vt.Status.Vtadmin.Available = available.Status
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

func (r *ReconcileVitessCluster) vtadminSpecs(ctx context.Context, vt *planetscalev2.VitessCluster, parentLabels map[string]string) ([]*vtadmin.Spec, error) {
	var cells []*planetscalev2.VitessCellTemplate
	if len(vt.Spec.VtAdmin.Cells) != 0 {
		// Deploy only to the specified cells.
		for _, cellName := range vt.Spec.VtAdmin.Cells {
			cell := vt.Spec.Cell(cellName)
			if cell == nil {
				r.recorder.Eventf(vt, corev1.EventTypeWarning, "InvalidSpec", "ignoring non-existent cell %q in spec.vtadmin.cells", cellName)
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

	// Make a vtadmin Deployment spec for each cell.
	specs := make([]*vtadmin.Spec, 0, len(cells))
	for idx, cell := range cells {
		// Copy parent labels map and add cell-specific label.
		labels := make(map[string]string, len(parentLabels)+1)
		for k, v := range parentLabels {
			labels[k] = v
		}
		labels[planetscalev2.CellLabel] = cell.Name

		// Merge ExtraFlags into a new map.
		extraFlags := make(map[string]string)
		update.StringMap(&extraFlags, vt.Spec.VtAdmin.ExtraFlags)

		discoverySecret, err := r.createDiscoverySecret(ctx, vt, cell)
		if err != nil {
			return nil, err
		}

		// We have already checked that atleast 1 value should be available in APIAddresses
		apiAddress := vt.Spec.VtAdmin.APIAddresses[0]
		if len(vt.Spec.VtAdmin.APIAddresses) > 1 {
			apiAddress = vt.Spec.VtAdmin.APIAddresses[idx]
		}

		webConfigSecret, err := r.createWebConfigSecret(ctx, vt, cell, apiAddress)
		if err != nil {
			return nil, err
		}

		specs = append(specs, &vtadmin.Spec{
			Cell:              cell,
			Discovery:         discoverySecret,
			Rbac:              vt.Spec.VtAdmin.Rbac,
			WebConfig:         webConfigSecret,
			Image:             vt.Spec.Images.Vtadmin,
			ClusterName:       vt.ObjectMeta.Name,
			ImagePullPolicy:   vt.Spec.ImagePullPolicies.Vtadmin,
			ImagePullSecrets:  vt.Spec.ImagePullSecrets,
			Labels:            labels,
			Replicas:          *vt.Spec.VtAdmin.Replicas,
			APIResources:      vt.Spec.VtAdmin.APIResources,
			WebResources:      vt.Spec.VtAdmin.WebResources,
			Affinity:          vt.Spec.VtAdmin.Affinity,
			ExtraFlags:        extraFlags,
			ExtraEnv:          vt.Spec.VtAdmin.ExtraEnv,
			ExtraVolumes:      vt.Spec.VtAdmin.ExtraVolumes,
			ExtraVolumeMounts: vt.Spec.VtAdmin.ExtraVolumeMounts,
			InitContainers:    vt.Spec.VtAdmin.InitContainers,
			SidecarContainers: vt.Spec.VtAdmin.SidecarContainers,
			Annotations:       vt.Spec.VtAdmin.Annotations,
			ExtraLabels:       vt.Spec.VtAdmin.ExtraLabels,
			Tolerations:       vt.Spec.VtAdmin.Tolerations,
		})
	}
	return specs, nil
}

func (r *ReconcileVitessCluster) createDiscoverySecret(ctx context.Context, vt *planetscalev2.VitessCluster, cell *planetscalev2.VitessCellTemplate) (*planetscalev2.SecretSource, error) {
	// Get the vtctld service
	vtctldService := corev1.Service{}
	err := r.client.Get(ctx, client.ObjectKey{
		Namespace: vt.Namespace,
		Name:      vtctld.ServiceName(vt.Name),
	}, &vtctldService)
	if err != nil {
		return nil, err
	}

	// Find the IP address from the service. This is randomly assigned.
	vtctldServiceIP := vtctldService.Spec.ClusterIP
	// The web and grpc ports should be set to the default values planetscalev2.DefaultWebPort and planetscalev2.DefaultGrpcPort
	// respectively, but since we have the service, we can just read them.
	var vtctldServiceWebPort, vtctldServiceGrpcPort int32
	for _, port := range vtctldService.Spec.Ports {
		if port.Name == planetscalev2.DefaultWebPortName {
			vtctldServiceWebPort = port.Port
		}
		if port.Name == planetscalev2.DefaultGrpcPortName {
			vtctldServiceGrpcPort = port.Port
		}
	}

	// Read the cell information
	vtc := planetscalev2.VitessCell{}
	err = r.client.Get(ctx, client.ObjectKey{
		Namespace: vt.Namespace,
		Name:      vitesscell.Name(vt.Name, cell.Name),
	}, &vtc)
	if err != nil {
		return nil, err
	}

	// Get the vtgate service from the cell
	vtgateService := corev1.Service{}
	err = r.client.Get(ctx, client.ObjectKey{
		Namespace: vtc.Namespace,
		Name:      vtgate.ServiceName(vt.Name, cell.Name),
	}, &vtgateService)
	if err != nil {
		return nil, err
	}

	// Find the IP address from the service. This is randomly assigned.
	vtgateServiceIP := vtgateService.Spec.ClusterIP
	// The grpc port should be set to the default value planetscalev2.DefaultGrpcPort,
	// but since we have the service, we can just read it.
	var vtgateServiceGrpcPort int32
	for _, port := range vtgateService.Spec.Ports {
		if port.Name == planetscalev2.DefaultGrpcPortName {
			vtgateServiceGrpcPort = port.Port
		}
	}

	// Variables to hold the key, value and secret name to use
	discoveryKey := "discovery.json"
	discoveryVal := fmt.Sprintf(`{
    "vtctlds": [
        {
            "host": {
                "fqdn": "%s:%d",
                "hostname": "%s:%d"
            }
        }
    ],
    "vtgates": [
        {
            "host": {
                "hostname": "%s:%d"
            }
        }
    ]
}`, vtctldServiceIP, vtctldServiceWebPort, vtctldServiceIP, vtctldServiceGrpcPort, vtgateServiceIP, vtgateServiceGrpcPort)
	secretName := vtadmin.DiscoverySecretName(vt.Name, cell.Name)

	// Create or update the secret
	err = r.createOrUpdateSecret(ctx, vt, secretName, discoveryKey, discoveryVal)
	if err != nil {
		return nil, err
	}

	// return the secret source, which must align with the secret we created above
	return &planetscalev2.SecretSource{
		Name: secretName,
		Key:  discoveryKey,
	}, nil
}

func (r *ReconcileVitessCluster) createWebConfigSecret(ctx context.Context, vt *planetscalev2.VitessCluster, cell *planetscalev2.VitessCellTemplate, apiAddress string) (*planetscalev2.SecretSource, error) {
	// Variables to hold the key, value and secret name to use
	configKey := vtadmin.WebConfigFileName
	configVal := fmt.Sprintf(`window.env = {
     'REACT_APP_VTADMIN_API_ADDRESS': "%s",
     'REACT_APP_FETCH_CREDENTIALS': "omit",
     'REACT_APP_ENABLE_EXPERIMENTAL_TABLET_DEBUG_VARS': false,
     'REACT_APP_BUGSNAG_API_KEY': "",
     'REACT_APP_DOCUMENT_TITLE': "",
     'REACT_APP_READONLY_MODE': %s,
 };`, apiAddress, convertReadOnlyFieldToString(vt.Spec.VtAdmin.ReadOnly))
	secretName := vtadmin.WebConfigSecretName(vt.Name, cell.Name)

	// Create or update the secret
	err := r.createOrUpdateSecret(ctx, vt, secretName, configKey, configVal)
	if err != nil {
		return nil, err
	}

	// return the secret source, which must align with the secret we created above
	return &planetscalev2.SecretSource{
		Name: secretName,
		Key:  configKey,
	}, nil
}

func convertReadOnlyFieldToString(readOnly *bool) string {
	if readOnly != nil && *readOnly {
		return "true"
	}
	return "false"
}

func (r *ReconcileVitessCluster) createOrUpdateSecret(ctx context.Context, vt *planetscalev2.VitessCluster, secretName, discoveryKey, discoveryVal string) error {
	desiredSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: vt.Namespace,
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			discoveryKey: discoveryVal,
		},
	}

	secret := corev1.Secret{}
	err := r.client.Get(ctx, client.ObjectKey{
		Name:      secretName,
		Namespace: vt.Namespace,
	}, &secret)
	if err != nil {
		// Create the secret
		return r.client.Create(ctx, desiredSecret)
	}
	// Update the secret
	return r.client.Update(ctx, desiredSecret)
}
