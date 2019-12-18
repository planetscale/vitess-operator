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

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/lockserver"
	"planetscale.dev/vitess-operator/pkg/operator/reconciler"
)

func (r *ReconcileVitessCluster) reconcileGlobalEtcd(ctx context.Context, vt *planetscalev2.VitessCluster) error {
	key := client.ObjectKey{Namespace: vt.Namespace, Name: lockserver.GlobalEtcdName(vt.Name)}
	labels := map[string]string{
		planetscalev2.ClusterLabel:   vt.Name,
		planetscalev2.ComponentLabel: planetscalev2.EtcdComponentName,
	}
	enabled := vt.Spec.GlobalLockserver.Etcd != nil

	// Initialize status only if etcd is enabled.
	if enabled {
		vt.Status.GlobalLockserver.Etcd = planetscalev2.NewEtcdLockserverStatus()
	}

	return r.reconciler.ReconcileObject(ctx, vt, key, labels, enabled, reconciler.Strategy{
		Kind: &planetscalev2.EtcdLockserver{},

		New: func(key client.ObjectKey) runtime.Object {
			return lockserver.NewEtcdLockserver(key, vt.Spec.GlobalLockserver.Etcd, labels, "")
		},
		UpdateInPlace: func(key client.ObjectKey, obj runtime.Object) {
			newObj := obj.(*planetscalev2.EtcdLockserver)
			lockserver.UpdateEtcdLockserver(newObj, vt.Spec.GlobalLockserver.Etcd, labels, "")
		},
		Status: func(key client.ObjectKey, obj runtime.Object) {
			curObj := obj.(*planetscalev2.EtcdLockserver)
			// Make a copy of status and erase things we don't care about.
			status := curObj.Status
			status.ObservedGeneration = 0
			vt.Status.GlobalLockserver.Etcd = &status
		},
		PrepareForTurndown: func(key client.ObjectKey, obj runtime.Object) *planetscalev2.OrphanStatus {
			// Make sure it's ok to delete this etcd cluster.
			// We err on the safe side since losing etcd can be very disruptive.
			// TODO(enisoc): Define some criteria for knowing it's safe to auto-delete etcd.
			//               For now, we require manual deletion.
			return planetscalev2.NewOrphanStatus("NotSupported", "Automatic turndown is not supported for etcd for safety reasons. The EtcdLockserver instance must be deleted manually.")
		},
	})
}
