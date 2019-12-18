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

package update

// Labels updates labels in 'dst' based on values in 'src'.
// It leaves any extra labels (found in 'dst' but not in 'src') untouched,
// since those might be set by someone else.
func Labels(dst *map[string]string, src map[string]string) {
	StringMap(dst, src)
}

// Annotations updates annotations in 'dst' based on values in 'src'.
// It leaves any extra annotations (found in 'dst' but not in 'src') untouched,
// since those might be set by someone else.
func Annotations(dst *map[string]string, src map[string]string) {
	StringMap(dst, src)
}

// StringMap mutates a destination map to include the key value pairs from a provided
// source map. This behaves as an update in place.
func StringMap(dst *map[string]string, src map[string]string) {
	if *dst == nil {
		*dst = make(map[string]string, len(src))
	}
	for k, v := range src {
		(*dst)[k] = v
	}
}
