package vitessshard

import (
	"context"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"vitess.io/vitess/go/vt/topo/topoproto"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/results"
	"planetscale.dev/vitess-operator/pkg/operator/rollout"
)

const (
	pvcFilesystemResizeAnnotation = "planetscale.com/pvc-filesystem-resize"
)

func (r *ReconcileVitessShard) reconcileDisk(ctx context.Context, vts *planetscalev2.VitessShard) (reconcile.Result, error) {
	resultBuilder := &results.Builder{}

	// If the user has specified their disk resizes to be handled externally, wait for a manual rollout to apply changes.
	if *vts.Spec.UpdateStrategy.DataVolumeClaimResize == planetscalev2.ExternalDataVolumeClaimResizeType {
		return resultBuilder.Result()
	}

	// If we're already cascading a disk size update, we don't need to look further.
	if rollout.Cascading(vts) {
		return resultBuilder.Result()
	}

	anythingChanged := false
	tabletPods, err := r.tabletPodsFromShard(ctx, vts)
	if err != nil {
		return resultBuilder.Error(err)
	}

	for i  := range vts.Spec.TabletPools {
		tabletPool := &vts.Spec.TabletPools[i]
		if tabletPool.DataVolumeClaimTemplate == nil {
			continue
		}

		requestedDiskQuantity, ok := tabletPool.DataVolumeClaimTemplate.Resources.Requests[v1.ResourceStorage]
		if !ok {
			continue
		}

		poolTablets, err := tabletKeysForPool(vts, tabletPool.Cell, tabletPool.Type)
		if err != nil {
			return resultBuilder.Error(err)
		}

		for _, tabletKey := range poolTablets {
			pod, ok := tabletPods[tabletKey]
			if !ok {
				continue
			}

			pvc, err := r.claimForTabletPod(ctx, pod)
			if apierrors.IsNotFound(err) {
				continue
			} else if err != nil {
				return resultBuilder.Error(err)
			}

			// If the PVC's current size is the same as the requested size, continue.
			currentDisk := pvc.Status.Capacity[v1.ResourceStorage]
			if currentDisk.Value() == requestedDiskQuantity.Value() {
				continue
			}

			// If we have reached this point in the loop, it indicates that there are disk size changes, so we
			// set the variable anythingChanged to true. If we successfully complete this loop without bailing,
			// we can be certain that we have disk size changes and that all required changes have been set.
			anythingChanged = true

			// If the PVC's disk spec does not equal the new requested disk, bail out.
			pvcDisk := pvc.Spec.Resources.Requests[v1.ResourceStorage]
			if pvcDisk.Value() != requestedDiskQuantity.Value() {
				r.recorder.Eventf(vts, v1.EventTypeNormal, "PVCResizeWaiting", "Waiting for PVC %v spec to reflect desired disk size %v.", pvc.Name, requestedDiskQuantity.String())
				return resultBuilder.Result()
			}

			// If the PVC does not have the FileSystemResizeCondition, bail out.
			if !checkPVCFileSystemResizeCondition(pvc) {
				r.recorder.Eventf(vts, v1.EventTypeNormal, "PVCResizeWaiting", "Waiting for PVC %v to be ready for filesystem resize.", pvc.Name)
				return resultBuilder.Result()
			}

			// If the pod does not have updates scheduled, bail out and wait until the scheduled annotation is applied.
			if !rollout.Scheduled(pod) {
				r.recorder.Eventf(vts, v1.EventTypeNormal, "PVCResizeWaiting", "Waiting for pod %v to be scheduled for restart.", pod.Name)
				return resultBuilder.Result()
			}
		}
	}

	// If disk size has changed and the changes are all ready, mark the shard as ready to cascade. Otherwise, skip this.
	if anythingChanged {
		rollout.Cascade(vts)
		err := r.client.Update(ctx, vts)
		if err != nil {
			 return resultBuilder.Error(err)
		}
	}

	return resultBuilder.Result()
}

func (r *ReconcileVitessShard) claimForTabletPod(ctx context.Context, pod *v1.Pod) (*v1.PersistentVolumeClaim, error) {
	pvc := &v1.PersistentVolumeClaim{}
	pvcKey := client.ObjectKey{
		Namespace: pod.Namespace,
		Name:      pod.Name,
	}
	err := r.client.Get(ctx, pvcKey, pvc)
	if err != nil {
		return nil, err
	}

	return pvc, nil
}

func tabletKeysForPool(vts *planetscalev2.VitessShard, poolCell string, poolType planetscalev2.VitessTabletPoolType) ([]string, error) {
	tabletKeys := vts.Status.TabletAliases()

	tabletsInCell := make([]string, 0, len(tabletKeys))
	for _, tabletKey := range tabletKeys {
		tablet := vts.Status.Tablets[tabletKey]
		tabletAlias, err := topoproto.ParseTabletAlias(tabletKey)
		if err != nil {
			return nil, err
		}

		if tablet.PoolType != string(poolType) || tabletAlias.Cell != poolCell{
			continue
		}

		tabletsInCell = append(tabletsInCell, tabletKey)
	}

	return tabletsInCell, nil
}