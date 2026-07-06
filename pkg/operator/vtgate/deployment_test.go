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
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
)

func TestBaseFlagsTabletRefreshInterval(t *testing.T) {
	t.Run("sets flag from cell value", func(t *testing.T) {
		spec := &Spec{
			Cell: &planetscalev2.VitessCellSpec{
				TabletRefreshInterval: &metav1.Duration{Duration: 40 * time.Second},
			},
		}

		got := spec.baseFlags()["tablet-refresh-interval"]
		if got != "40s" {
			t.Errorf("tablet-refresh-interval = %v, want 40s", got)
		}
	})

	t.Run("omits flag when cell value is nil", func(t *testing.T) {
		spec := &Spec{Cell: &planetscalev2.VitessCellSpec{}}

		if _, ok := spec.baseFlags()["tablet-refresh-interval"]; ok {
			t.Error("tablet-refresh-interval should be absent when unset, letting vtgate use its own default")
		}
	})
}
