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

package etcdlockserver

import (
	"context"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	podutil "k8s.io/kubernetes/pkg/api/v1/pod"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/etcd"
	"planetscale.dev/vitess-operator/pkg/operator/k8s"
	"planetscale.dev/vitess-operator/pkg/operator/reconciler"
	"planetscale.dev/vitess-operator/pkg/operator/results"
)

func (r *ReconcileEtcdLockserver) reconcileMembers(ctx context.Context, ls *planetscalev2.EtcdLockserver) (reconcile.Result, error) {
	resultBuilder := &results.Builder{}
	lockserverName := ls.Name

	labels := map[string]string{
		etcd.LockserverLabel: lockserverName,
	}

	// Generate spec for each desired etcd member.
	members := memberSpecs(ls, labels)

	// Generate keys (object names) for all desired members.
	// Keep a map back from generated names to the tablet specs.
	keys := make([]client.ObjectKey, 0, len(members))
	memberMap := make(map[client.ObjectKey]*etcd.Spec, len(members))
	for _, member := range members {
		// We use the same name for the Pod and the data volume PVC.
		podName := etcd.PodName(lockserverName, member.Index)
		member.DataVolumePVCName = podName

		key := client.ObjectKey{Namespace: ls.Namespace, Name: podName}
		keys = append(keys, key)
		memberMap[key] = member
	}

	// Reconcile member PVCs. Note that we use the same keys as the corresponding Pods.
	err := r.reconciler.ReconcileObjectSet(ctx, ls, keys, labels, reconciler.Strategy{
		Kind: &corev1.PersistentVolumeClaim{},

		New: func(key client.ObjectKey) runtime.Object {
			return etcd.NewPVC(key, memberMap[key])
		},
		UpdateInPlace: func(key client.ObjectKey, obj runtime.Object) {
			curObj := obj.(*corev1.PersistentVolumeClaim)
			etcd.UpdatePVCInPlace(curObj, memberMap[key])
		},
	})
	if err != nil {
		resultBuilder.Error(err)
	}

	// Reconcile member Pods.
	numPodsReady := 0
	err = r.reconciler.ReconcileObjectSet(ctx, ls, keys, labels, reconciler.Strategy{
		Kind: &corev1.Pod{},

		New: func(key client.ObjectKey) runtime.Object {
			return etcd.NewPod(key, memberMap[key])
		},
		UpdateInPlace: func(key client.ObjectKey, obj runtime.Object) {
			newObj := obj.(*corev1.Pod)
			etcd.UpdatePodInPlace(newObj, memberMap[key])
		},
		UpdateRollingRecreate: func(key client.ObjectKey, obj runtime.Object) {
			newObj := obj.(*corev1.Pod)
			etcd.UpdatePod(newObj, memberMap[key])
		},
		Status: func(key client.ObjectKey, obj runtime.Object) {
			curObj := obj.(*corev1.Pod)
			if _, cond := podutil.GetPodCondition(&curObj.Status, corev1.PodReady); cond != nil {
				if cond.Status == corev1.ConditionTrue {
					numPodsReady++
				}
			}
		},
	})
	if err != nil {
		resultBuilder.Error(err)
	}

	// Update status of the lockserver.
	if ls.Spec.LocalMemberIndex == nil {
		// We're deploying all members locally, so we can see all of them.
		// We should be available for queries if the number of Ready replicas is
		// enough to reach quorum.
		ls.Status.Available = k8s.ConditionStatus(numPodsReady >= etcd.QuorumSize)
	} else {
		// We're only deploying one member locally, so all we can report is
		// whether our one Pod is ready.
		ls.Status.Available = k8s.ConditionStatus(numPodsReady > 0)
	}

	return resultBuilder.Result()
}

// memberSpecs creates a list of etcd.Specs for desired members.
func memberSpecs(ls *planetscalev2.EtcdLockserver, parentLabels map[string]string) []*etcd.Spec {
	members := make([]*etcd.Spec, 0, etcd.NumReplicas)
	for i := 1; i <= etcd.NumReplicas; i++ {
		// If we're only deploying one member locally, skip the others.
		if ls.Spec.LocalMemberIndex != nil && int32(i) != *ls.Spec.LocalMemberIndex {
			continue
		}

		// Set member-specific labels and copy parent labels.
		labels := map[string]string{
			etcd.IndexLabel: strconv.FormatInt(int64(i), 10),
		}
		for k, v := range parentLabels {
			labels[k] = v
		}
		// Also add some extra labels used by other components to identify
		// our objects, even though this controller doesn't use those labels
		// in its selector.
		labels[planetscalev2.ComponentLabel] = planetscalev2.EtcdComponentName
		// Only add the cluster label if the EtcdLockserver has it.
		if _, hasClusterLabel := ls.Labels[planetscalev2.ClusterLabel]; hasClusterLabel {
			labels[planetscalev2.ClusterLabel] = ls.Labels[planetscalev2.ClusterLabel]
		}

		members = append(members, &etcd.Spec{
			LockserverName:    ls.Name,
			Image:             ls.Spec.Image,
			ImagePullPolicy:   ls.Spec.ImagePullPolicy,
			Resources:         ls.Spec.Resources,
			Labels:            labels,
			Zone:              ls.Spec.Zone,
			Index:             i,
			DataVolumePVCSpec: &ls.Spec.DataVolumeClaimTemplate,
			ExtraFlags:        ls.Spec.ExtraFlags,
			ExtraEnv:          ls.Spec.ExtraEnv,
			ExtraVolumes:      ls.Spec.ExtraVolumes,
			ExtraVolumeMounts: ls.Spec.ExtraVolumeMounts,
			InitContainers:    ls.Spec.InitContainers,
			Affinity:          ls.Spec.Affinity,
			Annotations:       ls.Spec.Annotations,
			AdvertisePeerURLs: ls.Spec.AdvertisePeerURLs,
		})
	}
	return members
}
