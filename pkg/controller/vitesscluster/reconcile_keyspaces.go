/*
Copyright 2020 PlanetScale Inc.

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

package vitesscluster

import (
	"context"
	"sort"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/lockserver"
	"planetscale.dev/vitess-operator/pkg/operator/reconciler"
	"planetscale.dev/vitess-operator/pkg/operator/rollout"
	"planetscale.dev/vitess-operator/pkg/operator/update"
	"planetscale.dev/vitess-operator/pkg/operator/vitesskeyspace"
)

func (r *ReconcileVitessCluster) reconcileKeyspaces(ctx context.Context, vt *planetscalev2.VitessCluster) error {
	labels := map[string]string{
		planetscalev2.ClusterLabel: vt.Name,
	}

	// Generate keys (object names) for all desired keyspaces.
	// Keep a map back from generated names to the keyspace specs.
	// Oh boy it's awkward right now that the k8s client calls object names keys.
	keys := make([]client.ObjectKey, 0, len(vt.Spec.Keyspaces))
	keyspaceMap := make(map[client.ObjectKey]*planetscalev2.VitessKeyspaceTemplate, len(vt.Spec.Keyspaces))
	for i := range vt.Spec.Keyspaces {
		keyspace := &vt.Spec.Keyspaces[i]
		key := client.ObjectKey{Namespace: vt.Namespace, Name: vitesskeyspace.Name(vt.Name, keyspace.Name)}
		keys = append(keys, key)
		keyspaceMap[key] = keyspace

		// Initialize a status entry for every desired keyspace, so it will be
		// listed even if we end up not having anything to report about it.
		vt.Status.Keyspaces[keyspace.Name] = planetscalev2.NewVitessClusterKeyspaceStatus(keyspace)
	}

	return r.reconciler.ReconcileObjectSet(ctx, vt, keys, labels, reconciler.Strategy{
		Kind: &planetscalev2.VitessKeyspace{},

		New: func(key client.ObjectKey) runtime.Object {
			return newVitessKeyspace(key, vt, labels, keyspaceMap[key])
		},
		UpdateInPlace: func(key client.ObjectKey, obj runtime.Object) {
			newObj := obj.(*planetscalev2.VitessKeyspace)
			updateVitessKeyspaceInPlace(key, newObj, vt, labels, keyspaceMap[key])
		},
		UpdateRollingInPlace: func(key client.ObjectKey, obj runtime.Object) {
			newObj := obj.(*planetscalev2.VitessKeyspace)
			updateVitessKeyspace(key, newObj, vt, labels, keyspaceMap[key])
		},
		Status: func(key client.ObjectKey, obj runtime.Object) {
			curObj := obj.(*planetscalev2.VitessKeyspace)

			status := vt.Status.Keyspaces[curObj.Spec.Name]
			status.PendingChanges = curObj.Annotations[rollout.ScheduledAnnotation]
			status.Shards = int32(len(curObj.Status.Shards))

			status.ReadyShards = 0
			status.UpdatedShards = 0
			status.Tablets = 0
			status.ReadyTablets = 0
			status.UpdatedTablets = 0
			cells := map[string]struct{}{}

			for _, shard := range curObj.Status.Shards {
				if shard.ReadyTablets == shard.Tablets {
					status.ReadyShards++
				}
				if shard.UpdatedTablets == shard.Tablets {
					status.UpdatedShards++
				}
				status.Tablets += shard.Tablets
				status.ReadyTablets += shard.ReadyTablets
				status.UpdatedTablets += shard.UpdatedTablets
				for _, cell := range shard.Cells {
					cells[cell] = struct{}{}
				}
			}

			for cell := range cells {
				status.Cells = append(status.Cells, cell)
			}
			sort.Strings(status.Cells)
		},
		OrphanStatus: func(key client.ObjectKey, obj runtime.Object, orphanStatus *planetscalev2.OrphanStatus) {
			curObj := obj.(*planetscalev2.VitessKeyspace)
			vt.Status.OrphanedKeyspaces[curObj.Spec.Name] = orphanStatus
		},
		PrepareForTurndown: func(key client.ObjectKey, obj runtime.Object) *planetscalev2.OrphanStatus {
			curObj := obj.(*planetscalev2.VitessKeyspace)

			// Make sure it's ok to delete this keyspace.
			// The user may specify to skip turndown safety checks.
			if curObj.Spec.TurndownPolicy == planetscalev2.VitessKeyspaceTurndownPolicyImmediate {
				return nil
			}

			// Otherwise, we err on the safe side since losing a keyspace accidentally is very disruptive.
			if curObj.Status.Idle == corev1.ConditionTrue {
				// The keyspace is not depoyed in any cells.
				return nil
			}
			// The keyspace is either not idle (Idle=False),
			// or we can't be sure whether it's idle (Idle=Unknown).
			return planetscalev2.NewOrphanStatus("NotIdle", "The keyspace can't be turned down because it's not idle. You must remove all tablet pools before removing the keyspace.")
		},
	})
}

// newVitessKeyspace expands a complete VitessKeyspace from a VitessKeyspaceTemplate.
//
// A VitessKeyspace consists of both user-configured parts, which come from VitessKeyspaceTemplate,
// plus auto-filled data that we propagate into each VitessKeyspace from here.
// This allows VitessKeyspace to do its job without looking at any other objects,
// and also lets us control when global changes roll out to each keyspace.
func newVitessKeyspace(key client.ObjectKey, vt *planetscalev2.VitessCluster, parentLabels map[string]string, keyspace *planetscalev2.VitessKeyspaceTemplate) *planetscalev2.VitessKeyspace {
	template := keyspace.DeepCopy()

	images := planetscalev2.VitessKeyspaceImages{}
	planetscalev2.DefaultVitessKeyspaceImages(&images, &vt.Spec.Images)

	// Copy parent labels map and add keyspace-specific label.
	labels := make(map[string]string, len(parentLabels)+1)
	for k, v := range parentLabels {
		labels[k] = v
	}
	labels[planetscalev2.KeyspaceLabel] = keyspace.Name

	var backupLocations []planetscalev2.VitessBackupLocation
	var backupEngine planetscalev2.VitessBackupEngine
	if vt.Spec.Backup != nil {
		backupLocations = vt.Spec.Backup.Locations
		backupEngine = vt.Spec.Backup.Engine
	}

	return &planetscalev2.VitessKeyspace{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: key.Namespace,
			Name:      key.Name,
			Labels:    labels,
		},
		Spec: planetscalev2.VitessKeyspaceSpec{
			VitessKeyspaceTemplate: *template,
			GlobalLockserver:       *lockserver.GlobalConnectionParams(&vt.Spec.GlobalLockserver, vt.Name),
			Images:                 images,
			ImagePullPolicies:      vt.Spec.ImagePullPolicies,
			ZoneMap:                vt.Spec.ZoneMap(),
			BackupLocations:        backupLocations,
			BackupEngine:           backupEngine,
			ExtraVitessFlags:       vt.Spec.ExtraVitessFlags,
			TopologyReconciliation: vt.Spec.TopologyReconciliation,
		},
	}
}

func updateVitessKeyspace(key client.ObjectKey, vtk *planetscalev2.VitessKeyspace, vt *planetscalev2.VitessCluster, parentLabels map[string]string, keyspace *planetscalev2.VitessKeyspaceTemplate) {
	newKeyspace := newVitessKeyspace(key, vt, parentLabels, keyspace)

	// Update labels, but ignore existing ones we don't set.
	update.Labels(&vtk.Labels, newKeyspace.Labels)

	// For now, everything in Spec is safe to update.
	vtk.Spec = newKeyspace.Spec
}

func updateVitessKeyspaceInPlace(key client.ObjectKey, vtk *planetscalev2.VitessKeyspace, vt *planetscalev2.VitessCluster, parentLabels map[string]string, keyspace *planetscalev2.VitessKeyspaceTemplate) {
	newKeyspace := newVitessKeyspace(key, vt, parentLabels, keyspace)

	// Update labels, but ignore existing ones we don't set.
	update.Labels(&vtk.Labels, newKeyspace.Labels)

	// Only update things that are safe to roll out immediately.
	vtk.Spec.TurndownPolicy = newKeyspace.Spec.TurndownPolicy
}
