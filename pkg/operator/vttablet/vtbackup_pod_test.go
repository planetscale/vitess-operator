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
	"sigs.k8s.io/controller-runtime/pkg/client"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
)

func TestNewBackupPodContainerPorts(t *testing.T) {
	tests := []struct {
		name        string
		extraFlags  map[string]string
		wantPortArg string
		want        []corev1.ContainerPort
	}{
		{
			name:        "port defaults to web port",
			wantPortArg: "--port=15000",
			want: []corev1.ContainerPort{{
				Name:          planetscalev2.DefaultWebPortName,
				Protocol:      corev1.ProtocolTCP,
				ContainerPort: 15000,
			}},
		},
		{
			name:        "port specified",
			extraFlags:  map[string]string{"port": "15000"},
			wantPortArg: "--port=15000",
			want: []corev1.ContainerPort{{
				Name:          planetscalev2.DefaultWebPortName,
				Protocol:      corev1.ProtocolTCP,
				ContainerPort: 15000,
			}},
		},
		{
			name:        "custom port specified",
			extraFlags:  map[string]string{"port": "18080"},
			wantPortArg: "--port=18080",
			want: []corev1.ContainerPort{{
				Name:          planetscalev2.DefaultWebPortName,
				Protocol:      corev1.ProtocolTCP,
				ContainerPort: 18080,
			}},
		},
		{
			name:        "port zero disables the declared port",
			extraFlags:  map[string]string{"port": "0"},
			wantPortArg: "--port=0",
			want:        nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tabletSpec := &Spec{
				Labels: map[string]string{
					planetscalev2.ClusterLabel: "example",
				},
				Images: planetscalev2.VitessKeyspaceImages{
					Mysqld: &planetscalev2.MysqldImage{
						Mysql80Compatible: "mysql:8.0.40",
					},
				},
				Vttablet: &planetscalev2.VttabletSpec{
					VtbackupExtraFlags: test.extraFlags,
				},
				Mysqld: &planetscalev2.MysqldSpec{},
				BackupLocation: &planetscalev2.VitessBackupLocation{
					Volume: &corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			}
			pod := NewBackupPod(
				client.ObjectKey{Namespace: "default", Name: "backup"},
				&BackupSpec{TabletSpec: tabletSpec},
				"mysql:8.0.40",
			)

			require.Len(t, pod.Spec.Containers, 1)
			assert.Contains(t, pod.Spec.Containers[0].Args, test.wantPortArg)
			assert.Equal(t, test.want, pod.Spec.Containers[0].Ports)
		})
	}
}
