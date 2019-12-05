/*
Copyright 2019 PlanetScale.

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

// Package names is used to generate and manipulate deterministic, unique names for Kubernetes objects.
package names

import (
	"crypto/md5"
	"encoding/hex"
	"strings"
)

// WARNING(enisoc): You can add new ways of generating names, but you can't change
//   the output of any existing function, because that breaks determinism across
//   version upgrades of the operator.

/*
Join builds a name by concatenating a number of parts with '-' as the separator.

It will append a hash at the end that depends only on the parts supplied.
If the function is called again with the same parts, in the same order,
the hash will also be the same. This determinism allows you to use the resulting
name to ensure idempotency when creating objects.

However, the hash will differ if the parts are rearranged, or if substrings
within parts are moved to adjacent parts. The resulting generated name,
while deterministic, is thus guaranteed to be unique for a given list of parts,
even if the parts themselves are allowed to contain the separator.

For example: Join("a-b", "c") != Join("a", "b-c")
Although both will begin with "a-b-c-", the hash at the end will be different.

Note that after objects are created with these hashes in their names,
it's often unnecessary to compute the hash to subsequently look those objects up.
Instead, objects should be labeled with key-value pairs corresponding to the
parts that went into the name, allowing direct look-up by label selector.
Since labels are stored as key-value pairs, there is no danger of those values
causing confusion if they happen to contain the separator.
*/
func Join(parts ...string) string {
	all := make([]string, 0, len(parts)+1)
	all = append(all, parts...)
	all = append(all, hash(parts))
	return strings.Join(all, "-")
}

// hash computes a hash for the given parts.
// DO NOT CHANGE THIS!
func hash(parts []string) string {
	h := md5.New()
	for _, part := range parts {
		h.Write([]byte(part))
		// It doesn't matter if the parts have nulls in them somehow.
		// The important thing is that this separator is not the same as '-'.
		// To collide, both the "hyphen-joined-string" and the hash must match,
		// but you can't mimic two different separators at the same time.
		h.Write([]byte{0})
	}
	sum := h.Sum(nil)
	// We don't need the whole sum; just take the first 32 bits.
	// We only care about avoiding collisions in the case when
	// the concatenated parts without the hash match exactly.
	// That leaves almost no degrees of freedom even if you're
	// trying to collide on purpose.
	return hex.EncodeToString(sum[:4])
}
