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

const (
	// hashBytes is the number of bytes included in the result of Hash().
	// This must never be changed since it would break backwards compatibility.
	hashBytes = 4

	// hashLength is the number of characters in the hex-encoded string returned from Hash().
	hashLength = 2 * hashBytes
)

// DeprecatedJoin is deprecated; use JoinWithConstraints instead.
//
// This function should be kept frozen since it's used to verify backwards compatibility.
func DeprecatedJoin(parts ...string) string {
	all := make([]string, 0, len(parts)+1)
	all = append(all, parts...)
	all = append(all, Hash(parts))
	return strings.Join(all, "-")
}

// DeprecatedJoinSalt is deprecated; use JoinSaltWithConstraints instead.
//
// This function should be kept frozen since it's used to verify backwards compatibility.
func DeprecatedJoinSalt(salt []string, parts ...string) string {
	// Include both the salt and name parts in the hash.
	hashParts := make([]string, 0, len(salt)+len(parts))
	hashParts = append(hashParts, salt...)
	hashParts = append(hashParts, parts...)

	// Exclude salt from the name itself.
	nameParts := make([]string, 0, len(parts)+1)
	nameParts = append(nameParts, parts...)
	nameParts = append(nameParts, Hash(hashParts))
	return strings.Join(nameParts, "-")
}

// Hash computes a hash suffix for the given name parts.
func Hash(parts []string) string {
	// DO NOT CHANGE THIS!

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
	return hex.EncodeToString(sum[:hashBytes])
}
