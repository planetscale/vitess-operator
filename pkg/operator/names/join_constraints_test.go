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

package names

import (
	"strings"
	"testing"
)

func TestJoinWithConstraints(t *testing.T) {
	cons := Constraints{
		MaxLength:      50,
		ValidFirstChar: isLowercaseLetter,
	}

	table := []struct {
		input      []string
		wantPrefix string
	}{
		{
			input:      []string{"UpperCase-Letters", "Are-Lowercased"},
			wantPrefix: "uppercase-letters-are-lowercased-",
		},
		{
			input:      []string{"allowed-symbols", "dont---change"},
			wantPrefix: "allowed-symbols-dont---change-",
		},
		{
			input:      []string{"disallowed_symbols", "are.replaced"},
			wantPrefix: "disallowed-symbols-are-replaced-",
		},
		{
			input:      []string{"really-really-ridiculously-long-inputs-are-truncated"},
			wantPrefix: "really-really-ridiculously-long-inputs----",
		},
		{
			input:      []string{"-disallowed first chars", "-are prefixed"},
			wantPrefix: "x-disallowed-first-chars--are-prefixed-",
		},
		{
			input:      []string{"Transformed first char", "is ok"},
			wantPrefix: "transformed-first-char-is-ok-",
		},
	}

	for _, test := range table {
		got := JoinWithConstraints(cons, test.input...)
		if !strings.HasPrefix(got, test.wantPrefix) {
			t.Errorf("JoinWithConstraints(%v) = %q; want prefix %q", test.input, got, test.wantPrefix)
		}
	}
}

// TestJoinWithConstraintsMaxLength checks that values are truncated to fit
// within the max length.
func TestJoinWithConstraintsMaxLength(t *testing.T) {
	cons := Constraints{
		MaxLength:      25,
		ValidFirstChar: isLowercaseAlphanumeric,
	}

	// The total length after truncation should be equal to MaxLength.
	out := JoinWithConstraints(cons, strings.Repeat("a", 20), strings.Repeat("b", 20))
	if len(out) != cons.MaxLength {
		t.Errorf("len(%q) = %v; want %v", out, len(out), cons.MaxLength)
	}

	// The outputs should still be unique thanks to the hash suffix,
	// even if the truncated portion is the same because the difference between
	// inputs is at the end that gets cut off.
	out1 := JoinWithConstraints(cons, strings.Repeat("a", 20), strings.Repeat("b", 100)+"1")
	out2 := JoinWithConstraints(cons, strings.Repeat("a", 20), strings.Repeat("b", 100)+"2")
	if out1 == out2 {
		t.Errorf("got same output for two different inputs: %v", out1)
	}
}

// TestJoinWithConstraintsTransform checks that outputs are still
// distinguishable (thanks to the hash) even if inputs differ only in ways that
// are otherwise invisible after transformation.
func TestJoinWithConstraintsTransform(t *testing.T) {
	cons := Constraints{
		MaxLength:      50,
		ValidFirstChar: isLowercaseAlphanumeric,
	}

	// The outputs should still be unique thanks to the hash suffix,
	// even if the differences between inputs are otherwise invisible after
	// transformation.
	out1 := JoinWithConstraints(cons, "disallowed_symbol")
	out2 := JoinWithConstraints(cons, "disallowed/symbol")
	if out1 == out2 {
		t.Errorf("got same output for two different inputs: %v", out1)
	}

	// The outputs should also be different from what regular Join() produces
	// for the transformed value.
	control := DeprecatedJoin("disallowed-symbol")
	if out1 == control {
		t.Errorf("got same output for two different inputs: %v", out1)
	}
	if out2 == control {
		t.Errorf("got same output for two different inputs: %v", out2)
	}
}

// TestJoinWithConstraintsCompatibility checks that JoinWithConstraints() is
// backwards-compatible with Join(). Inputs that don't trigger any constraints
// should give the exact same output (including the hash) as regular Join().
func TestJoinWithConstraintsCompatibility(t *testing.T) {
	cons := Constraints{
		MaxLength:      63,
		ValidFirstChar: isLowercaseAlphanumeric,
	}
	table := [][]string{
		{"abc", "def"},
		{"abc1", "2def3", "a-b-c"},
		{"1abc", "2def3", "a-b-c"},
		{"longnamepart1", "reallyreallyridiculouslylongnamepart2"},
	}
	for _, inputs := range table {
		want := DeprecatedJoin(inputs...)
		got := JoinWithConstraints(cons, inputs...)
		if got != want {
			t.Errorf("JoinWithConstraints(%v) = %q; want %q", inputs, got, want)
		}
	}
}

// TestJoinSaltWithConstraintsCompatibility checks that JoinSaltWithConstraints() is
// backwards-compatible with JoinSalt(). Inputs that don't trigger any constraints
// should give the exact same output (including the hash) as regular JoinSalt().
func TestJoinSaltWithConstraintsCompatibility(t *testing.T) {
	cons := Constraints{
		MaxLength:      63,
		ValidFirstChar: isLowercaseAlphanumeric,
	}
	salt := []string{"salt1", "salt2"}
	table := [][]string{
		{"abc", "def"},
		{"abc1", "2def3", "a-b-c"},
		{"1abc", "2def3", "a-b-c"},
		{"longnamepart1", "reallyreallyridiculouslylongnamepart2"},
	}
	for _, inputs := range table {
		want := DeprecatedJoinSalt(salt, inputs...)
		got := JoinSaltWithConstraints(cons, salt, inputs...)
		if got != want {
			t.Errorf("JoinSaltWithConstraints(%v) = %q; want %q", inputs, got, want)
		}
	}
}
