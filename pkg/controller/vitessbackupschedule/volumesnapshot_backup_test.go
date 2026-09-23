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

package vitessbackupschedule

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	kbatch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
)

func newSnapshotScheme() *runtime.Scheme {
	s := newScheme()
	s.AddKnownTypeWithName(volumeSnapshotGVK, &unstructured.Unstructured{})
	s.AddKnownTypeWithName(volumeSnapshotListGVK, &unstructured.UnstructuredList{})
	metav1.AddToGroupVersion(s, volumeSnapshotGVK.GroupVersion())
	return s
}

func TestChooseSnapshotTarget(t *testing.T) {
	tablet := func(tabletType string, volumeBound corev1.ConditionStatus) planetscalev2.VitessTabletStatus {
		return planetscalev2.VitessTabletStatus{Type: tabletType, DataVolumeBound: volumeBound}
	}

	testCases := []struct {
		name    string
		tablets map[string]planetscalev2.VitessTabletStatus
		want    string
		wantErr bool
	}{
		{
			name: "prefers rdonly over replica",
			tablets: map[string]planetscalev2.VitessTabletStatus{
				"zone1-1111111111": tablet("replica", corev1.ConditionTrue),
				"zone1-2222222222": tablet("rdonly", corev1.ConditionTrue),
				"zone1-3333333333": tablet("primary", corev1.ConditionTrue),
			},
			want: "zone1-2222222222",
		},
		{
			name: "falls back to replica and never picks the primary",
			tablets: map[string]planetscalev2.VitessTabletStatus{
				"zone1-1111111111": tablet("primary", corev1.ConditionTrue),
				"zone1-2222222222": tablet("replica", corev1.ConditionTrue),
			},
			want: "zone1-2222222222",
		},
		{
			name: "picks deterministically by sorted alias",
			tablets: map[string]planetscalev2.VitessTabletStatus{
				"zone1-2222222222": tablet("replica", corev1.ConditionTrue),
				"zone1-1111111111": tablet("replica", corev1.ConditionTrue),
				"zone1-3333333333": tablet("primary", corev1.ConditionTrue),
			},
			want: "zone1-1111111111",
		},
		{
			name: "skips tablets without a bound data volume",
			tablets: map[string]planetscalev2.VitessTabletStatus{
				"zone1-1111111111": tablet("replica", corev1.ConditionFalse),
				"zone1-2222222222": tablet("replica", corev1.ConditionTrue),
			},
			want: "zone1-2222222222",
		},
		{
			name: "errors when only the primary is available",
			tablets: map[string]planetscalev2.VitessTabletStatus{
				"zone1-1111111111": tablet("primary", corev1.ConditionTrue),
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			shard := planetscalev2.VitessShard{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster-commerce-x-x"},
				Status:     planetscalev2.VitessShardStatus{Tablets: tc.tablets},
			}
			got, err := chooseSnapshotTarget(&shard)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestReconcileStrategy_VolumeSnapshotCreatesSnapshot(t *testing.T) {
	scheme := newSnapshotScheme()

	shard := &planetscalev2.VitessShard{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster-commerce-x-x",
			Namespace: "default",
			Labels: map[string]string{
				planetscalev2.ClusterLabel:  "test-cluster",
				planetscalev2.KeyspaceLabel: "commerce",
			},
		},
		Status: planetscalev2.VitessShardStatus{
			Tablets: map[string]planetscalev2.VitessTabletStatus{
				"zone1-1111111111": {Type: "primary", DataVolumeBound: corev1.ConditionTrue},
				"zone1-2222222222": {Type: "replica", DataVolumeBound: corev1.ConditionTrue},
			},
		},
	}

	r := &ReconcileVitessBackupsSchedule{
		scheme: scheme,
		client: fake.NewClientBuilder().WithScheme(scheme).
			WithStatusSubresource(&planetscalev2.VitessShard{}, &planetscalev2.VitessBackupSchedule{}).
			WithObjects(shard).Build(),
	}

	strategy := planetscalev2.VitessBackupScheduleStrategy{
		Name:                    "commerce-x",
		Scope:                   planetscalev2.BackupScopeShard,
		Keyspace:                "commerce",
		Shard:                   "-",
		VolumeSnapshotClassName: "test-snapclass",
	}
	vbsc := planetscalev2.VitessBackupSchedule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hourly",
			Namespace: "default",
			// Far enough in the past for one run to be due, close enough to
			// stay under the missed-runs limit.
			CreationTimestamp: metav1.NewTime(time.Now().Add(-90 * time.Minute)),
		},
		Spec: planetscalev2.VitessBackupScheduleSpec{
			Cluster: "test-cluster",
			VitessBackupScheduleTemplate: planetscalev2.VitessBackupScheduleTemplate{
				Schedule:     "0 * * * *",
				BackupMethod: planetscalev2.BackupMethodVolumeSnapshot,
				Strategy:     []planetscalev2.VitessBackupScheduleStrategy{strategy},
			},
		},
		Status: planetscalev2.NewVitessBackupScheduleStatus(planetscalev2.VitessBackupScheduleStatus{}),
	}

	_, err := r.reconcileStrategy(t.Context(), strategy, ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "default", Name: "hourly"}}, vbsc)
	require.NoError(t, err)

	snaps := &unstructured.UnstructuredList{}
	snaps.SetGroupVersionKind(volumeSnapshotListGVK)
	require.NoError(t, r.client.List(t.Context(), snaps, client.InNamespace("default")))
	require.Len(t, snaps.Items, 1)

	snap := snaps.Items[0]
	require.Equal(t, string(planetscalev2.BackupMethodVolumeSnapshot), snap.GetLabels()[planetscalev2.BackupMethodLabel])
	require.Equal(t, "hourly", snap.GetLabels()[planetscalev2.BackupScheduleLabel])
	require.NotEmpty(t, snap.GetAnnotations()[scheduledTimeAnnotation])

	// The snapshot must target the data PVC of the non-primary tablet. The PVC
	// shares its name with the tablet Pod.
	pvcName, found, err := unstructured.NestedString(snap.Object, "spec", "source", "persistentVolumeClaimName")
	require.NoError(t, err)
	require.True(t, found)
	require.Contains(t, pvcName, "vttablet-zone1-2222222222")

	className, found, err := unstructured.NestedString(snap.Object, "spec", "volumeSnapshotClassName")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "test-snapclass", className)

	// The scheduled run must be recorded so the next reconcile doesn't re-fire.
	require.Contains(t, vbsc.Status.LastScheduledTimes, strategy.Name)

	// No Job must be created for the volumeSnapshot method.
	jobs := &kbatch.JobList{}
	require.NoError(t, r.client.List(t.Context(), jobs, client.InNamespace("default")))
	require.Empty(t, jobs.Items, "no Jobs expected for volumeSnapshot method")
}

func TestReconcileStrategy_VolumeSnapshotSkipsWhenNoTargetAvailable(t *testing.T) {
	scheme := newSnapshotScheme()

	// Shard whose only tablet is the primary: no valid snapshot target.
	shard := &planetscalev2.VitessShard{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster-commerce-x-x",
			Namespace: "default",
			Labels: map[string]string{
				planetscalev2.ClusterLabel:  "test-cluster",
				planetscalev2.KeyspaceLabel: "commerce",
			},
		},
		Status: planetscalev2.VitessShardStatus{
			Tablets: map[string]planetscalev2.VitessTabletStatus{
				"zone1-1111111111": {Type: "primary", DataVolumeBound: corev1.ConditionTrue},
			},
		},
	}

	r := &ReconcileVitessBackupsSchedule{
		scheme: scheme,
		client: fake.NewClientBuilder().WithScheme(scheme).
			WithStatusSubresource(&planetscalev2.VitessShard{}, &planetscalev2.VitessBackupSchedule{}).
			WithObjects(shard).Build(),
	}

	strategy := planetscalev2.VitessBackupScheduleStrategy{
		Name:     "commerce-x",
		Scope:    planetscalev2.BackupScopeShard,
		Keyspace: "commerce",
		Shard:    "-",
	}
	vbsc := planetscalev2.VitessBackupSchedule{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "hourly",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Now().Add(-90 * time.Minute)),
		},
		Spec: planetscalev2.VitessBackupScheduleSpec{
			Cluster: "test-cluster",
			VitessBackupScheduleTemplate: planetscalev2.VitessBackupScheduleTemplate{
				Schedule:     "0 * * * *",
				BackupMethod: planetscalev2.BackupMethodVolumeSnapshot,
				Strategy:     []planetscalev2.VitessBackupScheduleStrategy{strategy},
			},
		},
		Status: planetscalev2.NewVitessBackupScheduleStatus(planetscalev2.VitessBackupScheduleStatus{}),
	}

	// The run is skipped (no target), but the reconcile must not error out.
	_, err := r.reconcileStrategy(t.Context(), strategy, ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "default", Name: "hourly"}}, vbsc)
	require.NoError(t, err)

	snaps := &unstructured.UnstructuredList{}
	snaps.SetGroupVersionKind(volumeSnapshotListGVK)
	require.NoError(t, r.client.List(t.Context(), snaps, client.InNamespace("default")))
	require.Empty(t, snaps.Items)
	require.NotContains(t, vbsc.Status.LastScheduledTimes, strategy.Name)
}
