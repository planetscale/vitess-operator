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

package etcd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"planetscale.dev/vitess-operator/pkg/operator/desiredstatehash"
)

// TestUpdatePodKeepsAdmissionInjectedFields checks that fields mutating
// admission injects into a live Pod survive UpdatePod, so the reconciler does
// not see a permanent diff and recreate the Pod every reconcile.
func TestUpdatePodKeepsAdmissionInjectedFields(t *testing.T) {
	spec := &Spec{
		LockserverName:   "example-etcd",
		Image:            "quay.io/coreos/etcd:v3.5",
		ImagePullSecrets: []corev1.LocalObjectReference{{Name: "my-registry"}},
		Labels:           map[string]string{"app": "etcd"},
		Index:            1,
	}
	key := client.ObjectKey{Namespace: "vitess", Name: "example-etcd-1"}
	pod := NewPod(key, spec)

	// Simulate what the API server and OpenShift do to the Pod after creation.
	pod.Spec.ImagePullSecrets = append(pod.Spec.ImagePullSecrets, corev1.LocalObjectReference{Name: "default-dockercfg-p95fq"})
	for i := range pod.Spec.Containers {
		pod.Spec.Containers[i].SecurityContext = &corev1.SecurityContext{
			RunAsUser:      ptr.To(int64(1000650000)),
			RunAsNonRoot:   ptr.To(true),
			SELinuxOptions: &corev1.SELinuxOptions{Level: "s0:c26,c5"},
		}
	}
	live := pod.DeepCopy()

	UpdatePod(pod, spec)

	require.Len(t, pod.Spec.ImagePullSecrets, 2)
	assert.ElementsMatch(t, live.Spec.ImagePullSecrets, pod.Spec.ImagePullSecrets)
	for i := range pod.Spec.Containers {
		assert.True(t, equality.Semantic.DeepEqual(live.Spec.Containers[i].SecurityContext, pod.Spec.Containers[i].SecurityContext),
			"container %q securityContext = %#v; want %#v", pod.Spec.Containers[i].Name, pod.Spec.Containers[i].SecurityContext, live.Spec.Containers[i].SecurityContext)
	}

	// A second update must be a no-op, which is what the reconciler relies on.
	again := pod.DeepCopy()
	UpdatePod(again, spec)
	assert.True(t, equality.Semantic.DeepEqual(pod.Spec, again.Spec), "UpdatePod is not idempotent")
}

// TestUpdatePodRemovedPullSecretChangesDesiredState checks that dropping a
// pull secret from the spec still changes the desired-state hash, so the Pod is
// recreated instead of keeping the stale secret forever.
func TestUpdatePodRemovedPullSecretChangesDesiredState(t *testing.T) {
	key := client.ObjectKey{Namespace: "vitess", Name: "example-etcd-1"}
	with := &Spec{LockserverName: "example-etcd", Image: "etcd", Index: 1, ImagePullSecrets: []corev1.LocalObjectReference{{Name: "my-registry"}}}
	without := &Spec{LockserverName: "example-etcd", Image: "etcd", Index: 1}

	podWith := NewPod(key, with)
	podWithout := NewPod(key, without)

	assert.NotEqual(t, podWith.Annotations[desiredstatehash.Annotation], podWithout.Annotations[desiredstatehash.Annotation])
}
