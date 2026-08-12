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

package vitesscluster

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
)

// TestNewVitessCellTabletRefreshInterval verifies that the cluster-level
// TabletRefreshInterval is propagated down to the VitessCell it creates. The
// vtgate Deployment reads it from there to set --tablet-refresh-interval.
func TestNewVitessCellTabletRefreshInterval(t *testing.T) {
	key := client.ObjectKey{Namespace: "test", Name: "cluster1-cell1"}
	cell := &planetscalev2.VitessCellTemplate{Name: "cell1"}

	t.Run("propagates explicit value", func(t *testing.T) {
		vt := &planetscalev2.VitessCluster{
			Spec: planetscalev2.VitessClusterSpec{
				GlobalLockserver:      planetscalev2.LockserverSpec{Etcd: &planetscalev2.EtcdLockserverTemplate{}},
				TabletRefreshInterval: &metav1.Duration{Duration: 40 * time.Second},
			},
		}

		vtc := newVitessCell(key, vt, nil, cell)

		if vtc.Spec.TabletRefreshInterval == nil {
			t.Fatal("TabletRefreshInterval = nil, want 40s")
		}
		if got := vtc.Spec.TabletRefreshInterval.Duration; got != 40*time.Second {
			t.Errorf("TabletRefreshInterval = %s, want 40s", got)
		}
	})

	t.Run("materializes the shared default", func(t *testing.T) {
		vt := &planetscalev2.VitessCluster{
			Spec: planetscalev2.VitessClusterSpec{
				GlobalLockserver: planetscalev2.LockserverSpec{Etcd: &planetscalev2.EtcdLockserverTemplate{}},
			},
		}

		vtc := newVitessCell(key, vt, nil, cell)

		if vtc.Spec.TabletRefreshInterval == nil {
			t.Fatal("TabletRefreshInterval = nil, want 1m0s")
		}
		if got := vtc.Spec.TabletRefreshInterval.Duration; got != time.Minute {
			t.Errorf("TabletRefreshInterval = %v, want 1m0s", got)
		}
	})
}
