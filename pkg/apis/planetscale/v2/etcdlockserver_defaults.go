/*
Copyright 2019 PlanetScale.

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
)

func DefaultEtcdLockserverTemplate(etcd *EtcdLockserverTemplate) {
	if etcd.Image == "" {
		etcd.Image = defaultEtcdImage
	}
	if len(etcd.DataVolumeClaimTemplate.AccessModes) == 0 {
		etcd.DataVolumeClaimTemplate.AccessModes = []corev1.PersistentVolumeAccessMode{
			corev1.ReadWriteOnce,
		}
	}
	if _, isSet := etcd.DataVolumeClaimTemplate.Resources.Requests[corev1.ResourceStorage]; !isSet {
		if etcd.DataVolumeClaimTemplate.Resources.Requests == nil {
			etcd.DataVolumeClaimTemplate.Resources.Requests = make(corev1.ResourceList)
		}
		etcd.DataVolumeClaimTemplate.Resources.Requests[corev1.ResourceStorage] = *resource.NewQuantity(defaultEtcdStorageRequestBytes, resource.BinarySI)
	}
	if len(etcd.Resources.Requests) == 0 {
		etcd.Resources.Requests = corev1.ResourceList{
			corev1.ResourceCPU:    *resource.NewMilliQuantity(defaultEtcdCPUMillis, resource.DecimalSI),
			corev1.ResourceMemory: *resource.NewQuantity(defaultEtcdMemoryBytes, resource.BinarySI),
		}
	}
	if len(etcd.Resources.Limits) == 0 {
		etcd.Resources.Limits = corev1.ResourceList{
			corev1.ResourceMemory: *resource.NewQuantity(defaultEtcdMemoryBytes, resource.BinarySI),
		}
	}
}
