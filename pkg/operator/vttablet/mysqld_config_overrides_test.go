/*
Copyright 2026 PlanetScale Inc.

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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
)

const (
	sharedOverrides   = "max_connections=200"
	vtbackupOverrides = "innodb_flush_log_at_timeout=2"
)

func TestTabletSpecIgnoresVtbackupMysqldConfigOverrides(t *testing.T) {
	sharedPath := mysqldConfigOverridesMountPath + "/" + mysqldConfigOverridesFile

	tests := []struct {
		name                 string
		configOverrides      string
		wantExtraMyCnf       []string
		wantSharedAnnotation bool
		wantPodConfigVolume  bool
	}{
		{
			name:                 "shared overrides only",
			configOverrides:      sharedOverrides,
			wantExtraMyCnf:       []string{sharedPath, vtbackupExtraMyCnfFile},
			wantSharedAnnotation: true,
			wantPodConfigVolume:  true,
		},
		{
			name: "no overrides",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			spec := &Spec{
				Mysqld: &planetscalev2.MysqldSpec{
					ConfigOverrides: test.configOverrides,
				},
			}

			assert.Equal(t, test.wantExtraMyCnf, overrideMyCnfFiles(extraMyCnf.Get(spec)))

			anns := tabletAnnotations.Get(spec)
			if test.wantSharedAnnotation {
				assert.Equal(t, test.configOverrides, anns[mysqldConfigOverridesAnnotationName])
			} else {
				_, ok := anns[mysqldConfigOverridesAnnotationName]
				assert.False(t, ok)
			}
			_, ok := anns[mysqldVtbackupConfigOverridesAnnotationName]
			assert.False(t, ok)

			vol := findVolume(tabletVolumes.Get(spec), mysqldConfigOverridesVolumeName)
			if !test.wantPodConfigVolume {
				assert.Nil(t, vol)
				assert.Nil(t, findVolumeMount(tabletVolumeMounts.Get(spec), mysqldConfigOverridesVolumeName))
				return
			}
			require.NotNil(t, vol)
			require.NotNil(t, vol.DownwardAPI)
			assert.NotNil(t, findVolumeMount(tabletVolumeMounts.Get(spec), mysqldConfigOverridesVolumeName))
			assert.Equal(t, []string{mysqldConfigOverridesFile}, downwardAPIPaths(vol))
		})
	}
}

func TestNewPodDoesNotApplyVtbackupMysqldConfigOverrides(t *testing.T) {
	spec := mysqldOverrideTestSpec()
	spec.Mysqld.ConfigOverrides = sharedOverrides

	pod := NewPod(client.ObjectKey{Namespace: "default", Name: "tablet"}, spec)

	assert.Equal(t, sharedOverrides, pod.Annotations[mysqldConfigOverridesAnnotationName])
	_, hasVtbackup := pod.Annotations[mysqldVtbackupConfigOverridesAnnotationName]
	assert.False(t, hasVtbackup)

	mysqld := findContainer(pod.Spec.Containers, MysqldContainerName)
	require.NotNil(t, mysqld)
	assert.Equal(t, []string{
		mysqldConfigOverridesMountPath + "/" + mysqldConfigOverridesFile,
		vtbackupExtraMyCnfFile,
	}, overrideMyCnfFiles(strings.Split(envValue(mysqld.Env, extraMyCnfEnvVar), ":")))

	vol := findVolume(pod.Spec.Volumes, mysqldConfigOverridesVolumeName)
	require.NotNil(t, vol)
	assert.Equal(t, []string{mysqldConfigOverridesFile}, downwardAPIPaths(vol))
}

func TestNewBackupPodAppliesVtbackupMysqldConfigOverrides(t *testing.T) {
	spec := mysqldOverrideTestSpec()
	spec.Mysqld.ConfigOverrides = sharedOverrides

	pod := NewBackupPod(
		client.ObjectKey{Namespace: "default", Name: "backup"},
		&BackupSpec{
			TabletSpec:            spec,
			MysqldConfigOverrides: vtbackupOverrides,
		},
		"mysql:8.0.40",
	)

	assert.Equal(t, sharedOverrides, pod.Annotations[mysqldConfigOverridesAnnotationName])
	assert.Equal(t, vtbackupOverrides, pod.Annotations[mysqldVtbackupConfigOverridesAnnotationName])

	vtbackup := findContainer(pod.Spec.Containers, vtbackupContainerName)
	require.NotNil(t, vtbackup)
	assert.Equal(t, []string{
		mysqldConfigOverridesMountPath + "/" + mysqldConfigOverridesFile,
		mysqldConfigOverridesMountPath + "/" + mysqldVtbackupConfigOverridesFile,
		vtbackupExtraMyCnfFile,
	}, overrideMyCnfFiles(strings.Split(envValue(vtbackup.Env, extraMyCnfEnvVar), ":")))

	vol := findVolume(pod.Spec.Volumes, mysqldConfigOverridesVolumeName)
	require.NotNil(t, vol)
	assert.Equal(t, []string{mysqldConfigOverridesFile, mysqldVtbackupConfigOverridesFile}, downwardAPIPaths(vol))
	assert.NotNil(t, findVolumeMount(vtbackup.VolumeMounts, mysqldConfigOverridesVolumeName))
}

func TestNewBackupPodAppliesVtbackupOverridesWithoutSharedOverrides(t *testing.T) {
	spec := mysqldOverrideTestSpec()

	pod := NewBackupPod(
		client.ObjectKey{Namespace: "default", Name: "backup"},
		&BackupSpec{
			TabletSpec:            spec,
			MysqldConfigOverrides: vtbackupOverrides,
		},
		"mysql:8.0.40",
	)

	_, hasShared := pod.Annotations[mysqldConfigOverridesAnnotationName]
	assert.False(t, hasShared)
	assert.Equal(t, vtbackupOverrides, pod.Annotations[mysqldVtbackupConfigOverridesAnnotationName])

	vtbackup := findContainer(pod.Spec.Containers, vtbackupContainerName)
	require.NotNil(t, vtbackup)
	assert.Equal(t, []string{
		mysqldConfigOverridesMountPath + "/" + mysqldVtbackupConfigOverridesFile,
		vtbackupExtraMyCnfFile,
	}, overrideMyCnfFiles(strings.Split(envValue(vtbackup.Env, extraMyCnfEnvVar), ":")))

	vol := findVolume(pod.Spec.Volumes, mysqldConfigOverridesVolumeName)
	require.NotNil(t, vol)
	assert.Equal(t, []string{mysqldVtbackupConfigOverridesFile}, downwardAPIPaths(vol))
}

func TestNewBackupPodLeavesTabletSpecUnchanged(t *testing.T) {
	spec := mysqldOverrideTestSpec()
	spec.Mysqld.ConfigOverrides = sharedOverrides

	_ = NewBackupPod(
		client.ObjectKey{Namespace: "default", Name: "backup"},
		&BackupSpec{
			TabletSpec:            spec,
			MysqldConfigOverrides: vtbackupOverrides,
		},
		"mysql:8.0.40",
	)

	pod := NewPod(client.ObjectKey{Namespace: "default", Name: "tablet"}, spec)
	_, hasVtbackup := pod.Annotations[mysqldVtbackupConfigOverridesAnnotationName]
	assert.False(t, hasVtbackup)
}

func TestExtraMyCnfWithVtbackupOverrides(t *testing.T) {
	sharedPath := mysqldConfigOverridesMountPath + "/" + mysqldConfigOverridesFile
	vtbackupPath := mysqldConfigOverridesMountPath + "/" + mysqldVtbackupConfigOverridesFile

	assert.Equal(t, vtbackupPath+":"+vtbackupExtraMyCnfFile, extraMyCnfWithVtbackupOverrides(""))
	assert.Equal(t,
		strings.Join([]string{sharedPath, vtbackupPath, vtbackupExtraMyCnfFile}, ":"),
		extraMyCnfWithVtbackupOverrides(strings.Join([]string{sharedPath, vtbackupExtraMyCnfFile}, ":")),
	)
	assert.Equal(t,
		strings.Join([]string{sharedPath, vtbackupPath, vtbackupExtraMyCnfFile}, ":"),
		extraMyCnfWithVtbackupOverrides(strings.Join([]string{sharedPath, vtbackupPath, vtbackupExtraMyCnfFile}, ":")),
	)
}

func mysqldOverrideTestSpec() *Spec {
	return &Spec{
		Labels: map[string]string{
			planetscalev2.ClusterLabel: "example",
		},
		Images: planetscalev2.VitessKeyspaceImages{
			Mysqld: &planetscalev2.MysqldImage{
				Mysql80Compatible: "mysql:8.0.40",
			},
		},
		Vttablet: &planetscalev2.VttabletSpec{},
		Mysqld:   &planetscalev2.MysqldSpec{},
		BackupLocation: &planetscalev2.VitessBackupLocation{
			Volume: &corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}
}

func findVolume(volumes []corev1.Volume, name string) *corev1.Volume {
	for i := range volumes {
		if volumes[i].Name == name {
			return &volumes[i]
		}
	}
	return nil
}

func findVolumeMount(mounts []corev1.VolumeMount, name string) *corev1.VolumeMount {
	for i := range mounts {
		if mounts[i].Name == name {
			return &mounts[i]
		}
	}
	return nil
}

func findContainer(containers []corev1.Container, name string) *corev1.Container {
	for i := range containers {
		if containers[i].Name == name {
			return &containers[i]
		}
	}
	return nil
}

func downwardAPIPaths(vol *corev1.Volume) []string {
	if vol == nil || vol.DownwardAPI == nil {
		return nil
	}
	paths := make([]string, 0, len(vol.DownwardAPI.Items))
	for _, item := range vol.DownwardAPI.Items {
		paths = append(paths, item.Path)
	}
	return paths
}

func overrideMyCnfFiles(files []string) []string {
	var out []string
	for _, file := range files {
		if strings.HasPrefix(file, mysqldConfigOverridesMountPath+"/") || file == vtbackupExtraMyCnfFile {
			out = append(out, file)
		}
	}
	return out
}

func envValue(env []corev1.EnvVar, name string) string {
	for _, item := range env {
		if item.Name == name {
			return item.Value
		}
	}
	return ""
}
