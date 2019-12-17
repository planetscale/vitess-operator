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
	corev1 "k8s.io/api/core/v1"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/vitess"
)

func gcsBackupFlags(gcs *planetscalev2.GCSBackupLocation, clusterName string) vitess.Flags {
	return vitess.Flags{
		"backup_storage_implementation": gcsBackupStorageImplementationName,
		"gcs_backup_storage_bucket":     gcs.Bucket,
		"gcs_backup_storage_root":       rootKeyPrefix(gcs.KeyPrefix, clusterName),
	}
}

func gcsBackupVolumes(gcs *planetscalev2.GCSBackupLocation) []corev1.Volume {
	if gcs.AuthSecret == nil {
		return nil
	}
	return []corev1.Volume{
		{
			Name: gcsAuthSecretVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: gcs.AuthSecret.Name,
					Items: []corev1.KeyToPath{
						{
							Key:  gcs.AuthSecret.Key,
							Path: gcsAuthSecretFileName,
						},
					},
				},
			},
		},
	}
}

func gcsBackupVolumeMounts(gcs *planetscalev2.GCSBackupLocation) []corev1.VolumeMount {
	if gcs.AuthSecret == nil {
		return nil
	}
	return []corev1.VolumeMount{
		{
			Name:      gcsAuthSecretVolumeName,
			MountPath: gcsAuthSecretMountPath,
			ReadOnly:  true,
		},
	}
}

func gcsBackupEnvVars(gcs *planetscalev2.GCSBackupLocation) []corev1.EnvVar {
	if gcs.AuthSecret == nil {
		return nil
	}
	return []corev1.EnvVar{
		{
			Name:  "GOOGLE_APPLICATION_CREDENTIALS",
			Value: gcsAuthSecretFilePath,
		},
	}
}
