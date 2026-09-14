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

package vtorc

import (
	"testing"

	"github.com/stretchr/testify/assert"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
)

func newSpecForTest(image, cell string) *Spec {
	return &Spec{
		GlobalLockserver: planetscalev2.VitessLockserverParams{
			Implementation: "etcd2",
			Address:        "etcd:2379",
			RootPath:       "/vitess/global",
		},
		Keyspace: "commerce",
		Shard:    "-",
		Cell:     cell,
		Image:    image,
	}
}

func TestSpecFlagsAlwaysIncludesBaseFlags(t *testing.T) {
	spec := newSpecForTest("vitess/lite:v24.0.0-mysql80", "zone1")
	flags := spec.flags()

	for _, key := range []string{
		"topo_implementation",
		"topo_global_server_address",
		"topo_global_root",
		"port",
		"clusters_to_watch",
		"logtostderr",
	} {
		_, ok := flags[key]
		assert.Truef(t, ok, "expected base flag %q to be present", key)
	}

	assert.Equal(t, "commerce/-", flags["clusters_to_watch"])
}

func TestSpecFlagsCellGatedByVitessVersion(t *testing.T) {
	tests := []struct {
		name     string
		image    string
		wantCell bool
	}{
		{
			name:     "v24 emits cell",
			image:    "vitess/lite:v24.0.0-mysql80",
			wantCell: true,
		},
		{
			name:     "v25 emits cell",
			image:    "vitess/lite:v25.0.0-mysql80",
			wantCell: true,
		},
		{
			name:     "v24 rc emits cell",
			image:    "vitess/lite:v24.0.0-rc1-mysql80",
			wantCell: true,
		},
		{
			name:     "v23 does not emit cell",
			image:    "vitess/lite:v23.0.5-mysql80",
			wantCell: false,
		},
		{
			name:     "v22 does not emit cell",
			image:    "vitess/lite:v22.0.0-mysql80",
			wantCell: false,
		},
		{
			name:     "rolling mysql tag does not emit cell",
			image:    "vitess/lite:mysql80",
			wantCell: false,
		},
		{
			name:     "latest tag does not emit cell",
			image:    "vitess/lite:latest",
			wantCell: false,
		},
		{
			name:     "empty image does not emit cell",
			image:    "",
			wantCell: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			spec := newSpecForTest(tc.image, "zone1")
			flags := spec.flags()

			got, present := flags["cell"]
			if tc.wantCell {
				assert.True(t, present, "expected --cell flag to be present")
				assert.Equal(t, "zone1", got)
			} else {
				assert.False(t, present, "expected --cell flag to be absent, got %v", got)
			}
		})
	}
}
