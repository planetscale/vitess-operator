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

package vitess

import (
	"fmt"
	"sort"
	"strings"
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

// FormatArgsConvertBoolean returns the flags as a flattened list of
// command-line args with boolean values formatted as `--flag` or `--no-flag`.
// This format is used by some tools, like mysqld_exporter.
// Method is based on FormatArgs().
func (f Flags) FormatArgsConvertBoolean() []string {
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
		// We use two dashes (--) even though the standard flag parser
		// accepts either one or two dashes, because some wrappers like
		// pflags require two dashes.
		//
		// All boolean values are formatted as `--flag` or `--no-flag`.
		value := f[key].(string)
		switch value := strings.ToLower(value); value {
		case "true":
			args = append(args, fmt.Sprintf("--%v", key))
		case "false":
			args = append(args, fmt.Sprintf("--no-%v", key))
		default:
			args = append(args, fmt.Sprintf("--%v=%v", key, value))
		}
	}
	return args
}

// Merge sets the given flags, overwriting duplicates, including ones spelled
// differently (see Set).
//
// The given flags are applied in sorted order, so if they contain more than
// one spelling of the same flag, the one that sorts last wins every time.
func (f Flags) Merge(flags Flags) Flags {
	keys := make([]string, 0, len(flags))
	for key := range flags {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		f.Set(key, flags[key])
	}
	return f
}

// MergeStrings is Merge for string-valued flags, such as user-provided
// extraFlags.
func (f Flags) MergeStrings(flags map[string]string) Flags {
	merge := make(Flags, len(flags))
	for key, value := range flags {
		merge[key] = value
	}
	return f.Merge(merge)
}

// MergeStringFlags returns the union of string-valued flag maps, such as the
// cluster-wide extraVitessFlags and a component's extraFlags. A flag in a
// later map replaces the same flag from an earlier map, whichever spelling
// either one uses.
func MergeStringFlags(layers ...map[string]string) map[string]string {
	merged := Flags{}
	for _, layer := range layers {
		merged.MergeStrings(layer)
	}
	result := make(map[string]string, len(merged))
	for key, value := range merged {
		result[key] = value.(string)
	}
	return result
}

// Set sets the value of a flag. Any leading '-' characters are removed from
// the name, and any value already set under another spelling of the same flag
// is replaced, so the command line never carries both spellings.
func (f Flags) Set(name string, value interface{}) {
	name = strings.TrimLeft(name, "-")
	normalized := normalizeFlagName(name)
	for key := range f {
		if key != name && normalizeFlagName(key) == normalized {
			delete(f, key)
		}
	}
	f[name] = value
}

// Has reports whether the flag is set under any spelling.
func (f Flags) Has(name string) bool {
	normalized := normalizeFlagName(strings.TrimLeft(name, "-"))
	for key := range f {
		if normalizeFlagName(key) == normalized {
			return true
		}
	}
	return false
}

// normalizeFlagName returns the name Vitess resolves a flag to. Vitess
// binaries accept '_' in place of '-' in a flag name that has no '-' at all,
// except for the glog flags below, which keep their underscores. This mirrors
// NormalizeUnderscoresToDashes in vitess.io/vitess/go/vt/utils, which we can't
// call directly because it also prints a deprecation warning.
func normalizeFlagName(name string) string {
	switch name {
	case "log_dir", "log_link", "log_backtrace_at":
		return name
	}
	if strings.Contains(name, "-") {
		return name
	}
	return strings.ReplaceAll(name, "_", "-")
}
