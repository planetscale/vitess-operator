/*
Copyright 2019 PlanetScale.

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
	"planetscale.dev/vitess-operator/pkg/operator/lazy"
	"planetscale.dev/vitess-operator/pkg/operator/vitess"
	corev1 "k8s.io/api/core/v1"
)

func init() {
	// Add credentials volume for externally managed MySQL.
	tabletVolumes.Add(func(s lazy.Spec) []corev1.Volume {
		spec := s.(*Spec)
		if spec.ExternalDatastore == nil {
			return nil
		}
		return []corev1.Volume{
			{
				Name: externalDatastoreCredentialsVolumeName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: spec.ExternalDatastore.CredentialsSecret.Name,
						Items: []corev1.KeyToPath{
							{
								Key:  spec.ExternalDatastore.CredentialsSecret.Key,
								Path: externalDatastoreCredentialsFilename,
							},
						},
					},
				},
			},
		}
	})
	// Add ssl cert volume for externally managed MySQL.
	tabletVolumes.Add(func(s lazy.Spec) []corev1.Volume {
		spec := s.(*Spec)
		if spec.ExternalDatastore == nil || spec.ExternalDatastore.ServerCACertSecret == nil {
			return nil
		}
		return []corev1.Volume{
			{
				Name: externalDatastoreSSLCAVolumeName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: spec.ExternalDatastore.ServerCACertSecret.Name,
						Items: []corev1.KeyToPath{
							{
								Key:  spec.ExternalDatastore.ServerCACertSecret.Key,
								Path: externalDatastoreSSLCAFilename,
							},
						},
					},
				},
			},
		}
	})

	// Add mounts for external datastore credentials volume.
	tabletVolumeMounts.Add(func(s lazy.Spec) []corev1.VolumeMount {
		spec := s.(*Spec)
		if spec.ExternalDatastore == nil {
			return nil
		}
		return []corev1.VolumeMount{
			{
				Name:      externalDatastoreCredentialsVolumeName,
				MountPath: externalDatastoreCredentialsPath,
			},
		}
	})

	// Add mounts for external datastore ssl ca volume.
	tabletVolumeMounts.Add(func(s lazy.Spec) []corev1.VolumeMount {
		spec := s.(*Spec)
		if spec.ExternalDatastore == nil || spec.ExternalDatastore.ServerCACertSecret == nil {
			return nil
		}
		return []corev1.VolumeMount{
			{
				Name:      externalDatastoreSSLCAVolumeName,
				MountPath: externalDatastoreSSLCAPath,
			},
		}
	})

	// sets datastore specific vttablet flags.
	vttabletFlags.Add(func(s lazy.Spec) vitess.Flags {
		spec := s.(*Spec)
		flags := datastoreFlags(spec)
		return flags
	})
}

func datastoreFlags(spec *Spec) vitess.Flags {
	if spec.ExternalDatastore != nil {
		externalFlags := externalDatastoreFlags(spec)
		if spec.ExternalDatastore.ServerCACertSecret != nil {
			return externalFlags.Merge(externalDatastoreSSLCAFlags(spec))
		}
		return externalFlags
	}
	return localDatastoreFlags(spec)
}

func localDatastoreFlags(spec *Spec) vitess.Flags {
	return vitess.Flags{
		"db_charset":               dbConfigCharset,
		"db-config-app-uname":      dbConfigAppUname,
		"db-config-dba-uname":      dbConfigDbaUname,
		"db-config-repl-uname":     dbConfigReplUname,
		"db-config-filtered-uname": dbConfigFilteredUname,

		// Only in the case of local mysql do we want to use the vt_ prefix.
		"init_db_name_override": spec.localDatabaseName(),

		"enforce_strict_trans_tables": true,

		// TODO: Should this be configurable?
		"enable_replication_reporter": true,

		"enable_semi_sync": spec.EnableSemiSync,
		"mysqlctl_socket":  mysqlctlSocketPath,
	}
}

func externalDatastoreSSLCAFlags(spec *Spec) vitess.Flags {
	return vitess.Flags{
		"db_ssl_ca": externalDatastoreSSLCAFilePath,

		// TODO: See if this should be passed in rather than hard coded.
		"db_flags": enableSSLBitflag,
	}
}

func externalDatastoreFlags(spec *Spec) vitess.Flags {
	return vitess.Flags{
		"disable_active_reparents": true,
		"restore_from_backup":      false,
		"db_app_user":              spec.ExternalDatastore.User,
		"db_appdebug_user":         spec.ExternalDatastore.User,
		"db_allprivs_user":         spec.ExternalDatastore.User,
		"db_dba_user":              spec.ExternalDatastore.User,
		"db_filtered_user":         spec.ExternalDatastore.User,
		"db_repl_user":             spec.ExternalDatastore.User,
		"db-credentials-file":      externalDatastoreCredentialsFilePath,
		"db_host":                  spec.ExternalDatastore.Host,
		"db_port":                  spec.ExternalDatastore.Port,
		"init_db_name_override":    spec.ExternalDatastore.Database,

		// TODO: Should this be configurable?
		"enable_replication_reporter": true,

		"enforce_strict_trans_tables": false,
		"vreplication_tablet_type":    vreplicationTabletType,
	}
}
