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
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/vitess"
)

// rootKeyPrefix generates a root key prefix for the given cluster.
func rootKeyPrefix(userPrefix, clusterName string) string {
	// Remove trailing slashes, if any, since we add our own.
	userPrefix = strings.TrimRight(userPrefix, "/")
	if userPrefix == "" {
		return clusterName
	}
	return fmt.Sprintf("%s/%s", userPrefix, clusterName)
}

// StorageFlags returns the Vitess flags for configuring the backup storage location.
func StorageFlags(backupLocation *planetscalev2.VitessBackupLocation, clusterName string) vitess.Flags {
	switch {
	case backupLocation.GCS != nil:
		return gcsBackupFlags(backupLocation.GCS, clusterName)
	case backupLocation.S3 != nil:
		return s3BackupFlags(backupLocation.S3, clusterName)
	case backupLocation.Azblob != nil:
		return azblobBackupFlags(backupLocation.Azblob, clusterName)
	case backupLocation.Volume != nil:
		return fileBackupFlags(clusterName)
	}
	return nil
}

// StorageVolumes returns the Volumes for the configured backup storage location.
func StorageVolumes(backupLocation *planetscalev2.VitessBackupLocation) []corev1.Volume {
	switch {
	case backupLocation.GCS != nil:
		return gcsBackupVolumes(backupLocation.GCS)
	case backupLocation.S3 != nil:
		return s3BackupVolumes(backupLocation.S3)
	case backupLocation.Azblob != nil:
		return azblobBackupVolumes(backupLocation.Azblob)
	case backupLocation.Volume != nil:
		return fileBackupVolumes(backupLocation.Volume)
	}
	return nil
}

// StorageVolumeMounts returns the VolumeMounts for the configured backup storage location.
func StorageVolumeMounts(backupLocation *planetscalev2.VitessBackupLocation) []corev1.VolumeMount {
	switch {
	case backupLocation.GCS != nil:
		return gcsBackupVolumeMounts(backupLocation.GCS)
	case backupLocation.S3 != nil:
		return s3BackupVolumeMounts(backupLocation.S3)
	case backupLocation.Azblob != nil:
		return azblobBackupVolumeMounts(backupLocation.Azblob)
	case backupLocation.Volume != nil:
		return fileBackupVolumeMounts()
	}
	return nil
}

// StorageEnvVars returns the EnvVars for the configured backup storage location.
func StorageEnvVars(backupLocation *planetscalev2.VitessBackupLocation) []corev1.EnvVar {
	switch {
	case backupLocation.GCS != nil:
		return gcsBackupEnvVars(backupLocation.GCS)
	case backupLocation.S3 != nil:
		return s3BackupEnvVars(backupLocation.S3)
	}
	return nil
}
