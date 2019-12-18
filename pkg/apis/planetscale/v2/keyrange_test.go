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
	"math/rand"
	"reflect"
	"testing"
)

func TestKeyRangeString(t *testing.T) {
	table := []struct {
		start, end, want string
	}{
		{"", "", "-"},
		{"", "80", "-80"},
		{"80", "", "80-"},
		{"40", "50", "40-50"},
		{"", "8080", "-8080"},
		{"8080", "", "8080-"},
		{"4040", "4050", "4040-4050"},
	}
	for _, test := range table {
		kr := VitessKeyRange{Start: test.start, End: test.end}
		if got, want := kr.String(), test.want; got != want {
			t.Errorf("%v.String() = %q, want %q", kr, got, want)
		}
	}
}

func TestKeyRangeSafeName(t *testing.T) {
	table := []struct {
		start, end, want string
	}{
		{"", "", "x-x"},
		{"", "80", "x-80"},
		{"80", "", "80-x"},
		{"40", "50", "40-50"},
		{"", "8080", "x-8080"},
		{"8080", "", "8080-x"},
		{"4040", "4050", "4040-4050"},
	}
	for _, test := range table {
		kr := VitessKeyRange{Start: test.start, End: test.end}
		if got, want := kr.SafeName(), test.want; got != want {
			t.Errorf("%v.SafeName() = %q, want %q", kr, got, want)
		}
	}
}

func TestSortKeyRanges(t *testing.T) {
	krs := []VitessKeyRange{
		{"4040", "4050"},
		{"a0", ""},
		{"80", "a0"},
		{"80", ""},
		{"", "80"},
		{"80", "90"},
		{"70", "80"},
		{"", ""},
		{"07", "80"},
	}
	want := []VitessKeyRange{
		{"", "80"},
		{"", ""},
		{"07", "80"},
		{"4040", "4050"},
		{"70", "80"},
		{"80", "90"},
		{"80", "a0"},
		{"80", ""},
		{"a0", ""},
	}

	// Shuffle the input a bunch of times to catch bugs that depend on the order
	// of arguments passed to the 'less' function.
	for i := 0; i < 1000; i++ {
		rand.Shuffle(len(krs), func(i, j int) {
			krs[i], krs[j] = krs[j], krs[i]
		})

		SortKeyRanges(krs)

		if !reflect.DeepEqual(krs, want) {
			t.Fatalf("SortKeyRanges() = %v; want %v", krs, want)
		}
	}
}
