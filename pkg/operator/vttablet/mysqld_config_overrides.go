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
	"strings"

	corev1 "k8s.io/api/core/v1"

	"planetscale.dev/vitess-operator/pkg/operator/lazy"
	"planetscale.dev/vitess-operator/pkg/operator/update"
)

func mysqldConfigOverrides(spec *Spec) string {
	if spec.Mysqld == nil {
		return ""
	}
	return spec.Mysqld.ConfigOverrides
}

func mysqldOverrideVolumeFile() corev1.DownwardAPIVolumeFile {
	return corev1.DownwardAPIVolumeFile{
		Path:     mysqldConfigOverridesFile,
		FieldRef: &corev1.ObjectFieldSelector{FieldPath: mysqldConfigOverridesAnnotationFieldPath},
	}
}

func vtbackupMysqldOverrideVolumeFile() corev1.DownwardAPIVolumeFile {
	return corev1.DownwardAPIVolumeFile{
		Path:     mysqldVtbackupConfigOverridesFile,
		FieldRef: &corev1.ObjectFieldSelector{FieldPath: mysqldVtbackupConfigOverridesAnnotationFieldPath},
	}
}

func mysqldOverrideVolume(items []corev1.DownwardAPIVolumeFile) corev1.Volume {
	return corev1.Volume{
		Name: mysqldConfigOverridesVolumeName,
		VolumeSource: corev1.VolumeSource{
			DownwardAPI: &corev1.DownwardAPIVolumeSource{
				Items: items,
			},
		},
	}
}

func mysqldOverrideVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      mysqldConfigOverridesVolumeName,
		MountPath: mysqldConfigOverridesMountPath,
		ReadOnly:  true,
	}
}

func extraMyCnfValue(env []corev1.EnvVar) string {
	for _, item := range env {
		if item.Name == extraMyCnfEnvVar {
			return item.Value
		}
	}
	return ""
}

func extraMyCnfWithVtbackupOverrides(existing string) string {
	overridePath := mysqldConfigOverridesMountPath + "/" + mysqldVtbackupConfigOverridesFile
	var files []string
	if existing != "" {
		files = strings.Split(existing, ":")
	}
	out := make([]string, 0, len(files)+2)
	inserted := false
	for _, file := range files {
		if file == overridePath {
			inserted = true
		}
		if file == vtbackupExtraMyCnfFile && !inserted {
			out = append(out, overridePath)
			inserted = true
		}
		out = append(out, file)
	}
	if !inserted {
		out = append(out, overridePath, vtbackupExtraMyCnfFile)
	}
	return strings.Join(out, ":")
}

// applyVtbackupMysqldConfigOverrides mounts spec.vtbackup.mysqldConfigOverrides
// on a vtbackup Pod only. Serving vttablet Pods never call this.
func applyVtbackupMysqldConfigOverrides(overrides string, annotations *map[string]string, env *[]corev1.EnvVar, volumes *[]corev1.Volume, mounts *[]corev1.VolumeMount) {
	if len(overrides) == 0 {
		return
	}
	update.Annotations(annotations, map[string]string{
		mysqldVtbackupConfigOverridesAnnotationName: overrides,
	})
	update.Env(env, []corev1.EnvVar{{
		Name:  extraMyCnfEnvVar,
		Value: extraMyCnfWithVtbackupOverrides(extraMyCnfValue(*env)),
	}})

	added := false
	for i := range *volumes {
		vol := &(*volumes)[i]
		if vol.Name != mysqldConfigOverridesVolumeName {
			continue
		}
		if vol.DownwardAPI == nil {
			vol.DownwardAPI = &corev1.DownwardAPIVolumeSource{}
		}
		for _, item := range vol.DownwardAPI.Items {
			if item.Path == mysqldVtbackupConfigOverridesFile {
				added = true
				break
			}
		}
		if !added {
			vol.DownwardAPI.Items = append(vol.DownwardAPI.Items, vtbackupMysqldOverrideVolumeFile())
			added = true
		}
		break
	}
	if !added {
		update.Volumes(volumes, []corev1.Volume{mysqldOverrideVolume([]corev1.DownwardAPIVolumeFile{vtbackupMysqldOverrideVolumeFile()})})
	}
	update.VolumeMounts(mounts, []corev1.VolumeMount{mysqldOverrideVolumeMount()})
}

func init() {
	// Mount tablet-pool-specific my.cnf overrides.
	// Since these ought to be small, and updates should roll out slowly like
	// other Pod spec changes, we put it in an annotation that *doesn't* get
	// updated in-place, and then we mount it as a file in the Container.
	tabletAnnotations.Add(func(s lazy.Spec) map[string]string {
		spec := s.(*Spec)
		overrides := mysqldConfigOverrides(spec)
		if len(overrides) == 0 {
			return nil
		}
		return map[string]string{
			mysqldConfigOverridesAnnotationName: overrides,
		}
	})
	extraMyCnf.Add(func(s lazy.Spec) []string {
		spec := s.(*Spec)
		if len(mysqldConfigOverrides(spec)) == 0 {
			return nil
		}
		// Append an extra config file for vtbackup at the end to override any
		// settings from the custom ones; will be empty for normal vttablet.
		return []string{
			mysqldConfigOverridesMountPath + "/" + mysqldConfigOverridesFile,
			vtbackupExtraMyCnfFile,
		}
	})
	tabletVolumes.Add(func(s lazy.Spec) []corev1.Volume {
		spec := s.(*Spec)
		if len(mysqldConfigOverrides(spec)) == 0 {
			return nil
		}
		return []corev1.Volume{mysqldOverrideVolume([]corev1.DownwardAPIVolumeFile{mysqldOverrideVolumeFile()})}
	})
	tabletVolumeMounts.Add(func(s lazy.Spec) []corev1.VolumeMount {
		spec := s.(*Spec)
		if len(mysqldConfigOverrides(spec)) == 0 {
			return nil
		}
		return []corev1.VolumeMount{mysqldOverrideVolumeMount()}
	})
}
