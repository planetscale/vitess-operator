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
	"vitess.io/vitess/go/vt/topo/topoproto"

	corev1 "k8s.io/api/core/v1"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/lazy"
	"planetscale.dev/vitess-operator/pkg/operator/secrets"
	"planetscale.dev/vitess-operator/pkg/operator/vitess"
)

func init() {
	// Inject the Pod IP as an environment variable, so we can use it in the flags below.
	tabletEnvVars.Add(func(s lazy.Spec) []corev1.EnvVar {
		return []corev1.EnvVar{
			{
				Name: "POD_IP",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "status.podIP",
					},
				},
			},
		}
	})

	// Base vttablet flags.
	vttabletFlags.Add(func(s lazy.Spec) vitess.Flags {
		spec := s.(*Spec)
		return vitess.Flags{
			"logtostderr":                true,
			"topo_implementation":        spec.GlobalLockserver.Implementation,
			"topo_global_server_address": spec.GlobalLockserver.Address,
			"topo_global_root":           spec.GlobalLockserver.RootPath,
			"grpc_max_message_size":      grpcMaxMessageSize,

			"service_map": serviceMap,
			"port":        planetscalev2.DefaultWebPort,
			"grpc_port":   planetscalev2.DefaultGrpcPort,

			"tablet-path": topoproto.TabletAliasString(&spec.Alias),

			// We inject the POD_IP environment variable up above via the Pod Downward API.
			// The Pod args list natively expands environment variables in this format,
			// so we don't need to use a shell to launch vttablet.
			"tablet_hostname": "$(POD_IP)",

			"init_keyspace":    spec.KeyspaceName,
			"init_shard":       spec.KeyRange.String(),
			"init_tablet_type": spec.Type.InitTabletType(),

			"health_check_interval": healthCheckInterval,

			"queryserver-config-max-result-size":  queryserverConfigMaxResultSize,
			"queryserver-config-query-timeout":    queryserverConfigQueryTimeout,
			"queryserver-config-pool-size":        queryserverConfigPoolSize,
			"queryserver-config-stream-pool-size": queryserverConfigStreamPoolSize,
			"queryserver-config-transaction-cap":  queryserverConfigTransactionCap,
		}
	})

	// Base mysqlctld flags.
	mysqlctldFlags.Add(func(s lazy.Spec) vitess.Flags {
		spec := s.(*Spec)
		dbInitScript := secrets.Mount(&spec.DatabaseInitScriptSecret, dbInitScriptDirName)
		return vitess.Flags{
			"logtostderr":      true,
			"tablet_uid":       spec.Alias.Uid,
			"socket_file":      mysqlctlSocketPath,
			"mysql_socket":     mysqlSocketPath,
			"db_dba_user":      dbConfigDbaUname,
			"db_charset":       spec.dbConfigCharset(),
			"init_db_sql_file": dbInitScript.FilePath(),
			"wait_time":        mysqlctlWaitTime,
		}
	})

	// Base vtbackup flags.
	vtbackupFlags.Add(func(s lazy.Spec) vitess.Flags {
		backupSpec := s.(*BackupSpec)
		spec := backupSpec.TabletSpec
		dbInitScript := secrets.Mount(&spec.DatabaseInitScriptSecret, dbInitScriptDirName)
		return vitess.Flags{
			// vtbackup-specific flags.
			"concurrency":         vtbackupConcurrency,
			"initial_backup":      backupSpec.InitialBackup,
			"min_backup_interval": backupSpec.MinBackupInterval,
			"min_retention_time":  backupSpec.MinRetentionTime,
			"min_retention_count": backupSpec.MinRetentionCount,

			// Flags that are common to vttablet and mysqlctld.
			"logtostderr":                true,
			"topo_implementation":        spec.GlobalLockserver.Implementation,
			"topo_global_server_address": spec.GlobalLockserver.Address,
			"topo_global_root":           spec.GlobalLockserver.RootPath,

			"init_keyspace":         spec.KeyspaceName,
			"init_shard":            spec.KeyRange.String(),
			"init_db_name_override": spec.localDatabaseName(),

			"db_dba_user": dbConfigDbaUname,
			"db_charset":  spec.dbConfigCharset(),

			"init_db_sql_file": dbInitScript.FilePath(),

			"mysql_socket": mysqlSocketPath,
		}
	})
}
