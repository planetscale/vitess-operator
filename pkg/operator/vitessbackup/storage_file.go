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

package vitessbackup

import (
	"planetscale.dev/vitess-operator/pkg/operator/vitess"
	corev1 "k8s.io/api/core/v1"
)

func fileBackupFlags(clusterName string) vitess.Flags {
	return vitess.Flags{
		"backup_storage_implementation": fileBackupStorageImplementationName,
		"file_backup_storage_root":      rootKeyPrefix(fileBackupStorageMountPath, clusterName),
	}
}

func fileBackupVolumes(volume *corev1.VolumeSource) []corev1.Volume {
	return []corev1.Volume{
		{
			Name:         fileBackupStorageVolumeName,
			VolumeSource: *volume,
		},
	}
}

func fileBackupVolumeMounts(subPath string) []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      fileBackupStorageVolumeName,
			MountPath: fileBackupStorageMountPath,
			SubPath:   subPath,
		},
	}
}
