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
