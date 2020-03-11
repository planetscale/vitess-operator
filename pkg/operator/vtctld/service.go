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

package vtctld

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/names"
	"planetscale.dev/vitess-operator/pkg/operator/update"
)

// ServiceName returns the name of the vtctld Service for a cluster.
func ServiceName(clusterName string) string {
	return names.Join(clusterName, planetscalev2.VtctldComponentName)
}

// NewService creates a new Service object for vtctld.
func NewService(key client.ObjectKey, labels map[string]string) *corev1.Service {
	// Fill in the immutable parts.
	obj := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: key.Namespace,
			Name:      key.Name,
		},
	}
	// Set everything else.
	UpdateService(obj, labels)
	return obj
}

// UpdateService updates the mutable parts of the vtctld Service.
func UpdateService(obj *corev1.Service, labels map[string]string) {
	update.Labels(&obj.Labels, labels)

	obj.Spec.Selector = labels

	// Using named TargetPorts instead of hard-coded port numbers means that
	// each Pod can decide what port numbers to use.
	// The Pod just needs to assign the proper name to those ports so we
	// can find them.
	obj.Spec.Ports = []corev1.ServicePort{
		{
			Name:       planetscalev2.DefaultWebPortName,
			Protocol:   corev1.ProtocolTCP,
			Port:       planetscalev2.DefaultWebPort,
			TargetPort: intstr.FromString(planetscalev2.DefaultWebPortName),
		},
		{
			Name:       planetscalev2.DefaultGrpcPortName,
			Protocol:   corev1.ProtocolTCP,
			Port:       planetscalev2.DefaultGrpcPort,
			TargetPort: intstr.FromString(planetscalev2.DefaultGrpcPortName),
		},
	}
}
