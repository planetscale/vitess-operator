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

package vitesscell

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/reconciler"
	"planetscale.dev/vitess-operator/pkg/operator/vtgate"
)

func TestDeploymentTabletRefreshInterval(t *testing.T) {
	t.Run("reads the managed flag", func(t *testing.T) {
		deployment := vtgateDeployment("--tablet-refresh-interval=10s")

		got, err := deploymentTabletRefreshInterval(deployment)

		require.NoError(t, err)
		assert.Equal(t, 10*time.Second, got.Duration)
	})

	t.Run("reads the legacy flag", func(t *testing.T) {
		deployment := vtgateDeployment("--tablet_refresh_interval=20s")

		got, err := deploymentTabletRefreshInterval(deployment)

		require.NoError(t, err)
		assert.Equal(t, 20*time.Second, got.Duration)
	})

	t.Run("uses the Vitess default when no flag is present", func(t *testing.T) {
		deployment := vtgateDeployment()

		got, err := deploymentTabletRefreshInterval(deployment)

		require.NoError(t, err)
		assert.Equal(t, time.Minute, got.Duration)
	})

	t.Run("rejects an invalid flag", func(t *testing.T) {
		deployment := vtgateDeployment("--tablet-refresh-interval=invalid")

		_, err := deploymentTabletRefreshInterval(deployment)

		require.ErrorContains(t, err, "invalid tablet refresh interval")
	})
}

func TestDeploymentRolloutComplete(t *testing.T) {
	deployment := vtgateDeployment()
	deployment.Generation = 2
	deployment.Status = appsv1.DeploymentStatus{
		ObservedGeneration: 2,
		Replicas:           2,
		UpdatedReplicas:    2,
		AvailableReplicas:  2,
	}

	assert.True(t, deploymentRolloutComplete(deployment))

	deployment.Status.UpdatedReplicas = 1
	assert.False(t, deploymentRolloutComplete(deployment))
}

func vtgateDeployment(args ...string) *appsv1.Deployment {
	return &appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(int32(2)),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"planetscale.com/component": "vtgate"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name: "vtgate",
						Args: args,
					}},
				},
			},
		},
	}
}

// TestReconcileVtgateDoesNotRollRespelledArgs drives the real vtgate
// reconciliation against a Deployment whose command line only differs from the
// desired one in flag spelling, as written by an older operator. Upgrading the
// operator must leave the Deployment alone, while a new image still updates it.
func TestReconcileVtgateDoesNotRollRespelledArgs(t *testing.T) {
	vtc := &planetscalev2.VitessCell{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-zone1",
			Namespace: "default",
			Labels:    map[string]string{planetscalev2.ClusterLabel: "example"},
		},
		Spec: planetscalev2.VitessCellSpec{
			VitessCellTemplate: planetscalev2.VitessCellTemplate{Name: "zone1"},
			GlobalLockserver:   planetscalev2.VitessLockserverParams{Implementation: "etcd2", Address: "etcd:2379", RootPath: "/vitess/example/global"},
			AllCells:           []string{"zone1"},
			Images:             planetscalev2.VitessCellImages{Vtgate: "vitess/lite:v24.0.5"},
		},
	}
	planetscalev2.DefaultVitessCell(vtc)

	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))
	require.NoError(t, autoscalingv2.AddToScheme(scheme))
	require.NoError(t, planetscalev2.SchemeBuilder.AddToScheme(scheme))
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(vtc.DeepCopy()).Build()
	recorder := record.NewFakeRecorder(100)
	r := &ReconcileVitessCell{
		client:     k8sClient,
		scheme:     scheme,
		recorder:   recorder,
		reconciler: reconciler.New(k8sClient, scheme, recorder),
	}
	key := client.ObjectKey{Namespace: vtc.Namespace, Name: vtgate.DeploymentName("example", "zone1")}
	reconcileVtgate := func() *appsv1.Deployment {
		t.Helper()
		cell := vtc.DeepCopy()
		cell.Status = planetscalev2.NewVitessCellStatus()
		_, err := r.reconcileVtgate(t.Context(), cell, "vitess/lite:v24.0.5")
		require.NoError(t, err)
		deployment := &appsv1.Deployment{}
		require.NoError(t, k8sClient.Get(t.Context(), key, deployment))
		return deployment
	}

	// Create the Deployment, then respell its command line the way an older
	// operator wrote it.
	deployment := reconcileVtgate()
	container := &deployment.Spec.Template.Spec.Containers[0]
	require.Equal(t, "vtgate", container.Name)
	dashedArgs := container.Args
	container.Args = oldOperatorArgs(container.Args)
	require.NoError(t, k8sClient.Update(t.Context(), deployment))
	require.NoError(t, k8sClient.Get(t.Context(), key, deployment))

	live := reconcileVtgate()
	assert.Equal(t, deployment.ResourceVersion, live.ResourceVersion, "the Deployment was updated")
	assert.Equal(t, deployment.Spec.Template.Spec.Containers[0].Args, live.Spec.Template.Spec.Containers[0].Args)

	// A new image brings the new command line.
	vtc.Spec.Images.Vtgate = "vitess/lite:v25.0.0"
	live = reconcileVtgate()
	assert.Equal(t, "vitess/lite:v25.0.0", live.Spec.Template.Spec.Containers[0].Image)
	assert.Equal(t, dashedArgs, live.Spec.Template.Spec.Containers[0].Args)
}

// oldOperatorArgs respells a command line the way older operator versions
// wrote it: underscore flag names, plus --logtostderr.
func oldOperatorArgs(args []string) []string {
	old := []string{"--logtostderr=true"}
	for _, arg := range args {
		name, value, _ := strings.Cut(strings.TrimPrefix(arg, "--"), "=")
		old = append(old, "--"+strings.ReplaceAll(name, "-", "_")+"="+value)
	}
	return old
}
