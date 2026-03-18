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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
)

func TestReconcileStrategy_ClearsGeneratedScheduleWhenUsingExplicitSchedule(t *testing.T) {
	r := &ReconcileVitessBackupsSchedule{
		client: fake.NewClientBuilder().WithScheme(newScheme()).Build(),
	}

	strategy := planetscalev2.VitessBackupScheduleStrategy{
		Name:     "commerce-x",
		Scope:    planetscalev2.BackupScopeShard,
		Keyspace: "commerce",
		Shard:    "-",
	}
	vbsc := planetscalev2.VitessBackupSchedule{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "daily",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Now().Add(2 * time.Hour)),
		},
		Spec: planetscalev2.VitessBackupScheduleSpec{
			Cluster: "test-cluster",
			VitessBackupScheduleTemplate: planetscalev2.VitessBackupScheduleTemplate{
				Schedule: "0 0 * * *",
				Strategy: []planetscalev2.VitessBackupScheduleStrategy{strategy},
			},
		},
		Status: planetscalev2.VitessBackupScheduleStatus{
			GeneratedSchedules: map[string]string{strategy.Name: "17 5 * * *"},
			LastScheduledTimes: map[string]*metav1.Time{},
			NextScheduledTimes: map[string]*metav1.Time{},
		},
	}

	_, err := r.reconcileStrategy(t.Context(), strategy, ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "default", Name: "daily"}}, vbsc)
	require.NoError(t, err)
	require.NotContains(t, vbsc.Status.GeneratedSchedules, strategy.Name)
}

func TestReconcileStrategies_RejectsDuplicateEffectiveShardTargetsWithinSchedule(t *testing.T) {
	scheme := newScheme()
	commerce := &planetscalev2.VitessKeyspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster-commerce",
			Namespace: "default",
			Labels:    map[string]string{planetscalev2.ClusterLabel: "test-cluster"},
		},
		Spec: planetscalev2.VitessKeyspaceSpec{
			VitessKeyspaceTemplate: planetscalev2.VitessKeyspaceTemplate{Name: "commerce"},
		},
		Status: planetscalev2.VitessKeyspaceStatus{
			Shards: map[string]planetscalev2.VitessKeyspaceShardStatus{"-": {}},
		},
	}
	r := &ReconcileVitessBackupsSchedule{
		client: fake.NewClientBuilder().WithScheme(scheme).
			WithStatusSubresource(&planetscalev2.VitessKeyspace{}, &planetscalev2.VitessBackupSchedule{}).
			WithObjects(commerce).Build(),
	}

	vbsc := planetscalev2.VitessBackupSchedule{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "daily",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Now().Add(2 * time.Hour)),
		},
		Spec: planetscalev2.VitessBackupScheduleSpec{
			Cluster: "test-cluster",
			VitessBackupScheduleTemplate: planetscalev2.VitessBackupScheduleTemplate{
				Name:      "daily",
				Schedule:  "0 0 * * *",
				Resources: corev1.ResourceRequirements{},
				Strategy: []planetscalev2.VitessBackupScheduleStrategy{
					{Name: "cluster-all", Scope: planetscalev2.BackupScopeCluster},
					{Name: "commerce-explicit", Scope: planetscalev2.BackupScopeShard, Keyspace: "commerce", Shard: "-"},
				},
			},
		},
		Status: planetscalev2.NewVitessBackupScheduleStatus(planetscalev2.VitessBackupScheduleStatus{}),
	}

	_, err := r.reconcileStrategies(t.Context(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "default", Name: "daily"}}, vbsc)
	require.Error(t, err)
	require.ErrorContains(t, err, "terminal error:")
	require.ErrorContains(t, err, "duplicate effective shard target")
	require.ErrorContains(t, err, "commerce/-")
}

func TestReconcileStrategies_AllowsDuplicateTargetsAcrossDifferentSchedules(t *testing.T) {
	scheme := newScheme()
	commerce := &planetscalev2.VitessKeyspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster-commerce",
			Namespace: "default",
			Labels:    map[string]string{planetscalev2.ClusterLabel: "test-cluster"},
		},
		Spec: planetscalev2.VitessKeyspaceSpec{
			VitessKeyspaceTemplate: planetscalev2.VitessKeyspaceTemplate{Name: "commerce"},
		},
		Status: planetscalev2.VitessKeyspaceStatus{
			Shards: map[string]planetscalev2.VitessKeyspaceShardStatus{"-": {}},
		},
	}
	otherSchedule := &planetscalev2.VitessBackupSchedule{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "cluster-daily",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Now().Add(2 * time.Hour)),
		},
		Spec: planetscalev2.VitessBackupScheduleSpec{
			Cluster: "test-cluster",
			VitessBackupScheduleTemplate: planetscalev2.VitessBackupScheduleTemplate{
				Name:      "cluster-daily",
				Schedule:  "0 0 * * *",
				Resources: corev1.ResourceRequirements{},
				Strategy: []planetscalev2.VitessBackupScheduleStrategy{
					{Name: "cluster-all", Scope: planetscalev2.BackupScopeCluster},
				},
			},
		},
		Status: planetscalev2.NewVitessBackupScheduleStatus(planetscalev2.VitessBackupScheduleStatus{}),
	}
	r := &ReconcileVitessBackupsSchedule{
		client: fake.NewClientBuilder().WithScheme(scheme).
			WithStatusSubresource(&planetscalev2.VitessKeyspace{}, &planetscalev2.VitessBackupSchedule{}).
			WithObjects(commerce, otherSchedule).Build(),
	}

	vbsc := planetscalev2.VitessBackupSchedule{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "daily",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Now().Add(2 * time.Hour)),
		},
		Spec: planetscalev2.VitessBackupScheduleSpec{
			Cluster: "test-cluster",
			VitessBackupScheduleTemplate: planetscalev2.VitessBackupScheduleTemplate{
				Name:      "daily",
				Schedule:  "0 0 * * *",
				Resources: corev1.ResourceRequirements{},
				Strategy: []planetscalev2.VitessBackupScheduleStrategy{
					{Name: "commerce-explicit", Scope: planetscalev2.BackupScopeShard, Keyspace: "commerce", Shard: "-"},
				},
			},
		},
		Status: planetscalev2.NewVitessBackupScheduleStatus(planetscalev2.VitessBackupScheduleStatus{}),
	}

	_, err := r.reconcileStrategies(t.Context(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "default", Name: "daily"}}, vbsc)
	require.NoError(t, err)
}
