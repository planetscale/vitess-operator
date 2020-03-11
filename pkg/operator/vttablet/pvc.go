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

package vttablet

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"planetscale.dev/vitess-operator/pkg/operator/update"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewPVC creates a new vttablet PVC from a Spec.
func NewPVC(key client.ObjectKey, spec *Spec) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: key.Namespace,
			Name:      key.Name,
			Labels:    spec.Labels,
		},
		Spec: *spec.DataVolumePVCSpec,
	}
}

// UpdatePVCInPlace updates an existing vttablet PVC in-place.
func UpdatePVCInPlace(obj *corev1.PersistentVolumeClaim, spec *Spec) {
	// Update labels, but ignore existing ones we don't set.
	update.Labels(&obj.Labels, spec.Labels)

	// The only in-place spec update that's possible is volume expansion.
	curSize := obj.Spec.Resources.Requests[corev1.ResourceStorage]
	newSize := spec.DataVolumePVCSpec.Resources.Requests[corev1.ResourceStorage]
	if newSize.Cmp(curSize) > 0 {
		obj.Spec.Resources.Requests[corev1.ResourceStorage] = newSize
	}
}
