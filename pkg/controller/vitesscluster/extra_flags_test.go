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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
)

// TestVtctldExtraFlagsOverrideExtraVitessFlagsInEitherSpelling checks that the
// vtctld extraFlags replace the cluster-wide extraVitessFlags even when the two
// spell the same flag differently, instead of passing both spellings.
func TestVtctldExtraFlagsOverrideExtraVitessFlagsInEitherSpelling(t *testing.T) {
	vt := &planetscalev2.VitessCluster{
		Spec: planetscalev2.VitessClusterSpec{
			Cells:            []planetscalev2.VitessCellTemplate{{Name: "zone1"}},
			ExtraVitessFlags: map[string]string{"grpc_max_message_size": "1024", "db_charset": "utf8mb4"},
			VitessDashboard: &planetscalev2.VitessDashboardSpec{
				Replicas:   ptr.To(int32(1)),
				ExtraFlags: map[string]string{"grpc-max-message-size": "2048"},
			},
		},
	}
	vt.Name = "example"
	vt.Namespace = "default"

	specs := (&ReconcileVitessCluster{}).vtctldSpecs(vt, map[string]string{})
	require.Len(t, specs, 1)
	assert.Equal(t, map[string]string{
		"grpc-max-message-size": "2048",
		"db_charset":            "utf8mb4",
	}, specs[0].ExtraFlags)
}
