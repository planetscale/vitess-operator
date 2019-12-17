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
	// Mount the main data volume.
	tabletVolumes.Add(func(s lazy.Spec) []corev1.Volume {
		spec := s.(*Spec)
		if spec.DataVolumePVCSpec == nil {
			return nil
		}
		return []corev1.Volume{
			{
				Name: pvcVolumeName,
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: spec.DataVolumePVCName,
					},
				},
			},
		}
	})
	// Note that we mount a subpath of the main data volume as vtdataroot.
	// This allows us to store other persistent data in the main PVC
	// without having to attach a second PVC.
	tabletVolumeMounts.Add(func(s lazy.Spec) []corev1.VolumeMount {
		spec := s.(*Spec)
		if spec.DataVolumePVCSpec == nil {
			// If No DataVolume is Specified use the root volume as a shared root
			return []corev1.VolumeMount{
				{
					Name:      vtRootVolumeName,
					MountPath: vtDataRootPath,
					SubPath:   "vtdataroot",
				},
			}
		}
		return []corev1.VolumeMount{
			{
				Name:      pvcVolumeName,
				MountPath: vtDataRootPath,
				SubPath:   "vtdataroot",
			},
		}
	})

	// Mount shard-wide config files.
	tabletVolumes.Add(func(s lazy.Spec) []corev1.Volume {
		spec := s.(*Spec)
		return []corev1.Volume{
			{
				Name: "shard-config",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: spec.DatabaseInitScriptSecret.Name,
						Items: []corev1.KeyToPath{
							{
								Key:  spec.DatabaseInitScriptSecret.Key,
								Path: dbInitScriptFileName,
							},
						},
					},
				},
			},
		}
	})
	tabletVolumeMounts.Add(func(s lazy.Spec) []corev1.VolumeMount {
		return []corev1.VolumeMount{
			{
				Name:      "shard-config",
				MountPath: shardConfigPath,
				ReadOnly:  true,
			},
		}
	})
}
