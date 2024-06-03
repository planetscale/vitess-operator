/*
Copyright 2024 PlanetScale Inc.

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

	// Generate keys (object names) for all desired cells.
	// Keep a map back from generated names to the  specs.
	var keys []client.ObjectKey
	scheduleMap := make(map[client.ObjectKey]*planetscalev2.VitessBackupScheduleTemplate)
	if vt.Spec.Backup != nil {
		for i := range vt.Spec.Backup.Schedules {
			schedule := &vt.Spec.Backup.Schedules[i]
			key := client.ObjectKey{
				Namespace: vt.Namespace,
				Name:      vitessbackup.ScheduleName(vt.Name, schedule.Name),
			}
			keys = append(keys, key)
			scheduleMap[key] = schedule
		}
	}

	return r.reconciler.ReconcileObjectSet(ctx, vt, keys, labels, reconciler.Strategy{
		Kind: &planetscalev2.VitessBackupSchedule{},

		New: func(key client.ObjectKey) runtime.Object {
			vbsc := vitessbackup.NewVitessBackupSchedule(key, vt, scheduleMap[key], labels)
			if vbsc == nil {
				return &planetscalev2.VitessBackupSchedule{}
			}
			return vbsc
		},

		UpdateInPlace: func(key client.ObjectKey, obj runtime.Object) {
			newObj := obj.(*planetscalev2.VitessBackupSchedule)
			newVbsc := vitessbackup.NewVitessBackupSchedule(key, vt, scheduleMap[key], labels)
			if newVbsc == nil {
				return
			}
			newObj.Spec = newVbsc.Spec
		},

		PrepareForTurndown: func(key client.ObjectKey, newObj runtime.Object) *planetscalev2.OrphanStatus {
			// If we want to remove the schedule, delete it immediately.
			return nil
		},
	})
}
