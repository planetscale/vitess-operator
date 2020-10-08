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

package etcd

import (
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"planetscale.dev/vitess-operator/pkg/operator/update"
)

// NewPVC creates a new PVC.
// TODO: "Handle the case when labels are removed from ExtraLabels"
func NewPVC(key client.ObjectKey, spec *Spec) *corev1.PersistentVolumeClaim {
	// Store labels in labels obj because we need to add extra label and avoid mutating spec.Labels value
	labels := map[string]string{}
	update.Labels(&labels, spec.Labels)
	update.Labels(&labels, spec.ExtraLabels)

	logrus.WithFields(logrus.Fields{
		"labels": labels,
	}).Info("labels are here")

	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: key.Namespace,
			Name:      key.Name,
			Labels:    labels,
		},
		Spec: *spec.DataVolumePVCSpec,
	}
}

// UpdatePVCInPlace updates an existing PVC in-place.
func UpdatePVCInPlace(obj *corev1.PersistentVolumeClaim, spec *Spec) {
	// Update labels, but ignore existing ones we don't set.
	update.Labels(&obj.Labels, spec.Labels)
    // update extra labels
    update.Labels(&obj.Labels, spec.ExtraLabels)
	// The only in-place spec update that's possible is volume expansion.
	curSize := obj.Spec.Resources.Requests[corev1.ResourceStorage]
	newSize := spec.DataVolumePVCSpec.Resources.Requests[corev1.ResourceStorage]
	if newSize.Cmp(curSize) > 0 {
		obj.Spec.Resources.Requests[corev1.ResourceStorage] = newSize
	}
}
