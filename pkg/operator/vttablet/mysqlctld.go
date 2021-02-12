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
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/pointer"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/lazy"
)

const (
	vtRootInitScript = `set -ex
mkdir -p /mnt/vt/bin
cp --no-clobber /vt/bin/mysqlctld /mnt/vt/bin/
mkdir -p /mnt/vt/config
if [[ -d /vt/config/mycnf ]]; then
  cp --no-clobber -R /vt/config/mycnf /mnt/vt/config/
else
  mkdir -p /mnt/vt/config/mycnf
fi
mkdir -p /mnt/vt/vtdataroot
ln -sf /dev/stderr /mnt/vt/config/stderr.symlink
echo "log-error = /vt/config/stderr.symlink" > /mnt/vt/config/mycnf/log-error.cnf
echo "binlog_format=row" > /mnt/vt/config/mycnf/rbr.cnf
echo "socket = ` + mysqlSocketPath + `" > /mnt/vt/config/mycnf/socket.cnf
`

	mysqlSocketInitScript = `set -ex
cd ` + vtDataRootPath + `
for mycnf in $(find . -mindepth 2 -maxdepth 2 -path './vt_*/my.cnf'); do
  sed -i -e 's,^socket[ \t]*=.*$,socket = ` + mysqlSocketPath + `,' "${mycnf}"
done
`

	initCPURequestMillis   = 100
	initMemoryRequestBytes = 32 * (1 << 20)  // 32 MiB
	initMemoryLimitBytes   = 128 * (1 << 20) // 128 MiB
)

func init() {
	// Copy Vitess files needed by mysqlctld into the mysqld container,
	// which might be using a stock MySQL image.
	tabletVolumes.Add(func(s lazy.Spec) []corev1.Volume {
		return []corev1.Volume{
			{
				Name: vtRootVolumeName,
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
		}
	})
	tabletInitContainers.Add(func(s lazy.Spec) []corev1.Container {
		spec := s.(*Spec)

		securityContext := &corev1.SecurityContext{}
		if planetscalev2.DefaultVitessRunAsUser >= 0 {
			securityContext.RunAsUser = pointer.Int64Ptr(planetscalev2.DefaultVitessRunAsUser)
		}

		// Use an init container to copy only the files we need from the Vitess image.
		// Note specifically that we don't even copy init_db.sql to avoid accidentally using it.
		initContainers := []corev1.Container{
			{
				Name:            "init-vt-root",
				SecurityContext: securityContext,
				Image:           spec.Images.Vttablet,
				ImagePullPolicy: spec.ImagePullPolicies.Vttablet,
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      vtRootVolumeName,
						MountPath: "/mnt/vt",
					},
				},
				Command: []string{"bash", "-c"},
				Args:    []string{vtRootInitScript},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    *resource.NewMilliQuantity(initCPURequestMillis, resource.DecimalSI),
						corev1.ResourceMemory: *resource.NewQuantity(initMemoryRequestBytes, resource.BinarySI),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: *resource.NewQuantity(initMemoryLimitBytes, resource.BinarySI),
					},
				},
			},
		}

		// If we're using a PVC, add an init container to migrate the mysql UNIX
		// socket location before vttablet and mysqlctld start up. This is
		// needed to safely update tablet Pods with persistent volumes that were
		// deployed before we started customizing the UNIX socket location.
		if spec.DataVolumePVCSpec != nil {
			initContainers = append(initContainers, corev1.Container{
				Name:            "init-mysql-socket",
				SecurityContext: securityContext,
				Image:           spec.Images.Vttablet,
				ImagePullPolicy: spec.ImagePullPolicies.Vttablet,
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      pvcVolumeName,
						MountPath: vtDataRootPath,
						SubPath:   "vtdataroot",
					},
				},
				Command: []string{"bash", "-c"},
				Args:    []string{mysqlSocketInitScript},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    *resource.NewMilliQuantity(initCPURequestMillis, resource.DecimalSI),
						corev1.ResourceMemory: *resource.NewQuantity(initMemoryRequestBytes, resource.BinarySI),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: *resource.NewQuantity(initMemoryLimitBytes, resource.BinarySI),
					},
				},
			})
		}

		return initContainers
	})
	// Add mysqld-specific volume mounts.
	mysqldVolumeMounts.Add(func(s lazy.Spec) []corev1.VolumeMount {
		return []corev1.VolumeMount{
			{
				Name:      vtRootVolumeName,
				ReadOnly:  true,
				MountPath: vtBinPath,
				SubPath:   "bin",
			},
		}
	})
	// Add the config mount to both mysqld and vttablet, to make sure they
	// generate the same my.cnf.
	tabletVolumeMounts.Add(func(s lazy.Spec) []corev1.VolumeMount {
		return []corev1.VolumeMount{
			{
				Name:      vtRootVolumeName,
				ReadOnly:  true,
				MountPath: vtConfigPath,
				SubPath:   "config",
			},
			{
				Name:      vtRootVolumeName,
				ReadOnly:  false,
				MountPath: vtSocketPath,
				SubPath:   "socket",
			},
		}
	})
	// Tell mysqld to log to stderr instead of a file, so we can rely on
	// automatic rotation of container logs. Also configure the location of the
	// UNIX socket file. These config files are written out by vtRootInitScript.
	extraMyCnf.Add(func(s lazy.Spec) []string {
		return []string{
			vtMycnfPath + "/log-error.cnf",
			vtMycnfPath + "/socket.cnf",
		}
	})
}
