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

	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/etcd"
	"planetscale.dev/vitess-operator/pkg/operator/reconciler"
	"planetscale.dev/vitess-operator/pkg/operator/results"
)

func (r *ReconcileEtcdLockserver) reconcilePodDisruptionBudget(ctx context.Context, ls *planetscalev2.EtcdLockserver) (reconcile.Result, error) {
	resultBuilder := &results.Builder{}

	if !*ls.Spec.CreatePDB {
		return resultBuilder.Result()
	}

	lockserverName := ls.Name

	labels := map[string]string{
		etcd.LockserverLabel: lockserverName,
	}

	// Reconcile the PDB.
	// This tells `kubectl drain` not to delete a Pod if it would take the etcd cluster below quorum.
	key := client.ObjectKey{
		Namespace: ls.Namespace,
		Name:      etcd.PDBName(lockserverName),
	}
	err := r.reconciler.ReconcileObject(ctx, ls, key, labels, true, reconciler.Strategy{
		Kind: &policyv1.PodDisruptionBudget{},

		New: func(key client.ObjectKey) runtime.Object {
			return etcd.NewPDB(key, labels)
		},
		UpdateInPlace: func(key client.ObjectKey, obj runtime.Object) {
			curObj := obj.(*policyv1.PodDisruptionBudget)
			etcd.UpdatePDBInPlace(curObj, labels)
		},
	})
	if err != nil {
		resultBuilder.Error(err)
	}

	return resultBuilder.Result()
}
