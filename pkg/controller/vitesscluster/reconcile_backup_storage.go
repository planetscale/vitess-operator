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

	"planetscale.dev/vitess-operator/pkg/operator/update"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/reconciler"
	"planetscale.dev/vitess-operator/pkg/operator/vitessbackup"
)

func (r *ReconcileVitessCluster) reconcileBackupStorage(ctx context.Context, vt *planetscalev2.VitessCluster) error {
	labels := map[string]string{
		planetscalev2.ClusterLabel: vt.Name,
	}

	// Make a VitessBackupStorage object for each backup storage location.
	// Each VBS object will mirror metadata about Vitess backups from that
	// storage location (for any keyspace/shard in this cluster) into a set of
	// VitessBackup objects.
	keys := []client.ObjectKey{}
	vbsMap := map[client.ObjectKey]*planetscalev2.VitessBackupStorage{}

	if vt.Spec.Backup != nil {
		for i := range vt.Spec.Backup.Locations {
			location := &vt.Spec.Backup.Locations[i]
			key := client.ObjectKey{
				Namespace: vt.Namespace,
				Name:      vitessbackup.StorageObjectName(vt.Name, location.Name),
			}
			keys = append(keys, key)
			vbsMap[key] = newVitessBackupStorage(key, labels, location)
		}
	}

	return r.reconciler.ReconcileObjectSet(ctx, vt, keys, labels, reconciler.Strategy{
		Kind: &planetscalev2.VitessBackupStorage{},

		New: func(key client.ObjectKey) runtime.Object {
			return vbsMap[key]
		},
		UpdateInPlace: func(key client.ObjectKey, obj runtime.Object) {
			vbs := obj.(*planetscalev2.VitessBackupStorage)
			newObj := vbsMap[key]
			update.Labels(&vbs.Labels, newObj.Labels)
			vbs.Spec = newObj.Spec
		},
		Status: func(key client.ObjectKey, obj runtime.Object) {
			// TODO(enisoc): Summarize backup status for each storage location.
		},
	})
}

func newVitessBackupStorage(key client.ObjectKey, parentLabels map[string]string, location *planetscalev2.VitessBackupLocation) *planetscalev2.VitessBackupStorage {
	// Copy parent labels and add child-specific labels.
	labels := map[string]string{
		vitessbackup.LocationLabel: location.Name,
	}
	for k, v := range parentLabels {
		labels[k] = v
	}

	return &planetscalev2.VitessBackupStorage{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: key.Namespace,
			Name:      key.Name,
			Labels:    labels,
		},
		Spec: planetscalev2.VitessBackupStorageSpec{
			Location: *location,
		},
	}
}
