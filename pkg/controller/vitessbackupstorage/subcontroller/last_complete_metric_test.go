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

package subcontroller

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlclientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"vitess.io/vitess/go/vt/mysqlctl/backupstorage"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/reconciler"
	"planetscale.dev/vitess-operator/pkg/operator/vitessbackup"
)

// Tests in this file mutate package-level state (getBackupStorage and
// lastCompleteTimestamp). Do not use t.Parallel() on any test or subtest.

var (
	kr80  = planetscalev2.VitessKeyRange{Start: "", End: "80"}
	kr80x = planetscalev2.VitessKeyRange{Start: "80", End: ""}
)

func TestSetLastCompleteBackupMetric(t *testing.T) {
	startEarly := time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)
	finishEarly := time.Date(2026, time.March, 10, 12, 30, 0, 0, time.UTC)
	startLate := time.Date(2026, time.March, 10, 13, 0, 0, 0, time.UTC)
	finishLate := time.Date(2026, time.March, 10, 13, 10, 0, 0, time.UTC)
	finishLatest := time.Date(2026, time.March, 10, 13, 30, 0, 0, time.UTC)

	shard80 := metricTestShard("commerce", "-80", kr80, "s3")
	shard80x := metricTestShard("commerce", "80-", kr80x, "s3")
	shards := []planetscalev2.VitessShard{shard80, shard80x}

	t.Run("latest complete backup uses FinishedTime", func(t *testing.T) {
		lastCompleteTimestamp.Reset()
		t.Cleanup(lastCompleteTimestamp.Reset)

		descCh := make(chan *prometheus.Desc, 1)
		lastCompleteTimestamp.Describe(descCh)
		require.Contains(t, (<-descCh).String(), "vitess_operator_backup_last_complete_timestamp_seconds")

		// List the later-started backup first so "last one wins" would pick the
		// earlier backup and fail this assertion.
		setLastCompleteBackupMetric("cluster", "s3", []planetscalev2.VitessShard{shard80}, []planetscalev2.VitessBackup{
			newMetricBackup(t, "cluster", "commerce", kr80.SafeName(), "s3", startLate, &finishLate, true),
			newMetricBackup(t, "cluster", "commerce", kr80.SafeName(), "s3", startEarly, &finishEarly, true),
		})
		require.Equal(t, float64(finishLate.Unix()), lastCompleteMetricValue(t, "cluster", "commerce", "-80", "s3"))
		require.Len(t, collectLastComplete(t), 1)
	})

	t.Run("start-time selection beats later FinishedTime", func(t *testing.T) {
		lastCompleteTimestamp.Reset()
		t.Cleanup(lastCompleteTimestamp.Reset)

		// The later-started backup finished first. Max FinishedTime would
		// pick the earlier backup (finishLatest); start-time selection picks
		// the later start (finishLate). List the later-started backup first
		// so last-wins would also pick finishLatest.
		setLastCompleteBackupMetric("cluster", "s3", []planetscalev2.VitessShard{shard80}, []planetscalev2.VitessBackup{
			newMetricBackup(t, "cluster", "commerce", kr80.SafeName(), "s3", startLate, &finishLate, true),
			newMetricBackup(t, "cluster", "commerce", kr80.SafeName(), "s3", startEarly, &finishLatest, true),
		})
		require.Equal(t, float64(finishLate.Unix()), lastCompleteMetricValue(t, "cluster", "commerce", "-80", "s3"))
	})

	t.Run("missing FinishedTime uses StartTime", func(t *testing.T) {
		lastCompleteTimestamp.Reset()
		t.Cleanup(lastCompleteTimestamp.Reset)

		setLastCompleteBackupMetric("cluster", "s3", []planetscalev2.VitessShard{shard80}, []planetscalev2.VitessBackup{
			newMetricBackup(t, "cluster", "commerce", kr80.SafeName(), "s3", startLate, nil, true),
		})
		require.Equal(t, float64(startLate.Unix()), lastCompleteMetricValue(t, "cluster", "commerce", "-80", "s3"))
	})

	t.Run("incomplete backups are ignored", func(t *testing.T) {
		lastCompleteTimestamp.Reset()
		t.Cleanup(lastCompleteTimestamp.Reset)

		setLastCompleteBackupMetric("cluster", "s3", []planetscalev2.VitessShard{shard80}, []planetscalev2.VitessBackup{
			newMetricBackup(t, "cluster", "commerce", kr80.SafeName(), "s3", startEarly, &finishEarly, true),
			newMetricBackup(t, "cluster", "commerce", kr80.SafeName(), "s3", startLate, &finishLate, false),
		})
		require.Equal(t, float64(finishEarly.Unix()), lastCompleteMetricValue(t, "cluster", "commerce", "-80", "s3"))
	})

	t.Run("known shards without a complete backup are 0", func(t *testing.T) {
		lastCompleteTimestamp.Reset()
		t.Cleanup(lastCompleteTimestamp.Reset)

		setLastCompleteBackupMetric("cluster", "s3", shards, []planetscalev2.VitessBackup{
			newMetricBackup(t, "cluster", "commerce", kr80.SafeName(), "s3", startLate, &finishLate, false),
		})
		require.Equal(t, float64(0), lastCompleteMetricValue(t, "cluster", "commerce", "-80", "s3"))
		require.Equal(t, float64(0), lastCompleteMetricValue(t, "cluster", "commerce", "80-", "s3"))
		require.Len(t, collectLastComplete(t), 2)
	})

	t.Run("shards are independent and use Spec.Name labels", func(t *testing.T) {
		lastCompleteTimestamp.Reset()
		t.Cleanup(lastCompleteTimestamp.Reset)

		setLastCompleteBackupMetric("cluster", "s3", shards, []planetscalev2.VitessBackup{
			newMetricBackup(t, "cluster", "commerce", kr80.SafeName(), "s3", startEarly, &finishEarly, true),
			newMetricBackup(t, "cluster", "commerce", kr80x.SafeName(), "s3", startLate, &finishLate, true),
		})
		require.Equal(t, float64(finishEarly.Unix()), lastCompleteMetricValue(t, "cluster", "commerce", "-80", "s3"))
		require.Equal(t, float64(finishLate.Unix()), lastCompleteMetricValue(t, "cluster", "commerce", "80-", "s3"))
		require.NotContains(t, collectLastComplete(t), [4]string{"cluster", "commerce", "x-80", "s3"})
		require.NotContains(t, collectLastComplete(t), [4]string{"cluster", "commerce", "80-x", "s3"})
		require.Len(t, collectLastComplete(t), 2)
	})

	t.Run("only shards that back up to this location are reported", func(t *testing.T) {
		lastCompleteTimestamp.Reset()
		t.Cleanup(lastCompleteTimestamp.Reset)

		onA := metricTestShard("commerce", "-80", kr80, "a")
		onB := metricTestShard("commerce", "80-", kr80x, "b")
		noLocation := metricTestShard("customer", "-", planetscalev2.VitessKeyRange{})
		noLocation.Spec.TabletPools = []planetscalev2.VitessShardTabletPool{{Type: planetscalev2.ReplicaPoolType}}

		setLastCompleteBackupMetric("cluster", "b", []planetscalev2.VitessShard{onA, onB, noLocation}, []planetscalev2.VitessBackup{
			newMetricBackup(t, "cluster", "commerce", kr80.SafeName(), "b", startLate, &finishLate, true),
		})
		require.Equal(t, float64(0), lastCompleteMetricValue(t, "cluster", "commerce", "80-", "b"))
		require.Len(t, collectLastComplete(t), 1)
	})

	t.Run("pools with no backupLocationName use the location named empty", func(t *testing.T) {
		lastCompleteTimestamp.Reset()
		t.Cleanup(lastCompleteTimestamp.Reset)

		defaultShard := metricTestShard("commerce", "-80", kr80, "")
		setLastCompleteBackupMetric("cluster", "", []planetscalev2.VitessShard{defaultShard}, nil)
		require.Equal(t, float64(0), lastCompleteMetricValue(t, "cluster", "commerce", "-80", ""))

		setLastCompleteBackupMetric("cluster", "s3", []planetscalev2.VitessShard{defaultShard}, nil)
		require.Empty(t, collectLastComplete(t))
	})

	t.Run("a shard that leaves the inventory loses its series", func(t *testing.T) {
		lastCompleteTimestamp.Reset()
		t.Cleanup(lastCompleteTimestamp.Reset)

		backups := []planetscalev2.VitessBackup{
			newMetricBackup(t, "cluster", "commerce", kr80.SafeName(), "s3", startEarly, &finishEarly, true),
			newMetricBackup(t, "cluster", "commerce", kr80x.SafeName(), "s3", startLate, &finishLate, true),
		}
		setLastCompleteBackupMetric("cluster", "s3", shards, backups)
		require.Len(t, collectLastComplete(t), 2)

		// 80- is gone (for example after a reshard), but its complete backup
		// is still listed.
		setLastCompleteBackupMetric("cluster", "s3", []planetscalev2.VitessShard{shard80}, backups)
		require.Equal(t, float64(finishEarly.Unix()), lastCompleteMetricValue(t, "cluster", "commerce", "-80", "s3"))
		require.Len(t, collectLastComplete(t), 1)
	})
}

func TestReconcileBackups_LastCompleteBackupMetric(t *testing.T) {
	scheme := newTestScheme(t)
	namespace := "test-ns"
	clusterName := "test-cluster"
	locationName := "s3-backups"
	shard := newNamedTestShard(namespace, clusterName, "commerce", "-80", kr80, locationName)
	otherShard := newNamedTestShard(namespace, clusterName, "commerce", "80-", kr80x, locationName)
	backupDir := fmt.Sprintf("%s/%s", shard.Labels[planetscalev2.KeyspaceLabel], shard.Spec.Name)
	start := time.Date(2026, time.March, 10, 13, 0, 0, 0, time.UTC)
	finished := time.Date(2026, time.March, 10, 13, 10, 0, 0, time.UTC)
	backupName := newFakeBackupName(t, start, 100)

	oldGetBackupStorage := getBackupStorage
	t.Cleanup(func() {
		getBackupStorage = oldGetBackupStorage
		lastCompleteTimestamp.Reset()
	})

	t.Run("successful inventory sets the gauge", func(t *testing.T) {
		lastCompleteTimestamp.Reset()
		vbs := newTestBackupStorageCR(namespace, clusterName, locationName)
		r, _, _ := newTestReconciler(t, scheme, shard, otherShard, vbs)
		getBackupStorage = func() (backupstorage.BackupStorage, error) {
			return &fakeBackupStorage{backupsByDir: map[string][]backupstorage.BackupHandle{
				backupDir: {newManifestBackupHandle(backupDir, backupName, finished)},
			}}, nil
		}

		_, err := r.reconcileBackups(t.Context(), vbs)
		require.NoError(t, err)
		require.Equal(t, float64(finished.Unix()), lastCompleteMetricValue(t, clusterName, "commerce", "-80", locationName))
		require.Equal(t, float64(0), lastCompleteMetricValue(t, clusterName, "commerce", "80-", locationName))
		require.Len(t, collectLastComplete(t), 2)
	})

	t.Run("inventory failure leaves the metric untouched", func(t *testing.T) {
		lastCompleteTimestamp.Reset()
		lastCompleteTimestamp.WithLabelValues(clusterName, "commerce", "-80", locationName).Set(float64(finished.Unix()))

		vbs := newTestBackupStorageCR(namespace, clusterName, locationName)
		r, _, _ := newTestReconciler(t, scheme, shard, vbs)
		getBackupStorage = func() (backupstorage.BackupStorage, error) {
			return nil, fmt.Errorf("storage unavailable")
		}

		_, err := r.reconcileBackups(t.Context(), vbs)
		require.Error(t, err)
		require.Equal(t, float64(finished.Unix()), lastCompleteMetricValue(t, clusterName, "commerce", "-80", locationName))
		require.Len(t, collectLastComplete(t), 1)
	})

	t.Run("ReconcileObjectSet failure leaves the metric untouched", func(t *testing.T) {
		lastCompleteTimestamp.Reset()
		lastCompleteTimestamp.WithLabelValues(clusterName, "commerce", "-80", locationName).Set(float64(finished.Unix()))

		vbs := newTestBackupStorageCR(namespace, clusterName, locationName)
		r, _ := newTestReconcilerWithInterceptor(t, scheme, interceptor.Funcs{
			Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
				if _, ok := obj.(*planetscalev2.VitessBackup); ok {
					return fmt.Errorf("create failed")
				}
				return c.Create(ctx, obj, opts...)
			},
		}, shard, vbs)
		getBackupStorage = func() (backupstorage.BackupStorage, error) {
			return &fakeBackupStorage{backupsByDir: map[string][]backupstorage.BackupHandle{
				backupDir: {newManifestBackupHandle(backupDir, backupName, finished)},
			}}, nil
		}

		_, err := r.reconcileBackups(t.Context(), vbs)
		require.Error(t, err)
		require.Equal(t, float64(finished.Unix()), lastCompleteMetricValue(t, clusterName, "commerce", "-80", locationName))
		require.Len(t, collectLastComplete(t), 1)
	})

	t.Run("post-inventory List failure leaves the metric untouched", func(t *testing.T) {
		lastCompleteTimestamp.Reset()
		lastCompleteTimestamp.WithLabelValues(clusterName, "commerce", "-80", locationName).Set(float64(finished.Unix()))

		vbs := newTestBackupStorageCR(namespace, clusterName, locationName)
		vtbLists := 0
		r, _ := newTestReconcilerWithInterceptor(t, scheme, interceptor.Funcs{
			List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
				if _, ok := list.(*planetscalev2.VitessBackupList); ok {
					vtbLists++
					// ReconcileObjectSet lists once; the metric refresh lists again.
					if vtbLists >= 2 {
						return fmt.Errorf("VitessBackup list failed")
					}
				}
				return c.List(ctx, list, opts...)
			},
		}, shard, vbs)
		getBackupStorage = func() (backupstorage.BackupStorage, error) {
			return &fakeBackupStorage{backupsByDir: map[string][]backupstorage.BackupHandle{
				backupDir: {newManifestBackupHandle(backupDir, backupName, finished)},
			}}, nil
		}

		_, err := r.reconcileBackups(t.Context(), vbs)
		require.NoError(t, err)
		require.Equal(t, float64(finished.Unix()), lastCompleteMetricValue(t, clusterName, "commerce", "-80", locationName))
		require.Len(t, collectLastComplete(t), 1)
	})

	t.Run("successful inventory reports 0 when a known shard has no complete backup", func(t *testing.T) {
		lastCompleteTimestamp.Reset()
		vbs := newTestBackupStorageCR(namespace, clusterName, locationName)
		r, _, _ := newTestReconciler(t, scheme, shard, vbs)

		getBackupStorage = func() (backupstorage.BackupStorage, error) {
			return &fakeBackupStorage{backupsByDir: map[string][]backupstorage.BackupHandle{
				backupDir: {newManifestBackupHandle(backupDir, backupName, finished)},
			}}, nil
		}
		_, err := r.reconcileBackups(t.Context(), vbs)
		require.NoError(t, err)
		require.Equal(t, float64(finished.Unix()), lastCompleteMetricValue(t, clusterName, "commerce", "-80", locationName))

		// A later successful inventory that no longer sees the complete backup
		// (it was deleted from storage) still emits a 0 series for the shard.
		getBackupStorage = func() (backupstorage.BackupStorage, error) {
			return &fakeBackupStorage{backupsByDir: map[string][]backupstorage.BackupHandle{}}, nil
		}
		_, err = r.reconcileBackups(t.Context(), vbs)
		require.NoError(t, err)
		require.Equal(t, float64(0), lastCompleteMetricValue(t, clusterName, "commerce", "-80", locationName))
		require.Len(t, collectLastComplete(t), 1)
	})
}

func TestReconcileBackups_LastCompleteBackupMetricTwoLocations(t *testing.T) {
	scheme := newTestScheme(t)
	namespace := "test-ns"
	clusterName := "test-cluster"
	onA := newNamedTestShard(namespace, clusterName, "commerce", "-80", kr80, "a")
	onB := newNamedTestShard(namespace, clusterName, "commerce", "80-", kr80x, "b")
	start := time.Date(2026, time.March, 10, 13, 0, 0, 0, time.UTC)
	finished := time.Date(2026, time.March, 10, 13, 10, 0, 0, time.UTC)

	// A newer complete backup of 80- inventoried by location a, for example
	// from before its pool moved to b. It must not be reported for b.
	otherStart := time.Date(2026, time.March, 10, 14, 0, 0, 0, time.UTC)
	otherFinished := time.Date(2026, time.March, 10, 14, 10, 0, 0, time.UTC)
	fromA := newMetricBackup(t, clusterName, "commerce", kr80x.SafeName(), "a", otherStart, &otherFinished, true)
	fromA.Namespace = namespace
	fromA.Name = "backup-from-location-a"

	oldGetBackupStorage := getBackupStorage
	t.Cleanup(func() {
		getBackupStorage = oldGetBackupStorage
		lastCompleteTimestamp.Reset()
	})
	lastCompleteTimestamp.Reset()

	dirA := fmt.Sprintf("commerce/%s", onA.Spec.Name)
	dirB := fmt.Sprintf("commerce/%s", onB.Spec.Name)
	getBackupStorage = func() (backupstorage.BackupStorage, error) {
		return &fakeBackupStorage{backupsByDir: map[string][]backupstorage.BackupHandle{
			dirA: {newManifestBackupHandle(dirA, newFakeBackupName(t, start, 100), finished)},
			dirB: {newManifestBackupHandle(dirB, newFakeBackupName(t, start, 200), finished)},
		}}, nil
	}

	vbs := newTestBackupStorageCR(namespace, clusterName, "b")
	r, _, _ := newTestReconciler(t, scheme, onA, onB, &fromA, vbs)
	_, err := r.reconcileBackups(t.Context(), vbs)
	require.NoError(t, err)
	require.Equal(t, float64(finished.Unix()), lastCompleteMetricValue(t, clusterName, "commerce", "80-", "b"))
	require.Len(t, collectLastComplete(t), 1)
}

// metricTestShard returns a shard with one REPLICA pool per backup location.
func metricTestShard(keyspace, specName string, kr planetscalev2.VitessKeyRange, backupLocations ...string) planetscalev2.VitessShard {
	shard := planetscalev2.VitessShard{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				planetscalev2.KeyspaceLabel: keyspace,
			},
		},
		Spec: planetscalev2.VitessShardSpec{
			Name:     specName,
			KeyRange: kr,
		},
	}
	for _, location := range backupLocations {
		shard.Spec.TabletPools = append(shard.Spec.TabletPools, planetscalev2.VitessShardTabletPool{
			Type:               planetscalev2.ReplicaPoolType,
			BackupLocationName: location,
		})
	}
	return shard
}

func newNamedTestShard(namespace, clusterName, keyspaceName, specName string, kr planetscalev2.VitessKeyRange, backupLocation string) *planetscalev2.VitessShard {
	shard := newTestShard(namespace, clusterName, keyspaceName, kr)
	shard.Spec.Name = specName
	shard.Spec.TabletPools = []planetscalev2.VitessShardTabletPool{{
		Type:               planetscalev2.ReplicaPoolType,
		BackupLocationName: backupLocation,
	}}
	return shard
}

func newTestReconcilerWithInterceptor(t *testing.T, scheme *runtime.Scheme, funcs interceptor.Funcs, objs ...client.Object) (*ReconcileVitessBackupStorage, client.Client) {
	t.Helper()
	recorder := record.NewFakeRecorder(20)
	k8sClient := ctrlclientfake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).WithInterceptorFuncs(funcs).Build()
	return &ReconcileVitessBackupStorage{
		client:     k8sClient,
		scheme:     scheme,
		recorder:   recorder,
		reconciler: reconciler.New(k8sClient, scheme, recorder),
	}, k8sClient
}

func newMetricBackup(t *testing.T, cluster, keyspace, shard, location string, start time.Time, finished *time.Time, complete bool) planetscalev2.VitessBackup {
	t.Helper()
	vb := planetscalev2.VitessBackup{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				planetscalev2.ClusterLabel:  cluster,
				planetscalev2.KeyspaceLabel: keyspace,
				planetscalev2.ShardLabel:    shard,
				vitessbackup.LocationLabel:  location,
			},
		},
		Status: planetscalev2.VitessBackupStatus{
			StartTime: metav1.Time{Time: start},
			Complete:  complete,
		},
	}
	if finished != nil {
		ft := metav1.NewTime(*finished)
		vb.Status.FinishedTime = &ft
	}
	return vb
}

type manifestBackupHandle struct {
	fakeBackupHandle
	manifest string
}

func newManifestBackupHandle(dir, name string, finished time.Time) backupstorage.BackupHandle {
	return &manifestBackupHandle{
		fakeBackupHandle: fakeBackupHandle{dir: dir, name: name},
		manifest:         fmt.Sprintf(`{"BackupMethod":"builtin","FinishedTime":%q}`, finished.Format(time.RFC3339)),
	}
}

func (f *manifestBackupHandle) ReadFile(_ context.Context, filename string) (io.ReadCloser, error) {
	if filename != "MANIFEST" || f.manifest == "" {
		return nil, fmt.Errorf("not implemented")
	}
	return io.NopCloser(strings.NewReader(f.manifest)), nil
}

func lastCompleteMetricValue(t *testing.T, cluster, keyspace, shard, location string) float64 {
	t.Helper()
	values := collectLastComplete(t)
	value, ok := values[[4]string{cluster, keyspace, shard, location}]
	require.True(t, ok, "missing series cluster=%s keyspace=%s shard=%s location=%s in %v", cluster, keyspace, shard, location, values)
	return value
}

func collectLastComplete(t *testing.T) map[[4]string]float64 {
	t.Helper()
	ch := make(chan prometheus.Metric, 16)
	go func() {
		lastCompleteTimestamp.Collect(ch)
		close(ch)
	}()
	out := map[[4]string]float64{}
	for m := range ch {
		var pb dto.Metric
		require.NoError(t, m.Write(&pb))
		var key [4]string
		for _, lp := range pb.GetLabel() {
			switch lp.GetName() {
			case "cluster":
				key[0] = lp.GetValue()
			case "keyspace":
				key[1] = lp.GetValue()
			case "shard":
				key[2] = lp.GetValue()
			case "location":
				key[3] = lp.GetValue()
			}
		}
		out[key] = pb.GetGauge().GetValue()
	}
	return out
}
