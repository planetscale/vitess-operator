/*
Copyright 2019 PlanetScale Inc.

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
	"reflect"
	"testing"
)

func TestTranslationToVitessKeyRange(t *testing.T) {
	table := []struct {
		hexWidth int32
		parts    int32
		want     []VitessKeyRange
	}{
		{
			parts: 1,
			want: []VitessKeyRange{
				{"", ""},
			},
		},
		{
			parts: 3,
			want: []VitessKeyRange{
				{"", "55"}, {"55", "aa"}, {"aa", ""},
			},
		},
		{
			parts: 6,
			want: []VitessKeyRange{
				{"", "2a"},
				{"2a", "55"},
				{"55", "80"},
				{"80", "aa"},
				{"aa", "d5"},
				{"d5", ""},
			},
		},
		{
			parts: 8,
			want: []VitessKeyRange{
				{"", "20"},
				{"20", "40"},
				{"40", "60"},
				{"60", "80"},
				{"80", "a0"},
				{"a0", "c0"},
				{"c0", "e0"},
				{"e0", ""},
			},
		},
		{
			hexWidth: 2,
			parts:    7,
			want: []VitessKeyRange{
				{"", "2492"},
				{"2492", "4924"},
				{"4924", "6db6"},
				{"6db6", "9249"},
				{"9249", "b6db"},
				{"b6db", "db6d"},
				{"db6d", ""},
			},
		},
		{
			hexWidth: 2,
			parts:    8,
			want: []VitessKeyRange{
				{"", "2000"},
				{"2000", "4000"},
				{"4000", "6000"},
				{"6000", "8000"},
				{"8000", "a000"},
				{"a000", "c000"},
				{"c000", "e000"},
				{"e000", ""},
			},
		},
	}

	for _, test := range table {
		p := VitessKeyspaceEqualPartitioning{
			HexWidth: test.hexWidth,
			Parts:    test.parts,
		}
		if got, want := p.KeyRanges(), test.want; !reflect.DeepEqual(got, want) {
			t.Errorf("KeyRanges(%v) = %#v; want %#v", test.parts, got, want)
		}
	}
}

func TestVitessKeyspacePartitioningTotalReplicas(t *testing.T) {
	equalPartitioning := VitessKeyspacePartitioning{
		Equal: &VitessKeyspaceEqualPartitioning{
			Parts: 2,
			ShardTemplate: VitessShardTemplate{
				TabletPools: []VitessShardTabletPool{
					{
						Cell:     "cell1",
						Replicas: 1,
					},
					{
						Cell:     "cell2",
						Replicas: 2,
					},
				},
			},
		},
	}
	if got := equalPartitioning.TotalReplicas(); got != 6 {
		t.Errorf("equalPartitioning.TotalReplicas() = %v; want 6", got)
	}

	customPartitioning := VitessKeyspacePartitioning{
		Custom: &VitessKeyspaceCustomPartitioning{
			Shards: []VitessKeyspaceKeyRangeShard{
				{
					VitessShardTemplate: VitessShardTemplate{
						TabletPools: []VitessShardTabletPool{
							{
								Cell:     "cell1",
								Replicas: 1,
							},
							{
								Cell:     "cell2",
								Replicas: 2,
							},
						},
					},
				},
				{
					VitessShardTemplate: VitessShardTemplate{
						TabletPools: []VitessShardTabletPool{
							{
								Cell:     "cell1",
								Replicas: 1,
							},
							{
								Cell:     "cell2",
								Replicas: 2,
							},
						},
					},
				},
			},
		},
	}
	if got := customPartitioning.TotalReplicas(); got != 6 {
		t.Errorf("customPartitioning.TotalReplicas() = %v; want 6", got)
	}
}
