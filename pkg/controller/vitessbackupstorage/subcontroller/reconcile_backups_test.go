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

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlclientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"vitess.io/vitess/go/vt/mysqlctl/backupstorage"
	mysqlctlerrors "vitess.io/vitess/go/vt/mysqlctl/errors"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/reconciler"
	"planetscale.dev/vitess-operator/pkg/operator/vitessbackup"
)

type fakeBackupStorage struct {
	backupsByDir map[string][]backupstorage.BackupHandle
	closeCalls   int
}

func (f *fakeBackupStorage) ListBackups(_ context.Context, dir string) ([]backupstorage.BackupHandle, error) {
	return f.backupsByDir[dir], nil
}

func (f *fakeBackupStorage) StartBackup(context.Context, string, string) (backupstorage.BackupHandle, error) {
	return nil, fmt.Errorf("not implemented")
}

func (f *fakeBackupStorage) RemoveBackup(context.Context, string, string) error {
	return fmt.Errorf("not implemented")
}

func (f *fakeBackupStorage) Close() error {
	f.closeCalls++
	return nil
}

func (f *fakeBackupStorage) WithParams(backupstorage.Params) backupstorage.BackupStorage {
	return f
}

type fakeBackupHandle struct {
	dir  string
	name string
	mysqlctlerrors.PerFileErrorRecorder
}

func (f *fakeBackupHandle) Directory() string {
	return f.dir
}

func (f *fakeBackupHandle) Name() string {
	return f.name
}

func (f *fakeBackupHandle) AddFile(context.Context, string, int64) (io.WriteCloser, error) {
	return nil, fmt.Errorf("not implemented")
}

func (f *fakeBackupHandle) EndBackup(context.Context) error {
	return fmt.Errorf("not implemented")
}

func (f *fakeBackupHandle) AbortBackup(context.Context) error {
	return fmt.Errorf("not implemented")
}

func (f *fakeBackupHandle) ReadFile(context.Context, string) (io.ReadCloser, error) {
	return nil, fmt.Errorf("not implemented")
}

func newFakeBackupHandle(dir, name string) backupstorage.BackupHandle {
	return &fakeBackupHandle{dir: dir, name: name}
}

func newFakeBackupName(t *testing.T, ts time.Time, uid uint32) string {
	t.Helper()
	alias := fmt.Sprintf("zone1-%010d", uid)
	return fmt.Sprintf("%s.%s", ts.Format(vitessbackup.TimestampFormat), alias)
}

func newTestShard(namespace, clusterName, keyspaceName string, keyRange planetscalev2.VitessKeyRange) *planetscalev2.VitessShard {
	return &planetscalev2.VitessShard{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      fmt.Sprintf("%s-%s", keyspaceName, keyRange.SafeName()),
			Labels: map[string]string{
				planetscalev2.ClusterLabel:  clusterName,
				planetscalev2.KeyspaceLabel: keyspaceName,
			},
		},
		Spec: planetscalev2.VitessShardSpec{
			Name:     keyRange.SafeName(),
			KeyRange: keyRange,
		},
	}
}

func newTestBackupStorageCR(namespace, clusterName, locationName string) *planetscalev2.VitessBackupStorage {
	return &planetscalev2.VitessBackupStorage{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "backup-storage",
			Labels: map[string]string{
				planetscalev2.ClusterLabel: clusterName,
			},
		},
		Spec: planetscalev2.VitessBackupStorageSpec{
			Location: planetscalev2.VitessBackupLocation{Name: locationName},
		},
	}
}

func newTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, planetscalev2.SchemeBuilder.AddToScheme(scheme))
	return scheme
}

func newTestReconciler(t *testing.T, scheme *runtime.Scheme, objs ...client.Object) (*ReconcileVitessBackupStorage, client.Client, *record.FakeRecorder) {
	t.Helper()
	recorder := record.NewFakeRecorder(20)
	k8sClient := ctrlclientfake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	return &ReconcileVitessBackupStorage{
		client:     k8sClient,
		scheme:     scheme,
		recorder:   recorder,
		reconciler: reconciler.New(k8sClient, scheme, recorder),
	}, k8sClient, recorder
}

func collectEvents(t *testing.T, recorder *record.FakeRecorder) []string {
	t.Helper()
	var events []string
	for {
		select {
		case event := <-recorder.Events:
			events = append(events, event)
		default:
			return events
		}
	}
}

func TestReconcileBackups_MaxBackupsPerReconcile(t *testing.T) {
	scheme := newTestScheme(t)
	namespace := "test-ns"
	clusterName := "test-cluster"
	locationName := "s3-backups"
	shard := newTestShard(namespace, clusterName, "commerce", planetscalev2.VitessKeyRange{Start: "", End: "80"})

	oldLimit := *maxBackupsPerReconcile
	oldGetBackupStorage := getBackupStorage
	t.Cleanup(func() {
		*maxBackupsPerReconcile = oldLimit
		getBackupStorage = oldGetBackupStorage
	})

	baseTime := time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)

	t.Run("below limit succeeds and updates total count", func(t *testing.T) {
		*maxBackupsPerReconcile = 10
		vbs := newTestBackupStorageCR(namespace, clusterName, locationName)
		r, k8sClient, _ := newTestReconciler(t, scheme, shard)
		backupDir := fmt.Sprintf("%s/%s", shard.Labels[planetscalev2.KeyspaceLabel], shard.Spec.Name)
		storage := &fakeBackupStorage{backupsByDir: map[string][]backupstorage.BackupHandle{
			backupDir: {
				newFakeBackupHandle(backupDir, newFakeBackupName(t, baseTime, 100)),
				newFakeBackupHandle(backupDir, newFakeBackupName(t, baseTime.Add(time.Minute), 101)),
			},
		}}
		getBackupStorage = func() (backupstorage.BackupStorage, error) { return storage, nil }

		vbsCopy := vbs.DeepCopy()
		result, err := r.reconcileBackups(t.Context(), vbsCopy)
		require.NoError(t, err)
		require.False(t, result.Requeue)
		require.EqualValues(t, 2, vbsCopy.Status.TotalBackupCount)
		require.Equal(t, 1, storage.closeCalls)

		backupList := &planetscalev2.VitessBackupList{}
		require.NoError(t, k8sClient.List(t.Context(), backupList, &client.ListOptions{Namespace: namespace}))
		require.Len(t, backupList.Items, 2)
	})

	t.Run("exactly at limit succeeds", func(t *testing.T) {
		*maxBackupsPerReconcile = 2
		vbs := newTestBackupStorageCR(namespace, clusterName, locationName)
		r, _, _ := newTestReconciler(t, scheme, shard)
		backupDir := fmt.Sprintf("%s/%s", shard.Labels[planetscalev2.KeyspaceLabel], shard.Spec.Name)
		storage := &fakeBackupStorage{backupsByDir: map[string][]backupstorage.BackupHandle{
			backupDir: {
				newFakeBackupHandle(backupDir, newFakeBackupName(t, baseTime, 200)),
				newFakeBackupHandle(backupDir, newFakeBackupName(t, baseTime.Add(time.Minute), 201)),
			},
		}}
		getBackupStorage = func() (backupstorage.BackupStorage, error) { return storage, nil }

		vbsCopy := vbs.DeepCopy()
		_, err := r.reconcileBackups(t.Context(), vbsCopy)
		require.NoError(t, err)
		require.EqualValues(t, 2, vbsCopy.Status.TotalBackupCount)
		require.Equal(t, 1, storage.closeCalls)
	})

	t.Run("above limit fails and does not apply partial total count", func(t *testing.T) {
		*maxBackupsPerReconcile = 1
		vbs := newTestBackupStorageCR(namespace, clusterName, locationName)
		r, k8sClient, recorder := newTestReconciler(t, scheme, shard)
		backupDir := fmt.Sprintf("%s/%s", shard.Labels[planetscalev2.KeyspaceLabel], shard.Spec.Name)
		storage := &fakeBackupStorage{backupsByDir: map[string][]backupstorage.BackupHandle{
			backupDir: {
				newFakeBackupHandle(backupDir, newFakeBackupName(t, baseTime, 300)),
				newFakeBackupHandle(backupDir, newFakeBackupName(t, baseTime.Add(time.Minute), 301)),
			},
		}}
		getBackupStorage = func() (backupstorage.BackupStorage, error) { return storage, nil }

		vbsCopy := vbs.DeepCopy()
		_, err := r.reconcileBackups(t.Context(), vbsCopy)
		require.Error(t, err)
		require.Contains(t, err.Error(), "backup inventory exceeded limit")
		require.Contains(t, err.Error(), "partial inventory results were not applied")
		require.Zero(t, vbsCopy.Status.TotalBackupCount)
		require.Equal(t, 1, storage.closeCalls)

		events := collectEvents(t, recorder)
		require.NotEmpty(t, events)
		require.Contains(t, strings.Join(events, "\n"), "InventoryLimitExceeded")
		require.Contains(t, strings.Join(events, "\n"), "reduce retained backups")

		backupList := &planetscalev2.VitessBackupList{}
		require.NoError(t, k8sClient.List(t.Context(), backupList, &client.ListOptions{Namespace: namespace}))
		require.Empty(t, backupList.Items)
	})

	t.Run("zero limit disables cap", func(t *testing.T) {
		*maxBackupsPerReconcile = 0
		vbs := newTestBackupStorageCR(namespace, clusterName, locationName)
		r, _, _ := newTestReconciler(t, scheme, shard)
		backupDir := fmt.Sprintf("%s/%s", shard.Labels[planetscalev2.KeyspaceLabel], shard.Spec.Name)
		storage := &fakeBackupStorage{backupsByDir: map[string][]backupstorage.BackupHandle{
			backupDir: {
				newFakeBackupHandle(backupDir, newFakeBackupName(t, baseTime, 400)),
				newFakeBackupHandle(backupDir, newFakeBackupName(t, baseTime.Add(time.Minute), 401)),
			},
		}}
		getBackupStorage = func() (backupstorage.BackupStorage, error) { return storage, nil }

		vbsCopy := vbs.DeepCopy()
		_, err := r.reconcileBackups(t.Context(), vbsCopy)
		require.NoError(t, err)
		require.EqualValues(t, 2, vbsCopy.Status.TotalBackupCount)
		require.Equal(t, 1, storage.closeCalls)
	})
}

func TestReconcileBackups_InventoryLimitPreventsChildReconcile(t *testing.T) {
	scheme := newTestScheme(t)
	namespace := "test-ns"
	clusterName := "test-cluster"
	locationName := "s3-backups"
	vbs := newTestBackupStorageCR(namespace, clusterName, locationName)
	shard := newTestShard(namespace, clusterName, "commerce", planetscalev2.VitessKeyRange{Start: "", End: "80"})

	oldLimit := *maxBackupsPerReconcile
	oldGetBackupStorage := getBackupStorage
	t.Cleanup(func() {
		*maxBackupsPerReconcile = oldLimit
		getBackupStorage = oldGetBackupStorage
	})

	baseTime := time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)
	backupDir := fmt.Sprintf("%s/%s", shard.Labels[planetscalev2.KeyspaceLabel], shard.Spec.Name)
	*maxBackupsPerReconcile = 1
	storage := &fakeBackupStorage{backupsByDir: map[string][]backupstorage.BackupHandle{
		backupDir: {
			newFakeBackupHandle(backupDir, newFakeBackupName(t, baseTime, 500)),
			newFakeBackupHandle(backupDir, newFakeBackupName(t, baseTime.Add(time.Minute), 501)),
		},
	}}
	getBackupStorage = func() (backupstorage.BackupStorage, error) { return storage, nil }

	// Seed an existing backup object that would be dangerous to partially reconcile/delete.
	existingBackup := &planetscalev2.VitessBackup{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "existing-backup",
			Labels: map[string]string{
				planetscalev2.ClusterLabel:  clusterName,
				planetscalev2.KeyspaceLabel: "commerce",
				planetscalev2.ShardLabel:    "-80",
				vitessbackup.LocationLabel:  locationName,
			},
		},
	}
	r, k8sClient, _ := newTestReconciler(t, scheme, shard, existingBackup)

	vbsCopy := vbs.DeepCopy()
	_, err := r.reconcileBackups(t.Context(), vbsCopy)
	require.Error(t, err)

	current := &planetscalev2.VitessBackup{}
	require.NoError(t, k8sClient.Get(t.Context(), client.ObjectKey{Namespace: namespace, Name: "existing-backup"}, current))

	backupList := &planetscalev2.VitessBackupList{}
	require.NoError(t, k8sClient.List(t.Context(), backupList, &client.ListOptions{Namespace: namespace}))
	require.Len(t, backupList.Items, 1)
}

var _ backupstorage.BackupHandle = (*fakeBackupHandle)(nil)
var _ backupstorage.BackupStorage = (*fakeBackupStorage)(nil)
var _ client.Object = (*planetscalev2.VitessBackupStorage)(nil)
