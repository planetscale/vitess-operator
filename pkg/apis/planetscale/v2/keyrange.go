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

package v2

import (
	"fmt"
	"sort"
)

// String prints a VitessKeyRange in the format used within Vitess.
// Be aware that this format is not safe to use in Kubernetes object names or
// label values. Use SafeName() for those instead.
func (kr *VitessKeyRange) String() string {
	return fmt.Sprintf("%s-%s", kr.Start, kr.End)
}

// SafeName prints a VitessKeyRange in a format that's safe to use in a
// Kubernetes object name or label value. In particular, such names can
// neither begin nor end with '-' so we fill in a wildcard character 'x'
// to represent an unbounded start or end value (usually represented in
// Vitess by an empty string).
//
// WARNING: DO NOT change the behavior of this function, as that may
//          cause shards to be deleted.
func (kr *VitessKeyRange) SafeName() string {
	start, end := kr.Start, kr.End
	if start == "" {
		// Note that it would be incorrect to fill in "00" for Start.
		// The range "00-10" excludes one keyspace ID that "-10" includes.
		start = "x"
	}
	if end == "" {
		// Note that it would be incorrect to fill in "ff" for End.
		// The range "f0-ff" excludes many keyspace IDs that "f0-" includes.
		end = "x"
	}
	return fmt.Sprintf("%s-%s", start, end)
}

// SortKeyRanges sorts a slice of VitessKeyRange objects.
func SortKeyRanges(krs []VitessKeyRange) {
	sort.Slice(krs, func(i, j int) bool {
		if krs[i].Start < krs[j].Start {
			return true
		}
		if krs[i].Start == krs[j].Start {
			// Sort an empty End value *after* all non-empty values, instead of
			// before, as would be the case in normal lexicographical ordering.
			if krs[i].End != "" && krs[j].End == "" {
				return true
			}
			if krs[i].End == "" && krs[j].End != "" {
				return false
			}
			if krs[i].End < krs[j].End {
				return true
			}
		}
		return false
	})
}
