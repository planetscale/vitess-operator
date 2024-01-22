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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apitypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/conditions"
	"planetscale.dev/vitess-operator/pkg/operator/reconciler"
	"planetscale.dev/vitess-operator/pkg/operator/results"
	"planetscale.dev/vitess-operator/pkg/operator/secrets"
	"planetscale.dev/vitess-operator/pkg/operator/update"
	"planetscale.dev/vitess-operator/pkg/operator/vtgate"
)

type secretCellsMapper struct {
	client client.Client
}

// Map maps a Secret to a list of requests for VitessCells
// that reference the secret.
func (m *secretCellsMapper) Map(ctx context.Context, obj client.Object) []reconcile.Request {
	secret := obj.(*corev1.Secret)
	secretName := secret.Name

	cellList := &planetscalev2.VitessCellList{}
	opts := &client.ListOptions{
		Namespace: secret.Namespace,
	}
	err := m.client.List(ctx, cellList, opts)
	if err != nil {
		log.WithError(err).Error("failed to list VitessCells; unable to map Secrets to matching VitessCells")
		return nil
	}

	// Request reconciliation for all the VitessCells to which this VitessKeyspace is deployed.
	var requests []reconcile.Request
	for i := range cellList.Items {
		cell := &cellList.Items[i]
		if cell.Spec.Gateway.ReloadSecretNames().Has(secretName) {
			requests = append(requests, reconcile.Request{
				NamespacedName: apitypes.NamespacedName{
					Namespace: cell.Namespace,
					Name:      cell.Name,
				},
			})
		}
	}
	return requests
}

func (r *ReconcileVitessCell) reconcileVtgate(ctx context.Context, vtc *planetscalev2.VitessCell, mysqldImage string) (reconcile.Result, error) {
	clusterName := vtc.Labels[planetscalev2.ClusterLabel]

	key := client.ObjectKey{Namespace: vtc.Namespace, Name: vtgate.ServiceName(clusterName, vtc.Spec.Name)}
	labels := map[string]string{
		planetscalev2.ClusterLabel:   clusterName,
		planetscalev2.CellLabel:      vtc.Spec.Name,
		planetscalev2.ComponentLabel: planetscalev2.VtgateComponentName,
	}
	resultBuilder := results.Builder{}

	// Reconcile vtgate Service.
	err := r.reconciler.ReconcileObject(ctx, vtc, key, labels, true, reconciler.Strategy{
		Kind: &corev1.Service{},

		New: func(key client.ObjectKey) runtime.Object {
			svc := vtgate.NewService(key, labels)
			update.ServiceOverrides(svc, vtc.Spec.Gateway.Service)
			return svc
		},
		UpdateInPlace: func(key client.ObjectKey, obj runtime.Object) {
			svc := obj.(*corev1.Service)
			vtgate.UpdateService(svc, labels)
			update.InPlaceServiceOverrides(svc, vtc.Spec.Gateway.Service)
		},
		Status: func(key client.ObjectKey, obj runtime.Object) {
			svc := obj.(*corev1.Service)
			vtc.Status.Gateway.ServiceName = svc.Name
		},
	})
	if err != nil {
		// Record error but continue.
		resultBuilder.Error(err)
	}

	reloadSecretNames := vtc.Spec.Gateway.ReloadSecretNames()
	gatewaySecrets, err := secrets.GetByNames(ctx, r.client, vtc.Namespace, reloadSecretNames)
	if err != nil {
		// Record error and return, to avoid generating a Deployment based on incomplete information.
		return resultBuilder.Error(err)
	}

	annotations := map[string]string{
		"planetscale.com/secret-hash": secrets.ContentHash(gatewaySecrets...),
	}
	update.Annotations(&annotations, vtc.Spec.Gateway.Annotations)

	// Merge ExtraVitessFlags and ExtraFlags together into a new map.
	extraFlags := make(map[string]string)
	update.StringMap(&extraFlags, vtc.Spec.ExtraVitessFlags)
	update.StringMap(&extraFlags, vtc.Spec.Gateway.ExtraFlags)

	// Reconcile vtgate Deployment.
	spec := &vtgate.Spec{
		Cell:                          &vtc.Spec,
		Labels:                        labels,
		Replicas:                      *vtc.Spec.Gateway.Replicas,
		Resources:                     vtc.Spec.Gateway.Resources,
		Authentication:                &vtc.Spec.Gateway.Authentication,
		SecureTransport:               vtc.Spec.Gateway.SecureTransport,
		Affinity:                      vtc.Spec.Gateway.Affinity,
		ExtraFlags:                    extraFlags,
		ExtraEnv:                      vtc.Spec.Gateway.ExtraEnv,
		ExtraVolumes:                  vtc.Spec.Gateway.ExtraVolumes,
		ExtraVolumeMounts:             vtc.Spec.Gateway.ExtraVolumeMounts,
		InitContainers:                vtc.Spec.Gateway.InitContainers,
		SidecarContainers:             vtc.Spec.Gateway.SidecarContainers,
		Annotations:                   annotations,
		ExtraLabels:                   vtc.Spec.Gateway.ExtraLabels,
		Tolerations:                   vtc.Spec.Gateway.Tolerations,
		TopologySpreadConstraints:     vtc.Spec.Gateway.TopologySpreadConstraints,
		Lifecycle:                     vtc.Spec.Gateway.Lifecycle,
		TerminationGracePeriodSeconds: vtc.Spec.Gateway.TerminationGracePeriodSeconds,
	}
	key = client.ObjectKey{Namespace: vtc.Namespace, Name: vtgate.DeploymentName(clusterName, vtc.Spec.Name)}

	err = r.reconciler.ReconcileObject(ctx, vtc, key, labels, true, reconciler.Strategy{
		Kind: &appsv1.Deployment{},

		New: func(key client.ObjectKey) runtime.Object {
			return vtgate.NewDeployment(key, spec, mysqldImage)
		},
		UpdateInPlace: func(key client.ObjectKey, obj runtime.Object) {
			newObj := obj.(*appsv1.Deployment)
			vtgate.UpdateDeployment(newObj, spec, mysqldImage)
		},
		Status: func(key client.ObjectKey, obj runtime.Object) {
			curObj := obj.(*appsv1.Deployment)

			status := &vtc.Status.Gateway
			if available := conditions.Deployment(curObj.Status.Conditions, appsv1.DeploymentAvailable); available != nil {
				status.Available = available.Status
			}
		},
	})
	if err != nil {
		resultBuilder.Error(err)
	}

	return resultBuilder.Result()
}
