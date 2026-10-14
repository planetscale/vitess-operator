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

func TestMysqldConfigOverridesTabletVsBackup(t *testing.T) {
	sharedPath := mysqldConfigOverridesMountPath + "/" + mysqldConfigOverridesFile
	vtbackupPath := mysqldConfigOverridesMountPath + "/" + mysqldVtbackupConfigOverridesFile

	tests := []struct {
		name                    string
		forBackup               bool
		configOverrides         string
		vtbackupConfigOverrides string
		wantExtraMyCnf          []string
		wantSharedAnnotation    bool
		wantVtbackupAnnotation  bool
		wantPodConfigVolume     bool
		wantSharedVolumeFile    bool
		wantVtbackupVolumeFile  bool
	}{
		{
			name:                 "vttablet with shared overrides only",
			configOverrides:      sharedOverrides,
			wantExtraMyCnf:       []string{sharedPath, vtbackupExtraMyCnfFile},
			wantSharedAnnotation: true,
			wantPodConfigVolume:  true,
			wantSharedVolumeFile: true,
		},
		{
			name:                    "vttablet ignores vtbackup-only overrides",
			configOverrides:         sharedOverrides,
			vtbackupConfigOverrides: vtbackupOverrides,
			wantExtraMyCnf:          []string{sharedPath, vtbackupExtraMyCnfFile},
			wantSharedAnnotation:    true,
			wantPodConfigVolume:     true,
			wantSharedVolumeFile:    true,
		},
		{
			name:                    "vttablet with only vtbackup overrides is unchanged",
			vtbackupConfigOverrides: vtbackupOverrides,
			wantExtraMyCnf:          nil,
		},
		{
			name:                 "vtbackup with shared overrides only",
			forBackup:            true,
			configOverrides:      sharedOverrides,
			wantExtraMyCnf:       []string{sharedPath, vtbackupExtraMyCnfFile},
			wantSharedAnnotation: true,
			wantPodConfigVolume:  true,
			wantSharedVolumeFile: true,
		},
		{
			name:                    "vtbackup with vtbackup-only overrides",
			forBackup:               true,
			vtbackupConfigOverrides: vtbackupOverrides,
			wantExtraMyCnf:          []string{vtbackupPath, vtbackupExtraMyCnfFile},
			wantVtbackupAnnotation:  true,
			wantPodConfigVolume:     true,
			wantVtbackupVolumeFile:  true,
		},
		{
			name:                    "vtbackup with both override files in order",
			forBackup:               true,
			configOverrides:         sharedOverrides,
			vtbackupConfigOverrides: vtbackupOverrides,
			wantExtraMyCnf:          []string{sharedPath, vtbackupPath, vtbackupExtraMyCnfFile},
			wantSharedAnnotation:    true,
			wantVtbackupAnnotation:  true,
			wantPodConfigVolume:     true,
			wantSharedVolumeFile:    true,
			wantVtbackupVolumeFile:  true,
		},
		{
			name: "no overrides",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			spec := &Spec{
				forBackup: test.forBackup,
				Mysqld: &planetscalev2.MysqldSpec{
					ConfigOverrides:         test.configOverrides,
					VtbackupConfigOverrides: test.vtbackupConfigOverrides,
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
			if test.wantVtbackupAnnotation {
				assert.Equal(t, test.vtbackupConfigOverrides, anns[mysqldVtbackupConfigOverridesAnnotationName])
			} else {
				_, ok := anns[mysqldVtbackupConfigOverridesAnnotationName]
				assert.False(t, ok)
			}

			vol := findVolume(tabletVolumes.Get(spec), "pod-config")
			if !test.wantPodConfigVolume {
				assert.Nil(t, vol)
				assert.Nil(t, findVolumeMount(tabletVolumeMounts.Get(spec), "pod-config"))
				return
			}
			require.NotNil(t, vol)
			require.NotNil(t, vol.DownwardAPI)
			assert.NotNil(t, findVolumeMount(tabletVolumeMounts.Get(spec), "pod-config"))

			paths := downwardAPIPaths(vol)
			if test.wantSharedVolumeFile {
				assert.Contains(t, paths, mysqldConfigOverridesFile)
			} else {
				assert.NotContains(t, paths, mysqldConfigOverridesFile)
			}
			if test.wantVtbackupVolumeFile {
				assert.Contains(t, paths, mysqldVtbackupConfigOverridesFile)
			} else {
				assert.NotContains(t, paths, mysqldVtbackupConfigOverridesFile)
			}
		})
	}
}

func TestNewPodDoesNotApplyVtbackupConfigOverrides(t *testing.T) {
	spec := mysqldOverrideTestSpec()
	spec.Mysqld.ConfigOverrides = sharedOverrides
	spec.Mysqld.VtbackupConfigOverrides = vtbackupOverrides

	pod := NewPod(client.ObjectKey{Namespace: "default", Name: "tablet"}, spec)

	assert.Equal(t, sharedOverrides, pod.Annotations[mysqldConfigOverridesAnnotationName])
	_, hasVtbackup := pod.Annotations[mysqldVtbackupConfigOverridesAnnotationName]
	assert.False(t, hasVtbackup)

	mysqld := findContainer(pod.Spec.Containers, MysqldContainerName)
	require.NotNil(t, mysqld)
	assert.Equal(t, []string{
		mysqldConfigOverridesMountPath + "/" + mysqldConfigOverridesFile,
		vtbackupExtraMyCnfFile,
	}, overrideMyCnfFiles(strings.Split(envValue(mysqld.Env, "EXTRA_MY_CNF"), ":")))

	vol := findVolume(pod.Spec.Volumes, "pod-config")
	require.NotNil(t, vol)
	assert.Equal(t, []string{mysqldConfigOverridesFile}, downwardAPIPaths(vol))
}

func TestNewBackupPodAppliesVtbackupConfigOverrides(t *testing.T) {
	spec := mysqldOverrideTestSpec()
	spec.Mysqld.ConfigOverrides = sharedOverrides
	spec.Mysqld.VtbackupConfigOverrides = vtbackupOverrides

	pod := NewBackupPod(
		client.ObjectKey{Namespace: "default", Name: "backup"},
		&BackupSpec{TabletSpec: spec},
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
	}, overrideMyCnfFiles(strings.Split(envValue(vtbackup.Env, "EXTRA_MY_CNF"), ":")))

	vol := findVolume(pod.Spec.Volumes, "pod-config")
	require.NotNil(t, vol)
	assert.Equal(t, []string{mysqldConfigOverridesFile, mysqldVtbackupConfigOverridesFile}, downwardAPIPaths(vol))
}

func TestNewBackupPodAppliesVtbackupOverridesWithoutSharedOverrides(t *testing.T) {
	spec := mysqldOverrideTestSpec()
	spec.Mysqld.VtbackupConfigOverrides = vtbackupOverrides

	pod := NewBackupPod(
		client.ObjectKey{Namespace: "default", Name: "backup"},
		&BackupSpec{TabletSpec: spec},
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
	}, overrideMyCnfFiles(strings.Split(envValue(vtbackup.Env, "EXTRA_MY_CNF"), ":")))
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
