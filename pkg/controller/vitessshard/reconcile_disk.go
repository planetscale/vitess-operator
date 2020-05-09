package vitessshard

import (
	"context"

	v1 "k8s.io/api/core/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"vitess.io/vitess/go/vt/topo/topoproto"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/results"
	"planetscale.dev/vitess-operator/pkg/operator/rollout"
)

const (
	pvcDiskSizeAnnotation = "planetscale.com/pvc-filesystem-resize"
)

func (r *ReconcileVitessShard) reconcileDisk(ctx context.Context, vts *planetscalev2.VitessShard) (reconcile.Result, error) {
	resultBuilder := &results.Builder{}
	anythingChanged := false
	tabletPods, err := r.tabletPodsFromShard(ctx, vts)
	if err != nil {
		return resultBuilder.Error(err)
	}

	for i  := range vts.Spec.TabletPools {
		tabletPool := &vts.Spec.TabletPools[i]
		requestedDiskQuantity := tabletPool.DataVolumeClaimTemplate.Resources.Requests[v1.ResourceStorage]
		requestedDisk := requestedDiskQuantity.String()
		poolTablets, err := tabletsForPool(vts, tabletPool.Cell, string(tabletPool.Type))
		if err != nil {
			return resultBuilder.Error(err)
		}

		for _, tabletKey := range poolTablets {
			pod, ok := tabletPods[tabletKey]
			if !ok {
				continue
			}

			pvc, err := r.claimForTabletPod(ctx, pod)
			if err != nil {
				return resultBuilder.Error(err)
			}

			// If the PVC does not have the FileSystemResizeCondition, bail out.
			if !checkPVCFileSystemResizeCondition(pvc) {
				return resultBuilder.Result()
			}

			// If we have reached this point in the loop, it indicates that there are disk size changes, so we
			// set the variable anythingChanged to true. If we successfully complete this loop without bailing,
			// we can be certain that we have disk size changes and that all required changes have been set.
			anythingChanged = true

			// If the pod does not have updates scheduled, bail out and wait until the scheduled annotation is applied.
			if !rollout.Scheduled(pod) {
				return resultBuilder.Result()
			}

			// Finally, if the PVC's disk spec does not equal the new requested disk, bail out.
			pvcDisk := pvc.Spec.Resources.Requests[v1.ResourceStorage]
			if pvcDisk.String() != requestedDisk {
				r.recorder.Eventf(vts, v1.EventTypeNormal, "RolloutCheckPaused", "Waiting for pvc %v to reflect updated disk.", pvc.Name)
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
	var claimName string
	for i := range pod.Spec.Volumes {
		volume := &pod.Spec.Volumes[i]
		if volume.PersistentVolumeClaim != nil && volume.Name == pod.Name {
			claimName = volume.Name
			break
		}
	}

	pvc := &v1.PersistentVolumeClaim{}
	namespacedClaim := apitypes.NamespacedName{
		Namespace: pod.Namespace,
		Name:      claimName,
	}
	err := r.client.Get(ctx, namespacedClaim, pvc)
	if err != nil {
		return nil, err
	}

	return pvc, nil
}

func tabletsForPool(vts *planetscalev2.VitessShard, poolCell string, poolType string) ([]string, error) {
	tabletKeys := make([]string, 0, len(vts.Status.Tablets))
	for key := range vts.Status.Tablets {
		tabletKeys = append(tabletKeys, key)
	}

	tabletsInCell := make([]string, 0, len(tabletKeys))
	for _, tabletKey := range tabletKeys {
		tablet := vts.Status.Tablets[tabletKey]
		tabletAlias, err := topoproto.ParseTabletAlias(tabletKey)
		if err != nil {
			return tabletKeys, err
		}

		if tablet.PoolType != poolType || tabletAlias.Cell != poolCell{
			continue
		}

		tabletsInCell = append(tabletsInCell, tabletKey)
	}

	return tabletsInCell, nil
}