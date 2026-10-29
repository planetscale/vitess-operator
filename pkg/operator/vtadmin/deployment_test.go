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

package vtadmin

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		Cell:          &planetscalev2.VitessCellTemplate{Name: "zone1"},
		ClusterConfig: &planetscalev2.SecretSource{Name: "vtadmin", Key: "clusters.yaml"},
		Discovery:     &planetscalev2.SecretSource{Name: "vtadmin", Key: "discovery.json"},
		Rbac:          &planetscalev2.SecretSource{Name: "vtadmin", Key: "rbac.yaml"},
		WebConfig:     &planetscalev2.SecretSource{Name: "vtadmin", Key: "config.js"},
		Image:         "vitess/lite:v24.0.5",
	}
	key := client.ObjectKey{Namespace: "default", Name: "example-vtadmin"}

	args := containerArgs(t, NewDeployment(key, spec).Spec.Template.Spec.Containers, apiContainerName)
	assert.Contains(t, args, "--http-tracing=true")
	assertDashedFlags(t, args)

	// A user-provided flag replaces the operator's, whichever spelling it uses,
	// except the operator-managed RBAC config, which still wins.
	spec.ExtraFlags = map[string]string{"http_tracing": "false", "rbac_config": "/user/rbac.yaml"}
	args = containerArgs(t, NewDeployment(key, spec).Spec.Template.Spec.Containers, apiContainerName)
	assert.Contains(t, args, "--http_tracing=false")
	assertNoArgPrefix(t, args, "--http-tracing=")
	assertNoArgPrefix(t, args, "--rbac_config=")
}
