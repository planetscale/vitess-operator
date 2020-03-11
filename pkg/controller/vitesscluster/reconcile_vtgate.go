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

package vitesscluster

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/reconciler"
	"planetscale.dev/vitess-operator/pkg/operator/results"
	"planetscale.dev/vitess-operator/pkg/operator/vtgate"
)

func (r *ReconcileVitessCluster) reconcileVtgate(ctx context.Context, vt *planetscalev2.VitessCluster) (reconcile.Result, error) {
	key := client.ObjectKey{Namespace: vt.Namespace, Name: vtgate.ClusterServiceName(vt.Name)}
	labels := map[string]string{
		planetscalev2.ClusterLabel:   vt.Name,
		planetscalev2.ComponentLabel: planetscalev2.VtgateComponentName,
	}
	resultBuilder := results.Builder{}

	// Reconcile vtgate Service.
	err := r.reconciler.ReconcileObject(ctx, vt, key, labels, true, reconciler.Strategy{
		Kind: &corev1.Service{},

		New: func(key client.ObjectKey) runtime.Object {
			return vtgate.NewService(key, labels)
		},
		UpdateInPlace: func(key client.ObjectKey, obj runtime.Object) {
			newObj := obj.(*corev1.Service)
			vtgate.UpdateService(newObj, labels)
		},
		Status: func(key client.ObjectKey, obj runtime.Object) {
			curObj := obj.(*corev1.Service)
			vt.Status.GatewayServiceName = curObj.Name
		},
	})
	if err != nil {
		// Record error but continue.
		resultBuilder.Error(err)
	}

	return resultBuilder.Result()
}
