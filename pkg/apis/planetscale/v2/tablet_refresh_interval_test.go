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

package v2

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestTabletAvailableSeconds(t *testing.T) {
	cases := []struct {
		name    string
		refresh time.Duration
		want    int32
	}{
		{"vtgate default 60s -> 120", time.Minute, 120},
		{"30s -> 60", 30 * time.Second, 60},
		{"25s -> 50", 25 * time.Second, 50},
		{"1s -> 2", time.Second, 2},
		{"sub-second floors then clamps to 1", 500 * time.Millisecond, 1},
		{"zero clamps to 1", 0, 1},
		{"negative clamps to 1", -5 * time.Second, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := TabletAvailableSeconds(metav1.Duration{Duration: tc.refresh})
			if got != tc.want {
				t.Errorf("TabletAvailableSeconds(%s) = %d, want %d", tc.refresh, got, tc.want)
			}
		})
	}
}

func TestDefaultTabletRefreshIntervalOnShard(t *testing.T) {
	t.Run("defaults when unset", func(t *testing.T) {
		vts := &VitessShard{}
		DefaultVitessShard(vts)

		if vts.Spec.TabletRefreshInterval == nil {
			t.Fatalf("TabletRefreshInterval = nil, want default %s", defaultTabletRefreshInterval)
		}
		if got := vts.Spec.TabletRefreshInterval.Duration; got != defaultTabletRefreshInterval {
			t.Errorf("TabletRefreshInterval = %s, want %s", got, defaultTabletRefreshInterval)
		}
	})

	t.Run("preserves explicit value", func(t *testing.T) {
		vts := &VitessShard{
			Spec: VitessShardSpec{
				TabletRefreshInterval: &metav1.Duration{Duration: 25 * time.Second},
			},
		}
		DefaultVitessShard(vts)

		if got := vts.Spec.TabletRefreshInterval.Duration; got != 25*time.Second {
			t.Errorf("TabletRefreshInterval = %s, want 25s", got)
		}
	})
}

func TestDefaultTabletRefreshIntervalOnCell(t *testing.T) {
	vtc := &VitessCell{}
	DefaultVitessCell(vtc)

	if vtc.Spec.TabletRefreshInterval == nil {
		t.Fatalf("TabletRefreshInterval = nil, want default %s", defaultTabletRefreshInterval)
	}
	if got := vtc.Spec.TabletRefreshInterval.Duration; got != defaultTabletRefreshInterval {
		t.Errorf("TabletRefreshInterval = %s, want %s", got, defaultTabletRefreshInterval)
	}
}
