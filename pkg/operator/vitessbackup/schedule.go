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

package vitessbackup

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/names"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ScheduleName(clusterName string, scheduleName string) string {
	return names.JoinWithConstraints(names.DefaultConstraints, clusterName, "vbsc", scheduleName)
}

func NewVitessBackupSchedule(key client.ObjectKey, vt *planetscalev2.VitessCluster, vbsc *planetscalev2.VitessBackupScheduleTemplate, labels map[string]string) *planetscalev2.VitessBackupSchedule {
	if vt.Spec.Backup == nil || vbsc == nil || vbsc.Schedule == "" {
		return nil
	}

	spec := planetscalev2.VitessBackupScheduleSpec{
		// We simply re-apply the same template that was written by the user.
		VitessBackupScheduleTemplate: *vbsc,

		Cluster: vt.Name,

		// To take backups we only care about having the vtctldclient installed in the container.
		// For this reason, we re-use the vtctld Docker image and the same image pull policy.
		Image:           vt.Spec.Images.Vtctld,
		ImagePullPolicy: vt.Spec.ImagePullPolicies.Vtctld,
	}

	// Populate ExtraLabels from ClusterBackupSpec if available.
	if vt.Spec.Backup.ExtraLabels != nil {
		spec.ExtraLabels = make(map[string]string)
		for k, v := range vt.Spec.Backup.ExtraLabels {
			spec.ExtraLabels[k] = v
		}
	}

	return &planetscalev2.VitessBackupSchedule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
			Labels:    labels,
		},
		Spec: spec,
	}
}
