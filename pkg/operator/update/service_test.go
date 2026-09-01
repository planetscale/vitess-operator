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

package update

import (
	"testing"

	corev1 "k8s.io/api/core/v1"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
)

func TestServiceOverrides_NilIsNoOp(t *testing.T) {
	svc := &corev1.Service{}
	ServiceOverrides(svc, nil)
	if svc.Spec.Type != "" || svc.Spec.ClusterIP != "" {
		t.Errorf("nil overrides should not mutate Service: got %+v", svc.Spec)
	}
}

func TestServiceOverrides_AppliesAllFields(t *testing.T) {
	svc := &corev1.Service{}
	so := &planetscalev2.ServiceOverrides{
		Annotations:           map[string]string{"k": "v"},
		ClusterIP:             "10.0.0.10",
		Type:                  corev1.ServiceTypeLoadBalancer,
		LoadBalancerIP:        "192.0.2.10",
		ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyLocal,
	}
	ServiceOverrides(svc, so)

	if got := svc.Annotations["k"]; got != "v" {
		t.Errorf("annotations not applied: got %v", svc.Annotations)
	}
	if svc.Spec.ClusterIP != "10.0.0.10" {
		t.Errorf("ClusterIP not applied: got %q", svc.Spec.ClusterIP)
	}
	if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
		t.Errorf("Type not applied: got %q", svc.Spec.Type)
	}
	if svc.Spec.LoadBalancerIP != "192.0.2.10" {
		t.Errorf("LoadBalancerIP not applied: got %q", svc.Spec.LoadBalancerIP)
	}
	if svc.Spec.ExternalTrafficPolicy != corev1.ServiceExternalTrafficPolicyLocal {
		t.Errorf("ExternalTrafficPolicy not applied: got %q", svc.Spec.ExternalTrafficPolicy)
	}
}

func TestServiceOverrides_EmptyFieldsDoNotOverwrite(t *testing.T) {
	svc := &corev1.Service{
		Spec: corev1.ServiceSpec{
			Type:                  corev1.ServiceTypeNodePort,
			ClusterIP:             "10.0.0.99",
			LoadBalancerIP:        "192.0.2.99",
			ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyLocal,
		},
	}
	// All-empty overrides struct must not clobber pre-existing Spec fields.
	ServiceOverrides(svc, &planetscalev2.ServiceOverrides{})

	if svc.Spec.Type != corev1.ServiceTypeNodePort {
		t.Errorf("Type was unexpectedly reset: got %q", svc.Spec.Type)
	}
	if svc.Spec.ClusterIP != "10.0.0.99" {
		t.Errorf("ClusterIP was unexpectedly reset: got %q", svc.Spec.ClusterIP)
	}
	if svc.Spec.LoadBalancerIP != "192.0.2.99" {
		t.Errorf("LoadBalancerIP was unexpectedly reset: got %q", svc.Spec.LoadBalancerIP)
	}
	if svc.Spec.ExternalTrafficPolicy != corev1.ServiceExternalTrafficPolicyLocal {
		t.Errorf("ExternalTrafficPolicy was unexpectedly reset: got %q", svc.Spec.ExternalTrafficPolicy)
	}
}

func TestInPlaceServiceOverrides_AppliesMutableFieldsOnly(t *testing.T) {
	// InPlaceServiceOverrides MUST skip immutable fields (ClusterIP,
	// LoadBalancerIP) so reconciles of an existing Service don't fail
	// the apiserver's immutable-field validation.
	svc := &corev1.Service{
		Spec: corev1.ServiceSpec{
			ClusterIP:      "10.0.0.10",
			LoadBalancerIP: "192.0.2.10",
		},
	}
	so := &planetscalev2.ServiceOverrides{
		Annotations:           map[string]string{"k": "v"},
		ClusterIP:             "10.0.0.20", // must be ignored
		Type:                  corev1.ServiceTypeLoadBalancer,
		LoadBalancerIP:        "192.0.2.20", // must be ignored
		ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyCluster,
	}
	InPlaceServiceOverrides(svc, so)

	if svc.Spec.ClusterIP != "10.0.0.10" {
		t.Errorf("ClusterIP must be immutable in-place: got %q", svc.Spec.ClusterIP)
	}
	if svc.Spec.LoadBalancerIP != "192.0.2.10" {
		t.Errorf("LoadBalancerIP must be immutable in-place: got %q", svc.Spec.LoadBalancerIP)
	}
	if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
		t.Errorf("Type should be applied in-place: got %q", svc.Spec.Type)
	}
	if svc.Spec.ExternalTrafficPolicy != corev1.ServiceExternalTrafficPolicyCluster {
		t.Errorf("ExternalTrafficPolicy should be applied in-place: got %q", svc.Spec.ExternalTrafficPolicy)
	}
	if got := svc.Annotations["k"]; got != "v" {
		t.Errorf("annotations not applied: got %v", svc.Annotations)
	}
}
