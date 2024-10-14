/*
Copyright 2024 PlanetScale Inc.

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

package vtgate

import (
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"planetscale.dev/vitess-operator/pkg/operator/update"
)

// HpaSpec specifies all the internal parameters needed to create a HorizontalPodAutoscaler
// for vtgate.
type HpaSpec struct {
	Labels      map[string]string
	MinReplicas *int32
	MaxReplicas int32
	Behavior    *autoscalingv2.HorizontalPodAutoscalerBehavior `json:"behavior,omitempty"`
	Metrics     []autoscalingv2.MetricSpec                     `json:"metrics,omitempty"`
}

// NewHorizontalPodAutoscaler creates a new HorizontalPodAutoscaler object for vtgate.
func NewHorizontalPodAutoscaler(key client.ObjectKey, spec *HpaSpec) *autoscalingv2.HorizontalPodAutoscaler {
	// Fill in the immutable parts.
	obj := &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: key.Namespace,
			Name:      key.Name,
		},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				APIVersion: "planetscale.com/v2",
				Kind:       "VitessCell",
				Name:       key.Name,
			},
		},
	}
	// Set everything else.
	UpdateHorizontalPodAutoscaler(obj, spec)
	return obj
}

// UpdateHorizontalPodAutoscaler updates the mutable parts of the vtgate HorizontalPodAutoscaler.
func UpdateHorizontalPodAutoscaler(obj *autoscalingv2.HorizontalPodAutoscaler, spec *HpaSpec) {
	// Set labels on the HorizontalPodAutoscaler object.
	update.Labels(&obj.Labels, spec.Labels)

	// Set the specs for the HorizontalPodAutoscaler object.
	obj.Spec.MinReplicas = spec.MinReplicas
	obj.Spec.MaxReplicas = spec.MaxReplicas
	obj.Spec.Metrics = spec.Metrics
	obj.Spec.Behavior = spec.Behavior
}
