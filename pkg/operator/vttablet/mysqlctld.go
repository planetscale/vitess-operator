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
	"k8s.io/utils/pointer"

	"planetscale.dev/vitess-operator/pkg/operator/lazy"
)

const (
	vtRootInitScript = `set -ex
mkdir -p /mnt/vt/bin
cp /vt/bin/mysqlctld /mnt/vt/bin/
mkdir -p /mnt/vt/config
cp -R /vt/config/mycnf /mnt/vt/config/
mkdir -p /mnt/vt/vtdataroot
ln -sf /dev/stderr /mnt/vt/config/stderr.symlink
echo "log-error = /vt/config/stderr.symlink" > /mnt/vt/config/mycnf/log-error.cnf`
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
	// Use an init container to copy only the files we need from the Vitess image.
	// Note specifically that we don't even copy init_db.sql to avoid accidentally using it.
	tabletInitContainers.Add(func(s lazy.Spec) []corev1.Container {
		spec := s.(*Spec)
		return []corev1.Container{
			{
				Name: "init-vt-root",
				SecurityContext: &corev1.SecurityContext{
					RunAsUser: pointer.Int64Ptr(runAsUser),
				},
				Image: spec.Images.Vttablet,
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      vtRootVolumeName,
						MountPath: "/mnt/vt",
					},
				},
				Command: []string{"bash", "-c"},
				Args:    []string{vtRootInitScript},
			},
		}
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
		}
	})
	// Tell mysqld to log to stderr instead of a file, so we can rely on
	// automatic rotation of container logs. This config file is written out
	// by the vtRootInitScript.
	extraMyCnf.Add(func(s lazy.Spec) []string {
		return []string{"/vt/config/mycnf/log-error.cnf"}
	})
}
