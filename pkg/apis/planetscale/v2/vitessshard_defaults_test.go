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

package v2

import (
	"testing"

	"k8s.io/utils/ptr"
)

func TestDefaultVitessShardTabletAvailableSeconds(t *testing.T) {
	t.Run("defaults when unset", func(t *testing.T) {
		vts := &VitessShard{}
		DefaultVitessShard(vts)

		if vts.Spec.TabletAvailableSeconds == nil {
			t.Fatalf("TabletAvailableSeconds = nil, want default %d", DefaultTabletAvailableSeconds)
		}
		if got := *vts.Spec.TabletAvailableSeconds; got != DefaultTabletAvailableSeconds {
			t.Errorf("TabletAvailableSeconds = %d, want default %d", got, DefaultTabletAvailableSeconds)
		}
	})

	t.Run("preserves explicit value", func(t *testing.T) {
		vts := &VitessShard{
			Spec: VitessShardSpec{
				TabletAvailableSeconds: ptr.To(int32(90)),
			},
		}
		DefaultVitessShard(vts)

		if vts.Spec.TabletAvailableSeconds == nil {
			t.Fatal("TabletAvailableSeconds = nil, want 90")
		}
		if got := *vts.Spec.TabletAvailableSeconds; got != 90 {
			t.Errorf("TabletAvailableSeconds = %d, want 90", got)
		}
	})
}
