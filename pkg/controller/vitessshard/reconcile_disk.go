package vitessshard

import (
	"context"
	"strings"

	v1 "k8s.io/api/core/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/results"
	"planetscale.dev/vitess-operator/pkg/operator/rollout"
)

const (
	pvcDiskSizeAnnotation = "pvc-disk-size"
)

func (r *ReconcileVitessShard) reconcileDisk(ctx context.Context, vts *planetscalev2.VitessShard) (reconcile.Result, error) {
	resultBuilder := &results.Builder{}
	changed := false
	tabletKeys := tabletKeysFromShard(vts)
	tabletPods, err := r.tabletPodsFromShard(ctx, vts)
	if err != nil {
		return resultBuilder.Error(err)
	}

	for i  := range vts.Spec.TabletPools {
		tabletPool := &vts.Spec.TabletPools[i]
		requestedDisk := tabletPool.DataVolumeClaimTemplate.Resources.Requests[v1.ResourceStorage].String()
		poolTablets := tabletsForPool(vts, tabletKeys, tabletPool.Cell, string(tabletPool.Type))

		for _, tabletKey := range poolTablets {
			pod, ok := tabletPods[tabletKey]
			if !ok {
				continue
			}

			tabletDiskSize, ok := pod.Annotations[pvcDiskSizeAnnotation]
			if !ok {
				// If it's never been set before, apply the disk size annotation.
				setDiskSizeAnnotation(pod, requestedDisk)
				continue
			}

			// If disk size is unchanged, move on to the next tablet.
			if tabletDiskSize == requestedDisk {
				continue
			}

			// If there are disk size changes, mark the shard as changed to cascade later.
			changed = true

			pvc, err := r.claimForTabletPod(ctx, pod)
			if err != nil {
				return resultBuilder.Error(err)
			}

			// Make sure that the tablet's PVC has the updated disk size before proceeding.
			if pvc.Spec.Resources.Requests[v1.ResourceStorage].String() != requestedDisk {
				r.recorder.Eventf(vts, v1.EventTypeNormal, "RolloutCheckPaused", "Waiting for pvc %v to reflect updated disk.", pvc.Name)
				return resultBuilder.Result()
			}

			// Set the disk size annotation to schedule this Pod for changes.
			setDiskSizeAnnotation(pod, requestedDisk)
		}
	}

	if changed {
		rollout.Cascade(vts)
	}

	return resultBuilder.Result()
}

func (r *ReconcileVitessShard) claimForTabletPod(ctx context.Context, pod *v1.Pod) (*v1.PersistentVolumeClaim, error) {
	var claimName string
	for i := range pod.Spec.Volumes {
		volume := &pod.Spec.Volumes[i]
		if volume.PersistentVolumeClaim != nil {
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

func tabletsForPool(vts *planetscalev2.VitessShard, tabletKeys []string, poolCell string, poolType string) []string {
	tabletsInCell := make([]string, 0, len(tabletKeys))
	for _, tabletKey := range tabletKeys {
		tablet := vts.Status.Tablets[tabletKey]
		tabletCell := strings.Split(tabletKey, "-")[0]

		if tablet.Type != poolType || tabletCell != poolCell{
			continue
		}

		tabletsInCell = append(tabletsInCell, tabletKey)
	}

	return tabletsInCell
}

func setDiskSizeAnnotation(pod *v1.Pod, requestedDisk string) {
	ann := pod.GetAnnotations()
	if ann == nil {
		ann = make(map[string]string, 1)
	}
	ann[pvcDiskSizeAnnotation] = requestedDisk
	pod.SetAnnotations(ann)
}