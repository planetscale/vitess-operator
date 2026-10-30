/*
Copyright 2020 PlanetScale Inc.

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

func cephBackupFlags(ceph *planetscalev2.CephBackupLocation) vitess.Flags {
	flags := vitess.Flags{
		"backup-storage-implementation": cephBackupStorageImplementationName,
		"ceph-backup-storage-config":    secrets.Mount(&ceph.AuthSecret, cephAuthDirName).FilePath(),
	}
	return flags
}

func cephBackupVolumes(ceph *planetscalev2.CephBackupLocation) []corev1.Volume {
	return secrets.Mount(&ceph.AuthSecret, cephAuthDirName).PodVolumes()
}

func cephBackupVolumeMounts(ceph *planetscalev2.CephBackupLocation) []corev1.VolumeMount {
	return []corev1.VolumeMount{
		secrets.Mount(&ceph.AuthSecret, cephAuthDirName).ContainerVolumeMount(),
	}
}

func cephBackupEnvVars(ceph *planetscalev2.CephBackupLocation) []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name:  "CEPH_CREDENTIALS_FILE",
			Value: secrets.Mount(&ceph.AuthSecret, cephAuthDirName).FilePath(),
		},
	}
}
