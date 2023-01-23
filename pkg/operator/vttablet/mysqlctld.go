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

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/lazy"
)

const (
	mysqlSocketInitScript = `set -ex
cd ` + vtDataRootPath + `
for mycnf in $(find . -mindepth 2 -maxdepth 2 -path './vt_*/my.cnf'); do
  sed -i -e 's,^socket[ \t]*=.*$,socket = ` + mysqlSocketPath + `,' "${mycnf}"
done
`
)

func init() {
	// Vitess may run mysql containers with stock mysql images. In order to
	// load mysqlctld (or vtbackup) onto those containers, we:
	// - Set up an EmptyDir volume on the pod.
	// - Mount other volumes containing useful configs.
	// - Run an initContainer with the EmptyDir volume mounted, and run a
	//   script inside the initContainer which copies binaries and configs
	//   into place on the EmptyDir volume.
	// - Mount the same EmptyDir volume (now populated with useful things) on
	//   mysqld, vtbackup, or vttablet containers.
	tabletVolumes.Add(func(s lazy.Spec) []corev1.Volume {
		return []corev1.Volume{
			{
				Name: vtRootVolumeName,
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
			mysqldInitConfigMapVolume(),
			extraMycnfConfigMapVolume(),
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
						MountPath: mntRootVolumePath,
					},
					{
						Name:      mysqldInitVolumeName,
						MountPath: mntMysqldInitVolumePath},
					{
						Name:      extraMycnfVolumeName,
						MountPath: mntExtraMycnfVolumePath,
					},
				},
				Command: []string{mntMysqldInitCommand},
				Env: mysqldInitEnv(mysqldInitOpts{
					copyMysqld: true,
				}),
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
	// - Tell mysqld to log to stderr instead of a file, so we can rely on
	//   automatic rotation of container logs.
	// - Also configure the location of the UNIX socket file.
	// - Add configurations shared by vtbackup and vttablet.
	// - Add vtbackup- and vttablet-specific configurations.
	extraMyCnf.Add(func(s lazy.Spec) []string {
		cnfs := []string{logErrorCnfPath, socketCnfPath, vtCnfPath}

		switch s.(type) {
			case *BackupSpec:
				cnfs = append(cnfs, vtbackupCnfPath)
			case *Spec:
				cnfs = append(cnfs, vttabletCnfPath)
		}

		return cnfs
	})
}
