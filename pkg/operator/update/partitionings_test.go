/*
Copyright 2020 PlanetScale Inc.

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

package update

import (
	"encoding/json"
	"testing"

	"k8s.io/apimachinery/pkg/api/equality"
	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
)

func TestPartitioningSet(t *testing.T) {
	dst := []planetscalev2.VitessKeyspacePartitioning{
		testEqualPartitioning(2, "should be removed"),
		testEqualPartitioning(4, "original value from dst"),
	}
	src := []planetscalev2.VitessKeyspacePartitioning{
		testEqualPartitioning(4, "try to change value in src"),
		testEqualPartitioning(8, "new partitioning"),
	}
	want := []planetscalev2.VitessKeyspacePartitioning{
		testEqualPartitioning(4, "original value from dst"),
		testEqualPartitioning(8, "new partitioning"),
	}

	PartitioningSet(&dst, src)
	if !equality.Semantic.DeepEqual(dst, want) {
		t.Errorf("dst = %v\nwant: %v", toJSON(dst), toJSON(want))
	}
}

func testEqualPartitioning(parts int32, message string) planetscalev2.VitessKeyspacePartitioning {
	return planetscalev2.VitessKeyspacePartitioning{
		Equal: &planetscalev2.VitessKeyspaceEqualPartitioning{
			Parts: parts,
			ShardTemplate: planetscalev2.VitessShardTemplate{
				Annotations: map[string]string{
					"test": message,
				},
			},
		},
	}
}

func toJSON(obj interface{}) string {
	b, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		panic(err)
	}
	return string(b)
}
