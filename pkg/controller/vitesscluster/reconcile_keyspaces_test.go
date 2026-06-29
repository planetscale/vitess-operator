/*
Copyright 2024 PlanetScale Inc.

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
	"testing"

	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
)

// TestNewVitessKeyspaceTabletAvailableSeconds verifies that the cluster-level
// TabletAvailableSeconds is propagated down to the VitessKeyspace it creates.
func TestNewVitessKeyspaceTabletAvailableSeconds(t *testing.T) {
	key := client.ObjectKey{Namespace: "test", Name: "cluster1-keyspace1"}
	keyspace := &planetscalev2.VitessKeyspaceTemplate{Name: "keyspace1"}

	t.Run("propagates explicit value", func(t *testing.T) {
		vt := &planetscalev2.VitessCluster{
			Spec: planetscalev2.VitessClusterSpec{
				GlobalLockserver:       planetscalev2.LockserverSpec{Etcd: &planetscalev2.EtcdLockserverTemplate{}},
				TabletAvailableSeconds: ptr.To(int32(90)),
			},
		}

		vtk := newVitessKeyspace(key, vt, nil, keyspace)

		if vtk.Spec.TabletAvailableSeconds == nil {
			t.Fatal("TabletAvailableSeconds = nil, want 90")
		}
		if got := *vtk.Spec.TabletAvailableSeconds; got != 90 {
			t.Errorf("TabletAvailableSeconds = %d, want 90", got)
		}
	})

	t.Run("leaves nil unset for downstream defaulting", func(t *testing.T) {
		vt := &planetscalev2.VitessCluster{
			Spec: planetscalev2.VitessClusterSpec{
				GlobalLockserver: planetscalev2.LockserverSpec{Etcd: &planetscalev2.EtcdLockserverTemplate{}},
			},
		}

		vtk := newVitessKeyspace(key, vt, nil, keyspace)

		if vtk.Spec.TabletAvailableSeconds != nil {
			t.Errorf("TabletAvailableSeconds = %v, want nil", *vtk.Spec.TabletAvailableSeconds)
		}
	})
}
