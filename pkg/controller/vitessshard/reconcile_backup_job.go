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

package vitessshard

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/dump"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/desiredstatehash"
	"planetscale.dev/vitess-operator/pkg/operator/reconciler"
	"planetscale.dev/vitess-operator/pkg/operator/results"
	"planetscale.dev/vitess-operator/pkg/operator/update"
	"planetscale.dev/vitess-operator/pkg/operator/vitessbackup"
	"planetscale.dev/vitess-operator/pkg/operator/vttablet"
)

const (
	vtbackupDataVolumeHashAnnotation    = "planetscale.com/vtbackup-data-volume-hash"
	vtbackupDataVolumePodHashAnnotation = "planetscale.com/vtbackup-data-volume-pod-hash"
	vtbackupDataVolumeMountPath         = "/vt/vtdataroot"
	pvcBoundByControllerAnnotation      = "pv.kubernetes.io/bound-by-controller"
)

func (r *ReconcileVitessShard) reconcileBackupJob(ctx context.Context, vts *planetscalev2.VitessShard) (reconcile.Result, error) {
	resultBuilder := &results.Builder{}

	// Break early if we find we are using an externally managed MySQL, or if any tablet pools have nil for Mysqld,
	// because we should not be configuring backups in either case.
	if vts.Spec.UsingExternalDatastore() || !vts.Spec.AllPoolsUsingMysqld() {
		return resultBuilder.Result()
	}

	clusterName := vts.Labels[planetscalev2.ClusterLabel]
	keyspaceName := vts.Labels[planetscalev2.KeyspaceLabel]
	shardSafeName := vts.Spec.KeyRange.SafeName()

	labels := map[string]string{
		planetscalev2.ComponentLabel: planetscalev2.VtbackupComponentName,
		planetscalev2.ClusterLabel:   clusterName,
		planetscalev2.KeyspaceLabel:  keyspaceName,
		planetscalev2.ShardLabel:     shardSafeName,
		vitessbackup.TypeLabel:       vitessbackup.TypeInit,
	}

	allBackups, completeBackups, err := vitessbackup.GetBackups(ctx, vts.Namespace, clusterName, keyspaceName, shardSafeName,
		func(ctx context.Context, allBackupsList *planetscalev2.VitessBackupList, listOpts *client.ListOptions) error {
			return r.client.List(ctx, allBackupsList, listOpts)
		},
	)
	if err != nil {
		return resultBuilder.Error(err)
	}
	updateBackupStatus(vts, allBackups)

	// Generate keys (object names) for all desired backup Pods and PVCs.
	// Keep a map back from generated names to the backup specs.
	podKeys := []client.ObjectKey{}
	pvcKeys := []client.ObjectKey{}
	specMap := map[client.ObjectKey]*vttablet.BackupSpec{}

	// The object name for the initial backup Pod, if we end up needing one.
	initPodName := vttablet.InitialBackupPodName(clusterName, keyspaceName, vts.Spec.KeyRange)
	initPodKey := client.ObjectKey{
		Namespace: vts.Namespace,
		Name:      initPodName,
	}

	if len(completeBackups) == 0 && vts.Status.HasMaster != corev1.ConditionTrue {
		// Until we see at least one complete backup, we attempt to create an
		// "initial backup", which is a special imaginary backup created from
		// scratch (not from any tablet). If we're wrong and a backup exists
		// already, the idempotent vtbackup "initial backup" mode will just do
		// nothing and return success.
		initSpec := MakeVtbackupSpec(initPodKey, vts, labels, vitessbackup.TypeInit)
		if initSpec != nil {
			podKeys = append(podKeys, initPodKey)
			if initSpec.TabletSpec.DataVolumePVCSpec != nil {
				pvcKeys = append(pvcKeys, initPodKey)
			}
			specMap[initPodKey] = initSpec
		}
	} else {
		// We have at least one complete backup already.
		vts.Status.HasInitialBackup = corev1.ConditionTrue
	}

	// Reconcile vtbackup PVCs. Use the same key as the corresponding Pod,
	// but only if the Pod expects a PVC.
	err = r.reconciler.ReconcileObjectSet(ctx, vts, pvcKeys, labels, reconciler.Strategy{
		Kind: &corev1.PersistentVolumeClaim{},

		New: func(key client.ObjectKey) runtime.Object {
			pvc := vttablet.NewPVC(key, specMap[key].TabletSpec)
			updateVtbackupDataVolumePVCHash(&pvc.Annotations, specMap[key])
			return pvc
		},
		UpdateInPlace: func(key client.ObjectKey, obj runtime.Object) {
			pvc := obj.(*corev1.PersistentVolumeClaim)
			if hasVtbackupDataVolumePVCHash(pvc.Annotations) ||
				!vtbackupPVCDataVolumeMatches(pvc, specMap[key]) {
				return
			}
			updateVtbackupDataVolumePVCHash(&pvc.Annotations, specMap[key])
		},
		UpdateRecreate: func(key client.ObjectKey, obj runtime.Object) {
			// Delete the Pod first so it cannot keep using a PVC while the PVC
			// is being replaced.
			pod := &corev1.Pod{}
			if getErr := r.client.Get(ctx, key, pod); getErr == nil || !apierrors.IsNotFound(getErr) {
				return
			}

			pvc := obj.(*corev1.PersistentVolumeClaim)
			updateVtbackupDataVolumePVCHash(&pvc.Annotations, specMap[key])
		},
		PrepareForTurndown: func(key client.ObjectKey, obj runtime.Object) *planetscalev2.OrphanStatus {
			// Same as reconcileTablets, keep PVCs of Pods in any Phase
			pod := &corev1.Pod{}
			if getErr := r.client.Get(ctx, key, pod); getErr == nil || !apierrors.IsNotFound(getErr) {
				// If the get was successful, the Pod exists and we shouldn't delete the PVC.
				// If the get failed for any reason other than NotFound, we don't know if it's safe.
				return &planetscalev2.OrphanStatus{
					Reason:  "BackupRunning",
					Message: "Not deleting vtbackup PVC because vtbackup Pod still exists",
				}
			}
			return nil
		},
	})
	if err != nil {
		resultBuilder.Error(err)
	}

	// Reconcile vtbackup Pods.
	legacyDataVolumeLookupFailed := map[client.ObjectKey]struct{}{}
	err = r.reconciler.ReconcileObjectSet(ctx, vts, podKeys, labels, reconciler.Strategy{
		Kind: &corev1.Pod{},

		New: func(key client.ObjectKey) runtime.Object {
			return vttablet.NewBackupPod(key, specMap[key], vts.Spec.Images.Mysqld.Image())
		},
		UpdateInPlace: func(key client.ObjectKey, obj runtime.Object) {
			pod := obj.(*corev1.Pod)
			if hasVtbackupDataVolumePodHashes(pod.Annotations) ||
				!r.vtbackupPodDataVolumeMatches(pod, specMap[key]) {
				return
			}

			if specMap[key].TabletSpec.DataVolumePVCSpec != nil {
				pvc := &corev1.PersistentVolumeClaim{}
				if getErr := r.client.Get(ctx, key, pvc); getErr != nil {
					legacyDataVolumeLookupFailed[key] = struct{}{}
					_, _ = resultBuilder.Error(fmt.Errorf("verify legacy vtbackup data volume for Pod %s: %w", key, getErr))
					return
				}
				if !vtbackupPVCDataVolumeMatches(pvc, specMap[key]) {
					return
				}
			}

			updateVtbackupDataVolumePodHashes(&pod.Annotations, specMap[key])
		},
		UpdateRecreate: func(key client.ObjectKey, obj runtime.Object) {
			pod := obj.(*corev1.Pod)
			if pod.Status.Phase == corev1.PodRunning {
				return
			}
			if _, failed := legacyDataVolumeLookupFailed[key]; failed {
				return
			}
			updateVtbackupDataVolumePodHashes(&pod.Annotations, specMap[key])
		},
		Status: func(key client.ObjectKey, obj runtime.Object) {
			pod := obj.(*corev1.Pod)

			// If this status hook is telling us about the special init Pod,
			// we can update HasInitialBackup.
			if key == initPodKey {
				// If the Pod is Suceeded or Failed, we can update status.
				// Otherwise, we leave it as Unknown since we can't tell.
				switch pod.Status.Phase {
				case corev1.PodSucceeded:
					vts.Status.HasInitialBackup = corev1.ConditionTrue
				case corev1.PodFailed:
					vts.Status.HasInitialBackup = corev1.ConditionFalse
				}
			}
		},
		PrepareForTurndown: func(key client.ObjectKey, obj runtime.Object) *planetscalev2.OrphanStatus {
			// As soon as the new backup is complete, the backup policy logic
			// will say the vtbackup Pod is no longer needed. However, we still
			// need to give it a chance to finish running because it does
			// pruning of old backups after the new backup is complete.
			pod := obj.(*corev1.Pod)
			if pod.Status.Phase == corev1.PodRunning {
				return &planetscalev2.OrphanStatus{
					Reason:  "BackupRunning",
					Message: "Not deleting vtbackup Pod while it's still running",
				}
			}
			return nil
		},
	})
	if err != nil {
		resultBuilder.Error(err)
	}

	return resultBuilder.Result()
}

func MakeVtbackupSpec(key client.ObjectKey, vts *planetscalev2.VitessShard, parentLabels map[string]string, typ string) *vttablet.BackupSpec {
	// If we specifically set our cluster to avoid initial backups, bail early.
	if !*vts.Spec.Replication.InitializeBackup {
		return nil
	}

	if len(vts.Spec.TabletPools) == 0 {
		// No tablet pools are defined for this shard.
		// We don't know enough to make a vtbackup spec.
		return nil
	}

	// Make a vtbackup spec that's a similar shape to the first tablet pool.
	// This should give it enough resources to run mysqld and restore a backup,
	// since all tablets need to be able to do that, regardless of type.
	return vtbackupSpec(key, vts, parentLabels, &vts.Spec.TabletPools[0], typ)
}

func vtbackupSpec(key client.ObjectKey, vts *planetscalev2.VitessShard, parentLabels map[string]string, pool *planetscalev2.VitessShardTabletPool, backupType string) *vttablet.BackupSpec {
	keyspaceName := vts.Labels[planetscalev2.KeyspaceLabel]

	// Find the backup location for this pool.
	backupLocation := vts.Spec.BackupLocation(pool.BackupLocationName)
	if backupLocation == nil {
		// No backup location is configured, so we can't do anything.
		return nil
	}

	// Copy parent labels map and add child-specific labels.
	labels := map[string]string{
		vitessbackup.LocationLabel: backupLocation.Name,
		vitessbackup.TypeLabel:     backupType,
	}
	for k, v := range parentLabels {
		labels[k] = v
	}

	minBackupInterval := time.Duration(0)
	minRetentionTime := time.Duration(0)
	minRetentionCount := 1

	// Allocate a new map so we don't mutate inputs.
	annotations := map[string]string{}
	update.Annotations(&annotations, pool.Annotations)
	update.Annotations(&annotations, backupLocation.Annotations)

	dataVolumeClaimTemplate := pool.DataVolumeClaimTemplate
	if vts.Spec.Vtbackup != nil &&
		(backupType == vitessbackup.TypeInit || vts.Spec.Vtbackup.DataVolumeClaimTemplate != nil) {
		dataVolumeClaimTemplate = vts.Spec.Vtbackup.DataVolumeClaimTemplate
	}
	dataVolumeHash := desiredstatehash.NewBuilder()
	dataVolumeHash.AddString("pvc-spec", dump.ForHash(dataVolumeClaimTemplate))
	update.Annotations(&annotations, map[string]string{
		vtbackupDataVolumeHashAnnotation: dataVolumeHash.String(),
	})

	// Fill in the parts of a vttablet spec that make sense for vtbackup.
	tabletSpec := &vttablet.Spec{
		GlobalLockserver:         vts.Spec.GlobalLockserver,
		Labels:                   labels,
		Images:                   vts.Spec.Images,
		KeyRange:                 vts.Spec.KeyRange,
		Vttablet:                 &pool.Vttablet,
		Mysqld:                   pool.Mysqld,
		MysqldExporter:           pool.MysqldExporter,
		DataVolumePVCName:        key.Name,
		DataVolumePVCSpec:        dataVolumeClaimTemplate,
		KeyspaceName:             keyspaceName,
		DatabaseName:             vts.Spec.DatabaseName,
		DatabaseInitScriptSecret: vts.Spec.DatabaseInitScriptSecret,
		BackupLocation:           backupLocation,
		BackupEngine:             vts.Spec.BackupEngine,
		InitContainers:           pool.InitContainers,
		SidecarContainers:        pool.SidecarContainers,
		ExtraEnv:                 pool.ExtraEnv,
		ExtraVolumes:             pool.ExtraVolumes,
		ExtraVolumeMounts:        pool.ExtraVolumeMounts,
		Annotations:              annotations,
		Affinity:                 pool.Affinity,
		Tolerations:              pool.Tolerations,
		ImagePullSecrets:         vts.Spec.ImagePullSecrets,
	}

	backupSpec := &vttablet.BackupSpec{
		InitialBackup:     backupType == vitessbackup.TypeInit,
		MinBackupInterval: minBackupInterval,
		MinRetentionTime:  minRetentionTime,
		MinRetentionCount: minRetentionCount,

		TabletSpec: tabletSpec,
	}
	update.Annotations(&backupSpec.TabletSpec.Annotations, map[string]string{
		vtbackupDataVolumePodHashAnnotation: vtbackupPodDataVolumeHash(backupSpec),
	})
	return backupSpec
}

func updateVtbackupDataVolumePVCHash(annotations *map[string]string, spec *vttablet.BackupSpec) {
	update.Annotations(annotations, map[string]string{
		vtbackupDataVolumeHashAnnotation: spec.TabletSpec.Annotations[vtbackupDataVolumeHashAnnotation],
	})
}

func updateVtbackupDataVolumePodHashes(annotations *map[string]string, spec *vttablet.BackupSpec) {
	update.Annotations(annotations, map[string]string{
		vtbackupDataVolumeHashAnnotation:    spec.TabletSpec.Annotations[vtbackupDataVolumeHashAnnotation],
		vtbackupDataVolumePodHashAnnotation: spec.TabletSpec.Annotations[vtbackupDataVolumePodHashAnnotation],
	})
}

func hasVtbackupDataVolumePVCHash(annotations map[string]string) bool {
	_, ok := annotations[vtbackupDataVolumeHashAnnotation]
	return ok
}

func hasVtbackupDataVolumePodHashes(annotations map[string]string) bool {
	_, hasPVCHash := annotations[vtbackupDataVolumeHashAnnotation]
	_, hasPodHash := annotations[vtbackupDataVolumePodHashAnnotation]
	return hasPVCHash && hasPodHash
}

func vtbackupPodDataVolumeHash(spec *vttablet.BackupSpec) string {
	dataMount, dataSource, _ := vttablet.BackupPodDataVolume(spec.TabletSpec)
	hash := desiredstatehash.NewBuilder()
	hash.AddString("mount", dump.ForHash(dataMount))
	hash.AddString("source", dump.ForHash(dataSource))
	return hash.String()
}

func vtbackupPVCDataVolumeMatches(pvc *corev1.PersistentVolumeClaim, spec *vttablet.BackupSpec) bool {
	desired := spec.TabletSpec.DataVolumePVCSpec
	if desired == nil {
		return false
	}

	currentSpec := pvc.Spec.DeepCopy()
	desiredSpec := desired.DeepCopy()

	// When Kubernetes selects a PV, it assigns VolumeName and marks the PVC as
	// bound by the controller. A nil StorageClassName asks admission to select
	// the default class, which then appears in the stored PVC. VolumeMode defaults
	// to Filesystem, and the API server mirrors local DataSource and DataSourceRef
	// values on create.
	_, boundByController := pvc.Annotations[pvcBoundByControllerAnnotation]
	if desiredSpec.VolumeName == "" && boundByController {
		currentSpec.VolumeName = ""
	}
	if desiredSpec.StorageClassName == nil &&
		currentSpec.StorageClassName != nil && *currentSpec.StorageClassName != "" {
		currentSpec.StorageClassName = nil
	}
	defaultVtbackupPVCVolumeMode(currentSpec)
	defaultVtbackupPVCVolumeMode(desiredSpec)
	normalizeVtbackupPVCDataSources(currentSpec)
	normalizeVtbackupPVCDataSources(desiredSpec)

	return apiequality.Semantic.DeepEqual(currentSpec, desiredSpec)
}

func defaultVtbackupPVCVolumeMode(spec *corev1.PersistentVolumeClaimSpec) {
	if spec.VolumeMode != nil {
		return
	}
	mode := corev1.PersistentVolumeFilesystem
	spec.VolumeMode = &mode
}

func normalizeVtbackupPVCDataSources(spec *corev1.PersistentVolumeClaimSpec) {
	if spec.DataSourceRef != nil && spec.DataSourceRef.Namespace != nil && *spec.DataSourceRef.Namespace == "" {
		spec.DataSourceRef.Namespace = nil
	}

	if spec.DataSource != nil && spec.DataSourceRef == nil {
		spec.DataSourceRef = &corev1.TypedObjectReference{
			Kind: spec.DataSource.Kind,
			Name: spec.DataSource.Name,
		}
		if spec.DataSource.APIGroup != nil {
			apiGroup := *spec.DataSource.APIGroup
			spec.DataSourceRef.APIGroup = &apiGroup
		}
		return
	}

	if spec.DataSourceRef != nil && spec.DataSource == nil && spec.DataSourceRef.Namespace == nil {
		spec.DataSource = &corev1.TypedLocalObjectReference{
			Kind: spec.DataSourceRef.Kind,
			Name: spec.DataSourceRef.Name,
		}
		if spec.DataSourceRef.APIGroup != nil {
			apiGroup := *spec.DataSourceRef.APIGroup
			spec.DataSource.APIGroup = &apiGroup
		}
	}
}

func (r *ReconcileVitessShard) vtbackupPodDataVolumeMatches(pod *corev1.Pod, spec *vttablet.BackupSpec) bool {
	desiredPod := vttablet.NewBackupPod(
		client.ObjectKey{Namespace: pod.Namespace, Name: pod.Name},
		spec,
		spec.TabletSpec.Images.Mysqld.Image(),
	)
	if len(desiredPod.Spec.Containers) == 0 {
		return false
	}
	currentPod := pod.DeepCopy()
	r.scheme.Default(currentPod)
	r.scheme.Default(desiredPod)

	containerName := desiredPod.Spec.Containers[0].Name
	desiredMount, desiredSource, ok := vtbackupPodDataVolume(desiredPod, containerName)
	if !ok {
		return false
	}
	currentMount, currentSource, ok := vtbackupPodDataVolume(currentPod, containerName)
	if !ok {
		return false
	}

	return apiequality.Semantic.DeepEqual(currentMount, desiredMount) &&
		apiequality.Semantic.DeepEqual(currentSource, desiredSource)
}

func vtbackupPodDataVolume(pod *corev1.Pod, containerName string) (*corev1.VolumeMount, *corev1.VolumeSource, bool) {
	var dataMount *corev1.VolumeMount
	for i := range pod.Spec.Containers {
		container := &pod.Spec.Containers[i]
		if container.Name != containerName {
			continue
		}
		for j := range container.VolumeMounts {
			mount := &container.VolumeMounts[j]
			if mount.MountPath != vtbackupDataVolumeMountPath {
				continue
			}
			if dataMount != nil {
				return nil, nil, false
			}
			dataMount = mount
		}
		break
	}
	if dataMount == nil {
		return nil, nil, false
	}

	for i := range pod.Spec.Volumes {
		volume := &pod.Spec.Volumes[i]
		if volume.Name == dataMount.Name {
			return dataMount, &volume.VolumeSource, true
		}
	}
	return nil, nil, false
}

func updateBackupStatus(vts *planetscalev2.VitessShard, allBackups []planetscalev2.VitessBackup) {
	// If no backup locations are configured, there's nothing to do.
	if len(vts.Spec.BackupLocations) == 0 {
		return
	}

	// Initialize status for each backup location.
	locationStatus := map[string]*planetscalev2.ShardBackupLocationStatus{}
	for i := range vts.Spec.BackupLocations {
		location := &vts.Spec.BackupLocations[i]
		status := planetscalev2.NewShardBackupLocationStatus(location.Name)
		locationStatus[location.Name] = status
		vts.Status.BackupLocations = append(vts.Status.BackupLocations, status)
	}

	// Report stats on backups, grouped by location.
	for i := range allBackups {
		backup := &allBackups[i]
		locationName := backup.Labels[vitessbackup.LocationLabel]
		location := locationStatus[locationName]
		if location == nil {
			// This is not one of the locations we care about.
			continue
		}

		if backup.Status.Complete {
			location.CompleteBackups++

			if location.LatestCompleteBackupTime == nil || backup.Status.StartTime.After(location.LatestCompleteBackupTime.Time) {
				location.LatestCompleteBackupTime = &backup.Status.StartTime
			}
		} else {
			location.IncompleteBackups++
		}
	}
}
