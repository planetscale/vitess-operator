/*
Copyright 2026 PlanetScale Inc.

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

package vtorc

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestDeploymentHostNetwork(t *testing.T) {
	key := client.ObjectKey{Namespace: "ns", Name: "vtorc"}

	check := func(t *testing.T, podSpec corev1.PodSpec, wantHostNetwork bool, wantDNSPolicy corev1.DNSPolicy) {
		t.Helper()
		if podSpec.HostNetwork != wantHostNetwork {
			t.Errorf("HostNetwork = %v, want %v", podSpec.HostNetwork, wantHostNetwork)
		}
		if podSpec.DNSPolicy != wantDNSPolicy {
			t.Errorf("DNSPolicy = %v, want %v", podSpec.DNSPolicy, wantDNSPolicy)
		}
	}

	t.Run("disabled by default", func(t *testing.T) {
		obj := NewDeployment(key, &Spec{})
		check(t, obj.Spec.Template.Spec, false, corev1.DNSClusterFirst)
	})

	t.Run("enabled", func(t *testing.T) {
		obj := NewDeployment(key, &Spec{HostNetwork: true})
		check(t, obj.Spec.Template.Spec, true, corev1.DNSClusterFirstWithHostNet)
	})

	t.Run("disabling reverts an existing Deployment", func(t *testing.T) {
		obj := NewDeployment(key, &Spec{HostNetwork: true})
		UpdateDeployment(obj, &Spec{})
		check(t, obj.Spec.Template.Spec, false, corev1.DNSClusterFirst)
	})
}
