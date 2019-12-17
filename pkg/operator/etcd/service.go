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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"planetscale.dev/vitess-operator/pkg/operator/update"
)

const (
	clientPortName   = "client"
	clientPortNumber = 2379

	peerPortName   = "peer"
	peerPortNumber = 2380
)

// ClientServiceName returns the name of the etcd client Service.
func ClientServiceName(lockserverName string) string {
	return lockserverName + "-client"
}

// PeerServiceName returns the name of the etcd peer headless Service.
func PeerServiceName(lockserverName string) string {
	return lockserverName + "-peer"
}

// NewClientService creates a new client Service.
func NewClientService(key client.ObjectKey, labels map[string]string) *corev1.Service {
	// Fill in the immutable parts.
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: key.Namespace,
			Name:      key.Name,
		},
	}

	// Set everything else.
	UpdateClientService(svc, labels)
	return svc
}

// UpdateClientService updates the mutable parts of the client Service.
func UpdateClientService(svc *corev1.Service, labels map[string]string) {
	update.Labels(&svc.Labels, labels)

	svc.Spec.Selector = labels
	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       clientPortName,
			Protocol:   corev1.ProtocolTCP,
			Port:       clientPortNumber,
			TargetPort: intstr.FromString(clientPortName),
		},
	}
}

// NewPeerService creates a new client Service.
func NewPeerService(key client.ObjectKey, labels map[string]string) *corev1.Service {
	// Fill in the immutable parts.
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: key.Namespace,
			Name:      key.Name,
		},
	}

	// Use a headless service because it never makes sense to anonymously
	// target a peer. This headless service exists only to tell Kubernetes to
	// maintain a DNS entry for each member Pod for peers to find each other.
	svc.Spec.Type = corev1.ServiceTypeClusterIP
	svc.Spec.ClusterIP = corev1.ClusterIPNone

	// Don't wait for Pods to become Ready before creating DNS entries for them.
	svc.Spec.PublishNotReadyAddresses = true

	// Set everything else.
	UpdatePeerService(svc, labels)
	return svc
}

// UpdatePeerService updates the mutable parts of the client Service.
func UpdatePeerService(svc *corev1.Service, labels map[string]string) {
	update.Labels(&svc.Labels, labels)

	svc.Spec.Selector = labels
	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       peerPortName,
			Protocol:   corev1.ProtocolTCP,
			Port:       peerPortNumber,
			TargetPort: intstr.FromString(peerPortName),
		},
	}
}
