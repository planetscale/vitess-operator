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
	"vitess.io/vitess/go/vt/mysqlctl/backupstorage"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/vitessbackup"
)

// Tests in this file mutate package-level state (getBackupStorage and
// lastCompleteTimestamp). Do not use t.Parallel() on any test or subtest.

func TestSetLastCompleteBackupMetric(t *testing.T) {
	startEarly := time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)
	finishEarly := time.Date(2026, time.March, 10, 12, 30, 0, 0, time.UTC)
	startLate := time.Date(2026, time.March, 10, 13, 0, 0, 0, time.UTC)
	finishLate := time.Date(2026, time.March, 10, 13, 10, 0, 0, time.UTC)

	t.Run("latest complete backup uses FinishedTime", func(t *testing.T) {
		lastCompleteTimestamp.Reset()
		t.Cleanup(lastCompleteTimestamp.Reset)

		descCh := make(chan *prometheus.Desc, 1)
		lastCompleteTimestamp.Describe(descCh)
		require.Contains(t, (<-descCh).String(), "vitess_operator_backup_last_complete_timestamp_seconds")

		setLastCompleteBackupMetric([]planetscalev2.VitessBackup{
			newMetricBackup(t, "cluster", "commerce", "x-80", "s3", startEarly, &finishEarly, true),
			newMetricBackup(t, "cluster", "commerce", "x-80", "s3", startLate, &finishLate, true),
		})
		require.Equal(t, float64(finishLate.Unix()), lastCompleteMetricValue(t, "cluster", "commerce", "x-80", "s3"))
		require.Len(t, collectLastComplete(t), 1)
	})

	t.Run("missing FinishedTime uses StartTime", func(t *testing.T) {
		lastCompleteTimestamp.Reset()
		t.Cleanup(lastCompleteTimestamp.Reset)

		setLastCompleteBackupMetric([]planetscalev2.VitessBackup{
			newMetricBackup(t, "cluster", "commerce", "x-80", "s3", startLate, nil, true),
		})
		require.Equal(t, float64(startLate.Unix()), lastCompleteMetricValue(t, "cluster", "commerce", "x-80", "s3"))
	})

	t.Run("incomplete backups are ignored", func(t *testing.T) {
		lastCompleteTimestamp.Reset()
		t.Cleanup(lastCompleteTimestamp.Reset)

		setLastCompleteBackupMetric([]planetscalev2.VitessBackup{
			newMetricBackup(t, "cluster", "commerce", "x-80", "s3", startEarly, &finishEarly, true),
			newMetricBackup(t, "cluster", "commerce", "x-80", "s3", startLate, &finishLate, false),
		})
		require.Equal(t, float64(finishEarly.Unix()), lastCompleteMetricValue(t, "cluster", "commerce", "x-80", "s3"))
	})

	t.Run("no complete backups omits the series", func(t *testing.T) {
		lastCompleteTimestamp.Reset()
		t.Cleanup(lastCompleteTimestamp.Reset)

		setLastCompleteBackupMetric([]planetscalev2.VitessBackup{
			newMetricBackup(t, "cluster", "commerce", "x-80", "s3", startLate, &finishLate, false),
		})
		require.Empty(t, collectLastComplete(t))
	})

	t.Run("shards and locations are independent", func(t *testing.T) {
		lastCompleteTimestamp.Reset()
		t.Cleanup(lastCompleteTimestamp.Reset)

		startOther := time.Date(2026, time.March, 11, 9, 0, 0, 0, time.UTC)
		finishOther := time.Date(2026, time.March, 11, 9, 15, 0, 0, time.UTC)
		setLastCompleteBackupMetric([]planetscalev2.VitessBackup{
			newMetricBackup(t, "cluster", "commerce", "x-80", "s3", startEarly, &finishEarly, true),
			newMetricBackup(t, "cluster", "commerce", "80-x", "s3", startLate, &finishLate, true),
			newMetricBackup(t, "cluster", "commerce", "x-80", "gcs", startOther, &finishOther, true),
		})
		require.Equal(t, float64(finishEarly.Unix()), lastCompleteMetricValue(t, "cluster", "commerce", "x-80", "s3"))
		require.Equal(t, float64(finishLate.Unix()), lastCompleteMetricValue(t, "cluster", "commerce", "80-x", "s3"))
		require.Equal(t, float64(finishOther.Unix()), lastCompleteMetricValue(t, "cluster", "commerce", "x-80", "gcs"))
		require.Len(t, collectLastComplete(t), 3)
	})
}

func TestReconcileBackups_LastCompleteBackupMetric(t *testing.T) {
	scheme := newTestScheme(t)
	namespace := "test-ns"
	clusterName := "test-cluster"
	locationName := "s3-backups"
	shard := newTestShard(namespace, clusterName, "commerce", planetscalev2.VitessKeyRange{Start: "", End: "80"})
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
		r, _, _ := newTestReconciler(t, scheme, shard, vbs)
		getBackupStorage = func() (backupstorage.BackupStorage, error) {
			return &fakeBackupStorage{backupsByDir: map[string][]backupstorage.BackupHandle{
				backupDir: {newManifestBackupHandle(backupDir, backupName, finished)},
			}}, nil
		}

		_, err := r.reconcileBackups(t.Context(), vbs)
		require.NoError(t, err)
		require.Equal(t, float64(finished.Unix()), lastCompleteMetricValue(t, clusterName, "commerce", "x-80", locationName))
	})

	t.Run("inventory failure leaves the metric untouched", func(t *testing.T) {
		lastCompleteTimestamp.Reset()
		lastCompleteTimestamp.WithLabelValues(clusterName, "commerce", "x-80", locationName).Set(float64(finished.Unix()))

		vbs := newTestBackupStorageCR(namespace, clusterName, locationName)
		r, _, _ := newTestReconciler(t, scheme, shard, vbs)
		getBackupStorage = func() (backupstorage.BackupStorage, error) {
			return nil, fmt.Errorf("storage unavailable")
		}

		_, err := r.reconcileBackups(t.Context(), vbs)
		require.Error(t, err)
		require.Equal(t, float64(finished.Unix()), lastCompleteMetricValue(t, clusterName, "commerce", "x-80", locationName))
		require.Len(t, collectLastComplete(t), 1)
	})

	t.Run("successful inventory removes stale series", func(t *testing.T) {
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
		require.Equal(t, float64(finished.Unix()), lastCompleteMetricValue(t, clusterName, "commerce", "x-80", locationName))

		// A later successful inventory that no longer sees the complete backup
		// (it was deleted from storage, so the VitessBackup object is removed)
		// should drop the series rather than leave a stale value or publish 0.
		getBackupStorage = func() (backupstorage.BackupStorage, error) {
			return &fakeBackupStorage{backupsByDir: map[string][]backupstorage.BackupHandle{}}, nil
		}
		_, err = r.reconcileBackups(t.Context(), vbs)
		require.NoError(t, err)
		require.Empty(t, collectLastComplete(t))
	})
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
	require.True(t, ok, "missing series cluster=%s keyspace=%s shard=%s location=%s", cluster, keyspace, shard, location)
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
