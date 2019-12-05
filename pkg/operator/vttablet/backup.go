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
	"fmt"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/lazy"
	"planetscale.dev/vitess-operator/pkg/operator/vitess"
	"planetscale.dev/vitess-operator/pkg/operator/vitessbackup"
	corev1 "k8s.io/api/core/v1"
)

func xtrabackupFlags(spec *Spec, backupThreads, restoreThreads int) vitess.Flags {
	flags := vitess.Flags{
		"xtrabackup_user":         xtrabackupUser,
		"xtrabackup_stream_mode":  xtrabackupStreamMode,
		"xtrabackup_stripes":      xtrabackupStripeCount,
		"xtrabackup_backup_flags": fmt.Sprintf("--parallel=%d", backupThreads),
		"xbstream_restore_flags":  fmt.Sprintf("--parallel=%d", restoreThreads),
		"backup_storage_compress": true,
	}

	return flags
}

func init() {
	vttabletFlags.Add(func(s lazy.Spec) vitess.Flags {
		spec := s.(*Spec)
		if spec.BackupLocation == nil || spec.Mysqld == nil {
			return nil
		}
		flags := vitess.Flags{
			"restore_from_backup":          true,
			"restore_concurrency":          restoreConcurrency,
			"wait_for_backup_interval":     waitForBackupInterval,
			"backup_engine_implementation": string(spec.BackupEngine),
		}
		if spec.BackupEngine == planetscalev2.VitessBackupEngineXtraBackup {
			// When vttablets take backups, we let them keep serving, so we
			// limit to single-threaded to reduce the impact.
			backupThreads := 1
			// When vttablets are restoring, they can't serve at the same time
			// anyway, so let the restore use all available CPUs for this Pod.
			// This is cheating a bit, since xtrabackup technically counts
			// against only the vttablet container, but we allow CPU bursting,
			// and we happen to know that our mysqld container is not using its
			// CPU request (reservation) during restore since it's stopped.
			mysqlCPU := spec.Mysqld.Resources.Requests[corev1.ResourceCPU]
			vttabletCPU := spec.Vttablet.Resources.Requests[corev1.ResourceCPU]
			restoreThreads := int(mysqlCPU.Value() + vttabletCPU.Value())
			if restoreThreads < 1 {
				restoreThreads = 1
			}
			flags.Merge(xtrabackupFlags(spec, backupThreads, restoreThreads))
		}
		clusterName := spec.Labels[planetscalev2.ClusterLabel]
		storageLocationFlags := vitessbackup.StorageFlags(spec.BackupLocation, clusterName)
		return flags.Merge(storageLocationFlags)
	})

	vtbackupFlags.Add(func(s lazy.Spec) vitess.Flags {
		backupSpec := s.(*BackupSpec)
		spec := backupSpec.TabletSpec
		if spec.BackupLocation == nil || spec.Mysqld == nil {
			return nil
		}
		flags := vitess.Flags{
			"backup_engine_implementation": string(spec.BackupEngine),
		}
		if spec.BackupEngine == planetscalev2.VitessBackupEngineXtraBackup {
			// A vtbackup Pod is given the same resources as the mysqld
			// container for a vttablet in the shard would be given.
			// We let vtbackup use all available CPUs during both backup and
			// restore, since it is not serving queries anyway.
			vtbackupCPU := spec.Mysqld.Resources.Requests[corev1.ResourceCPU]
			threads := int(vtbackupCPU.Value())
			if threads < 1 {
				threads = 1
			}
			flags.Merge(xtrabackupFlags(spec, threads, threads))
		}
		clusterName := spec.Labels[planetscalev2.ClusterLabel]
		storageLocationFlags := vitessbackup.StorageFlags(spec.BackupLocation, clusterName)
		return flags.Merge(storageLocationFlags)
	})

	tabletVolumes.Add(func(s lazy.Spec) []corev1.Volume {
		spec := s.(*Spec)
		if spec.BackupLocation == nil || spec.Mysqld == nil {
			return nil
		}
		return vitessbackup.StorageVolumes(spec.BackupLocation)
	})

	vttabletVolumeMounts.Add(func(s lazy.Spec) []corev1.VolumeMount {
		spec := s.(*Spec)
		if spec.BackupLocation == nil || spec.Mysqld == nil {
			return nil
		}
		return vitessbackup.StorageVolumeMounts(spec.BackupLocation)
	})

	vttabletEnvVars.Add(func(s lazy.Spec) []corev1.EnvVar {
		spec := s.(*Spec)
		if spec.BackupLocation == nil || spec.Mysqld == nil {
			return nil
		}
		return vitessbackup.StorageEnvVars(spec.BackupLocation)
	})
}
