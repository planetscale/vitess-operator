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

package vitessshard

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
)

// TestExtraFlagsOverrideExtraVitessFlagsInEitherSpelling checks that a
// component's extraFlags replace the cluster-wide extraVitessFlags even when
// the two spell the same flag differently, instead of passing both spellings.
func TestExtraFlagsOverrideExtraVitessFlagsInEitherSpelling(t *testing.T) {
	shard := newVitessShard("commerce", []planetscalev2.VitessShardTabletPool{{
		Cell:     "zone1",
		Type:     planetscalev2.ReplicaPoolType,
		Replicas: 1,
		Vttablet: planetscalev2.VttabletSpec{
			ExtraFlags: map[string]string{"db-charset": "latin1"},
		},
	}})
	shard.Spec.ExtraVitessFlags = map[string]string{
		"db_charset":            "utf8mb4",
		"grpc_max_message_size": "1024",
	}
	shard.Spec.VitessOrchestrator = &planetscalev2.VitessOrchestratorSpec{
		ExtraFlags: map[string]string{"--grpc-max-message-size": "2048"},
	}
	labels := map[string]string{planetscalev2.ClusterLabel: "example"}

	t.Run("vttablet", func(t *testing.T) {
		tablets := vttabletSpecs(shard, labels)
		require.NotEmpty(t, tablets)
		for _, tablet := range tablets {
			assert.Equal(t, map[string]string{
				"db-charset":            "latin1",
				"grpc_max_message_size": "1024",
			}, tablet.Vttablet.ExtraFlags)
		}
	})

	t.Run("vtorc", func(t *testing.T) {
		specs := (&ReconcileVitessShard{}).vtorcSpecs(shard, labels)
		require.Len(t, specs, 1)
		assert.Equal(t, map[string]string{
			"db_charset":            "utf8mb4",
			"grpc-max-message-size": "2048",
		}, specs[0].ExtraFlags)
	})
}
