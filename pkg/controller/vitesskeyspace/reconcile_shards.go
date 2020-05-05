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

package vitesskeyspace

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/reconciler"
	"planetscale.dev/vitess-operator/pkg/operator/rollout"
	"planetscale.dev/vitess-operator/pkg/operator/update"
	"planetscale.dev/vitess-operator/pkg/operator/vitessshard"
)

func (r *ReconcileVitessKeyspace) reconcileShards(ctx context.Context, vtk *planetscalev2.VitessKeyspace) error {
	clusterName := vtk.Labels[planetscalev2.ClusterLabel]

	labels := map[string]string{
		planetscalev2.ClusterLabel:  clusterName,
		planetscalev2.KeyspaceLabel: vtk.Spec.Name,
	}

	// Compute the set of all desired shards based on the defined partitionings.
	shards := vtk.Spec.ShardTemplates()

	// Generate keys (object names) for all desired shards.
	// Keep a map back from generated names to the shard specs.
	keys := make([]client.ObjectKey, 0, len(shards))
	shardMap := make(map[client.ObjectKey]*planetscalev2.VitessKeyspaceKeyRangeShard, len(shards))
	for _, shard := range shards {
		key := client.ObjectKey{Namespace: vtk.Namespace, Name: vitessshard.Name(clusterName, vtk.Spec.Name, shard.KeyRange)}
		keys = append(keys, key)
		shardMap[key] = shard

		// Initialize a status entry for every desired shard, so it will be
		// listed even if we end up not having anything to report about it.
		vtk.Status.Shards[shard.KeyRange.String()] = planetscalev2.NewVitessKeyspaceShardStatus(shard)
	}

	return r.reconciler.ReconcileObjectSet(ctx, vtk, keys, labels, reconciler.Strategy{
		Kind: &planetscalev2.VitessShard{},

		New: func(key client.ObjectKey) runtime.Object {
			return newVitessShard(key, vtk, labels, shardMap[key])
		},
		UpdateInPlace: func(key client.ObjectKey, obj runtime.Object) {
			newObj := obj.(*planetscalev2.VitessShard)
			updateVitessShardInPlace(key, newObj, vtk, labels, shardMap[key])
		},
		UpdateRollingInPlace: func(key client.ObjectKey, obj runtime.Object) {
			newObj := obj.(*planetscalev2.VitessShard)
			updateVitessShard(key, newObj, vtk, labels, shardMap[key])
		},
		Status: func(key client.ObjectKey, obj runtime.Object) {
			curObj := obj.(*planetscalev2.VitessShard)

			status := vtk.Status.Shards[curObj.Spec.KeyRange.String()]
			status.Cells = curObj.Status.Cells
			if curObj.Status.HasMaster != "" {
				status.HasMaster = curObj.Status.HasMaster
			}
			status.Tablets = int32(len(curObj.Status.Tablets))
			status.PendingChanges = curObj.Annotations[rollout.ScheduledAnnotation]

			status.ReadyTablets = 0
			status.UpdatedTablets = 0
			for _, tablet := range curObj.Status.Tablets {
				if tablet.Ready == corev1.ConditionTrue {
					status.ReadyTablets++
				}
				if tablet.PendingChanges == "" {
					status.UpdatedTablets++
				}
			}
		},
		OrphanStatus: func(key client.ObjectKey, obj runtime.Object, orphanStatus *planetscalev2.OrphanStatus) {
			curObj := obj.(*planetscalev2.VitessShard)
			vtk.Status.OrphanedShards[curObj.Spec.Name] = orphanStatus
		},
		PrepareForTurndown: func(key client.ObjectKey, obj runtime.Object) *planetscalev2.OrphanStatus {
			// Make sure it's ok to delete this shard.
			// We err on the safe side since losing a shard accidentally is very disruptive.
			curObj := obj.(*planetscalev2.VitessShard)
			if curObj.Status.Idle == corev1.ConditionTrue {
				// The shard is not in any serving partitioning anywhere.
				return nil
			}
			// The shard is either in a serving partitioning (Idle=False),
			// or we can't be sure whether it's serving (Idle=Unknown).
			return planetscalev2.NewOrphanStatus("Serving", "The shard can't be turned down because it's potentially in the serving set. You must migrate all served types in all cells to another shard before removing this shard.")
		},
	})
}

// newVitessShard expands a complete VitessShard from a VitessShardTemplate.
//
// A VitessShard consists of both user-configured parts, which come from VitessShardTemplate,
// plus auto-filled data that we propagate into each VitessShard from here.
// This allows VitessShard to do its job without looking at any other objects,
// and also lets us control when global changes roll out to each cell.
func newVitessShard(key client.ObjectKey, vtk *planetscalev2.VitessKeyspace, parentLabels map[string]string, shard *planetscalev2.VitessKeyspaceKeyRangeShard) *planetscalev2.VitessShard {
	template := shard.VitessShardTemplate.DeepCopy()

	// Copy parent labels map and add shard-specific label.
	labels := make(map[string]string, len(parentLabels)+1)
	for k, v := range parentLabels {
		labels[k] = v
	}
	labels[planetscalev2.ShardLabel] = shard.KeyRange.SafeName()

	return &planetscalev2.VitessShard{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   key.Namespace,
			Name:        key.Name,
			Labels:      labels,
			Annotations: template.Annotations,
		},
		Spec: planetscalev2.VitessShardSpec{
			VitessShardTemplate:    *template,
			GlobalLockserver:       vtk.Spec.GlobalLockserver,
			Images:                 vtk.Spec.Images,
			ImagePullPolicies:      vtk.Spec.ImagePullPolicies,
			Name:                   shard.KeyRange.String(),
			KeyRange:               shard.KeyRange,
			ZoneMap:                vtk.Spec.ZoneMap,
			BackupLocations:        vtk.Spec.BackupLocations,
			BackupEngine:           vtk.Spec.BackupEngine,
			ExtraVitessFlags:       vtk.Spec.ExtraVitessFlags,
			TopologyReconciliation: vtk.Spec.TopologyReconciliation,
		},
	}
}

func updateVitessShard(key client.ObjectKey, vts *planetscalev2.VitessShard, vtk *planetscalev2.VitessKeyspace, parentLabels map[string]string, shard *planetscalev2.VitessKeyspaceKeyRangeShard) {
	newShard := newVitessShard(key, vtk, parentLabels, shard)

	// Update labels, but ignore existing ones we don't set.
	update.Labels(&vts.Labels, newShard.Labels)

	updateVitessShardAnnotations(vts, newShard)

	// For now, everything in Spec is safe to update.
	vts.Spec = newShard.Spec
}

func updateVitessShardAnnotations(vts *planetscalev2.VitessShard, newShard *planetscalev2.VitessShard) {
	// Remove old annotations that shouldn't be there that we injected previously.
	// This must be done before we update vts.Spec.
	differentAnnotations := differentKeys(vts.Spec.Annotations, newShard.Spec.Annotations)
	for _, annotation := range differentAnnotations {
		delete(vts.Annotations, annotation)
	}

	// Update annotations we set.
	update.Annotations(&vts.Annotations, newShard.Annotations)
}

func updateVitessShardInPlace(key client.ObjectKey, vts *planetscalev2.VitessShard, vtk *planetscalev2.VitessKeyspace, parentLabels map[string]string, shard *planetscalev2.VitessKeyspaceKeyRangeShard) {
	newShard := newVitessShard(key, vtk, parentLabels, shard)

	// For now, only disk size & annotations are safe to update in place.
	update.ShardDiskSize(vts.Spec.TabletPools, newShard.Spec.TabletPools)

	updateVitessShardAnnotations(vts, newShard)
}

// differentKeys returns keys from an older map instance that are no longer in a newer map instance.
func differentKeys(oldMap, newMap map[string]string) []string {
	var differentKeys []string
	for k := range oldMap {
		if _, exist := newMap[k]; !exist {
			differentKeys = append(differentKeys, k)
		}
	}

	return differentKeys
}
