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

package vtgate

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		Cell: &planetscalev2.VitessCellSpec{
			VitessCellTemplate:    planetscalev2.VitessCellTemplate{Name: "zone1"},
			GlobalLockserver:      planetscalev2.VitessLockserverParams{Implementation: "etcd2", Address: "etcd:2379", RootPath: "/vitess/example/global"},
			AllCells:              []string{"zone1"},
			TabletRefreshInterval: &metav1.Duration{Duration: 40 * time.Second},
		},
		Authentication: &planetscalev2.VitessGatewayAuthentication{
			Static: &planetscalev2.VitessGatewayStaticAuthentication{
				Secret: &planetscalev2.SecretSource{Name: "auth", Key: "users.json"},
			},
		},
		SecureTransport: &planetscalev2.VitessGatewaySecureTransport{
			Required: true,
			TLS: &planetscalev2.VitessGatewayTLSSecureTransport{
				CertSecret:         &planetscalev2.SecretSource{Name: "tls", Key: "cert.pem"},
				KeySecret:          &planetscalev2.SecretSource{Name: "tls", Key: "key.pem"},
				ClientCACertSecret: &planetscalev2.SecretSource{Name: "tls", Key: "ca.pem"},
			},
		},
	}
	key := client.ObjectKey{Namespace: "default", Name: "example-zone1-vtgate"}

	args := containerArgs(t, NewDeployment(key, spec, "mysql:8.0.40").Spec.Template.Spec.Containers, containerName)
	assert.Contains(t, args, "--mysql-auth-server-impl=static")
	assert.Contains(t, args, "--grpc-ca=/vt/secrets/vtgate-tls-ca-cert/ca.pem")
	assertDashedFlags(t, args)

	// A user-provided flag replaces the operator's, whichever spelling it uses.
	spec.ExtraFlags = map[string]string{"cells_to_watch": "zone1,zone2", "mysql_server_ssl_cert": "/user/cert.pem"}
	args = containerArgs(t, NewDeployment(key, spec, "mysql:8.0.40").Spec.Template.Spec.Containers, containerName)
	assert.Contains(t, args, "--cells_to_watch=zone1,zone2")
	assert.Contains(t, args, "--mysql_server_ssl_cert=/user/cert.pem")
	assertNoArgPrefix(t, args, "--cells-to-watch=")
	assertNoArgPrefix(t, args, "--mysql-server-ssl-cert=")
}
