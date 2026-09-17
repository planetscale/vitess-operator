/*
Copyright 2022 PlanetScale Inc.

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

package vitesscluster

import (
	"context"
	"encoding/json"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/vitesscell"
	"planetscale.dev/vitess-operator/pkg/operator/vtadmin"
	"planetscale.dev/vitess-operator/pkg/operator/vtctld"
	"planetscale.dev/vitess-operator/pkg/operator/vtgate"
)

func TestCreateDiscoverySecretAddresses(t *testing.T) {
	tests := []struct {
		name, vtctldIP, vtgateIP                string
		wantWeb, wantVtctldGRPC, wantVtgateGRPC string
	}{
		{
			name: "IPv4", vtctldIP: "10.0.0.1", vtgateIP: "10.0.0.2",
			wantWeb: "10.0.0.1:15000", wantVtctldGRPC: "10.0.0.1:15999", wantVtgateGRPC: "10.0.0.2:16999",
		},
		{
			name: "IPv6", vtctldIP: "fd01:10:100:1a01::c0f0", vtgateIP: "fd01:10:100:1a01::c0f1",
			wantWeb: "[fd01:10:100:1a01::c0f0]:15000", wantVtctldGRPC: "[fd01:10:100:1a01::c0f0]:15999", wantVtgateGRPC: "[fd01:10:100:1a01::c0f1]:16999",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			scheme := runtime.NewScheme()
			if err := corev1.AddToScheme(scheme); err != nil {
				t.Fatal(err)
			}
			if err := planetscalev2.SchemeBuilder.AddToScheme(scheme); err != nil {
				t.Fatal(err)
			}
			vt := &planetscalev2.VitessCluster{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"}}
			cell := &planetscalev2.VitessCellTemplate{Name: "zone1"}
			k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{Name: vtctld.ServiceName(vt.Name), Namespace: vt.Namespace},
					Spec: corev1.ServiceSpec{ClusterIP: tt.vtctldIP, Ports: []corev1.ServicePort{
						{Name: planetscalev2.DefaultWebPortName, Port: 15000},
						{Name: planetscalev2.DefaultGrpcPortName, Port: 15999},
					}},
				},
				&planetscalev2.VitessCell{ObjectMeta: metav1.ObjectMeta{Name: vitesscell.Name(vt.Name, cell.Name), Namespace: vt.Namespace}},
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{Name: vtgate.ServiceName(vt.Name, cell.Name), Namespace: vt.Namespace},
					Spec: corev1.ServiceSpec{ClusterIP: tt.vtgateIP, Ports: []corev1.ServicePort{
						{Name: planetscalev2.DefaultGrpcPortName, Port: 16999},
					}},
				},
			).Build()
			r := &ReconcileVitessCluster{client: k8sClient}
			discovery, _, err := r.createDiscoverySecret(ctx, vt, cell)
			if err != nil {
				t.Fatal(err)
			}
			if discovery.Name != vtadmin.DiscoverySecretName(vt.Name, cell.Name) {
				t.Fatalf("unexpected discovery secret: %+v", discovery)
			}
			secret := &corev1.Secret{}
			if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: vt.Namespace, Name: discovery.Name}, secret); err != nil {
				t.Fatal(err)
			}
			// The fake client retains StringData rather than applying the API
			// server's conversion to Data.
			var config struct {
				Vtctlds []struct {
					Host struct{ FQDN, Hostname string }
				}
				Vtgates []struct{ Host struct{ Hostname string } }
			}
			if err := json.Unmarshal([]byte(secret.StringData[discovery.Key]), &config); err != nil {
				t.Fatal(err)
			}
			if len(config.Vtctlds) != 1 || len(config.Vtgates) != 1 {
				t.Fatalf("unexpected discovery configuration: %+v", config)
			}
			if got := config.Vtctlds[0].Host.FQDN; got != tt.wantWeb {
				t.Errorf("vtctld web = %q, want %q", got, tt.wantWeb)
			}
			if got := config.Vtctlds[0].Host.Hostname; got != tt.wantVtctldGRPC {
				t.Errorf("vtctld gRPC = %q, want %q", got, tt.wantVtctldGRPC)
			}
			if got := config.Vtgates[0].Host.Hostname; got != tt.wantVtgateGRPC {
				t.Errorf("vtgate gRPC = %q, want %q", got, tt.wantVtgateGRPC)
			}
		})
	}
}
