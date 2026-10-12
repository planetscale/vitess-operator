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

package update

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/utils/ptr"
)

func TestTolerations(t *testing.T) {
	// Make sure we don't touch tolerations that were already there.
	val := []corev1.Toleration{
		{Key: "alreadyExists"},
	}
	want := []corev1.Toleration{
		{Key: "alreadyExists"},
		{Key: "newKey"},
	}

	Tolerations(&val, []corev1.Toleration{
		{Key: "newKey"},
	})

	if !equality.Semantic.DeepEqual(val, want) {
		t.Errorf("val = %#v; want %#v", val, want)
	}
}

func TestSecurityContextKeepsInjectedFields(t *testing.T) {
	// Fields the operator has no opinion on must survive an update, since
	// admission webhooks (e.g. OpenShift SCCs) inject them into the live Pod.
	val := &corev1.SecurityContext{
		RunAsUser:      ptr.To(int64(1000650000)),
		RunAsNonRoot:   ptr.To(true),
		SELinuxOptions: &corev1.SELinuxOptions{Level: "s0:c26,c5"},
		SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
		Capabilities:   &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
		RunAsGroup:     ptr.To(int64(2000)),
	}
	want := &corev1.SecurityContext{
		RunAsUser:      ptr.To(int64(1000650000)),
		RunAsNonRoot:   ptr.To(true),
		SELinuxOptions: &corev1.SELinuxOptions{Level: "s0:c26,c5"},
		SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
		Capabilities:   &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
		// The desired value wins where the operator does have an opinion.
		RunAsGroup:               ptr.To(int64(999)),
		AllowPrivilegeEscalation: ptr.To(false),
	}

	SecurityContext(&val, &corev1.SecurityContext{
		RunAsGroup:               ptr.To(int64(999)),
		AllowPrivilegeEscalation: ptr.To(false),
	})

	if !equality.Semantic.DeepEqual(val, want) {
		t.Errorf("val = %#v; want %#v", val, want)
	}
}

func TestSecurityContextWithoutOpinion(t *testing.T) {
	// A nil desired value means we have no opinion, so an injected context stays.
	val := &corev1.SecurityContext{RunAsNonRoot: ptr.To(true)}
	want := &corev1.SecurityContext{RunAsNonRoot: ptr.To(true)}

	SecurityContext(&val, nil)

	if !equality.Semantic.DeepEqual(val, want) {
		t.Errorf("val = %#v; want %#v", val, want)
	}

	// With nothing to preserve, the desired value is taken as is.
	var empty *corev1.SecurityContext
	SecurityContext(&empty, &corev1.SecurityContext{RunAsUser: ptr.To(int64(1000))})
	if !equality.Semantic.DeepEqual(empty, &corev1.SecurityContext{RunAsUser: ptr.To(int64(1000))}) {
		t.Errorf("empty = %#v", empty)
	}
}

func TestLocalObjectReferences(t *testing.T) {
	// Make sure we don't touch references that were already there, such as the
	// imagePullSecrets the ServiceAccount admission plugin injects.
	val := []corev1.LocalObjectReference{
		{Name: "default-dockercfg-p95fq"},
		{Name: "alreadyExists"},
	}
	want := []corev1.LocalObjectReference{
		{Name: "default-dockercfg-p95fq"},
		{Name: "alreadyExists"},
		{Name: "newSecret"},
	}

	LocalObjectReferences(&val, []corev1.LocalObjectReference{
		{Name: "alreadyExists"},
		{Name: "newSecret"},
	})

	if !equality.Semantic.DeepEqual(val, want) {
		t.Errorf("val = %#v; want %#v", val, want)
	}
}
