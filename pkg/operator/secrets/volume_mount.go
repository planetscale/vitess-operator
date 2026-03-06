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

package secrets

import (
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
)

const (
	// VolumeMountRootDir is the directory on the container filesystem in
	// which SecretSource volumes are mounted. Each volume is mounted in a
	// subdirectory with the specified DirName.
	VolumeMountRootDir = "/vt/secrets"

	volumeMountMode = 0444
)

// VolumeMount represents a mounted SecretSource.
type VolumeMount struct {
	// Secret is the SecretSource from which to load data.
	Secret *planetscalev2.SecretSource
	// DirName is the name of the directory under VolumeMountDir in which to
	// mount this SecretSource.
	DirName string
	// AbsolutePath stores the absolute path to use instead of the generated path
	// with /vt/secret as the prefix
	AbsolutePath string
}

// Mount creates a VolumeMount for a given SecretSource.
func Mount(secretSource *planetscalev2.SecretSource, dirName string) *VolumeMount {
	return &VolumeMount{
		Secret:  secretSource,
		DirName: dirName,
	}
}

// VolumeName returns the name of the Pod Volume to be mounted.
func (v *VolumeMount) VolumeName() string {
	if v.Secret.VolumeName != "" {
		return v.Secret.VolumeName
	}
	return v.DirName + "-secret"
}

// DirPath returns the absolute path to the mounted SecretSource volume.
func (v *VolumeMount) DirPath() string {
	if len(v.AbsolutePath) > 0 {
		return v.AbsolutePath
	}
	return filepath.Join(VolumeMountRootDir, v.DirName)
}

// FilePath returns the absolute path to the mounted file.
func (v *VolumeMount) FilePath() string {
	return filepath.Join(v.DirPath(), v.Secret.Key)
}

// PodVolumes returns the Volumes, if any, that should be added to Pods that need this secret.
func (v *VolumeMount) PodVolumes() []corev1.Volume {
	// We only create a Volume if we were asked to mount a Secret by name.
	// Otherwise, we assume the provided VolumeName already exists in the Pod.
	if v.Secret.Name == "" {
		return nil
	}

	return []corev1.Volume{
		{
			Name: v.VolumeName(),
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  v.Secret.Name,
					DefaultMode: ptr.To(int32(volumeMountMode)),
					Items: []corev1.KeyToPath{
						{
							Key:  v.Secret.Key,
							Path: v.Secret.Key,
						},
					},
				},
			},
		},
	}
}

// ContainerVolumeMount returns the VolumeMount that should be added to
// Containers that need this secret.
func (v *VolumeMount) ContainerVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      v.VolumeName(),
		MountPath: v.DirPath(),
		ReadOnly:  true,
	}
}
