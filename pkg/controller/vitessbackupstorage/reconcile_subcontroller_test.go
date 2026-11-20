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

package vitessbackupstorage

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

func TestSetSubcontrollerMetricsPort(t *testing.T) {
	metricsPort := corev1.ContainerPort{Name: "metrics", ContainerPort: 8383, Protocol: corev1.ProtocolTCP}

	t.Run("adds the named port", func(t *testing.T) {
		container := &corev1.Container{}
		setSubcontrollerMetricsPort(container)
		require.Equal(t, []corev1.ContainerPort{metricsPort}, container.Ports)
	})

	t.Run("replaces a forked port with the same name or number", func(t *testing.T) {
		container := &corev1.Container{Ports: []corev1.ContainerPort{
			{Name: "http", ContainerPort: 8080},
			{Name: "http-metrics", ContainerPort: 8383},
			{Name: "metrics", ContainerPort: 9090},
		}}
		setSubcontrollerMetricsPort(container)
		require.Equal(t, []corev1.ContainerPort{{Name: "http", ContainerPort: 8080}, metricsPort}, container.Ports)
	})
}
