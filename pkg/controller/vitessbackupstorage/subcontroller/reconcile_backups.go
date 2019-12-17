/*
Copyright 2019 PlanetScale Inc.

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
	"time"

	"planetscale.dev/vitess-operator/pkg/operator/reconciler"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apilabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"vitess.io/vitess/go/vt/mysqlctl"
	"vitess.io/vitess/go/vt/mysqlctl/backupstorage"
	_ "vitess.io/vitess/go/vt/mysqlctl/filebackupstorage"
	_ "vitess.io/vitess/go/vt/mysqlctl/gcsbackupstorage"
	_ "vitess.io/vitess/go/vt/mysqlctl/s3backupstorage"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/results"
	"planetscale.dev/vitess-operator/pkg/operator/vitessbackup"
)

const (
	readFileTimeout = 1 * time.Second
)

func (r *ReconcileVitessBackupStorage) reconcileBackups(ctx context.Context, vbs *planetscalev2.VitessBackupStorage) (reconcile.Result, error) {
	resultBuilder := results.Builder{}
	clusterName := vbs.Labels[planetscalev2.ClusterLabel]
	backupLocationName := vbs.Spec.Location.Name

	parentLabels := map[string]string{
		planetscalev2.ClusterLabel: clusterName,
		vitessbackup.LocationLabel: backupLocationName,
	}

	// Make a list of all desired VitessBackup object keys.
	keys := []client.ObjectKey{}
	// Keep a map from object key to the desired object content.
	backupObjects := map[client.ObjectKey]*planetscalev2.VitessBackup{}
	// Keep a map from object key to Vitess BackupHandle.
	backupHandles := map[client.ObjectKey]backupstorage.BackupHandle{}

	// List VitessShard objects for this cluster.
	shardList := &planetscalev2.VitessShardList{}
	clusterLabels := apilabels.Set{
		planetscalev2.ClusterLabel: clusterName,
	}
	listOpts := &client.ListOptions{
		Namespace:     vbs.Namespace,
		LabelSelector: apilabels.SelectorFromSet(clusterLabels),
	}
	err := r.client.List(ctx, listOpts, shardList)
	if err != nil {
		r.recorder.Eventf(vbs, corev1.EventTypeWarning, "ListFailed", "failed to list shards: %v", err)
		return resultBuilder.Error(err)
	}

	backupStorage, err := backupstorage.GetBackupStorage()
	if err != nil {
		r.recorder.Eventf(vbs, corev1.EventTypeWarning, "OpenFailed", "failed to open backup storage client: %v", err)
		return resultBuilder.Error(err)
	}
	defer backupStorage.Close()

	// List backups for each shard in this storage location.
	for i := range shardList.Items {
		shard := &shardList.Items[i]
		keyspaceName := shard.Labels[planetscalev2.KeyspaceLabel]

		// Note that this we don't include the cluster prefix. That's added
		// automatically by the backup storage client, based on flags that
		// should be passed in by the parent controller, just like they would be
		// passed to vttablet or vtbackup.
		backupDir := fmt.Sprintf("%s/%s", keyspaceName, shard.Spec.Name)
		backups, err := backupStorage.ListBackups(ctx, backupDir)
		if err != nil {
			r.recorder.Eventf(vbs, corev1.EventTypeWarning, "ListFailed", "failed to list backups for shard %v/%v: %v", keyspaceName, shard.Spec.Name, err)
			return resultBuilder.Error(err)
		}

		// Copy parent labels and add shard-specific labels.
		labels := map[string]string{
			planetscalev2.KeyspaceLabel: keyspaceName,
			planetscalev2.ShardLabel:    shard.Spec.KeyRange.SafeName(),
		}
		for k, v := range parentLabels {
			labels[k] = v
		}

		// For each backup, generate a VitessBackup object.
		for _, backup := range backups {
			// Unfortunately, the backup time is not stored anywhere except
			// the name, so we have to parse it.
			backupTime, tabletAlias, err := vitessbackup.ParseBackupName(backup.Name())
			if err != nil {
				r.recorder.Eventf(vbs, corev1.EventTypeWarning, "InvalidBackup", "invalid backup name %q: %v", backup.Name(), err)
				return resultBuilder.Error(err)
			}

			key := client.ObjectKey{
				Namespace: vbs.Namespace,
				Name:      vitessbackup.ObjectName(clusterName, backupLocationName, keyspaceName, shard.Spec.KeyRange, backupTime, tabletAlias),
			}
			keys = append(keys, key)
			backupHandles[key] = backup
			backupObjects[key] = &planetscalev2.VitessBackup{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Status: planetscalev2.VitessBackupStatus{
					StartTime:        metav1.NewTime(backupTime),
					StorageDirectory: backup.Directory(),
					StorageName:      backup.Name(),
				},
			}
			vbs.Status.TotalBackupCount++
		}
	}

	// Now reconcile the set of VitessBackup objects.
	err = r.reconciler.ReconcileObjectSet(ctx, vbs, keys, parentLabels, reconciler.Strategy{
		Kind: &planetscalev2.VitessBackup{},

		New: func(key client.ObjectKey) runtime.Object {
			vb := backupObjects[key]
			// Since we're creating a new object, we don't have any past status.
			// Check the status once. If the backup is complete, we won't check
			// again since backups don't change after they're completed, except
			// when they're deleted.
			if backup := backupHandles[key]; backup != nil {
				updateBackupStatus(ctx, vb, backup)
			}
			return vb
		},
		UpdateInPlace: func(key client.ObjectKey, obj runtime.Object) {
			vb := obj.(*planetscalev2.VitessBackup)

			if vb.Status.Complete {
				// If we already saw that the backup was complete at some point,
				// we'll assume it stays complete rather than constantly rechecking.
				return
			}

			// We haven't seen that this backup was completed yet. Check again.
			if backup := backupHandles[key]; backup != nil {
				updateBackupStatus(ctx, vb, backup)
			}
		},
	})
	if err != nil {
		resultBuilder.Error(err)
	}

	return resultBuilder.Result()
}

func updateBackupStatus(ctx context.Context, vb *planetscalev2.VitessBackup, backup backupstorage.BackupHandle) {
	// Check if it's complete by looking for the MANIFEST file.
	// If any errors are encountered, we assume it's not complete yet.
	readCtx, cancel := context.WithTimeout(ctx, readFileTimeout)
	defer cancel()

	manifest, err := mysqlctl.GetBackupManifest(readCtx, backup)
	if err != nil {
		return
	}

	// If we got here, the MANIFEST file exists and is valid JSON.
	// That's the only way to tell that a Vitess backup is complete.
	vb.Status.Complete = true
	vb.Status.Position = manifest.Position.String()
	vb.Status.Engine = manifest.BackupMethod
	if finishedTime, err := time.Parse(time.RFC3339, manifest.FinishedTime); err == nil {
		vb.Status.FinishedTime.Time = finishedTime
	} else {
		log.Warningf("Can't parse FinishedTime from MANIFEST of backup %v/%v: %v", backup.Directory(), backup.Name(), err)
	}
}
