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
	"sort"
	"strconv"
	"time"

	topodatapb "vitess.io/vitess/go/vt/proto/topodata"
	"vitess.io/vitess/go/vt/topo/topoproto"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	podutil "k8s.io/kubernetes/pkg/api/v1/pod"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/drain"
	"planetscale.dev/vitess-operator/pkg/operator/k8s"
	"planetscale.dev/vitess-operator/pkg/operator/reconciler"
	"planetscale.dev/vitess-operator/pkg/operator/results"
	"planetscale.dev/vitess-operator/pkg/operator/rollout"
	"planetscale.dev/vitess-operator/pkg/operator/toposerver"
	"planetscale.dev/vitess-operator/pkg/operator/update"
	"planetscale.dev/vitess-operator/pkg/operator/vttablet"
)

const (
	// tabletAvailableTime is how long a tablet Pod must be consistently Ready
	// before it is considered Available. This accounts for the time it takes
	// for vtgates to discover that the tablet is Ready and update their routing
	// tables. If a tablet is Ready but vtgates don't know it yet, then it isn't
	// actually available for serving queries yet.
	tabletAvailableTime = 30 * time.Second

	// observedShardGenerationAnnotationKey is used to set the shard generation
	// that is observed at the time an UpdateInPlace is called for a pod.
	observedShardGenerationAnnotationKey = "planetscale.com/observed-shard-generation"
)

func (r *ReconcileVitessShard) reconcileTablets(ctx context.Context, vts *planetscalev2.VitessShard) (reconcile.Result, error) {
	resultBuilder := &results.Builder{}
	clusterName := vts.Labels[planetscalev2.ClusterLabel]

	labels := map[string]string{
		planetscalev2.ComponentLabel: planetscalev2.VttabletComponentName,
		planetscalev2.ClusterLabel:   vts.Labels[planetscalev2.ClusterLabel],
		planetscalev2.KeyspaceLabel:  vts.Labels[planetscalev2.KeyspaceLabel],
		planetscalev2.ShardLabel:     vts.Spec.KeyRange.SafeName(),
	}

	// Remember which cells we deploy any tablets in.
	deployedCells := map[string]struct{}{}
	defer func() {
		// Sort the list of cells so the order is consistent.
		vts.Status.Cells = make([]string, 0, len(deployedCells))
		for cellName := range deployedCells {
			vts.Status.Cells = append(vts.Status.Cells, cellName)
		}
		sort.Strings(vts.Status.Cells)
	}()

	// Compute the set of all desired tablets based on the config.
	tablets := vttabletSpecs(vts, labels)

	// Generate podKeys (object names) for all desired tablet pods and pvcKeys for desired PVCs.
	//
	// Keep a map back from generated names to the tablet specs.
	pvcKeys := make([]client.ObjectKey, 0, len(tablets))
	podKeys := make([]client.ObjectKey, 0, len(tablets))
	tabletMap := make(map[client.ObjectKey]*vttablet.Spec, len(tablets))
	for _, tablet := range tablets {
		podName := vttablet.PodName(clusterName, tablet.Alias)
		key := client.ObjectKey{Namespace: vts.Namespace, Name: podName}

		if tablet.DataVolumePVCSpec != nil {
			// We use the same name for the Pod and the main data volume PVC.
			tablet.DataVolumePVCName = podName

			pvcKeys = append(pvcKeys, key)
		}

		podKeys = append(podKeys, key)

		tabletMap[key] = tablet

		deployedCells[tablet.Alias.Cell] = struct{}{}

		// Initialize a status entry for every desired tablet, so it will be
		// listed even if we end up not having anything to report about it.
		vts.Status.Tablets[tablet.AliasStr] = planetscalev2.NewVitessTabletStatus(tablet.Type, tablet.Index)
	}

	// Reconcile vttablet PVCs. Note that we use the same keys as the corresponding Pods.
	err := r.reconciler.ReconcileObjectSet(ctx, vts, pvcKeys, labels, reconciler.Strategy{
		Kind: &corev1.PersistentVolumeClaim{},

		New: func(key client.ObjectKey) runtime.Object {
			tablet := tabletMap[key]

			// The PVC doesn't exist, so it can't be bound.
			vts.Status.Tablets[tablet.AliasStr].DataVolumeBound = corev1.ConditionFalse

			return vttablet.NewPVC(key, tablet)
		},
		UpdateInPlace: func(key client.ObjectKey, obj runtime.Object) {
			curObj := obj.(*corev1.PersistentVolumeClaim)
			vttablet.UpdatePVCInPlace(curObj, tabletMap[key])
		},
		Status: func(key client.ObjectKey, obj runtime.Object) {
			tablet := tabletMap[key]
			curObj := obj.(*corev1.PersistentVolumeClaim)

			vts.Status.Tablets[tablet.AliasStr].DataVolumeBound = k8s.ConditionStatus(curObj.Status.Phase == corev1.ClaimBound)
		},
		PrepareForTurndown: func(key client.ObjectKey, obj runtime.Object) *planetscalev2.OrphanStatus {
			// Make sure it's ok to delete this PVC. We gate this on whether the
			// corresponding Pod still exists. That way if we decide to keep a
			// Pod around (see the other PrepareForTurndown below), we won't try
			// to delete the PVC out from under it.
			pod := &corev1.Pod{}
			if getErr := r.client.Get(ctx, key, pod); getErr == nil || !apierrors.IsNotFound(getErr) {
				// If the get was successful, the Pod exists and we shouldn't delete the PVC.
				// If the get failed for any reason other than NotFound, we don't know if it's safe.
				return planetscalev2.NewOrphanStatus("PodExists", "not deleting tablet PVC because tablet Pod still exists")
			}
			return nil
		},
	})
	if err != nil {
		resultBuilder.Error(err)
	}

	// Reconcile vttablet Pods.
	err = r.reconciler.ReconcileObjectSet(ctx, vts, podKeys, labels, reconciler.Strategy{
		Kind: &corev1.Pod{},

		New: func(key client.ObjectKey) runtime.Object {
			tablet := tabletMap[key]
			tabletStatus := vts.Status.Tablets[tablet.AliasStr]

			// The Pod doesn't exist, so it can't be running or ready.
			tabletStatus.Running = corev1.ConditionFalse
			tabletStatus.Ready = corev1.ConditionFalse
			tabletStatus.Available = corev1.ConditionFalse

			return vttablet.NewPod(key, tablet)
		},
		UpdateInPlace: func(key client.ObjectKey, obj runtime.Object) {
			newObj := obj.(*corev1.Pod)
			tablet := tabletMap[key]
			vttablet.UpdatePodInPlace(newObj, tablet)
			newObj.Annotations[observedShardGenerationAnnotationKey] = strconv.FormatInt(vts.Generation, 10)
		},
		UpdateRollingRecreate: func(key client.ObjectKey, obj runtime.Object) {
			newObj := obj.(*corev1.Pod)
			tablet := tabletMap[key]
			vttablet.UpdatePod(newObj, tablet)
		},
		Status: func(key client.ObjectKey, obj runtime.Object) {
			pod := obj.(*corev1.Pod)
			tablet := tabletMap[key]
			tabletStatus := vts.Status.Tablets[tablet.AliasStr]

			tabletStatus.Running = k8s.ConditionStatus(pod.Status.Phase == corev1.PodRunning)
			if _, cond := podutil.GetPodCondition(&pod.Status, corev1.PodReady); cond != nil {
				tabletStatus.Ready = cond.Status
				tabletStatus.Available = tabletAvailableStatus(resultBuilder, pod, cond)
			}
			tabletStatus.PendingChanges = pod.Annotations[rollout.ScheduledAnnotation]

			observedShardGenerationVal := pod.Annotations[observedShardGenerationAnnotationKey]
			if observedShardGenerationVal == "" {
				return
			}
			observedShardGeneration, err := strconv.ParseInt(observedShardGenerationVal, 10, 64)
			if err != nil {
				return
			}

			if vts.Status.LowestPodGeneration == 0 || observedShardGeneration < vts.Status.LowestPodGeneration {
				vts.Status.LowestPodGeneration = observedShardGeneration
			}
		},
		OrphanStatus: func(key client.ObjectKey, obj runtime.Object, orphanStatus *planetscalev2.OrphanStatus) {
			curObj := obj.(*corev1.Pod)
			tabletAlias := vttablet.AliasFromPod(curObj)
			tabletAliasStr := topoproto.TabletAliasString(&tabletAlias)

			vts.Status.OrphanedTablets[tabletAliasStr] = orphanStatus

			// Since we're keeping this tablet, remember that we're still in that cell.
			deployedCells[tabletAlias.Cell] = struct{}{}
		},
		PrepareForTurndown: func(key client.ObjectKey, obj runtime.Object) *planetscalev2.OrphanStatus {
			// Don't hold our slot in the reconcile work queue for too long.
			ctx, cancel := context.WithTimeout(ctx, topoReconcileTimeout)
			defer cancel()

			curObj := obj.(*corev1.Pod)
			tabletAlias := vttablet.AliasFromPod(curObj)

			// Drain before turn-down.
			if !drain.Finished(curObj) {
				drain.Start(curObj, "turning down unwanted tablet")
				return planetscalev2.NewOrphanStatus("Draining", "waiting for the tablet to be drained before turn-down")
			}

			// Make sure the tablet is not the master.
			isMaster, err := isTabletMaster(ctx, vts, tabletAlias)
			if err != nil {
				return planetscalev2.NewOrphanStatus("MasterUnknown", "unable to determine whether this tablet is the master")
			}
			if isMaster {
				return planetscalev2.NewOrphanStatus("Master", "this tablet is the master")
			}

			// Make sure the desired tablets are healthy before removing one.
			// We don't want to risk causing more disruption if the shard isn't
			// at full strength. The reconciler will have already processed all
			// desired tablets before it starts trying to delete undesired tablets,
			// so we can assume Status is up to date for all desired tablets.
			for _, tablet := range vts.Status.Tablets {
				if tablet.Ready != corev1.ConditionTrue {
					return planetscalev2.NewOrphanStatus("ShardNotHealthy", "the remaining, desired tablets in the shard are not all healthy")
				}
			}

			return nil
		},
	})
	if err != nil {
		resultBuilder.Error(err)
	} else {

	}

	return resultBuilder.Result()
}

// vttabletSpecs creates a list of vttablet Specs for a VitessShard.
func vttabletSpecs(vts *planetscalev2.VitessShard, parentLabels map[string]string) []*vttablet.Spec {
	keyspaceName := vts.Labels[planetscalev2.KeyspaceLabel]

	var tablets []*vttablet.Spec

	for poolIndex := range vts.Spec.TabletPools {
		pool := &vts.Spec.TabletPools[poolIndex]

		// Find the backup location for this pool.
		backupLocation := vts.Spec.BackupLocation(pool.BackupLocationName)

		// Within each pool, tablets are assigned a 1-based index.
		for tabletIndex := int32(1); tabletIndex <= pool.Replicas; tabletIndex++ {
			tabletAlias := topodatapb.TabletAlias{
				Cell: pool.Cell,
				Uid:  vttablet.UID(pool.Cell, keyspaceName, vts.Spec.KeyRange, pool.Type, uint32(tabletIndex)),
			}

			// Copy parent labels map and add tablet-specific labels.
			labels := make(map[string]string, len(parentLabels)+4)
			for k, v := range parentLabels {
				labels[k] = v
			}
			labels[planetscalev2.CellLabel] = tabletAlias.Cell
			labels[planetscalev2.TabletUidLabel] = strconv.FormatUint(uint64(tabletAlias.Uid), 10)
			labels[planetscalev2.TabletTypeLabel] = string(pool.Type)
			labels[planetscalev2.TabletIndexLabel] = strconv.FormatUint(uint64(tabletIndex), 10)

			// Merge ExtraVitessFlags into the tablet spec ExtraFlags field.
			extraFlags := make(map[string]string)
			update.StringMap(&extraFlags, vts.Spec.ExtraVitessFlags)
			update.StringMap(&extraFlags, pool.Vttablet.ExtraFlags)

			// Make shallow copy of pool.Vttablet to avoid mutating input.
			vttabletcpy := pool.Vttablet
			vttabletcpy.ExtraFlags = extraFlags

			annotations := map[string]string{
				drain.SupportedAnnotation: "ensure that the tablet is not a master",
			}
			update.Annotations(&annotations, pool.Annotations)

			tablets = append(tablets, &vttablet.Spec{
				GlobalLockserver:         vts.Spec.GlobalLockserver,
				Labels:                   labels,
				Images:                   vts.Spec.Images,
				ImagePullPolicies:        vts.Spec.ImagePullPolicies,
				Index:                    tabletIndex,
				KeyRange:                 vts.Spec.KeyRange,
				Alias:                    tabletAlias,
				AliasStr:                 topoproto.TabletAliasString(&tabletAlias),
				Zone:                     vts.Spec.ZoneMap[tabletAlias.Cell],
				Vttablet:                 &vttabletcpy,
				Mysqld:                   pool.Mysqld,
				ExternalDatastore:        pool.ExternalDatastore,
				Type:                     pool.Type,
				DataVolumePVCSpec:        pool.DataVolumeClaimTemplate,
				KeyspaceName:             keyspaceName,
				DatabaseInitScriptSecret: vts.Spec.DatabaseInitScriptSecret,
				EnableSemiSync:           vts.Spec.Replication.EnforceSemiSync,
				Annotations:              annotations,
				BackupLocation:           backupLocation,
				BackupEngine:             vts.Spec.BackupEngine,
				Affinity:                 pool.Affinity,
				ExtraEnv:                 pool.ExtraEnv,
				ExtraVolumes:             pool.ExtraVolumes,
				InitContainers:           pool.InitContainers,
				SidecarContainers:        pool.SidecarContainers,
				ExtraVolumeMounts:        pool.ExtraVolumeMounts,
			})
		}
	}

	return tablets
}

func isTabletMaster(ctx context.Context, vts *planetscalev2.VitessShard, tabletAlias topodatapb.TabletAlias) (bool, error) {
	ts, err := toposerver.Open(ctx, vts.Spec.GlobalLockserver)
	if err != nil {
		return true, err
	}
	defer ts.Close()

	// Only check the global shard record for the master alias.
	// We don't check the individual tablet's record (what the tablet thinks it is)
	// because it's important to allow deletion of false masters.
	keyspaceName := vts.Labels[planetscalev2.KeyspaceLabel]
	shard, err := ts.GetShard(ctx, keyspaceName, vts.Spec.Name)
	if err != nil {
		return true, err
	}

	return topoproto.TabletAliasEqual(shard.MasterAlias, &tabletAlias), nil
}

func tabletAvailableStatus(resultBuilder *results.Builder, pod *corev1.Pod, readyCond *corev1.PodCondition) corev1.ConditionStatus {
	// If the Pod is being deleted, we immediately mark it unavailable even
	// though it might not have transitioned to Unready yet.
	if pod.DeletionTimestamp != nil {
		return corev1.ConditionFalse
	}

	// If it's not Ready, it can't be Available.
	if readyCond.Status != corev1.ConditionTrue {
		return corev1.ConditionFalse
	}

	// A tablet is Available if it's been consistently Ready for long enough.
	// Note that this is sensitive to clock skew between us and the k8s master,
	// but it's the same trade-off that k8s controllers make to determine Pod
	// availability.
	readyDuration := time.Since(readyCond.LastTransitionTime.Time)
	if readyDuration >= tabletAvailableTime {
		return corev1.ConditionTrue
	}

	// The Pod is Ready now, but it hasn't been Ready for long enough to
	// consider it Available. We need to request a manual requeue to check again
	// later because we're just waiting for time to pass; we don't expect
	// anything in the Pod status to change and trigger a watch event.
	resultBuilder.RequeueAfter(tabletAvailableTime - readyDuration)
	return corev1.ConditionFalse
}
