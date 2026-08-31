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

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
)

func TestPoolLabelsWithPoolName(t *testing.T) {
	// A named pool must not select the other pools in its (cell,type) pair as if they were the same pool
	spec := &Spec{Labels: map[string]string{
		planetscalev2.ComponentLabel:      planetscalev2.VttabletComponentName,
		planetscalev2.ClusterLabel:        "default",
		planetscalev2.KeyspaceLabel:       "commerce",
		planetscalev2.ShardLabel:          "x-x",
		planetscalev2.CellLabel:           "zone1",
		planetscalev2.TabletTypeLabel:     string(planetscalev2.ReplicaPoolType),
		planetscalev2.TabletPoolNameLabel: "node-a",
	}}

	if got := spec.poolLabels()[planetscalev2.TabletPoolNameLabel]; got != "node-a" {
		t.Errorf("expected pool name %q in the pool selector, got %q", "node-a", got)
	}
}

func TestPoolLabelsWithoutPoolName(t *testing.T) {
	// A pool with no name must keep the selector it has today, or its Pods are recreated on upgrade
	// An ExternalDatastore pool with no name carries the label with an empty value, and still counts as unnamed
	for _, labelValue := range []string{"", "unset"} {
		spec := &Spec{Labels: map[string]string{
			planetscalev2.ComponentLabel:  planetscalev2.VttabletComponentName,
			planetscalev2.ClusterLabel:    "default",
			planetscalev2.KeyspaceLabel:   "commerce",
			planetscalev2.ShardLabel:      "x-x",
			planetscalev2.CellLabel:       "zone1",
			planetscalev2.TabletTypeLabel: string(planetscalev2.ReplicaPoolType),
		}}
		if labelValue != "unset" {
			spec.Labels[planetscalev2.TabletPoolNameLabel] = labelValue
		}

		if _, ok := spec.poolLabels()[planetscalev2.TabletPoolNameLabel]; ok {
			t.Errorf("pool name label %q: expected no pool name in the pool selector", labelValue)
		}
	}
}
