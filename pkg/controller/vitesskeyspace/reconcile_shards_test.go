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

package vitesskeyspace

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
)

// TestNewVitessShardTabletRefreshInterval verifies that the keyspace-level
// TabletRefreshInterval is propagated down to the VitessShard it creates.
func TestNewVitessShardTabletRefreshInterval(t *testing.T) {
	key := client.ObjectKey{Namespace: "test", Name: "cluster1-keyspace1-x-x"}
	shard := &planetscalev2.VitessKeyspaceKeyRangeShard{
		KeyRange: planetscalev2.VitessKeyRange{},
	}

	t.Run("propagates explicit value", func(t *testing.T) {
		vtk := &planetscalev2.VitessKeyspace{
			Spec: planetscalev2.VitessKeyspaceSpec{
				TabletRefreshInterval: &metav1.Duration{Duration: 40 * time.Second},
			},
		}

		vts := newVitessShard(key, vtk, nil, shard)

		if vts.Spec.TabletRefreshInterval == nil {
			t.Fatal("TabletRefreshInterval = nil, want 40s")
		}
		if got := vts.Spec.TabletRefreshInterval.Duration; got != 40*time.Second {
			t.Errorf("TabletRefreshInterval = %s, want 40s", got)
		}
	})

	t.Run("leaves nil unset for downstream defaulting", func(t *testing.T) {
		vtk := &planetscalev2.VitessKeyspace{}

		vts := newVitessShard(key, vtk, nil, shard)

		if vts.Spec.TabletRefreshInterval != nil {
			t.Errorf("TabletRefreshInterval = %v, want nil", *vts.Spec.TabletRefreshInterval)
		}
	})
}
