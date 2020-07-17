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

func DefaultEtcdLockserver(ls *EtcdLockserver) {
	DefaultEtcdLockserverSpec(&ls.Spec)
}

func DefaultEtcdLockserverSpec(ls *EtcdLockserverSpec) {
	DefaultEtcdLockserverTemplate(&ls.EtcdLockserverTemplate)
}

func DefaultEtcdLockserverTemplate(ls *EtcdLockserverTemplate) {
	if ls.Image == "" {
		ls.Image = defaultEtcdImage
	}
	if len(ls.DataVolumeClaimTemplate.AccessModes) == 0 {
		ls.DataVolumeClaimTemplate.AccessModes = []corev1.PersistentVolumeAccessMode{
			corev1.ReadWriteOnce,
		}
	}
	if _, isSet := ls.DataVolumeClaimTemplate.Resources.Requests[corev1.ResourceStorage]; !isSet {
		if ls.DataVolumeClaimTemplate.Resources.Requests == nil {
			ls.DataVolumeClaimTemplate.Resources.Requests = make(corev1.ResourceList)
		}
		ls.DataVolumeClaimTemplate.Resources.Requests[corev1.ResourceStorage] = *resource.NewQuantity(defaultEtcdStorageRequestBytes, resource.BinarySI)
	}
	if len(ls.Resources.Requests) == 0 {
		ls.Resources.Requests = corev1.ResourceList{
			corev1.ResourceCPU:    *resource.NewMilliQuantity(defaultEtcdCPUMillis, resource.DecimalSI),
			corev1.ResourceMemory: *resource.NewQuantity(defaultEtcdMemoryBytes, resource.BinarySI),
		}
	}
	if len(ls.Resources.Limits) == 0 {
		ls.Resources.Limits = corev1.ResourceList{
			corev1.ResourceMemory: *resource.NewQuantity(defaultEtcdMemoryBytes, resource.BinarySI),
		}
	}
	if ls.CreatePDB == nil {
		ls.CreatePDB = pointer.BoolPtr(defaultEtcdCreatePDB)
	}
	if ls.CreateClientService == nil {
		ls.CreateClientService = pointer.BoolPtr(defaultEtcdCreateClientService)
	}
	if ls.CreatePeerService == nil {
		ls.CreatePeerService = pointer.BoolPtr(defaultEtcdCreatePeerService)
	}
	DefaultServiceOverrides(&ls.ClientService)
	DefaultServiceOverrides(&ls.PeerService)
}
