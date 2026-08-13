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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/reconciler"
	"planetscale.dev/vitess-operator/pkg/operator/vitessbackup"
	"planetscale.dev/vitess-operator/pkg/operator/vttablet"
)

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
		name       string
		backupType string
		vtbackup   *planetscalev2.VitessShardVtbackup
		want       *corev1.PersistentVolumeClaimSpec
	}{
		{
			name:       "initial backup without override inherits pool data volume",
			backupType: vitessbackup.TypeInit,
			vtbackup:   nil,
			want:       poolPVC,
		},
		{
			name:       "initial backup with empty override drops the PVC (emptyDir)",
			backupType: vitessbackup.TypeInit,
			vtbackup:   &planetscalev2.VitessShardVtbackup{},
			want:       nil,
		},
		{
			name:       "initial backup uses override template",
			backupType: vitessbackup.TypeInit,
			vtbackup:   &planetscalev2.VitessShardVtbackup{DataVolumeClaimTemplate: overridePVC},
			want:       overridePVC,
		},
		{
			name:       "scheduled backup without override inherits pool data volume",
			backupType: vitessbackup.TypeUpdate,
			vtbackup:   nil,
			want:       poolPVC,
		},
		{
			name:       "scheduled backup with empty override inherits pool data volume",
			backupType: vitessbackup.TypeUpdate,
			vtbackup:   &planetscalev2.VitessShardVtbackup{},
			want:       poolPVC,
		},
		{
			name:       "scheduled backup uses override template",
			backupType: vitessbackup.TypeUpdate,
			vtbackup:   &planetscalev2.VitessShardVtbackup{DataVolumeClaimTemplate: overridePVC},
			want:       overridePVC,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vts := &planetscalev2.VitessShard{
				Spec: planetscalev2.VitessShardSpec{
					VitessShardTemplate: planetscalev2.VitessShardTemplate{
						Vtbackup: tc.vtbackup,
						TabletPools: []planetscalev2.VitessShardTabletPool{
							{
								Cell:                    "cell1",
								Type:                    planetscalev2.ReplicaPoolType,
								DataVolumeClaimTemplate: poolPVC,
							},
							{
								Cell:                    "cell2",
								Type:                    planetscalev2.RdonlyPoolType,
								DataVolumeClaimTemplate: poolPVC,
							},
						},
						Replication: planetscalev2.VitessReplicationSpec{
							InitializeBackup: ptr.To(true),
						},
					},
					BackupLocations: []planetscalev2.VitessBackupLocation{{Name: ""}},
				},
			}
			vts.Labels = map[string]string{planetscalev2.KeyspaceLabel: "keyspace1"}

			key := client.ObjectKey{Namespace: "default", Name: "init-pod"}
			spec := MakeVtbackupSpec(key, vts, nil, tc.backupType)
			require.NotNil(t, spec)
			assert.Same(t, tc.want, spec.TabletSpec.DataVolumePVCSpec)
		})
	}
}

func TestReconcileBackupJobConvergesDataVolumeChanges(t *testing.T) {
	poolPVC := backupPVCSpec("100Gi", "")
	smallPVC := backupPVCSpec("10Gi", "cheap-disk")
	largePVC := backupPVCSpec("20Gi", "fast-disk")

	cases := []struct {
		name        string
		oldVtbackup *planetscalev2.VitessShardVtbackup
		newVtbackup *planetscalev2.VitessShardVtbackup
		wantPVC     *corev1.PersistentVolumeClaimSpec
	}{
		{
			name:        "PVC to emptyDir",
			oldVtbackup: nil,
			newVtbackup: &planetscalev2.VitessShardVtbackup{},
			wantPVC:     nil,
		},
		{
			name:        "emptyDir to inherited PVC",
			oldVtbackup: &planetscalev2.VitessShardVtbackup{},
			newVtbackup: nil,
			wantPVC:     poolPVC,
		},
		{
			name: "custom PVC template change",
			oldVtbackup: &planetscalev2.VitessShardVtbackup{
				DataVolumeClaimTemplate: smallPVC,
			},
			newVtbackup: &planetscalev2.VitessShardVtbackup{
				DataVolumeClaimTemplate: largePVC,
			},
			wantPVC: largePVC,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vts := backupTestShard(tc.oldVtbackup, poolPVC)
			key := initialBackupKey(vts)
			labels := initialBackupLabels(vts)
			oldSpec := MakeVtbackupSpec(key, vts, labels, vitessbackup.TypeInit)
			require.NotNil(t, oldSpec)

			oldPod := vttablet.NewBackupPod(key, oldSpec, vts.Spec.Images.Mysqld.Image())
			oldPod.UID = types.UID("old-pod")
			oldPod.Status.Phase = corev1.PodPending

			objects := []client.Object{vts.DeepCopy(), oldPod}
			if oldSpec.TabletSpec.DataVolumePVCSpec != nil {
				oldPVC := vttablet.NewPVC(key, oldSpec.TabletSpec)
				oldPVC.UID = types.UID("old-pvc")
				objects = append(objects, oldPVC)
			}

			scheme := backupTestScheme(t)
			k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
			recorder := record.NewFakeRecorder(100)
			r := &ReconcileVitessShard{
				client:     k8sClient,
				scheme:     scheme,
				recorder:   recorder,
				reconciler: reconciler.New(k8sClient, scheme, recorder),
			}

			vts.Spec.Vtbackup = tc.newVtbackup
			for range 4 {
				_, err := r.reconcileBackupJob(t.Context(), vts)
				require.NoError(t, err)
			}

			pod := &corev1.Pod{}
			require.NoError(t, k8sClient.Get(t.Context(), key, pod))
			if tc.wantPVC == nil {
				for _, volume := range pod.Spec.Volumes {
					assert.Nil(t, volume.PersistentVolumeClaim, "initial backup Pod references PVC %q", volume.Name)
				}

				pvc := &corev1.PersistentVolumeClaim{}
				err := k8sClient.Get(t.Context(), key, pvc)
				assert.True(t, apierrors.IsNotFound(err), "PVC should be removed, got: %v", err)
				return
			}

			foundDataVolume := false
			for _, volume := range pod.Spec.Volumes {
				if volume.PersistentVolumeClaim != nil {
					foundDataVolume = true
					assert.Equal(t, key.Name, volume.PersistentVolumeClaim.ClaimName)
				}
			}
			assert.True(t, foundDataVolume, "initial backup Pod should reference its PVC")

			pvc := &corev1.PersistentVolumeClaim{}
			require.NoError(t, k8sClient.Get(t.Context(), key, pvc))
			assert.Equal(t, tc.wantPVC.StorageClassName, pvc.Spec.StorageClassName)
			assert.Equal(t,
				tc.wantPVC.Resources.Requests[corev1.ResourceStorage],
				pvc.Spec.Resources.Requests[corev1.ResourceStorage],
			)
		})
	}
}

func TestReconcileBackupJobPreservesRunningPodWhenDataVolumeChanges(t *testing.T) {
	poolPVC := backupPVCSpec("100Gi", "")
	vts := backupTestShard(nil, poolPVC)
	key := initialBackupKey(vts)
	oldSpec := MakeVtbackupSpec(key, vts, initialBackupLabels(vts), vitessbackup.TypeInit)
	require.NotNil(t, oldSpec)

	oldPod := vttablet.NewBackupPod(key, oldSpec, vts.Spec.Images.Mysqld.Image())
	oldPod.UID = types.UID("running-pod")
	oldPod.Status.Phase = corev1.PodRunning
	oldPVC := vttablet.NewPVC(key, oldSpec.TabletSpec)
	oldPVC.UID = types.UID("running-pvc")

	scheme := backupTestScheme(t)
	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(vts.DeepCopy(), oldPod, oldPVC).
		Build()
	recorder := record.NewFakeRecorder(100)
	r := &ReconcileVitessShard{
		client:     k8sClient,
		scheme:     scheme,
		recorder:   recorder,
		reconciler: reconciler.New(k8sClient, scheme, recorder),
	}

	vts.Spec.Vtbackup = &planetscalev2.VitessShardVtbackup{}
	for range 4 {
		_, err := r.reconcileBackupJob(t.Context(), vts)
		require.NoError(t, err)
	}

	pod := &corev1.Pod{}
	require.NoError(t, k8sClient.Get(t.Context(), key, pod))
	assert.Equal(t, types.UID("running-pod"), pod.UID)

	pvc := &corev1.PersistentVolumeClaim{}
	require.NoError(t, k8sClient.Get(t.Context(), key, pvc))
	assert.Equal(t, types.UID("running-pvc"), pvc.UID)
}

func backupPVCSpec(size, storageClass string) *corev1.PersistentVolumeClaimSpec {
	spec := &corev1.PersistentVolumeClaimSpec{
		AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
		Resources: corev1.VolumeResourceRequirements{
			Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(size)},
		},
	}
	if storageClass != "" {
		spec.StorageClassName = ptr.To(storageClass)
	}
	return spec
}

func backupTestShard(vtbackup *planetscalev2.VitessShardVtbackup, poolPVC *corev1.PersistentVolumeClaimSpec) *planetscalev2.VitessShard {
	return &planetscalev2.VitessShard{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-commerce-x-x",
			Namespace: "default",
			UID:       types.UID("shard"),
			Labels: map[string]string{
				planetscalev2.ClusterLabel:  "example",
				planetscalev2.KeyspaceLabel: "commerce",
			},
		},
		Spec: planetscalev2.VitessShardSpec{
			VitessShardTemplate: planetscalev2.VitessShardTemplate{
				Vtbackup: vtbackup,
				TabletPools: []planetscalev2.VitessShardTabletPool{{
					Cell:                    "zone1",
					Type:                    planetscalev2.ReplicaPoolType,
					Replicas:                1,
					DataVolumeClaimTemplate: poolPVC,
					Vttablet:                planetscalev2.VttabletSpec{},
					Mysqld:                  &planetscalev2.MysqldSpec{},
				}},
				Replication: planetscalev2.VitessReplicationSpec{
					InitializeBackup: ptr.To(true),
				},
			},
			Images: planetscalev2.VitessKeyspaceImages{
				Vtbackup: "vitess/vtbackup:test",
				Mysqld: &planetscalev2.MysqldImage{
					Mysql80Compatible: "vitess/lite:test",
				},
			},
			BackupLocations: []planetscalev2.VitessBackupLocation{{
				Volume: &corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			}},
		},
	}
}

func initialBackupKey(vts *planetscalev2.VitessShard) client.ObjectKey {
	return client.ObjectKey{
		Namespace: vts.Namespace,
		Name: vttablet.InitialBackupPodName(
			vts.Labels[planetscalev2.ClusterLabel],
			vts.Labels[planetscalev2.KeyspaceLabel],
			vts.Spec.KeyRange,
		),
	}
}

func initialBackupLabels(vts *planetscalev2.VitessShard) map[string]string {
	return map[string]string{
		planetscalev2.ComponentLabel: planetscalev2.VtbackupComponentName,
		planetscalev2.ClusterLabel:   vts.Labels[planetscalev2.ClusterLabel],
		planetscalev2.KeyspaceLabel:  vts.Labels[planetscalev2.KeyspaceLabel],
		planetscalev2.ShardLabel:     vts.Spec.KeyRange.SafeName(),
		vitessbackup.TypeLabel:       vitessbackup.TypeInit,
	}
}

func backupTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, planetscalev2.SchemeBuilder.AddToScheme(scheme))
	return scheme
}
