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

// TestUpdatePodRemovedPullSecretChangesDesiredState checks that dropping a
// pull secret from the spec still changes the desired-state hash, so the Pod
// is recreated instead of keeping the stale secret forever.
func TestUpdatePodRemovedPullSecretChangesDesiredState(t *testing.T) {
	key := client.ObjectKey{Namespace: "vitess", Name: "example-vttablet-zone1-100"}
	podWith := NewPod(key, testPodSpec(corev1.LocalObjectReference{Name: "my-registry"}))
	podWithout := NewPod(key, testPodSpec())

	assert.NotEqual(t, podWith.Annotations[desiredstatehash.Annotation], podWithout.Annotations[desiredstatehash.Annotation])
}
