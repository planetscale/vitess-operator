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
	"planetscale.dev/vitess-operator/pkg/operator/secrets"
	"planetscale.dev/vitess-operator/pkg/operator/vitess"
)

func gcsBackupFlags(gcs *planetscalev2.GCSBackupLocation, clusterName string) vitess.Flags {
	return vitess.Flags{
		"backup-storage-implementation": gcsBackupStorageImplementationName,
		"gcs-backup-storage-bucket":     gcs.Bucket,
		"gcs-backup-storage-root":       rootKeyPrefix(gcs.KeyPrefix, clusterName),
	}
}

func gcsBackupVolumes(gcs *planetscalev2.GCSBackupLocation) []corev1.Volume {
	if gcs.AuthSecret == nil {
		return nil
	}
	return secrets.Mount(gcs.AuthSecret, gcsAuthDirName).PodVolumes()
}

func gcsBackupVolumeMounts(gcs *planetscalev2.GCSBackupLocation) []corev1.VolumeMount {
	if gcs.AuthSecret == nil {
		return nil
	}
	return []corev1.VolumeMount{
		secrets.Mount(gcs.AuthSecret, gcsAuthDirName).ContainerVolumeMount(),
	}
}

func gcsBackupEnvVars(gcs *planetscalev2.GCSBackupLocation) []corev1.EnvVar {
	if gcs.AuthSecret == nil {
		return nil
	}
	return []corev1.EnvVar{
		{
			Name:  "GOOGLE_APPLICATION_CREDENTIALS",
			Value: secrets.Mount(gcs.AuthSecret, gcsAuthDirName).FilePath(),
		},
	}
}
