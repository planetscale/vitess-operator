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

package v2

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/pointer"
)

// DefaultVitessCell fills in API-level defaults for a VitessCell object.
func DefaultVitessCell(vtc *VitessCell) {
	DefaultLocalLockserver(&vtc.Spec.Lockserver)
	DefaultVitessGateway(&vtc.Spec.Gateway)
	DefaultTopoReconcileConfig(&vtc.Spec.TopologyReconciliation)
}

func DefaultLocalLockserver(ls *LockserverSpec) {
	if ls.External != nil {
		// There's nothing to do if an external lockserver was specified.
		// We'll just pass those params to Vitess.
		return
	}
	if ls.Etcd != nil {
		// If etcd was specified, fill in defaults.
		DefaultEtcdLockserverTemplate(ls.Etcd)
	}
}

func DefaultVitessGateway(gtway *VitessCellGatewaySpec) {
	if gtway.Replicas == nil {
		gtway.Replicas = pointer.Int32Ptr(defaultVtgateReplicas)
	}
	if len(gtway.Resources.Requests) == 0 {
		gtway.Resources.Requests = corev1.ResourceList{
			corev1.ResourceCPU:    *resource.NewMilliQuantity(defaultVtgateCPUMillis, resource.DecimalSI),
			corev1.ResourceMemory: *resource.NewQuantity(defaultVtgateMemoryBytes, resource.BinarySI),
		}
	}
	if len(gtway.Resources.Limits) == 0 {
		gtway.Resources.Limits = corev1.ResourceList{
			corev1.ResourceMemory: *resource.NewQuantity(defaultVtgateMemoryBytes, resource.BinarySI),
		}
	}
}

// DefaultVitessCellImages fills in unspecified keyspace-level images from cluster-level defaults.
// The clusterDefaults should have already had its unspecified fields filled in with operator defaults.
func DefaultVitessCellImages(dst *VitessCellImages, clusterDefaults *VitessImages) {
	if dst.Vtgate == "" {
		dst.Vtgate = clusterDefaults.Vtgate
	}
}
