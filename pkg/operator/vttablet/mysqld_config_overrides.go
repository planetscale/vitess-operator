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

func mysqldConfigOverrides(spec *Spec) string {
	if spec.Mysqld == nil {
		return ""
	}
	return spec.Mysqld.ConfigOverrides
}

func vtbackupMysqldConfigOverrides(spec *Spec) string {
	if !spec.forBackup || spec.Mysqld == nil {
		return ""
	}
	return spec.Mysqld.VtbackupConfigOverrides
}

func mysqldOverrideVolumeFiles(spec *Spec) []corev1.DownwardAPIVolumeFile {
	var items []corev1.DownwardAPIVolumeFile
	if len(mysqldConfigOverrides(spec)) > 0 {
		items = append(items, corev1.DownwardAPIVolumeFile{
			Path:     mysqldConfigOverridesFile,
			FieldRef: &corev1.ObjectFieldSelector{FieldPath: mysqldConfigOverridesAnnotationFieldPath},
		})
	}
	if len(vtbackupMysqldConfigOverrides(spec)) > 0 {
		items = append(items, corev1.DownwardAPIVolumeFile{
			Path:     mysqldVtbackupConfigOverridesFile,
			FieldRef: &corev1.ObjectFieldSelector{FieldPath: mysqldVtbackupConfigOverridesAnnotationFieldPath},
		})
	}
	return items
}

func init() {
	// Mount tablet-pool-specific my.cnf overrides.
	// Since these ought to be small, and updates should roll out slowly like
	// other Pod spec changes, we put it in an annotation that *doesn't* get
	// updated in-place, and then we mount it as a file in the Container.
	tabletAnnotations.Add(func(s lazy.Spec) map[string]string {
		spec := s.(*Spec)
		anns := map[string]string{}
		if overrides := mysqldConfigOverrides(spec); len(overrides) > 0 {
			anns[mysqldConfigOverridesAnnotationName] = overrides
		}
		if overrides := vtbackupMysqldConfigOverrides(spec); len(overrides) > 0 {
			anns[mysqldVtbackupConfigOverridesAnnotationName] = overrides
		}
		if len(anns) == 0 {
			return nil
		}
		return anns
	})
	extraMyCnf.Add(func(s lazy.Spec) []string {
		spec := s.(*Spec)
		hasShared := len(mysqldConfigOverrides(spec)) > 0
		hasVtbackup := len(vtbackupMysqldConfigOverrides(spec)) > 0
		if !hasShared && !hasVtbackup {
			return nil
		}
		// Shared overrides apply to both vttablet and vtbackup. vtbackup-only
		// overrides come next so they can replace shared settings. The
		// operator-generated vtbackup.cnf is last so it still wins for
		// sync_binlog and innodb_flush_log_at_trx_commit. That last file
		// is empty for normal vttablet Pods.
		files := []string{}
		if hasShared {
			files = append(files, mysqldConfigOverridesMountPath+"/"+mysqldConfigOverridesFile)
		}
		if hasVtbackup {
			files = append(files, mysqldConfigOverridesMountPath+"/"+mysqldVtbackupConfigOverridesFile)
		}
		files = append(files, vtbackupExtraMyCnfFile)
		return files
	})
	tabletVolumes.Add(func(s lazy.Spec) []corev1.Volume {
		spec := s.(*Spec)
		items := mysqldOverrideVolumeFiles(spec)
		if len(items) == 0 {
			return nil
		}
		return []corev1.Volume{
			{
				Name: "pod-config",
				VolumeSource: corev1.VolumeSource{
					DownwardAPI: &corev1.DownwardAPIVolumeSource{
						Items: items,
					},
				},
			},
		}
	})
	tabletVolumeMounts.Add(func(s lazy.Spec) []corev1.VolumeMount {
		spec := s.(*Spec)
		if len(mysqldOverrideVolumeFiles(spec)) == 0 {
			return nil
		}
		return []corev1.VolumeMount{
			{
				Name:      "pod-config",
				MountPath: mysqldConfigOverridesMountPath,
				ReadOnly:  true,
			},
		}
	})
}
