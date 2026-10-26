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

package contenthash

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"
)

func TestContainersUpdates(t *testing.T) {
	table := []struct {
		description   string
		before, after []corev1.Container
		wantEqual     bool
	}{
		{
			description: "container added",
			before: []corev1.Container{
				{Name: "container-1"},
				{Name: "container-2"},
			},
			after: []corev1.Container{
				{Name: "container-1"},
				{Name: "container-2"},
				{Name: "container-3"},
			},
			wantEqual: false,
		},
		{
			description: "container removed",
			before: []corev1.Container{
				{Name: "container-1"},
				{Name: "container-2"},
				{Name: "container-3"},
			},
			after: []corev1.Container{
				{Name: "container-1"},
				{Name: "container-2"},
			},
			wantEqual: false,
		},
		{
			description: "container order changed",
			before: []corev1.Container{
				{Name: "container-1"},
				{Name: "container-2"},
				{Name: "container-3"},
			},
			after: []corev1.Container{
				{Name: "container-3"},
				{Name: "container-2"},
				{Name: "container-1"},
			},
			wantEqual: true,
		},
		{
			description: "volume mount added",
			before: []corev1.Container{
				{
					Name: "container",
					VolumeMounts: []corev1.VolumeMount{
						{MountPath: "path-1"},
						{MountPath: "path-2"},
					},
				},
			},
			after: []corev1.Container{
				{
					Name: "container",
					VolumeMounts: []corev1.VolumeMount{
						{MountPath: "path-1"},
						{MountPath: "path-2"},
						{MountPath: "path-3"},
					},
				},
			},
			wantEqual: false,
		},
		{
			description: "volume mount removed",
			before: []corev1.Container{
				{
					Name: "container",
					VolumeMounts: []corev1.VolumeMount{
						{MountPath: "path-1"},
						{MountPath: "path-2"},
						{MountPath: "path-3"},
					},
				},
			},
			after: []corev1.Container{
				{
					Name: "container",
					VolumeMounts: []corev1.VolumeMount{
						{MountPath: "path-1"},
						{MountPath: "path-2"},
					},
				},
			},
			wantEqual: false,
		},
		{
			description: "volume mount order changed",
			before: []corev1.Container{
				{
					Name: "container",
					VolumeMounts: []corev1.VolumeMount{
						{MountPath: "path-1"},
						{MountPath: "path-2"},
						{MountPath: "path-3"},
					},
				},
			},
			after: []corev1.Container{
				{
					Name: "container",
					VolumeMounts: []corev1.VolumeMount{
						{MountPath: "path-3"},
						{MountPath: "path-2"},
						{MountPath: "path-1"},
					},
				},
			},
			wantEqual: true,
		},
		{
			description: "resource request removed",
			before: []corev1.Container{
				{
					Name: "container",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("1"),
						},
					},
				},
			},
			after: []corev1.Container{
				{
					Name: "container",
				},
			},
			wantEqual: false,
		},
		{
			description: "resource limit removed",
			before: []corev1.Container{
				{
					Name: "container",
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("1"),
						},
					},
				},
			},
			after: []corev1.Container{
				{
					Name: "container",
				},
			},
			wantEqual: false,
		},
	}

	for _, tc := range table {
		got := ContainersUpdates(tc.before) == ContainersUpdates(tc.after)
		want := tc.wantEqual
		if got != want {
			t.Errorf("%v: equal = %v; want %v", tc.description, got, want)
		}
	}
}

func TestContainerSecurityContexts(t *testing.T) {
	privileged := &corev1.SecurityContext{Privileged: ptr.To(true)}
	nonRoot := &corev1.SecurityContext{RunAsNonRoot: ptr.To(true)}

	base := ContainerSecurityContexts([]corev1.Container{
		{Name: "a", SecurityContext: privileged},
		{Name: "b", SecurityContext: nonRoot},
	})

	// Container order doesn't matter.
	if got := ContainerSecurityContexts([]corev1.Container{
		{Name: "b", SecurityContext: nonRoot},
		{Name: "a", SecurityContext: privileged},
	}); got != base {
		t.Errorf("hash changed with container order: %q != %q", got, base)
	}

	// Containers without a security context are left out.
	if got := ContainerSecurityContexts([]corev1.Container{
		{Name: "a", SecurityContext: privileged},
		{Name: "b", SecurityContext: nonRoot},
		{Name: "c"},
	}); got != base {
		t.Errorf("hash changed with a container that has no security context: %q != %q", got, base)
	}

	// Removing a field changes the hash.
	if got := ContainerSecurityContexts([]corev1.Container{
		{Name: "a", SecurityContext: &corev1.SecurityContext{}},
		{Name: "b", SecurityContext: nonRoot},
	}); got == base {
		t.Error("hash didn't change when a field was removed")
	}
}
