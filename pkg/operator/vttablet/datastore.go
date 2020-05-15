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
	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/lazy"
	"planetscale.dev/vitess-operator/pkg/operator/secrets"
	"planetscale.dev/vitess-operator/pkg/operator/vitess"
)

func init() {
	// Add secret Volumes for externally managed MySQL.
	tabletVolumes.Add(func(s lazy.Spec) []corev1.Volume {
		spec := s.(*Spec)
		if spec.ExternalDatastore == nil {
			return nil
		}
		credentialsFile := secrets.Mount(&spec.ExternalDatastore.CredentialsSecret, externalDatastoreCredentialsDirName)
		vols := credentialsFile.PodVolumes()
		if spec.ExternalDatastore.ServerCACertSecret != nil {
			caCertFile := secrets.Mount(spec.ExternalDatastore.ServerCACertSecret, externalDatastoreCACertDirName)
			vols = append(vols, caCertFile.PodVolumes()...)
		}
		return vols
	})
	// Mount secret Volumes for externally managed MySQL.
	tabletVolumeMounts.Add(func(s lazy.Spec) []corev1.VolumeMount {
		spec := s.(*Spec)
		if spec.ExternalDatastore == nil {
			return nil
		}
		credentialsFile := secrets.Mount(&spec.ExternalDatastore.CredentialsSecret, externalDatastoreCredentialsDirName)
		mounts := []corev1.VolumeMount{
			credentialsFile.ContainerVolumeMount(),
		}
		if spec.ExternalDatastore.ServerCACertSecret != nil {
			caCertFile := secrets.Mount(spec.ExternalDatastore.ServerCACertSecret, externalDatastoreCACertDirName)
			mounts = append(mounts, caCertFile.ContainerVolumeMount())
		}
		return mounts
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
		"db_charset":               spec.dbConfigCharset(),
		"db-config-app-uname":      dbConfigAppUname,
		"db-config-dba-uname":      dbConfigDbaUname,
		"db-config-repl-uname":     dbConfigReplUname,
		"db-config-filtered-uname": dbConfigFilteredUname,

		// Only in the case of local mysql do we want to use the vt_ prefix.
		"init_db_name_override": spec.localDatabaseName(),

		"enforce_strict_trans_tables": true,

		// TODO: Should this be configurable?
		"enable_replication_reporter": true,

		"enable_semi_sync":  spec.EnableSemiSync,
		"mysqlctl_socket":   mysqlctlSocketPath,
		"mycnf_socket_file": mysqlSocketPath,
	}
}

func externalDatastoreSSLCAFlags(spec *Spec) vitess.Flags {
	caCertFile := secrets.Mount(spec.ExternalDatastore.ServerCACertSecret, externalDatastoreCACertDirName)
	return vitess.Flags{
		"db_ssl_ca": caCertFile.FilePath(),

		// TODO: See if this should be passed in rather than hard coded.
		"db_flags": enableSSLBitflag,
	}
}

func externalDatastoreFlags(spec *Spec) vitess.Flags {
	credentialsFile := secrets.Mount(&spec.ExternalDatastore.CredentialsSecret, externalDatastoreCredentialsDirName)

	flags := vitess.Flags{
		"disable_active_reparents": true,
		"restore_from_backup":      false,
		"db_app_user":              spec.ExternalDatastore.User,
		"db_appdebug_user":         spec.ExternalDatastore.User,
		"db_allprivs_user":         spec.ExternalDatastore.User,
		"db_dba_user":              spec.ExternalDatastore.User,
		"db_filtered_user":         spec.ExternalDatastore.User,
		"db_repl_user":             spec.ExternalDatastore.User,
		"db-credentials-file":      credentialsFile.FilePath(),
		"db_host":                  spec.ExternalDatastore.Host,
		"db_port":                  spec.ExternalDatastore.Port,
		"init_db_name_override":    spec.ExternalDatastore.Database,

		// TODO: Should this be configurable?
		"enable_replication_reporter": true,

		"enforce_strict_trans_tables": false,
		"vreplication_tablet_type":    vreplicationTabletType,
	}

	if spec.Type == planetscalev2.ExternalMasterPoolType {
		flags["demote_master_type"] = "SPARE"
	}

	return flags
}
