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
	"planetscale.dev/vitess-operator/pkg/operator/vitess"
)

func azblobBackupFlags(azblob *planetscalev2.AzblobBackupLocation, clusterName string) vitess.Flags {
	return vitess.Flags{
		"backup_storage_implementation": azblobBackupStorageImplementationName,
		"azblob_backup_container_name":  azblob.Container,
		"azblob_backup_storage_root":    rootKeyPrefix(azblob.KeyPrefix, clusterName),
	}
}

func azblobBackupEnvVars(azblob *planetscalev2.AzblobBackupLocation) []corev1.EnvVar {
	if azblob.AuthSecret == nil {
		return nil
	}
	return []corev1.EnvVar{
		{
			Name: "VITESS_AZBLOB_ACCOUNT_NAME",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: azblob.AuthSecret.Name,
					},
					Key: azblobAccountName,
				},
			},
		}, {
			Name: "VITESS_AZBLOB_ACCOUNT_KEY",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: azblob.AuthSecret.Name,
					},
					Key: azblobAccountKeyName,
				},
			},
		},
	}
}
