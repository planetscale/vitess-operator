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

package vitess

import (
	"fmt"
	"sort"
)

// Flags represents values for flags to be passed to Vitess binaries.
//
// The keys for this map should *not* have any leading '-' characters.
// These will be added when the flags are formatted as a command line.
//
// The values can be any type that can be converted to string with
// the `%v` format specifier in fmt.Sprintf().
type Flags map[string]interface{}

// FormatArgs returns the flags as a flattened list of command-line args
// in the format needed for a Container spec.
func (f Flags) FormatArgs() []string {
	// Sort flag names so the ordering is deterministic,
	// which is important when diffing object specs.
	// This also makes it easier for humans to find things.
	keys := make([]string, 0, len(f))
	for key := range f {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// Make formatted args list.
	args := make([]string, 0, len(f))
	for _, key := range keys {
		// These args are passed to the command as a string array,
		// so we don't need to worry about quotes or escaping.
		//
		// We don't distinguish between boolean and non-boolean
		// flags, because the flag parsing library that Vitess uses
		// always allows you to use the `--key=value` format, even for
		// boolean flags.
		//
		// We use two dashes (--) even though the standard flag parser
		// accepts either one or two dashes, because some wrappers like
		// pflags require two dashes.
		args = append(args, fmt.Sprintf("--%v=%v", key, f[key]))
	}
	return args
}

// Merge sets the given flags, overwriting duplicates.
func (f Flags) Merge(flags Flags) Flags {
	for key, value := range flags {
		f[key] = value
	}
	return f
}
