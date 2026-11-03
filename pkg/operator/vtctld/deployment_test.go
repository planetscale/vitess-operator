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

package vtctld

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
)

func containerArgs(t *testing.T, containers []corev1.Container, name string) []string {
	t.Helper()
	for i := range containers {
		if containers[i].Name == name {
			return containers[i].Args
		}
	}
	require.Failf(t, "container not found", "no %q container", name)
	return nil
}

// assertDashedFlags checks that every flag in args uses the dashed spelling,
// since Vitess removes the underscore spellings, and that the deprecated glog
// flags are gone.
func assertDashedFlags(t *testing.T, args []string) {
	t.Helper()
	for _, arg := range args {
		name, _, _ := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		assert.NotContains(t, name, "_", "flag %q", arg)
		assert.NotContains(t, []string{"logtostderr", "alsologtostderr"}, name, "flag %q", arg)
	}
}

func assertNoArgPrefix(t *testing.T, args []string, prefix string) {
	t.Helper()
	for _, arg := range args {
		assert.False(t, strings.HasPrefix(arg, prefix), "unexpected %q", arg)
	}
}

func TestDeploymentFlags(t *testing.T) {
	spec := &Spec{
		GlobalLockserver: &planetscalev2.VitessLockserverParams{Implementation: "etcd2", Address: "etcd:2379", RootPath: "/vitess/example/global"},
		Cell:             &planetscalev2.VitessCellTemplate{Name: "zone1"},
		Image:            "vitess/lite:v24.0.5",
		BackupEngine:     planetscalev2.VitessBackupEngineXtraBackup,
		BackupLocation: &planetscalev2.VitessBackupLocation{
			GCS: &planetscalev2.GCSBackupLocation{Bucket: "backups"},
		},
	}
	key := client.ObjectKey{Namespace: "default", Name: "example-zone1-vtctld"}

	args := containerArgs(t, NewDeployment(key, spec, "mysql:8.0.40").Spec.Template.Spec.Containers, containerName)
	assert.Contains(t, args, "--topo-global-server-address=etcd:2379")
	assertDashedFlags(t, args)

	// A user-provided flag replaces the operator's, whichever spelling it uses.
	spec.ExtraFlags = map[string]string{"backup_storage_compress": "false"}
	args = containerArgs(t, NewDeployment(key, spec, "mysql:8.0.40").Spec.Template.Spec.Containers, containerName)
	assert.Contains(t, args, "--backup_storage_compress=false")
	assertNoArgPrefix(t, args, "--backup-storage-compress=")
}

// oldOperatorArgs respells a command line the way older operator versions
// wrote it: underscore flag names, plus --logtostderr.
func oldOperatorArgs(args []string) []string {
	old := []string{"--logtostderr=true"}
	for _, arg := range args {
		name, value, _ := strings.Cut(strings.TrimPrefix(arg, "--"), "=")
		old = append(old, "--"+strings.ReplaceAll(name, "-", "_")+"="+value)
	}
	return old
}

func setContainerArgs(containers []corev1.Container, name string, args []string) {
	for i := range containers {
		if containers[i].Name == name {
			containers[i].Args = args
		}
	}
}

// TestUpdateDeploymentKeepsRespelledArgs checks that a Deployment whose command
// line only differs in flag spelling, as written by an older operator, isn't
// rolled, and that a new image still brings the new command line.
func TestUpdateDeploymentKeepsRespelledArgs(t *testing.T) {
	key := client.ObjectKey{Namespace: "default", Name: "example"}
	spec := &Spec{
		GlobalLockserver: &planetscalev2.VitessLockserverParams{Implementation: "etcd2", Address: "etcd:2379", RootPath: "/vitess/example/global"},
		Cell:             &planetscalev2.VitessCellTemplate{Name: "zone1"},
		Image:            "vitess/lite:v24.0.5",
	}
	newObj := func() *appsv1.Deployment { return NewDeployment(key, spec, "mysql:8.0.40") }
	updateObj := func(obj *appsv1.Deployment) { UpdateDeployment(obj, spec, "mysql:8.0.40") }
	setImage := func() { spec.Image = "vitess/lite:v25.0.0" }

	live := newObj()
	containers := live.Spec.Template.Spec.Containers
	setContainerArgs(containers, containerName, oldOperatorArgs(containerArgs(t, containers, containerName)))

	updated := live.DeepCopy()
	updateObj(updated)
	assert.Equal(t, live, updated)

	setImage()
	updated = live.DeepCopy()
	updateObj(updated)
	assertDashedFlags(t, containerArgs(t, updated.Spec.Template.Spec.Containers, containerName))
}
