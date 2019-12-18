/*
Copyright 2019 PlanetScale Inc.

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
	corev1 "k8s.io/api/core/v1"

	"planetscale.dev/vitess-operator/pkg/operator/lazy"
)

func init() {
	// Mount tablet-pool-specific my.cnf overrides.
	// Since these ought to be small, and updates should roll out slowly like
	// other Pod spec changes, we put it in an annotation that *doesn't* get
	// updated in-place, and then we mount it as a file in the Container.
	tabletAnnotations.Add(func(s lazy.Spec) map[string]string {
		spec := s.(*Spec)
		if spec.Mysqld == nil || len(spec.Mysqld.ConfigOverrides) == 0 {
			return nil
		}
		return map[string]string{
			mysqldConfigOverridesAnnotationName: spec.Mysqld.ConfigOverrides,
		}
	})
	extraMyCnf.Add(func(s lazy.Spec) []string {
		spec := s.(*Spec)
		if spec.Mysqld == nil || len(spec.Mysqld.ConfigOverrides) == 0 {
			return nil
		}
		return []string{"/pod-config/mysqld-config-overrides"}
	})
	tabletVolumes.Add(func(s lazy.Spec) []corev1.Volume {
		spec := s.(*Spec)
		if spec.Mysqld == nil || len(spec.Mysqld.ConfigOverrides) == 0 {
			return nil
		}
		return []corev1.Volume{
			{
				Name: "pod-config",
				VolumeSource: corev1.VolumeSource{
					DownwardAPI: &corev1.DownwardAPIVolumeSource{
						Items: []corev1.DownwardAPIVolumeFile{
							{Path: "mysqld-config-overrides", FieldRef: &corev1.ObjectFieldSelector{FieldPath: mysqldConfigOverridesAnnotationFieldPath}},
						},
					},
				},
			},
		}
	})
	tabletVolumeMounts.Add(func(s lazy.Spec) []corev1.VolumeMount {
		spec := s.(*Spec)
		if spec.Mysqld == nil || len(spec.Mysqld.ConfigOverrides) == 0 {
			return nil
		}
		return []corev1.VolumeMount{
			{
				Name:      "pod-config",
				MountPath: "/pod-config",
				ReadOnly:  true,
			},
		}
	})
}
