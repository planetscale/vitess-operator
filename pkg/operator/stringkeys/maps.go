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

// Package stringkeys returns literal comma separated list of keys from a map, and provides helper functions for
// returning slice from comma separated list
package stringkeys

import (
	"sort"
	"strings"
)

// StringMapKeys returns a comma separated list of only the keys in the map.
func StringMapKeys(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	return strings.Join(keys, ",")
}

// DifferentKeys returns a list of different keys between a comma separated list of keys, and the keys in a map.
func DifferentKeys(m map[string]string, stringKeys string) []string {
	var differentKeys []string
	for _, k := range strings.Split(stringKeys, ",") {
		if _, exist := m[k]; !exist {
			differentKeys = append(differentKeys, k)
		}
	}

	return differentKeys
}
