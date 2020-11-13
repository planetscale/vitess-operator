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
	// transformations and including the hash suffix. This must be at least 12
	// or else there won't be room for the separator and hash suffix for even
	// the shortest possible input. Passing a value less than 12 will result in
	// a panic.
	MaxLength int
	// ValidFirstChar is a function that returns whether the given rune is
	// allowed as the first byte in the output.
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

// JoinWithConstraints works like Join except it enforces some constraints on
// the resulting name while maintaining uniqueness and determinism with respect
// to the input values.
func JoinWithConstraints(cons Constraints, parts ...string) string {
	return JoinSaltWithConstraints(cons, nil, parts...)
}

// JoinSaltWithConstraints works like JoinSalt except it enforces some
// constraints on the resulting name while maintaining uniqueness and
// determinism with respect to the input values.
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
	predictedLength := JoinLength(newParts...)
	if predictedLength <= cons.MaxLength {
		newParts = append(newParts, hash)
		return strings.Join(newParts, "-")
	}

	// Otherwise, we need to truncate the partial result before appending the
	// hash to ensure the full hash fits. We need to cut off enough to get back
	// to MaxLength, and then a little extra to make room for the
	// triple-separator mark we use to indicate that the name was truncated.
	cutLength := predictedLength - cons.MaxLength + 2
	partialResult := strings.Join(newParts, "-")
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
