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

package partitioning

import (
	"bytes"
	"encoding/hex"
	"reflect"
	"testing"

	topodatapb "vitess.io/vitess/go/vt/proto/topodata"
)

func TestEqualPartitoningEqualKeyRanges(t *testing.T) {
	// Test some small values.
	table := []struct {
		parts uint64
		want  []topodatapb.KeyRange
	}{
		{
			parts: 1,
			want: []topodatapb.KeyRange{
				genKr("", ""),
			},
		},
		{
			parts: 2,
			want: []topodatapb.KeyRange{
				genKr("", "80"), genKr("80", ""),
			},
		},
		{
			parts: 3,
			want: []topodatapb.KeyRange{
				genKr("", "55"), genKr("55", "aa"), genKr("aa", ""),
			},
		},
		{
			parts: 4,
			want: []topodatapb.KeyRange{
				genKr("", "40"), genKr("40", "80"), genKr("80", "c0"), genKr("c0", ""),
			},
		},
		{
			parts: 5,
			want: []topodatapb.KeyRange{
				genKr("", "33"), genKr("33", "66"), genKr("66", "99"), genKr("99", "cc"), genKr("cc", ""),
			},
		},
		{
			parts: 10,
			want: []topodatapb.KeyRange{
				genKr("", "19"), genKr("19", "33"), genKr("33", "4c"), genKr("4c", "66"), genKr("66", "7f"), genKr("7f", "99"), genKr("99", "b3"), genKr("b3", "cc"), genKr("cc", "e6"), genKr("e6", ""),
			},
		},
	}
	for _, test := range table {
		if got, want := EqualKeyRanges(test.parts), test.want; !reflect.DeepEqual(got, want) {
			t.Errorf("EqualKeyRanges(%v) = %#v; want %#v", test.parts, got, want)
		}
	}

	// Spot check some portions of large values.
	table2 := []struct {
		parts  uint64
		checks map[int]topodatapb.KeyRange
	}{
		{
			parts: 16,
			checks: map[int]topodatapb.KeyRange{
				0:  genKr("", "10"),
				15: genKr("f0", ""),
			},
		},
		{
			parts: 64,
			checks: map[int]topodatapb.KeyRange{
				0:  genKr("", "04"),
				63: genKr("fc", ""),
			},
		},
		{
			// This would have been a pathological case if we didn't
			// use a large enough number as the numerator.
			parts: 129,
			checks: map[int]topodatapb.KeyRange{
				0:   genKr("", "01"),
				128: genKr("fe", ""),
			},
		},
		{
			parts: 256,
			checks: map[int]topodatapb.KeyRange{
				0:   genKr("", "01"),
				255: genKr("ff", ""),
			},
		},
		{
			parts: 500,
			checks: map[int]topodatapb.KeyRange{
				0:   genKr("", "0083"),
				499: genKr("ff7c", ""),
			},
		},
		{
			parts: 512,
			checks: map[int]topodatapb.KeyRange{
				0:   genKr("", "0080"),
				511: genKr("ff80", ""),
			},
		},
		{
			parts: 4096,
			checks: map[int]topodatapb.KeyRange{
				0:    genKr("", "0010"),
				4095: genKr("fff0", ""),
			},
		},
		{
			parts: 65536,
			checks: map[int]topodatapb.KeyRange{
				0:     genKr("", "0001"),
				65535: genKr("ffff", ""),
			},
		},
	}
	for _, test := range table2 {
		got := EqualKeyRanges(test.parts)

		for i, want := range test.checks {
			if !reflect.DeepEqual(got[i], want) {
				t.Errorf("EqualKeyRanges(%v)[%d] = %v; want %v", test.parts, i, got[i], want)
			}
		}
	}

	// Verify some invariants.
	for parts := uint64(1); parts < 1000; parts++ {
		krs := EqualKeyRanges(parts)

		// Start and end are unbounded.
		if got, want := krs[0].Start, []byte(nil); !bytes.Equal(got, want) {
			t.Errorf("EqualKeyRanges(%v)[%d].Start = %#v; want %#v", parts, 0, got, want)
		}
		if got, want := krs[parts-1].End, []byte(nil); !bytes.Equal(got, want) {
			t.Errorf("EqualKeyRanges(%v)[%d].Start = %q; want %q", parts, parts-1, got, want)
		}

		for i, kr := range krs {
			if i > 0 {
				// No gap from previous range.
				if got, want := kr.Start, krs[i-1].End; !bytes.Equal(got, want) {
					t.Errorf("EqualKeyRanges(%v)[%d].Start = %q; want %q", parts, i, got, want)
				}
			}
			// Start and end of each range are not the same (unless n=1).
			if parts > 1 && bytes.Equal(kr.Start, kr.End) {
				t.Errorf("EqualKeyRanges(%v)[%d].Start == End", parts, i)
			}
		}
	}
}

/*************************/
/* TEST HELPER FUNCTIONS */
/*************************/

// MustDecodeHexString helps so we can decode in place without worrying about the error in genKr.  This assumes
// that the string provided is always a valid hex string, which should always be true for manually
// written tests.
func MustDecodeHexString(s string) []byte {
	if s == "" {
		return []byte(nil)
	}
	decoded, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return decoded
}

// genKr easily facilitate generating a topodatapb.KeyRange with only start and end hex strings.
func genKr(start string, end string) topodatapb.KeyRange {
	kr := topodatapb.KeyRange{}

	if start != "" {
		kr.Start = MustDecodeHexString(start)
	}
	if end != "" {
		kr.End = MustDecodeHexString(end)
	}

	return kr
}
