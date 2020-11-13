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
	"fmt"
	"strings"
	"unicode"
)

const (
	// truncationMark is a special separator used when appending the hash to a
	// truncated name to indicate that truncation occurred.
	truncationMark = "---"

	// minTruncatedLength is the shortest possible length of a name that had to
	// be truncated. There has to be at least one character in front of the
	// truncationMark since names can't start with '-', then the truncationMark
	// itself, and finally the hash.
	minTruncatedLength = 1 + len(truncationMark) + hashLength
)

// Constraints specifies rules that the output of JoinWithConstraints must follow.
type Constraints struct {
	// MaxLength is the maximum length of the output, to be enforced after any
	// transformations and including the hash suffix. If a name has to be
	// truncated to fit within this maximum length, the hash at the end will be
	// preceded by a special truncation mark: "---" rather than the usual "-".
	//
	// MaxLength must be at least 12 because that's the shortest possible
	// truncated value (1 char + truncation mark + hash). Passing a value less
	// than 12 will result in a panic.
	MaxLength int
	// ValidFirstChar is a function that returns whether the given rune is
	// allowed as the first character in the output.
	ValidFirstChar func(r rune) bool
}

var (
	// DefaultConstraints are the name constraints for objects in Kubernetes
	// that don't have any special rules.
	DefaultConstraints = Constraints{
		MaxLength:      253,
		ValidFirstChar: isLowercaseAlphanumeric,
	}
	// ServiceConstraints are name constraints for Service objects.
	ServiceConstraints = Constraints{
		MaxLength:      63,
		ValidFirstChar: isLowercaseLetter,
	}
)

/*
JoinWithConstraints builds a name by concatenating a number of parts with '-' as
the separator, and then enforcing some constraints on the resulting name while
maintaining uniqueness and determinism with respect to the input values.

It will append a hash at the end that depends only on the parts supplied.
If the function is called again with the same parts, in the same order,
the hash will also be the same. This determinism allows you to use the resulting
name to ensure idempotency when creating objects.

However, the hash will differ if the parts are rearranged, or if substrings
within parts are moved to adjacent parts. The resulting generated name,
while deterministic, is thus guaranteed to be unique for a given list of parts,
even if the parts themselves are allowed to contain the separator.

For example: JoinWithConstraints(cons, "a-b", "c") != JoinWithConstraints(cons, "a", "b-c")
Although both will begin with "a-b-c-", the hash at the end will be different.

The constraints passed in should be appropriate for the kind of object
(e.g. Pod, Service) whose name is being generated, to ensure the name is
accepted by Kubernetes validation. Most objects in Kubernetes accept any name
that conforms to the DefaultConstraints, with the notable exception of Service
objects which must conform to ServiceConstraints. Custom constraints, such as
for a CRD that adds its own naming requirements, can be expressed by defining a
new Constraints object.

Note that after objects are created with these hashes in their names,
it's often unnecessary to compute the hash to subsequently look those objects up.
Instead, objects should be labeled with key-value pairs corresponding to the
parts that went into the name, allowing direct look-up by label selector.
Since labels are stored as key-value pairs, there is no danger of those values
causing confusion if they happen to contain the separator.
*/
func JoinWithConstraints(cons Constraints, parts ...string) string {
	return JoinSaltWithConstraints(cons, nil, parts...)
}

// JoinSaltWithConstraints works like JoinWithConstraints except the appended
// hash includes additional, hidden salt values that don't get concatenated onto
// the human-readable part of the name.
//
// This can be used to ensure generation of deterministic, unique names when
// some of the determining input values are things that humans shouldn't need to
// pay attention to.
func JoinSaltWithConstraints(cons Constraints, salt []string, parts ...string) string {
	// Always panic immediately if specified Constraints are invalid so we
	// notice the programming error even if the inputs don't happen to trigger
	// the constraints.
	if cons.MaxLength < minTruncatedLength {
		panic(fmt.Sprintf("MaxLength of %v is invalid; must be at least %v", cons.MaxLength, minTruncatedLength))
	}

	if len(parts) == 0 {
		return ""
	}

	// Generate the hash suffix with the original input values so the name will
	// be unique regardless of any transformation or truncation we may have done
	// on the rest of the name.
	hashParts := make([]string, 0, len(salt)+len(parts))
	hashParts = append(hashParts, salt...)
	hashParts = append(hashParts, parts...)
	hash := Hash(hashParts)

	// Transform the input parts to ensure they meet the constraints.
	newParts := make([]string, 0, len(parts)+1)
	transform := func(r rune) rune {
		if isLowercaseAlphanumeric(r) || r == '-' {
			return r
		}
		if isUppercaseLetter(r) {
			return unicode.ToLower(r)
		}
		return '-'
	}
	for _, part := range parts {
		newParts = append(newParts, strings.Map(transform, part))
	}

	// From here on, we can assume the strings in newParts contain only ASCII,
	// which simplifies offset-based access.

	// Check if we need to add a prefix to make sure the first character is valid.
	firstPart := newParts[0]
	if len(firstPart) == 0 || !cons.ValidFirstChar(rune(firstPart[0])) {
		newParts[0] = "x" + firstPart
	}

	// If the predicted length is ok, we just need to append the hash.
	partialResult := strings.Join(newParts, "-")
	predictedLength := len(partialResult) + 1 + len(hash)
	if predictedLength <= cons.MaxLength {
		return partialResult + "-" + hash
	}

	// Otherwise, we need to truncate the partial result before appending the
	// hash to ensure the full hash fits. We need to cut off enough to get back
	// to MaxLength, and then a little extra to make room for the
	// triple-separator mark we use to indicate that the name was truncated.
	cutLength := predictedLength - cons.MaxLength + 2
	partialResult = partialResult[:len(partialResult)-cutLength]
	return partialResult + truncationMark + hash
}

func isLowercaseLetter(r rune) bool {
	return r >= 'a' && r <= 'z'
}

func isUppercaseLetter(r rune) bool {
	return r >= 'A' && r <= 'Z'
}

func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

func isLowercaseAlphanumeric(r rune) bool {
	return isLowercaseLetter(r) || isDigit(r)
}
