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
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
)

func readyShardStatus() planetscalev2.VitessShardStatus {
	return planetscalev2.VitessShardStatus{
		HasMaster:        corev1.ConditionTrue,
		HasInitialBackup: corev1.ConditionTrue,
		Tablets: map[string]planetscalev2.VitessTabletStatus{
			"zone1-1": {Ready: corev1.ConditionTrue},
			"zone1-2": {Ready: corev1.ConditionTrue},
		},
	}
}

func mustBuildExpansionContext(t *testing.T, r *ReconcileVitessBackupsSchedule, vbsc planetscalev2.VitessBackupSchedule) strategyExpansionContext {
	t.Helper()
	ctx, err := r.buildStrategyExpansionContext(context.Background(), vbsc)
	require.NoError(t, err)
	return ctx
}

func newScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = planetscalev2.SchemeBuilder.AddToScheme(s)
	_ = corev1.AddToScheme(s)
	return s
}

func TestExpandStrategy_ShardScope(t *testing.T) {
	r := &ReconcileVitessBackupsSchedule{
		client: fake.NewClientBuilder().WithScheme(newScheme()).Build(),
	}

	strategy := planetscalev2.VitessBackupScheduleStrategy{
		Name:     "my-shard",
		Scope:    planetscalev2.BackupScopeShard,
		Keyspace: "commerce",
		Shard:    "-",
	}
	vbsc := planetscalev2.VitessBackupSchedule{
		Spec: planetscalev2.VitessBackupScheduleSpec{
			Cluster: "test-cluster",
		},
	}

	result, err := r.expandStrategy(context.Background(), strategy, vbsc, mustBuildExpansionContext(t, r, vbsc))
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, "my-shard", result[0].Name)
	require.Equal(t, "commerce", result[0].Keyspace)
}

func TestExpandStrategy_EmptyScopeDefaultsToShard(t *testing.T) {
	r := &ReconcileVitessBackupsSchedule{
		client: fake.NewClientBuilder().WithScheme(newScheme()).Build(),
	}

	strategy := planetscalev2.VitessBackupScheduleStrategy{
		Name:     "legacy",
		Keyspace: "commerce",
		Shard:    "-80",
	}
	vbsc := planetscalev2.VitessBackupSchedule{
		Spec: planetscalev2.VitessBackupScheduleSpec{Cluster: "c"},
	}

	result, err := r.expandStrategy(context.Background(), strategy, vbsc, mustBuildExpansionContext(t, r, vbsc))
	require.NoError(t, err)
	require.Len(t, result, 1)
}

func TestExpandStrategy_KeyspaceScope(t *testing.T) {
	scheme := newScheme()
	ks := &planetscalev2.VitessKeyspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster-commerce",
			Namespace: "default",
			Labels: map[string]string{
				planetscalev2.ClusterLabel: "test-cluster",
			},
		},
		Spec: planetscalev2.VitessKeyspaceSpec{
			VitessKeyspaceTemplate: planetscalev2.VitessKeyspaceTemplate{
				Name: "commerce",
			},
		},
		Status: planetscalev2.VitessKeyspaceStatus{
			Shards: map[string]planetscalev2.VitessKeyspaceShardStatus{
				"-80": {},
				"80-": {},
			},
		},
	}

	r := &ReconcileVitessBackupsSchedule{
		client: fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&planetscalev2.VitessKeyspace{}).WithObjects(ks).Build(),
	}

	strategy := planetscalev2.VitessBackupScheduleStrategy{
		Name:     "commerce-all",
		Scope:    planetscalev2.BackupScopeKeyspace,
		Keyspace: "commerce",
	}
	vbsc := planetscalev2.VitessBackupSchedule{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
		Spec:       planetscalev2.VitessBackupScheduleSpec{Cluster: "test-cluster"},
	}

	result, err := r.expandStrategy(context.Background(), strategy, vbsc, mustBuildExpansionContext(t, r, vbsc))
	require.NoError(t, err)
	require.Len(t, result, 2, "expected 2 strategies (one per shard)")

	// Verify strategies have correct fields
	names := make(map[string]bool)
	for _, s := range result {
		require.Equal(t, planetscalev2.BackupScopeShard, s.Scope)
		require.Equal(t, "commerce", s.Keyspace)
		require.NotEmpty(t, s.Shard)
		require.False(t, names[s.Name], "duplicate strategy name: %s", s.Name)
		names[s.Name] = true
	}
}

func TestExpandStrategy_ClusterScope(t *testing.T) {
	scheme := newScheme()
	ks1 := &planetscalev2.VitessKeyspace{
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
	ks2 := &planetscalev2.VitessKeyspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster-customer",
			Namespace: "default",
			Labels:    map[string]string{planetscalev2.ClusterLabel: "test-cluster"},
		},
		Spec: planetscalev2.VitessKeyspaceSpec{
			VitessKeyspaceTemplate: planetscalev2.VitessKeyspaceTemplate{Name: "customer"},
		},
		Status: planetscalev2.VitessKeyspaceStatus{
			Shards: map[string]planetscalev2.VitessKeyspaceShardStatus{
				"-80": {},
				"80-": {},
			},
		},
	}

	r := &ReconcileVitessBackupsSchedule{
		client: fake.NewClientBuilder().WithScheme(scheme).
			WithStatusSubresource(&planetscalev2.VitessKeyspace{}).
			WithObjects(ks1, ks2).Build(),
	}

	strategy := planetscalev2.VitessBackupScheduleStrategy{
		Name:  "all",
		Scope: planetscalev2.BackupScopeCluster,
	}
	vbsc := planetscalev2.VitessBackupSchedule{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster-daily", Namespace: "default"},
		Spec: planetscalev2.VitessBackupScheduleSpec{
			Cluster: "test-cluster",
			VitessBackupScheduleTemplate: planetscalev2.VitessBackupScheduleTemplate{
				Strategy: []planetscalev2.VitessBackupScheduleStrategy{strategy},
			},
		},
	}

	result, err := r.expandStrategy(context.Background(), strategy, vbsc, mustBuildExpansionContext(t, r, vbsc))
	require.NoError(t, err)
	// commerce has 1 shard, customer has 2 shards = 3 total
	require.Len(t, result, 3)
}

func TestExpandStrategy_ClusterScopeAutoExclusion(t *testing.T) {
	scheme := newScheme()
	ks1 := &planetscalev2.VitessKeyspace{
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
	ks2 := &planetscalev2.VitessKeyspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster-customer",
			Namespace: "default",
			Labels:    map[string]string{planetscalev2.ClusterLabel: "test-cluster"},
		},
		Spec: planetscalev2.VitessKeyspaceSpec{
			VitessKeyspaceTemplate: planetscalev2.VitessKeyspaceTemplate{Name: "customer"},
		},
		Status: planetscalev2.VitessKeyspaceStatus{
			Shards: map[string]planetscalev2.VitessKeyspaceShardStatus{
				"-80": {},
				"80-": {},
			},
		},
	}

	// Create a separate VitessBackupSchedule that has a Keyspace-scope strategy for "customer"
	otherSchedule := &planetscalev2.VitessBackupSchedule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "customer-frequent",
			Namespace: "default",
			Labels:    map[string]string{planetscalev2.ClusterLabel: "test-cluster"},
		},
		Spec: planetscalev2.VitessBackupScheduleSpec{
			Cluster: "test-cluster",
			VitessBackupScheduleTemplate: planetscalev2.VitessBackupScheduleTemplate{
				Name:     "customer-frequent",
				Schedule: "0 */6 * * *",
				Strategy: []planetscalev2.VitessBackupScheduleStrategy{
					{
						Name:     "customer-all",
						Scope:    planetscalev2.BackupScopeKeyspace,
						Keyspace: "customer",
					},
				},
				Resources: corev1.ResourceRequirements{},
			},
		},
	}

	r := &ReconcileVitessBackupsSchedule{
		client: fake.NewClientBuilder().WithScheme(scheme).
			WithStatusSubresource(&planetscalev2.VitessKeyspace{}, &planetscalev2.VitessBackupSchedule{}).
			WithObjects(ks1, ks2, otherSchedule).Build(),
	}

	strategy := planetscalev2.VitessBackupScheduleStrategy{
		Name:  "all",
		Scope: planetscalev2.BackupScopeCluster,
	}
	vbsc := planetscalev2.VitessBackupSchedule{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster-daily", Namespace: "default", Labels: map[string]string{planetscalev2.ClusterLabel: "test-cluster"}},
		Spec: planetscalev2.VitessBackupScheduleSpec{
			Cluster: "test-cluster",
			VitessBackupScheduleTemplate: planetscalev2.VitessBackupScheduleTemplate{
				Strategy: []planetscalev2.VitessBackupScheduleStrategy{strategy},
			},
		},
	}

	result, err := r.expandStrategy(context.Background(), strategy, vbsc, mustBuildExpansionContext(t, r, vbsc))
	require.NoError(t, err)
	// customer is excluded (has Keyspace-scope override), only commerce's 1 shard remains
	require.Len(t, result, 1, "expected 1 strategy (customer excluded)")
	require.Equal(t, "commerce", result[0].Keyspace)
}

func TestExpandStrategy_SelfAutoExclusion(t *testing.T) {
	scheme := newScheme()
	ks1 := &planetscalev2.VitessKeyspace{
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
	ks2 := &planetscalev2.VitessKeyspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster-customer",
			Namespace: "default",
			Labels:    map[string]string{planetscalev2.ClusterLabel: "test-cluster"},
		},
		Spec: planetscalev2.VitessKeyspaceSpec{
			VitessKeyspaceTemplate: planetscalev2.VitessKeyspaceTemplate{Name: "customer"},
		},
		Status: planetscalev2.VitessKeyspaceStatus{
			Shards: map[string]planetscalev2.VitessKeyspaceShardStatus{"-80": {}, "80-": {}},
		},
	}

	r := &ReconcileVitessBackupsSchedule{
		client: fake.NewClientBuilder().WithScheme(scheme).
			WithStatusSubresource(&planetscalev2.VitessKeyspace{}).
			WithObjects(ks1, ks2).Build(),
	}

	// Same schedule has both Cluster and Keyspace strategies
	clusterStrategy := planetscalev2.VitessBackupScheduleStrategy{
		Name:  "all",
		Scope: planetscalev2.BackupScopeCluster,
	}
	ksStrategy := planetscalev2.VitessBackupScheduleStrategy{
		Name:     "customer-all",
		Scope:    planetscalev2.BackupScopeKeyspace,
		Keyspace: "customer",
	}
	vbsc := planetscalev2.VitessBackupSchedule{
		ObjectMeta: metav1.ObjectMeta{Name: "mixed", Namespace: "default"},
		Spec: planetscalev2.VitessBackupScheduleSpec{
			Cluster: "test-cluster",
			VitessBackupScheduleTemplate: planetscalev2.VitessBackupScheduleTemplate{
				Strategy: []planetscalev2.VitessBackupScheduleStrategy{clusterStrategy, ksStrategy},
			},
		},
	}

	result, err := r.expandStrategy(context.Background(), clusterStrategy, vbsc, mustBuildExpansionContext(t, r, vbsc))
	require.NoError(t, err)
	// customer excluded from Cluster scope because same schedule has Keyspace scope for it
	require.Len(t, result, 1, "expected 1 strategy (self-exclusion of customer)")
	require.Equal(t, "commerce", result[0].Keyspace)
}

func TestExpandStrategy_ShardScopeDoesNotExclude(t *testing.T) {
	scheme := newScheme()
	ks1 := &planetscalev2.VitessKeyspace{
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
	ks2 := &planetscalev2.VitessKeyspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster-customer",
			Namespace: "default",
			Labels:    map[string]string{planetscalev2.ClusterLabel: "test-cluster"},
		},
		Spec: planetscalev2.VitessKeyspaceSpec{
			VitessKeyspaceTemplate: planetscalev2.VitessKeyspaceTemplate{Name: "customer"},
		},
		Status: planetscalev2.VitessKeyspaceStatus{
			Shards: map[string]planetscalev2.VitessKeyspaceShardStatus{"-80": {}, "80-": {}},
		},
	}

	// A Shard-scope strategy for customer should NOT exclude customer from Cluster scope
	otherSchedule := &planetscalev2.VitessBackupSchedule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hot-shard",
			Namespace: "default",
			Labels:    map[string]string{planetscalev2.ClusterLabel: "test-cluster"},
		},
		Spec: planetscalev2.VitessBackupScheduleSpec{
			Cluster: "test-cluster",
			VitessBackupScheduleTemplate: planetscalev2.VitessBackupScheduleTemplate{
				Name:     "hot-shard",
				Schedule: "0 */6 * * *",
				Strategy: []planetscalev2.VitessBackupScheduleStrategy{
					{
						Name:     "customer-hot",
						Scope:    planetscalev2.BackupScopeShard,
						Keyspace: "customer",
						Shard:    "-80",
					},
				},
				Resources: corev1.ResourceRequirements{},
			},
		},
	}

	r := &ReconcileVitessBackupsSchedule{
		client: fake.NewClientBuilder().WithScheme(scheme).
			WithStatusSubresource(&planetscalev2.VitessKeyspace{}, &planetscalev2.VitessBackupSchedule{}).
			WithObjects(ks1, ks2, otherSchedule).Build(),
	}

	strategy := planetscalev2.VitessBackupScheduleStrategy{
		Name:  "all",
		Scope: planetscalev2.BackupScopeCluster,
	}
	vbsc := planetscalev2.VitessBackupSchedule{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster-daily", Namespace: "default", Labels: map[string]string{planetscalev2.ClusterLabel: "test-cluster"}},
		Spec: planetscalev2.VitessBackupScheduleSpec{
			Cluster: "test-cluster",
			VitessBackupScheduleTemplate: planetscalev2.VitessBackupScheduleTemplate{
				Strategy: []planetscalev2.VitessBackupScheduleStrategy{strategy},
			},
		},
	}

	result, err := r.expandStrategy(context.Background(), strategy, vbsc, mustBuildExpansionContext(t, r, vbsc))
	require.NoError(t, err)
	// Shard-scope does NOT exclude, so all 3 shards (1 commerce + 2 customer) should appear
	require.Len(t, result, 3, "shard-scope does not exclude")
}

func TestExpandStrategy_EmptyKeyspace(t *testing.T) {
	scheme := newScheme()
	// Keyspace exists but has no shards yet
	ks := &planetscalev2.VitessKeyspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster-commerce",
			Namespace: "default",
			Labels:    map[string]string{planetscalev2.ClusterLabel: "test-cluster"},
		},
		Spec: planetscalev2.VitessKeyspaceSpec{
			VitessKeyspaceTemplate: planetscalev2.VitessKeyspaceTemplate{Name: "commerce"},
		},
		Status: planetscalev2.VitessKeyspaceStatus{
			Shards: map[string]planetscalev2.VitessKeyspaceShardStatus{},
		},
	}

	r := &ReconcileVitessBackupsSchedule{
		client: fake.NewClientBuilder().WithScheme(scheme).
			WithStatusSubresource(&planetscalev2.VitessKeyspace{}).
			WithObjects(ks).Build(),
	}

	strategy := planetscalev2.VitessBackupScheduleStrategy{
		Name:     "commerce-all",
		Scope:    planetscalev2.BackupScopeKeyspace,
		Keyspace: "commerce",
	}
	vbsc := planetscalev2.VitessBackupSchedule{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
		Spec:       planetscalev2.VitessBackupScheduleSpec{Cluster: "test-cluster"},
	}

	result, err := r.expandStrategy(context.Background(), strategy, vbsc, mustBuildExpansionContext(t, r, vbsc))
	require.NoError(t, err)
	require.Empty(t, result, "expected 0 strategies for empty keyspace")
}

func TestExpandStrategy_NamingUniqueness(t *testing.T) {
	scheme := newScheme()
	ks := &planetscalev2.VitessKeyspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster-commerce",
			Namespace: "default",
			Labels:    map[string]string{planetscalev2.ClusterLabel: "test-cluster"},
		},
		Spec: planetscalev2.VitessKeyspaceSpec{
			VitessKeyspaceTemplate: planetscalev2.VitessKeyspaceTemplate{Name: "commerce"},
		},
		Status: planetscalev2.VitessKeyspaceStatus{
			Shards: map[string]planetscalev2.VitessKeyspaceShardStatus{
				"-40":   {},
				"40-80": {},
				"80-c0": {},
				"c0-":   {},
			},
		},
	}

	r := &ReconcileVitessBackupsSchedule{
		client: fake.NewClientBuilder().WithScheme(scheme).
			WithStatusSubresource(&planetscalev2.VitessKeyspace{}).
			WithObjects(ks).Build(),
	}

	strategy := planetscalev2.VitessBackupScheduleStrategy{
		Name:     "commerce-all",
		Scope:    planetscalev2.BackupScopeKeyspace,
		Keyspace: "commerce",
	}
	vbsc := planetscalev2.VitessBackupSchedule{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
		Spec:       planetscalev2.VitessBackupScheduleSpec{Cluster: "test-cluster"},
	}

	result, err := r.expandStrategy(context.Background(), strategy, vbsc, mustBuildExpansionContext(t, r, vbsc))
	require.NoError(t, err)
	require.Len(t, result, 4)

	names := make(map[string]bool)
	for _, s := range result {
		require.False(t, names[s.Name], "duplicate strategy name: %s", s.Name)
		names[s.Name] = true
	}
}

func TestBuildStrategyExpansionContextCachesShardsByKeyspace(t *testing.T) {
	scheme := newScheme()
	ks1 := &planetscalev2.VitessKeyspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster-commerce",
			Namespace: "default",
			Labels:    map[string]string{planetscalev2.ClusterLabel: "test-cluster"},
		},
		Spec: planetscalev2.VitessKeyspaceSpec{
			VitessKeyspaceTemplate: planetscalev2.VitessKeyspaceTemplate{Name: "commerce"},
		},
		Status: planetscalev2.VitessKeyspaceStatus{
			Shards: map[string]planetscalev2.VitessKeyspaceShardStatus{"-80": {}, "80-": {}},
		},
	}

	r := &ReconcileVitessBackupsSchedule{
		client: fake.NewClientBuilder().WithScheme(scheme).
			WithStatusSubresource(&planetscalev2.VitessKeyspace{}).
			WithObjects(ks1).Build(),
	}

	vbsc := planetscalev2.VitessBackupSchedule{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
		Spec:       planetscalev2.VitessBackupScheduleSpec{Cluster: "test-cluster"},
	}

	ctx := mustBuildExpansionContext(t, r, vbsc)
	require.Equal(t, []string{"-80", "80-"}, ctx.shardsByKeyspace["commerce"])
	require.Len(t, ctx.allKeyspaces, 1)
}

func TestShardReadyForScheduledBackup(t *testing.T) {
	tests := []struct {
		name   string
		status planetscalev2.VitessShardStatus
		want   bool
	}{
		{name: "ready shard", status: readyShardStatus(), want: true},
		{name: "missing master", status: planetscalev2.VitessShardStatus{HasInitialBackup: corev1.ConditionTrue, Tablets: readyShardStatus().Tablets}},
		{name: "missing initial backup", status: planetscalev2.VitessShardStatus{HasMaster: corev1.ConditionTrue, Tablets: readyShardStatus().Tablets}},
		{name: "unready tablet", status: planetscalev2.VitessShardStatus{HasMaster: corev1.ConditionTrue, HasInitialBackup: corev1.ConditionTrue, Tablets: map[string]planetscalev2.VitessTabletStatus{"zone1-1": {Ready: corev1.ConditionFalse}}}},
		{name: "no tablets", status: planetscalev2.VitessShardStatus{HasMaster: corev1.ConditionTrue, HasInitialBackup: corev1.ConditionTrue}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, shardReadyForScheduledBackup(planetscalev2.VitessShard{Status: tt.status}))
		})
	}
}

func TestCreateJobPodWaitsForShardBootstrap(t *testing.T) {
	initBackup := true
	vts := &planetscalev2.VitessShard{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-commerce-x-x",
			Namespace: "default",
			Labels: map[string]string{
				planetscalev2.ClusterLabel:  "example",
				planetscalev2.KeyspaceLabel: "commerce",
			},
		},
		Spec: planetscalev2.VitessShardSpec{
			VitessShardTemplate: planetscalev2.VitessShardTemplate{
				TabletPools: []planetscalev2.VitessShardTabletPool{{
					Cell:     "zone1",
					Type:     planetscalev2.ReplicaPoolType,
					Replicas: 1,
					Vttablet: planetscalev2.VttabletSpec{},
					Mysqld:   &planetscalev2.MysqldSpec{},
				}},
				Replication: planetscalev2.VitessReplicationSpec{
					InitializeBackup: &initBackup,
				},
			},
			Images: planetscalev2.VitessKeyspaceImages{
				Vtbackup: "vitess/lite:mysql80",
				Vttablet: "vitess/lite:mysql80",
				Vtorc:    "vitess/lite:mysql80",
				Mysqld: &planetscalev2.MysqldImage{
					Mysql80Compatible: "vitess/lite:mysql80",
				},
			},
			KeyRange:        planetscalev2.VitessKeyRange{},
			BackupLocations: []planetscalev2.VitessBackupLocation{{Name: ""}},
		},
		Status: planetscalev2.VitessShardStatus{
			HasMaster:        corev1.ConditionTrue,
			HasInitialBackup: corev1.ConditionFalse,
			Tablets: map[string]planetscalev2.VitessTabletStatus{
				"zone1-1": {Ready: corev1.ConditionFalse},
			},
		},
	}

	r := &ReconcileVitessBackupsSchedule{
		client: fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(vts).Build(),
	}
	vbsc := planetscalev2.VitessBackupSchedule{ObjectMeta: metav1.ObjectMeta{Namespace: "default"}, Spec: planetscalev2.VitessBackupScheduleSpec{Cluster: "example"}}
	strategy := planetscalev2.VitessBackupScheduleStrategy{Name: "commerce-x", Keyspace: "commerce", Shard: "-"}
	_, _, err := r.createJobPod(context.Background(), vbsc, strategy, "test-job", planetscalev2.VitessKeyRange{}, map[string]string{})
	require.ErrorIs(t, err, errWaitingForShardBootstrap)
}

func TestCreateJobPodAllowsReadyShardWithoutCompletedBackupCRs(t *testing.T) {
	initBackup := true
	vts := &planetscalev2.VitessShard{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-commerce-x-x",
			Namespace: "default",
			Labels: map[string]string{
				planetscalev2.ClusterLabel:  "example",
				planetscalev2.KeyspaceLabel: "commerce",
			},
		},
		Spec: planetscalev2.VitessShardSpec{
			VitessShardTemplate: planetscalev2.VitessShardTemplate{
				TabletPools: []planetscalev2.VitessShardTabletPool{{
					Cell:     "zone1",
					Type:     planetscalev2.ReplicaPoolType,
					Replicas: 1,
					Vttablet: planetscalev2.VttabletSpec{},
					Mysqld:   &planetscalev2.MysqldSpec{},
				}},
				Replication: planetscalev2.VitessReplicationSpec{
					InitializeBackup: &initBackup,
				},
			},
			Images: planetscalev2.VitessKeyspaceImages{
				Vtbackup: "vitess/lite:mysql80",
				Vttablet: "vitess/lite:mysql80",
				Vtorc:    "vitess/lite:mysql80",
				Mysqld: &planetscalev2.MysqldImage{
					Mysql80Compatible: "vitess/lite:mysql80",
				},
			},
			KeyRange:        planetscalev2.VitessKeyRange{},
			BackupLocations: []planetscalev2.VitessBackupLocation{{Name: ""}},
		},
		Status: readyShardStatus(),
	}

	r := &ReconcileVitessBackupsSchedule{
		client: fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(vts).Build(),
	}
	vbsc := planetscalev2.VitessBackupSchedule{ObjectMeta: metav1.ObjectMeta{Namespace: "default"}, Spec: planetscalev2.VitessBackupScheduleSpec{Cluster: "example"}}
	strategy := planetscalev2.VitessBackupScheduleStrategy{Name: "commerce-x", Keyspace: "commerce", Shard: "-"}
	pod, spec, err := r.createJobPod(context.Background(), vbsc, strategy, "test-job", planetscalev2.VitessKeyRange{}, map[string]string{})
	require.NoError(t, err)
	require.NotNil(t, pod)
	require.NotNil(t, spec)
	require.False(t, spec.InitialBackup)
}
