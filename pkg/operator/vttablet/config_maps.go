package vttablet

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
)

type mysqldInitOpts struct {
	copyCerts    bool
	copyMysqld   bool
	copyVtbackup bool
}

// extraMycnfConfigMapVolume generates a volume that contains a bunch of
// different MySQL cnf files.
func extraMycnfConfigMapVolume() corev1.Volume {
	return corev1.Volume{
		Name: extraMycnfVolumeName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: extraMycnfConfigMapName,
				},
			},
		},
	}
}

// mysqldInitConfigMapVolume generates a Volume containing
// a script that is meant to be run inside an initContainer.
//
// The script prepares and EmptyDir volume with binaries and
// configs so that the volume can be mounted and used inside
// of a mysqld, vtbackup, or vttablet container.
func mysqldInitConfigMapVolume() corev1.Volume {
	return corev1.Volume{
		Name: mysqldInitVolumeName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: mysqldInitConfigMapName,
				},
				Items: []corev1.KeyToPath{
					{
						Key:  mysqldInitKey,
						Path: mysqldInitKey,
						Mode: pointer.Int32(0o777),
					},
				},
			},
		},
	}
}

// mysqldInitEnv generates environment variables that, when
// passed to the mysqld-init script, controls the actions of
// that script.
func mysqldInitEnv(opts mysqldInitOpts) []corev1.EnvVar {
	env := []corev1.EnvVar{
		{Name: "EXTRA_MYCNF_VOLUME_PATH", Value: mntExtraMycnfVolumePath},
		{Name: "MNT_BIN_PATH", Value: mntBinPath},
		{Name: "MNT_CERTS_PATH", Value: mntCertsPath},
		{Name: "MNT_CONFIG_PATH", Value: mntConfigPath},
		{Name: "MNT_DATA_ROOT_PATH", Value: mntDataRootPath},
		{Name: "MNT_MYCNF_PATH", Value: mntMycnfPath},
		{Name: "MNT_ROOT_PATH", Value: mntRootVolumePath},
		{Name: "MNT_STDERR_SYMLINK_PATH", Value: mntStderrSymlinkPath},
		{Name: "MYSQLD_COMMAND", Value: mysqldCommand},
		{Name: "VT_BIN_PATH", Value: vtBinPath},
		{Name: "VT_CONFIG_PATH", Value: vtConfigPath},
		{Name: "VT_MYCNF_PATH", Value: vtMycnfPath},
		{Name: "VTBACKUP_COMMAND", Value: vtbackupCommand},
	}

	if opts.copyCerts {
		env = append(env, corev1.EnvVar{Name: "COPY_CERTS", Value: "1"})
	} else {
		env = append(env, corev1.EnvVar{Name: "COPY_CERTS", Value: "0"})
	}

	if opts.copyMysqld {
		env = append(env, corev1.EnvVar{Name: "COPY_MYSQLD", Value: "1"})
	} else {
		env = append(env, corev1.EnvVar{Name: "COPY_MYSQLD", Value: "0"})
	}

	if opts.copyVtbackup {
		env = append(env, corev1.EnvVar{Name: "COPY_VTBACKUP", Value: "1"})
	} else {
		env = append(env, corev1.EnvVar{Name: "COPY_VTBACKUP", Value: "0"})
	}

	return env
}
