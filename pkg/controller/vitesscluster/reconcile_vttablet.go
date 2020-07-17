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
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/reconciler"
	"planetscale.dev/vitess-operator/pkg/operator/results"
	"planetscale.dev/vitess-operator/pkg/operator/update"
	"planetscale.dev/vitess-operator/pkg/operator/vttablet"
)

func (r *ReconcileVitessCluster) reconcileVttablet(ctx context.Context, vt *planetscalev2.VitessCluster) (reconcile.Result, error) {
	resultBuilder := results.Builder{}

	key := client.ObjectKey{
		Namespace: vt.Namespace,
		Name:      vttablet.ServiceName(vt.Name),
	}
	labels := map[string]string{
		planetscalev2.ClusterLabel:   vt.Name,
		planetscalev2.ComponentLabel: planetscalev2.VttabletComponentName,
	}

	// Reconcile vttablet Service.
	err := r.reconciler.ReconcileObject(ctx, vt, key, labels, true, reconciler.Strategy{
		Kind: &corev1.Service{},

		New: func(key client.ObjectKey) runtime.Object {
			svc := vttablet.NewService(key, labels)
			update.ServiceOverrides(svc, vt.Spec.TabletService)
			return svc
		},
		UpdateInPlace: func(key client.ObjectKey, obj runtime.Object) {
			svc := obj.(*corev1.Service)
			vttablet.UpdateService(svc, labels)
			update.InPlaceServiceOverrides(svc, vt.Spec.TabletService)
		},
	})
	if err != nil {
		resultBuilder.Error(err)
	}

	return resultBuilder.Result()
}
