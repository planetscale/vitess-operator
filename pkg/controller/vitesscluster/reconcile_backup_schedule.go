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
	"planetscale.dev/vitess-operator/pkg/operator/vitessbackup"
	"sigs.k8s.io/controller-runtime/pkg/client"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/reconciler"
)

func (r *ReconcileVitessCluster) reconcileBackupSchedule(ctx context.Context, vt *planetscalev2.VitessCluster) error {
	labels := map[string]string{
		planetscalev2.ClusterLabel: vt.Name,
	}

	key := client.ObjectKey{
		Namespace: vt.Namespace,
		Name:      vitessbackup.ScheduleName(vt.Name),
	}

	return r.reconciler.ReconcileObject(ctx, vt, key, labels, true, reconciler.Strategy{
		Kind: &planetscalev2.VitessBackupSchedule{},

		New: func(key client.ObjectKey) runtime.Object {
			return vitessbackup.NewVitessBackupSchedule(key, vt, labels)
		},

		UpdateInPlace: func(key client.ObjectKey, obj runtime.Object) {
			newObj := obj.(*planetscalev2.VitessBackupSchedule)
			newVbscObj := vitessbackup.NewVitessBackupSchedule(key, vt, labels)
			if newVbscObj == nil {
				return
			}
			newObj.Spec = newVbscObj.(*planetscalev2.VitessBackupSchedule).Spec
		},

		PrepareForTurndown: func(key client.ObjectKey, newObj runtime.Object) *planetscalev2.OrphanStatus {
			// If we want to remove the schedule, delete it immediately.
			return nil
		},
	})
}
