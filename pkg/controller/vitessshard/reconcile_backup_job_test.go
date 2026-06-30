/*
Copyright 2026 PlanetScale Inc.

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

package vitessshard

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/vitessbackup"
)

// TestVtbackupSpecDataVolume covers how the pool-level vtbackup override drives
// the data volume of vtbackup Pods: inherit by default, drop the PVC when the
// override is present but empty, or use an explicit override template.
func TestVtbackupSpecDataVolume(t *testing.T) {
	poolPVC := &corev1.PersistentVolumeClaimSpec{
		AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
		Resources: corev1.VolumeResourceRequirements{
			Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("100Gi")},
		},
	}
	overridePVC := &corev1.PersistentVolumeClaimSpec{
		StorageClassName: ptr.To("cheap-disk"),
		AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
		Resources: corev1.VolumeResourceRequirements{
			Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("10Gi")},
		},
	}

	cases := []struct {
		name     string
		vtbackup *planetscalev2.VitessShardTabletPoolVtbackup
		want     *corev1.PersistentVolumeClaimSpec
	}{
		{
			name:     "no override inherits pool data volume",
			vtbackup: nil,
			want:     poolPVC,
		},
		{
			name:     "empty override drops the PVC (emptyDir)",
			vtbackup: &planetscalev2.VitessShardTabletPoolVtbackup{},
			want:     nil,
		},
		{
			name:     "override template is used",
			vtbackup: &planetscalev2.VitessShardTabletPoolVtbackup{DataVolumeClaimTemplate: overridePVC},
			want:     overridePVC,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vts := &planetscalev2.VitessShard{
				Spec: planetscalev2.VitessShardSpec{
					VitessShardTemplate: planetscalev2.VitessShardTemplate{
						TabletPools: []planetscalev2.VitessShardTabletPool{{
							Cell:                    "cell1",
							Type:                    planetscalev2.ReplicaPoolType,
							DataVolumeClaimTemplate: poolPVC,
							Vtbackup:                tc.vtbackup,
						}},
						Replication: planetscalev2.VitessReplicationSpec{
							InitializeBackup: ptr.To(true),
						},
					},
					BackupLocations: []planetscalev2.VitessBackupLocation{{Name: ""}},
				},
			}
			vts.Labels = map[string]string{planetscalev2.KeyspaceLabel: "keyspace1"}

			key := client.ObjectKey{Namespace: "default", Name: "init-pod"}
			spec := MakeVtbackupSpec(key, vts, nil, vitessbackup.TypeInit)
			if spec == nil {
				t.Fatal("MakeVtbackupSpec returned nil")
			}
			if got := spec.TabletSpec.DataVolumePVCSpec; got != tc.want {
				t.Errorf("DataVolumePVCSpec = %v, want %v", got, tc.want)
			}
		})
	}
}
