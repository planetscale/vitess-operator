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

func s3BackupFlags(s3 *planetscalev2.S3BackupLocation, clusterName string) vitess.Flags {
	flags := vitess.Flags{
		"backup_storage_implementation": s3BackupStorageImplementationName,
		"s3_backup_aws_region":          s3.Region,
		"s3_backup_storage_bucket":      s3.Bucket,
		"s3_backup_storage_root":        rootKeyPrefix(s3.KeyPrefix, clusterName),
		"s3_backup_force_path_style":    s3.ForcePathStyle,
	}
	if len(s3.Endpoint) > 0 {
		flags["s3_backup_aws_endpoint"] = s3.Endpoint
	}
	return flags
}

func s3BackupVolumes(s3 *planetscalev2.S3BackupLocation) []corev1.Volume {
	if s3.AuthSecret == nil {
		return nil
	}
	return secrets.Mount(s3.AuthSecret, s3AuthDirName).PodVolumes()
}

func s3BackupVolumeMounts(s3 *planetscalev2.S3BackupLocation) []corev1.VolumeMount {
	if s3.AuthSecret == nil {
		return nil
	}
	return []corev1.VolumeMount{
		secrets.Mount(s3.AuthSecret, s3AuthDirName).ContainerVolumeMount(),
	}
}

func s3BackupEnvVars(s3 *planetscalev2.S3BackupLocation) []corev1.EnvVar {
	if s3.AuthSecret == nil {
		return nil
	}
	return []corev1.EnvVar{
		{
			Name:  "AWS_SHARED_CREDENTIALS_FILE",
			Value: secrets.Mount(s3.AuthSecret, s3AuthDirName).FilePath(),
		},
	}
}
