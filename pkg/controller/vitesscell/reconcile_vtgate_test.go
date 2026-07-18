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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
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
