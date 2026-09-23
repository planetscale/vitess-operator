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
	"fmt"
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"vitess.io/vitess/go/vt/topo/topoproto"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/vttablet"
)

var (
	volumeSnapshotGVK = schema.GroupVersionKind{
		Group:   "snapshot.storage.k8s.io",
		Version: "v1",
		Kind:    "VolumeSnapshot",
	}
	volumeSnapshotListGVK = schema.GroupVersionKind{
		Group:   "snapshot.storage.k8s.io",
		Version: "v1",
		Kind:    "VolumeSnapshotList",
	}
)

// snapshotsList classifies the VolumeSnapshots owned by a schedule for one
// shard, mirroring jobsList for the Job-based methods.
type snapshotsList struct {
	// ready snapshots have status.readyToUse == true.
	ready []*unstructured.Unstructured
	// failed snapshots carry a status.error and are not ready.
	failed []*unstructured.Unstructured
	// pending snapshots have no terminal status yet.
	pending []*unstructured.Unstructured
}

// getSnapshotsList lists the VolumeSnapshots previously created by this
// schedule for the given keyspace/shard and computes the most recent scheduled
// time, mirroring getJobsList.
func (r *ReconcileVitessBackupsSchedule) getSnapshotsList(
	ctx context.Context,
	req ctrl.Request,
	vbsc planetscalev2.VitessBackupSchedule,
	keyspace string,
	shardSafeName string,
) (snapshotsList, *time.Time, error) {
	existing := &unstructured.UnstructuredList{}
	existing.SetGroupVersionKind(volumeSnapshotListGVK)

	err := r.client.List(ctx, existing, client.InNamespace(req.Namespace), client.MatchingLabels{
		planetscalev2.BackupScheduleLabel: vbsc.Name,
		planetscalev2.ClusterLabel:        vbsc.Spec.Cluster,
		planetscalev2.KeyspaceLabel:       keyspace,
		planetscalev2.ShardLabel:          shardSafeName,
	})
	if err != nil && !apierrors.IsNotFound(err) {
		log.WithError(err).Error("unable to list VolumeSnapshots in cluster")
		return snapshotsList{}, nil, err
	}

	var snaps snapshotsList
	var mostRecentTime *time.Time

	for i := range existing.Items {
		snap := &existing.Items[i]
		switch {
		case snapshotReady(snap):
			snaps.ready = append(snaps.ready, snap)
		case snapshotHasError(snap):
			snaps.failed = append(snaps.failed, snap)
		default:
			snaps.pending = append(snaps.pending, snap)
		}

		timeRaw := snap.GetAnnotations()[scheduledTimeAnnotation]
		if timeRaw == "" {
			continue
		}
		scheduledTime, err := time.Parse(time.RFC3339, timeRaw)
		if err != nil {
			log.WithError(err).Errorf("unable to parse schedule time for existing VolumeSnapshot, found: %s", timeRaw)
			continue
		}
		if mostRecentTime == nil || mostRecentTime.Before(scheduledTime) {
			mostRecentTime = &scheduledTime
		}
	}
	return snaps, mostRecentTime, nil
}

func snapshotReady(snap *unstructured.Unstructured) bool {
	ready, found, err := unstructured.NestedBool(snap.Object, "status", "readyToUse")
	return err == nil && found && ready
}

func snapshotHasError(snap *unstructured.Unstructured) bool {
	_, found, err := unstructured.NestedMap(snap.Object, "status", "error")
	return err == nil && found
}

// cleanupSnapshotsWithLimit removes VolumeSnapshots ordered from oldest to
// newest, keeping at most "limit" snapshots, mirroring cleanupJobsWithLimit.
func (r *ReconcileVitessBackupsSchedule) cleanupSnapshotsWithLimit(ctx context.Context, snaps []*unstructured.Unstructured, limit int32) {
	if limit == -1 {
		return
	}

	sort.SliceStable(snaps, func(i, j int) bool {
		ti := snaps[i].GetCreationTimestamp()
		tj := snaps[j].GetCreationTimestamp()
		return ti.Before(&tj)
	})

	for i, snap := range snaps {
		if int32(i) >= int32(len(snaps))-limit {
			break
		}
		if err := r.client.Delete(ctx, snap, client.PropagationPolicy(metav1.DeletePropagationBackground)); err != nil {
			log.WithError(err).Errorf("unable to delete old VolumeSnapshot: %s", snap.GetName())
		} else {
			log.Infof("deleted old VolumeSnapshot: %s", snap.GetName())
		}
	}
}

// chooseSnapshotTarget picks the tablet whose data volume should be
// snapshotted. It prefers rdonly tablets over replicas, never picks the
// primary, and requires the tablet's data volume to be bound. Candidates are
// sorted so the choice is deterministic across reconciles.
func chooseSnapshotTarget(shard *planetscalev2.VitessShard) (string, error) {
	for _, want := range []string{"rdonly", "replica"} {
		var candidates []string
		for alias, tablet := range shard.Status.Tablets {
			if tablet.Type != want {
				continue
			}
			if tablet.DataVolumeBound != corev1.ConditionTrue {
				continue
			}
			candidates = append(candidates, alias)
		}
		if len(candidates) > 0 {
			sort.Strings(candidates)
			return candidates[0], nil
		}
	}
	return "", fmt.Errorf("no non-primary tablet with a bound data volume found in shard %s", shard.Name)
}

// createVolumeSnapshot builds a VolumeSnapshot object for the data volume of
// one non-primary tablet in the strategy's shard. The object is built as
// unstructured so the operator does not need to link the external-snapshotter
// client; the snapshot.storage.k8s.io/v1 CRDs must be installed in the cluster.
func (r *ReconcileVitessBackupsSchedule) createVolumeSnapshot(
	ctx context.Context,
	vbsc planetscalev2.VitessBackupSchedule,
	strategy planetscalev2.VitessBackupScheduleStrategy,
	meta metav1.ObjectMeta,
) (*unstructured.Unstructured, error) {
	shard, err := r.getShardFromKeyspace(ctx, vbsc.Namespace, vbsc.Spec.Cluster, strategy.Keyspace, strategy.Shard)
	if err != nil {
		return nil, err
	}
	targetAlias, err := chooseSnapshotTarget(&shard)
	if err != nil {
		return nil, err
	}
	tabletAlias, err := topoproto.ParseTabletAlias(targetAlias)
	if err != nil {
		return nil, fmt.Errorf("unable to parse tablet alias %q: %v", targetAlias, err)
	}
	// The tablet's data PVC shares its name with the tablet Pod.
	pvcName := vttablet.PodName(vbsc.Spec.Cluster, *tabletAlias)

	snap := &unstructured.Unstructured{}
	snap.SetGroupVersionKind(volumeSnapshotGVK)
	snap.SetNamespace(meta.Namespace)
	snap.SetName(meta.Name)
	snap.SetLabels(meta.Labels)
	snap.SetAnnotations(meta.Annotations)

	spec := map[string]interface{}{
		"source": map[string]interface{}{
			"persistentVolumeClaimName": pvcName,
		},
	}
	if strategy.VolumeSnapshotClassName != "" {
		spec["volumeSnapshotClassName"] = strategy.VolumeSnapshotClassName
	}
	snap.Object["spec"] = spec

	// Unlike Jobs, VolumeSnapshots are the backups themselves, so they are
	// deliberately NOT owned by the VitessBackupSchedule: deleting the schedule
	// must not cascade-delete the backups. Retention is enforced by the
	// label-based history cleanup instead. (CloudNativePG makes the same call
	// with its snapshotOwnerReference default of "none"; making this
	// configurable can come later.)
	return snap, nil
}
