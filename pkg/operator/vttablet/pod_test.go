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

package vttablet

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	topodatapb "vitess.io/vitess/go/vt/proto/topodata"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/desiredstatehash"
)

func testPodSpec(pullSecrets ...corev1.LocalObjectReference) *Spec {
	return &Spec{
		Alias:             topodatapb.TabletAlias{Cell: "zone1", Uid: 100},
		AliasStr:          "zone1-100",
		Type:              planetscalev2.ReplicaPoolType,
		Zone:              "zone1",
		Labels:            map[string]string{"app": "vttablet"},
		Images:            planetscalev2.VitessKeyspaceImages{Vttablet: "vitess/lite", Mysqld: &planetscalev2.MysqldImage{Mysql80Compatible: "vitess/lite"}},
		ImagePullSecrets:  pullSecrets,
		KeyspaceName:      "commerce",
		Vttablet:          &planetscalev2.VttabletSpec{},
		Mysqld:            &planetscalev2.MysqldSpec{},
		DataVolumePVCName: "zone1-100",
	}
}

// TestUpdatePodKeepsAdmissionInjectedFields checks that fields mutating
// admission injects into a live Pod survive UpdatePod, so the reconciler does
// not see a permanent diff and recreate the tablet every reconcile.
func TestUpdatePodKeepsAdmissionInjectedFields(t *testing.T) {
	// Run with no opinion on the UID, as on OpenShift, where the SCC assigns one
	// (--default_vitess_run_as_user=-1).
	defer func(uid int64) { planetscalev2.DefaultVitessRunAsUser = uid }(planetscalev2.DefaultVitessRunAsUser)
	planetscalev2.DefaultVitessRunAsUser = -1

	spec := testPodSpec(corev1.LocalObjectReference{Name: "my-registry"})
	key := client.ObjectKey{Namespace: "vitess", Name: "example-vttablet-zone1-100"}
	pod := NewPod(key, spec)

	// Simulate what the ServiceAccount admission plugin and OpenShift's SCC do
	// to the Pod after creation.
	pod.Spec.ImagePullSecrets = append(pod.Spec.ImagePullSecrets, corev1.LocalObjectReference{Name: "default-dockercfg-p95fq"})
	inject := func(containers []corev1.Container) {
		for i := range containers {
			sc := containers[i].SecurityContext
			if sc == nil {
				sc = &corev1.SecurityContext{}
				containers[i].SecurityContext = sc
			}
			sc.RunAsUser = ptr.To(int64(1000650000))
			sc.RunAsNonRoot = ptr.To(true)
			sc.SELinuxOptions = &corev1.SELinuxOptions{Level: "s0:c26,c5"}
		}
	}
	inject(pod.Spec.InitContainers)
	inject(pod.Spec.Containers)
	live := pod.DeepCopy()

	UpdatePod(pod, spec)

	require.Len(t, pod.Spec.ImagePullSecrets, 2)
	assert.ElementsMatch(t, live.Spec.ImagePullSecrets, pod.Spec.ImagePullSecrets)
	check := func(want, got []corev1.Container) {
		require.Len(t, got, len(want))
		for i := range got {
			assert.True(t, equality.Semantic.DeepEqual(want[i].SecurityContext, got[i].SecurityContext),
				"container %q securityContext = %#v; want %#v", got[i].Name, got[i].SecurityContext, want[i].SecurityContext)
		}
	}
	check(live.Spec.InitContainers, pod.Spec.InitContainers)
	check(live.Spec.Containers, pod.Spec.Containers)

	// A second update must be a no-op, which is what the reconciler relies on.
	again := pod.DeepCopy()
	UpdatePod(again, spec)
	assert.True(t, equality.Semantic.DeepEqual(pod.Spec, again.Spec), "UpdatePod is not idempotent")
}

// TestUpdatePodDesiredSecurityContextWins checks that where the operator does
// have an opinion, such as the default UID, it still overrides an injected value.
func TestUpdatePodDesiredSecurityContextWins(t *testing.T) {
	defer func(uid int64) { planetscalev2.DefaultVitessRunAsUser = uid }(planetscalev2.DefaultVitessRunAsUser)
	planetscalev2.DefaultVitessRunAsUser = 999

	spec := testPodSpec()
	key := client.ObjectKey{Namespace: "vitess", Name: "example-vttablet-zone1-100"}
	pod := NewPod(key, spec)
	for i := range pod.Spec.Containers {
		pod.Spec.Containers[i].SecurityContext = &corev1.SecurityContext{
			RunAsUser:    ptr.To(int64(1000650000)),
			RunAsNonRoot: ptr.To(true),
		}
	}

	UpdatePod(pod, spec)

	for i := range pod.Spec.Containers {
		sc := pod.Spec.Containers[i].SecurityContext
		require.NotNil(t, sc)
		assert.Equal(t, int64(999), ptr.Deref(sc.RunAsUser, 0), "container %q", pod.Spec.Containers[i].Name)
		assert.True(t, ptr.Deref(sc.RunAsNonRoot, false), "container %q keeps injected runAsNonRoot", pod.Spec.Containers[i].Name)
	}
}

// TestUpdatePodPullSecrets pins how pull secrets reach a running tablet Pod.
// They are merged rather than hashed: an added secret still changes the Pod,
// a removed one stays until the Pod is recreated for another reason, and the
// desired-state hash ignores them so that an operator upgrade doesn't roll
// every Pod of a cluster that pulls from a private registry.
func TestUpdatePodPullSecrets(t *testing.T) {
	key := client.ObjectKey{Namespace: "vitess", Name: "example-vttablet-zone1-100"}
	secret := corev1.LocalObjectReference{Name: "my-registry"}

	assert.Equal(t,
		NewPod(key, testPodSpec()).Annotations[desiredstatehash.Annotation],
		NewPod(key, testPodSpec(secret)).Annotations[desiredstatehash.Annotation],
		"pull secrets must not change the desired-state hash")

	added := NewPod(key, testPodSpec())
	UpdatePod(added, testPodSpec(secret))
	assert.Contains(t, added.Spec.ImagePullSecrets, secret, "an added secret reaches the Pod")

	removed := NewPod(key, testPodSpec(secret))
	UpdatePod(removed, testPodSpec())
	assert.Contains(t, removed.Spec.ImagePullSecrets, secret, "a removed secret stays until the Pod is recreated")
}

// TestUpdatePodRemovedSecurityContextFieldChangesDesiredState checks that
// removing a field from the securityContext of a user-supplied init container
// or sidecar changes the desired-state hash. The Pod merge keeps fields that
// the desired context doesn't set, so without the hash the removal would never
// reach the Pod and nothing would report it. Each kind is changed on its own,
// so each hash item is pinned separately.
func TestUpdatePodRemovedSecurityContextFieldChangesDesiredState(t *testing.T) {
	key := client.ObjectKey{Namespace: "vitess", Name: "example-vttablet-zone1-100"}
	privileged := &corev1.SecurityContext{Privileged: ptr.To(true), RunAsUser: ptr.To(int64(0)), RunAsNonRoot: ptr.To(true)}
	hardened := &corev1.SecurityContext{RunAsNonRoot: ptr.To(true)}

	tests := []struct {
		name      string
		container string
		set       func(spec *Spec, sc *corev1.SecurityContext)
	}{
		{
			name:      "init container",
			container: "user-init",
			set: func(spec *Spec, sc *corev1.SecurityContext) {
				spec.InitContainers[0].SecurityContext = sc
			},
		},
		{
			name:      "sidecar container",
			container: "user-sidecar",
			set: func(spec *Spec, sc *corev1.SecurityContext) {
				spec.SidecarContainers[0].SecurityContext = sc
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			specWith := func(sc *corev1.SecurityContext) *Spec {
				// The other kind keeps the same context in both specs, so its
				// item stays in the hash and only the changed one can move it.
				spec := testPodSpec()
				spec.InitContainers = []corev1.Container{{Name: "user-init", Image: "init", SecurityContext: hardened.DeepCopy()}}
				spec.SidecarContainers = []corev1.Container{{Name: "user-sidecar", Image: "sidecar", SecurityContext: hardened.DeepCopy()}}
				tc.set(spec, sc)
				return spec
			}

			live := NewPod(key, specWith(privileged.DeepCopy()))
			merged := live.DeepCopy()
			UpdatePod(merged, specWith(hardened.DeepCopy()))

			c := findContainer(t, merged, tc.container)
			assert.True(t, ptr.Deref(c.SecurityContext.Privileged, false), "the merge alone keeps the removed field")
			assert.NotEqual(t, live.Annotations[desiredstatehash.Annotation], merged.Annotations[desiredstatehash.Annotation],
				"the desired-state hash must change so the Pod is recreated")
		})
	}
}

func findContainer(t *testing.T, pod *corev1.Pod, name string) *corev1.Container {
	t.Helper()
	for _, list := range [][]corev1.Container{pod.Spec.InitContainers, pod.Spec.Containers} {
		for i := range list {
			if list[i].Name == name {
				return &list[i]
			}
		}
	}
	t.Fatalf("container %q not found", name)
	return nil
}

// TestUpdatePodUserContainersWithoutSecurityContextKeepDesiredState checks
// that user-supplied containers without a securityContext don't add to the
// desired-state hash, so their Pods are not recreated on upgrade.
func TestUpdatePodUserContainersWithoutSecurityContextKeepDesiredState(t *testing.T) {
	key := client.ObjectKey{Namespace: "vitess", Name: "example-vttablet-zone1-100"}
	spec := testPodSpec()
	spec.SidecarContainers = []corev1.Container{{Name: "sidecar", Image: "sidecar"}}

	withContext := testPodSpec()
	withContext.SidecarContainers = []corev1.Container{{Name: "sidecar", Image: "sidecar", SecurityContext: &corev1.SecurityContext{}}}

	assert.Equal(t, NewPod(key, testPodSpec()).Annotations[desiredstatehash.Annotation], NewPod(key, spec).Annotations[desiredstatehash.Annotation])
	assert.NotEqual(t, NewPod(key, spec).Annotations[desiredstatehash.Annotation], NewPod(key, withContext).Annotations[desiredstatehash.Annotation])
}
