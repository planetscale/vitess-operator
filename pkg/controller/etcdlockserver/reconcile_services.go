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
)

func (r *ReconcileEtcdLockserver) reconcileServices(ctx context.Context, ls *planetscalev2.EtcdLockserver) (reconcile.Result, error) {
	resultBuilder := &results.Builder{}
	lockserverName := ls.Name

	labels := map[string]string{
		etcd.LockserverLabel: lockserverName,
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
				return etcd.NewClientService(key, labels)
			},
			UpdateInPlace: func(key client.ObjectKey, obj runtime.Object) {
				curObj := obj.(*corev1.Service)
				etcd.UpdateClientService(curObj, labels)
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
				return etcd.NewPeerService(key, labels)
			},
			UpdateInPlace: func(key client.ObjectKey, obj runtime.Object) {
				curObj := obj.(*corev1.Service)
				etcd.UpdatePeerService(curObj, labels)
			},
		})
		if err != nil {
			resultBuilder.Error(err)
		}
	}

	return resultBuilder.Result()
}
