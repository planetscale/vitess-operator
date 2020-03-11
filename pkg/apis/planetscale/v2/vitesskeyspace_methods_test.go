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

package v2

import (
	"reflect"
	"testing"
)

func TestTranslationToVitessKeyRange(t *testing.T) {
	// Quick test - calling KeyRanges method should call KeyRanges function in partitioning package, and then
	// translate raw Vitess KeyRange to VitessKeyRange.  Most of business logic tests are unit tested within
	// partitioning package.
	table := []struct {
		parts int32
		want  []VitessKeyRange
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
	}

	for _, test := range table {
		p := VitessKeyspaceEqualPartitioning{Parts: test.parts}
		if got, want := p.KeyRanges(), test.want; !reflect.DeepEqual(got, want) {
			t.Errorf("KeyRanges(%v) = %#v; want %#v", test.parts, got, want)
		}
	}
}
