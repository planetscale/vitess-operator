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

	t.Run("uses default when cell value is nil", func(t *testing.T) {
		spec := &Spec{Cell: &planetscalev2.VitessCellSpec{}}

		got := spec.baseFlags()["tablet-refresh-interval"]
		if got != "1m0s" {
			t.Errorf("tablet-refresh-interval = %v, want 1m0s", got)
		}
	})

	t.Run("uses default when cell value is non-positive", func(t *testing.T) {
		spec := &Spec{
			Cell: &planetscalev2.VitessCellSpec{
				TabletRefreshInterval: &metav1.Duration{Duration: 0},
			},
		}

		got := spec.baseFlags()["tablet-refresh-interval"]
		if got != "1m0s" {
			t.Errorf("tablet-refresh-interval = %v, want 1m0s", got)
		}
	})
}

func TestApplyExtraFlagsIgnoresTabletRefreshInterval(t *testing.T) {
	// The operator owns tablet-refresh-interval to keep it coupled to the
	// tablet-availability gate, so extra flags must not override it in
	// either spelling.
	for _, spelling := range []string{"tablet-refresh-interval", "tablet_refresh_interval", "--tablet-refresh-interval"} {
		t.Run(spelling, func(t *testing.T) {
			spec := &Spec{
				Cell: &planetscalev2.VitessCellSpec{
					TabletRefreshInterval: &metav1.Duration{Duration: 40 * time.Second},
				},
			}
			flags := spec.baseFlags()

			applyExtraFlags(flags, map[string]string{
				spelling:     "5s",
				"other-flag": "kept",
			})

			if got := flags["tablet-refresh-interval"]; got != "40s" {
				t.Errorf("tablet-refresh-interval = %v, want 40s (extra flag %q must not override it)", got, spelling)
			}
			if _, ok := flags["tablet_refresh_interval"]; ok {
				t.Errorf("extra flag %q must not introduce the underscore alias as a second flag", spelling)
			}
			if got := flags["other-flag"]; got != "kept" {
				t.Errorf("other-flag = %v, want kept (unrelated extra flags must still apply)", got)
			}
		})
	}
}
