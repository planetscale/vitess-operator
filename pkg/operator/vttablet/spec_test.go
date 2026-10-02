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

package vttablet

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/utils/ptr"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
)

func TestPoolLabelsPoolName(t *testing.T) {
	tests := []struct {
		name      string
		poolName  *string
		external  bool
		wantLabel bool
	}{
		{name: "unnamed pool without label", poolName: nil, wantLabel: false},
		{name: "unnamed external pool with empty label", poolName: ptr.To(""), external: true, wantLabel: false},
		{name: "named external pool", poolName: ptr.To("fast-storage"), external: true, wantLabel: false},
		{name: "named managed pool", poolName: ptr.To("fast-storage"), wantLabel: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := &Spec{Labels: map[string]string{
				planetscalev2.CellLabel:       "zone1",
				planetscalev2.TabletTypeLabel: string(planetscalev2.ReplicaPoolType),
			}}
			if tt.poolName != nil {
				spec.Labels[planetscalev2.TabletPoolNameLabel] = *tt.poolName
			}
			if tt.external {
				spec.ExternalDatastore = &planetscalev2.ExternalDatastore{}
			}

			value, ok := spec.poolLabels()[planetscalev2.TabletPoolNameLabel]
			assert.Equal(t, tt.wantLabel, ok)
			if tt.wantLabel {
				assert.Equal(t, *tt.poolName, value)
			}
		})
	}
}
