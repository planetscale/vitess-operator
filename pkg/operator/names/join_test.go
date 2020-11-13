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

package names

import (
	"strings"
	"testing"
)

// TestJoin checks determinism and uniqueness.
func TestJoin(t *testing.T) {
	// Check that it starts with the parts joined by '-'.
	if got, want := DeprecatedJoin("one", "two", "three"), "one-two-three-"; !strings.HasPrefix(got, want) {
		t.Errorf("got %q, want prefix %q", got, want)
	}
	if got, want := JoinWithConstraints(DefaultConstraints, "one", "two", "three"), "one-two-three-"; !strings.HasPrefix(got, want) {
		t.Errorf("got %q, want prefix %q", got, want)
	}

	// Check determinism and uniqueness.
	table := []struct {
		name        string
		a, b        []string
		shouldEqual bool
	}{
		{
			name:        "same parts, same order",
			a:           []string{"one", "two", "three"},
			b:           []string{"one", "two", "three"},
			shouldEqual: true,
		},
		{
			name:        "same parts, different order",
			a:           []string{"one", "two", "three"},
			b:           []string{"one", "three", "two"},
			shouldEqual: false,
		},
		{
			name:        "different parts",
			a:           []string{"one", "two", "three"},
			b:           []string{"one", "two", "four"},
			shouldEqual: false,
		},
		{
			name:        "substring moved to adjacent part",
			a:           []string{"one-two", "three-four"},
			b:           []string{"one", "two-three-four"},
			shouldEqual: false,
		},
		{
			name:        "one part split into two parts",
			a:           []string{"one-two", "three-four"},
			b:           []string{"one-two", "three", "four"},
			shouldEqual: false,
		},
	}
	for _, test := range table {
		if got := DeprecatedJoin(test.a...) == DeprecatedJoin(test.b...); got != test.shouldEqual {
			t.Errorf("DeprecatedJoin: %s: got %v; want %v", test.name, got, test.shouldEqual)
		}
		if got := JoinWithConstraints(DefaultConstraints, test.a...) == JoinWithConstraints(DefaultConstraints, test.b...); got != test.shouldEqual {
			t.Errorf("JoinWithConstraints: %s: got %v; want %v", test.name, got, test.shouldEqual)
		}
	}
}

// TestJoinSalt checks that the salt affects the hash without appearing in the name.
func TestJoinSalt(t *testing.T) {
	salt := []string{"salt1", "salt2"}
	parts := []string{"hello", "world"}
	want := "hello-world-462f1b88"
	if got := DeprecatedJoinSalt(salt, parts...); got != want {
		t.Errorf("DeprecatedJoinSalt(%v, %v) = %q, want %q", salt, parts, got, want)
	}
	if got := JoinSaltWithConstraints(DefaultConstraints, salt, parts...); got != want {
		t.Errorf("JoinSaltWithConstraints(%v, %v) = %q, want %q", salt, parts, got, want)
	}

	salt = []string{"salt1-salt2"}
	parts = []string{"hello", "world"}
	want = "hello-world-c65378ee"
	if got := DeprecatedJoinSalt(salt, parts...); got != want {
		t.Errorf("DeprecatedJoinSalt(%v, %v) = %q, want %q", salt, parts, got, want)
	}
	if got := JoinSaltWithConstraints(DefaultConstraints, salt, parts...); got != want {
		t.Errorf("JoinSaltWithConstraints(%v, %v) = %q, want %q", salt, parts, got, want)
	}
}

// TestJoinHash checks that nobody changed the hash function for Join.
func TestJoinHash(t *testing.T) {
	// DO NOT CHANGE THIS TEST!
	// This is intentionally a change-detection test. If it breaks, you messed up.
	parts := []string{"hello", "world"}
	want := "hello-world-1dd41005"
	if got := DeprecatedJoin(parts...); got != want {
		t.Fatalf("DeprecatedJoin(%v) = %q, want %q", parts, got, want)
	}
	if got := JoinWithConstraints(DefaultConstraints, parts...); got != want {
		t.Fatalf("JoinWithConstraints(%v) = %q, want %q", parts, got, want)
	}
}
