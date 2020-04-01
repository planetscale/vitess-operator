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
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/names"
	"planetscale.dev/vitess-operator/pkg/operator/update"
)

const (
	vtbackupInitScript = `set -ex
mkdir -p /mnt/vt/bin
cp --no-clobber /vt/bin/vtbackup /mnt/vt/bin/
mkdir -p /mnt/vt/config
if [[ -d /vt/config/mycnf ]]; then
  cp --no-clobber -R /vt/config/mycnf /mnt/vt/config/
else
  mkdir -p /mnt/vt/config/mycnf
fi
ln -sf /dev/stderr /mnt/vt/config/stderr.symlink
echo "log-error = /vt/config/stderr.symlink" > /mnt/vt/config/mycnf/log-error.cnf
echo "binlog_format=row" > /mnt/vt/config/mycnf/rbr.cnf
mkdir -p /mnt/vt/certs
cp --no-clobber /etc/ssl/certs/ca-certificates.crt /mnt/vt/certs/`
)

// BackupSpec is the spec for a Backup Pod.
type BackupSpec struct {
	// TabletSpec is the spec for a vttablet Pod. A backup Pod is a special kind
	// of tablet, so it needs much of the same configuration.
	TabletSpec *Spec

	// InitialBackup means don't try to restore from backup, because there
	// aren't any. Instead, bootstrap the shard with a backup of an empty
	// database.
	InitialBackup bool
	// MinBackupInterval is the minimum spacing between backups.
	// A new backup will only be taken if it's been at least this long since the
	// most recent backup.
	MinBackupInterval time.Duration
	// MinRetentionTime is the minimum time to retain each backup.
	// Each backup will be kept until it is at least this old.
	// A retention time of 0 means never delete any backups at all.
	MinRetentionTime time.Duration
	// MinRetentionCount is the minimum number of backups to retain.
	// Even if a backup is past the MinRetentionTime, it will not be deleted if
	// doing so would take the total number of backups below MinRetentionCount.
	MinRetentionCount int
}

// BackupPodName returns the name of the Pod for a periodic vtbackup job.
// The Pod name incorporates the time of the latest backup so a stale backup job
// (started a long time ago) will never be mistaken for a current one.
func BackupPodName(clusterName, keyspaceName string, keyRange planetscalev2.VitessKeyRange, backupLocationName string, lastBackupTime time.Time) string {
	timestamp := strconv.FormatInt(lastBackupTime.Unix(), 16)
	if backupLocationName == "" {
		return names.Join(clusterName, keyspaceName, keyRange.SafeName(), planetscalev2.VtbackupComponentName, timestamp)
	}
	return names.Join(clusterName, keyspaceName, keyRange.SafeName(), planetscalev2.VtbackupComponentName, backupLocationName, timestamp)
}

// InitialBackupPodName returns the name of the Pod for an initial vtbackup job.
func InitialBackupPodName(clusterName, keyspaceName string, keyRange planetscalev2.VitessKeyRange) string {
	return names.Join(clusterName, keyspaceName, keyRange.SafeName(), planetscalev2.VtbackupComponentName, "init")
}

// NewBackupPod creates a new vtbackup Pod, which is like a special kind of
// minimal tablet used to run backups as a batch process.
func NewBackupPod(key client.ObjectKey, backupSpec *BackupSpec) *corev1.Pod {
	tabletSpec := backupSpec.TabletSpec

	// Include vttablet env vars, since we run some vttablet code like backups.
	env := append(vttabletEnvVars.Get(tabletSpec), tabletEnvVars.Get(tabletSpec)...)
	// Add vtbackup-specific env vars.
	env = append(env, corev1.EnvVar{
		Name:  "HOME",
		Value: homeDir,
	})
	// Mount everything for both vttablet and mysqld, since vtbackup does both
	// jobs. We also need an additional mount to get SSL certs.
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      vtRootVolumeName,
			ReadOnly:  true,
			MountPath: sslCertsPath,
			SubPath:   "certs",
		},
	}
	volumeMounts = append(volumeMounts, mysqldVolumeMounts.Get(tabletSpec)...)
	volumeMounts = append(volumeMounts, tabletVolumeMounts.Get(tabletSpec)...)
	volumeMounts = append(volumeMounts, vttabletVolumeMounts.Get(tabletSpec)...)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   key.Namespace,
			Name:        key.Name,
			Labels:      tabletSpec.Labels,
			Annotations: tabletAnnotations.Get(tabletSpec),
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyOnFailure,
			Volumes:       tabletVolumes.Get(tabletSpec),
			SecurityContext: &corev1.PodSecurityContext{
				FSGroup: pointer.Int64Ptr(fsGroup),
			},
			InitContainers: []corev1.Container{
				{
					Name: "init-vt-root",
					SecurityContext: &corev1.SecurityContext{
						RunAsUser: pointer.Int64Ptr(runAsUser),
					},
					// We only use the vtbackup image to steal the vtbackup binary.
					// When we actually run it, we run inside the mysqld image.
					Image:           tabletSpec.Images.Vtbackup,
					ImagePullPolicy: tabletSpec.ImagePullPolicies.Vtbackup,
					VolumeMounts: []corev1.VolumeMount{
						{Name: vtRootVolumeName, MountPath: "/mnt/vt"},
					},
					Command: []string{"bash", "-c"},
					// Copy vtbackup binary instead of mysqlctld.
					// Also we need to copy certs over, since we use HTTPS for
					// some backup storage locations.
					Args: []string{vtbackupInitScript},
				},
			},
			Containers: []corev1.Container{
				{
					Name: vtbackupContainerName,
					// Use the mysqld container, as if we are running mysqlctld.
					Image:           tabletSpec.Images.Mysqld.Image(),
					ImagePullPolicy: tabletSpec.ImagePullPolicies.Mysqld,
					Command:         []string{vtbackupCommand},
					Args:            vtbackupFlags.Get(backupSpec).FormatArgs(),
					Resources:       tabletSpec.Mysqld.Resources,
					SecurityContext: &corev1.SecurityContext{
						RunAsUser: pointer.Int64Ptr(runAsUser),
					},
					Env:          env,
					VolumeMounts: volumeMounts,
				},
			},
		},
	}

	update.PodContainers(&pod.Spec.InitContainers, backupSpec.TabletSpec.InitContainers)
	update.PodContainers(&pod.Spec.Containers, backupSpec.TabletSpec.SidecarContainers)
	return pod
}
