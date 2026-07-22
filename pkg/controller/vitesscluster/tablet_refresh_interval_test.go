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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
)

func TestRequiredTabletRefreshInterval(t *testing.T) {
	desired := metav1.Duration{Duration: 10 * time.Second}
	statuses := map[string]planetscalev2.VitessClusterCellStatus{
		"zone1": {
			TabletRefreshInterval: &metav1.Duration{Duration: 10 * time.Second},
		},
		"zone2": {
			TabletRefreshInterval: &metav1.Duration{Duration: 60 * time.Second},
		},
	}

	got := requiredTabletRefreshInterval(desired, statuses)

	assert.Equal(t, 60*time.Second, got.Duration)
}

func TestTabletRefreshIntervalObservedByCells(t *testing.T) {
	cells := []planetscalev2.VitessCellTemplate{{Name: "zone1"}, {Name: "zone2"}}
	desired := metav1.Duration{Duration: 10 * time.Second}

	t.Run("waits for every cell", func(t *testing.T) {
		statuses := map[string]planetscalev2.VitessClusterCellStatus{
			"zone1": {
				TabletRefreshInterval: &metav1.Duration{Duration: 10 * time.Second},
			},
			"zone2": {},
		}

		assert.False(t, tabletRefreshIntervalObservedByCells(desired, cells, statuses))
	})

	t.Run("requires the desired interval", func(t *testing.T) {
		statuses := map[string]planetscalev2.VitessClusterCellStatus{
			"zone1": {
				TabletRefreshInterval: &metav1.Duration{Duration: 10 * time.Second},
			},
			"zone2": {
				TabletRefreshInterval: &metav1.Duration{Duration: 60 * time.Second},
			},
		}

		assert.False(t, tabletRefreshIntervalObservedByCells(desired, cells, statuses))
	})

	t.Run("reports convergence", func(t *testing.T) {
		statuses := map[string]planetscalev2.VitessClusterCellStatus{
			"zone1": {
				TabletRefreshInterval: &metav1.Duration{Duration: 10 * time.Second},
			},
			"zone2": {
				TabletRefreshInterval: &metav1.Duration{Duration: 10 * time.Second},
			},
		}

		assert.True(t, tabletRefreshIntervalObservedByCells(desired, cells, statuses))
	})
}

func TestTabletRefreshIntervalObservedByKeyspaces(t *testing.T) {
	keyspaces := []planetscalev2.VitessKeyspaceTemplate{{Name: "commerce"}, {Name: "customer"}}
	requiredInterval := metav1.Duration{Duration: 60 * time.Second}

	t.Run("waits for every keyspace", func(t *testing.T) {
		statuses := map[string]planetscalev2.VitessClusterKeyspaceStatus{
			"commerce": {
				TabletRefreshInterval: &metav1.Duration{Duration: 60 * time.Second},
			},
			"customer": {},
		}

		assert.False(t, tabletRefreshIntervalObservedByKeyspaces(requiredInterval, keyspaces, statuses))
	})

	t.Run("accepts a more conservative gate", func(t *testing.T) {
		statuses := map[string]planetscalev2.VitessClusterKeyspaceStatus{
			"commerce": {
				TabletRefreshInterval: &metav1.Duration{Duration: 60 * time.Second},
			},
			"customer": {
				TabletRefreshInterval: &metav1.Duration{Duration: 90 * time.Second},
			},
		}

		assert.True(t, tabletRefreshIntervalObservedByKeyspaces(requiredInterval, keyspaces, statuses))
	})
}

func TestStagedTabletRefreshInterval(t *testing.T) {
	desired := metav1.Duration{Duration: 60 * time.Second}
	current := &metav1.Duration{Duration: 10 * time.Second}

	require.Same(t, current, stagedTabletRefreshInterval(current, desired, false))

	got := stagedTabletRefreshInterval(current, desired, true)
	require.NotNil(t, got)
	assert.Equal(t, desired.Duration, got.Duration)
}

func TestStagedTabletRefreshGateInterval(t *testing.T) {
	current := &metav1.Duration{Duration: 60 * time.Second}
	lowerTarget := metav1.Duration{Duration: 10 * time.Second}

	got := stagedTabletRefreshGateInterval(current, lowerTarget, false)
	require.NotNil(t, got)
	assert.Equal(t, current.Duration, got.Duration)

	got = stagedTabletRefreshGateInterval(current, lowerTarget, true)
	require.NotNil(t, got)
	assert.Equal(t, lowerTarget.Duration, got.Duration)

	higherTarget := metav1.Duration{Duration: 90 * time.Second}
	got = stagedTabletRefreshGateInterval(current, higherTarget, false)
	require.NotNil(t, got)
	assert.Equal(t, higherTarget.Duration, got.Duration)
}

func TestObservedKeyspaceTabletRefreshInterval(t *testing.T) {
	target := metav1.Duration{Duration: 60 * time.Second}
	current := &metav1.Duration{Duration: 10 * time.Second}
	observed := &metav1.Duration{Duration: 60 * time.Second}

	assert.Nil(t, observedKeyspaceTabletRefreshInterval(current, observed, target))

	current = target.DeepCopy()
	require.Same(t, observed, observedKeyspaceTabletRefreshInterval(current, observed, target))
}
