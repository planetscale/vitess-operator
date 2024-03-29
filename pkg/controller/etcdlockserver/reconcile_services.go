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

package etcdlockserver

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/etcd"
	"planetscale.dev/vitess-operator/pkg/operator/reconciler"
	"planetscale.dev/vitess-operator/pkg/operator/results"
	"planetscale.dev/vitess-operator/pkg/operator/update"
)

func (r *ReconcileEtcdLockserver) reconcileServices(ctx context.Context, ls *planetscalev2.EtcdLockserver) (reconcile.Result, error) {
	resultBuilder := &results.Builder{}
	lockserverName := ls.Name

	labels := map[string]string{
		etcd.LockserverLabel: lockserverName,
	}
	// Backward-compatible labels change
	// We shouldn't pass the extended labels to the reconciler directly, because it compares the labels,
	// but we can use the extended labels in the reconciler.Strategy for creation/update.
	extendedLabels := map[string]string{
		etcd.LockserverLabel:         lockserverName,
		planetscalev2.ComponentLabel: planetscalev2.EtcdComponentName,
	}
	if _, hasClusterLabel := ls.Labels[planetscalev2.ClusterLabel]; hasClusterLabel {
		extendedLabels[planetscalev2.ClusterLabel] = ls.Labels[planetscalev2.ClusterLabel]
	}

	// Reconcile the client Service.
	if *ls.Spec.CreateClientService {
		clientSvcKey := client.ObjectKey{
			Namespace: ls.Namespace,
			Name:      etcd.ClientServiceName(lockserverName),
		}
		err := r.reconciler.ReconcileObject(ctx, ls, clientSvcKey, labels, true, reconciler.Strategy{
			Kind: &corev1.Service{},

			New: func(key client.ObjectKey) runtime.Object {
				svc := etcd.NewClientService(key, extendedLabels)
				update.ServiceOverrides(svc, ls.Spec.ClientService)
				return svc
			},
			UpdateInPlace: func(key client.ObjectKey, obj runtime.Object) {
				svc := obj.(*corev1.Service)
				etcd.UpdateClientService(svc, extendedLabels)
				update.InPlaceServiceOverrides(svc, ls.Spec.ClientService)
			},
		})
		if err != nil {
			resultBuilder.Error(err)
		}
	}

	// Reconcile the peer Service.
	if *ls.Spec.CreatePeerService {
		peerSvcKey := client.ObjectKey{
			Namespace: ls.Namespace,
			Name:      etcd.PeerServiceName(lockserverName),
		}
		err := r.reconciler.ReconcileObject(ctx, ls, peerSvcKey, labels, true, reconciler.Strategy{
			Kind: &corev1.Service{},

			New: func(key client.ObjectKey) runtime.Object {
				svc := etcd.NewPeerService(key, extendedLabels)
				update.ServiceOverrides(svc, ls.Spec.PeerService)
				return svc
			},
			UpdateInPlace: func(key client.ObjectKey, obj runtime.Object) {
				svc := obj.(*corev1.Service)
				etcd.UpdatePeerService(svc, extendedLabels)
				update.InPlaceServiceOverrides(svc, ls.Spec.PeerService)
			},
		})
		if err != nil {
			resultBuilder.Error(err)
		}
	}

	return resultBuilder.Result()
}
