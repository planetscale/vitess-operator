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

// TestUpdatePodPullSecrets pins how pull secrets reach a running etcd Pod.
// They are merged rather than hashed: an added secret still changes the Pod,
// a removed one stays until the Pod is recreated for another reason, and the
// desired-state hash ignores them. etcd members are never released
// automatically, so a hash item here would leave every member of a cluster
// with pull secrets pending after an operator upgrade.
func TestUpdatePodPullSecrets(t *testing.T) {
	key := client.ObjectKey{Namespace: "vitess", Name: "example-etcd-1"}
	secret := corev1.LocalObjectReference{Name: "my-registry"}
	spec := func(secrets ...corev1.LocalObjectReference) *Spec {
		return &Spec{LockserverName: "example-etcd", Image: "etcd", Index: 1, ImagePullSecrets: secrets}
	}

	assert.Equal(t,
		NewPod(key, spec()).Annotations[desiredstatehash.Annotation],
		NewPod(key, spec(secret)).Annotations[desiredstatehash.Annotation],
		"pull secrets must not change the desired-state hash")

	added := NewPod(key, spec())
	UpdatePod(added, spec(secret))
	assert.Contains(t, added.Spec.ImagePullSecrets, secret, "an added secret reaches the Pod")

	removed := NewPod(key, spec(secret))
	UpdatePod(removed, spec())
	assert.Contains(t, removed.Spec.ImagePullSecrets, secret, "a removed secret stays until the Pod is recreated")
}

// TestUpdatePodRemovedSecurityContextFieldChangesDesiredState checks that
// removing a field from the securityContext of a user-supplied init container
// or sidecar changes the desired-state hash, so the Pod is recreated with the
// hardened context instead of keeping the removed field. Each kind is changed
// on its own, so each hash item is pinned separately.
func TestUpdatePodRemovedSecurityContextFieldChangesDesiredState(t *testing.T) {
	key := client.ObjectKey{Namespace: "vitess", Name: "example-etcd-1"}
	privileged := &corev1.SecurityContext{Privileged: ptr.To(true), RunAsNonRoot: ptr.To(true)}
	hardened := &corev1.SecurityContext{RunAsNonRoot: ptr.To(true)}

	tests := []struct {
		name string
		set  func(spec *Spec, sc *corev1.SecurityContext)
	}{
		{"init container", func(spec *Spec, sc *corev1.SecurityContext) { spec.InitContainers[0].SecurityContext = sc }},
		{"sidecar container", func(spec *Spec, sc *corev1.SecurityContext) { spec.SidecarContainers[0].SecurityContext = sc }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			specWith := func(sc *corev1.SecurityContext) *Spec {
				// The other kind keeps the same context in both specs, so its
				// item stays in the hash and only the changed one can move it.
				spec := &Spec{
					LockserverName:    "example-etcd",
					Image:             "etcd",
					Index:             1,
					InitContainers:    []corev1.Container{{Name: "user-init", Image: "init", SecurityContext: hardened.DeepCopy()}},
					SidecarContainers: []corev1.Container{{Name: "user-sidecar", Image: "sidecar", SecurityContext: hardened.DeepCopy()}},
				}
				tc.set(spec, sc)
				return spec
			}

			podPrivileged := NewPod(key, specWith(privileged.DeepCopy()))
			podHardened := NewPod(key, specWith(hardened.DeepCopy()))

			assert.NotEqual(t, podPrivileged.Annotations[desiredstatehash.Annotation], podHardened.Annotations[desiredstatehash.Annotation])
		})
	}
}
