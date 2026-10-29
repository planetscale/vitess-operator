/*
Copyright 2024 PlanetScale Inc.

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
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestFormatArgs tests the FormatArgs method of the Flags type.
func TestFormatArgs(t *testing.T) {
	tests := []struct {
		name  string
		flags Flags
		want  []string
	}{
		{
			name:  "empty flags",
			flags: Flags{},
			want:  []string{},
		},
		{
			name: "single flag",
			flags: Flags{
				"flag1": "value1",
			},
			want: []string{"--flag1=value1"},
		},
		{
			name: "multiple flags",
			flags: Flags{
				"flag2": "value2",
				"flag3": "value3",
			},
			want: []string{"--flag2=value2", "--flag3=value3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.flags.FormatArgs()
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestFormatArgsConvertBoolean tests the FormatArgsConvertBoolean method of the Flags type.
func TestFormatArgsConvertBoolean(t *testing.T) {
	tests := []struct {
		name  string
		flags Flags
		want  []string
	}{
		{
			name:  "empty flags",
			flags: Flags{},
			want:  []string{},
		},
		{
			name: "boolean flag true",
			flags: Flags{
				"flag1": "true",
			},
			want: []string{"--flag1"},
		},
		{
			name: "boolean flag false",
			flags: Flags{
				"flag2": "false",
			},
			want: []string{"--no-flag2"},
		},
		{
			name: "non-boolean flag",
			flags: Flags{
				"flag3": "value3",
			},
			want: []string{"--flag3=value3"},
		},
		{
			name: "multiple flags",
			flags: Flags{
				"flag4": "true",
				"flag5": "false",
				"flag6": "value6",
			},
			want: []string{"--flag4", "--no-flag5", "--flag6=value6"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.flags.FormatArgsConvertBoolean()
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestMerge tests the Merge method of the Flags type.
func TestMerge(t *testing.T) {
	tests := []struct {
		name   string
		flags  Flags
		merge  Flags
		result Flags
	}{
		{
			name: "merge empty flags",
			flags: Flags{
				"flag1": "value1",
			},
			merge: Flags{},
			result: Flags{
				"flag1": "value1",
			},
		},
		{
			name: "merge non-empty flags",
			flags: Flags{
				"flag1": "value1",
			},
			merge: Flags{
				"flag2": "value2",
			},
			result: Flags{
				"flag1": "value1",
				"flag2": "value2",
			},
		},
		{
			name: "merge duplicate flags",
			flags: Flags{
				"flag1": "value1",
			},
			merge: Flags{
				"flag1": "value2",
			},
			result: Flags{
				"flag1": "value2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.flags.Merge(tt.merge)
			assert.Equal(t, tt.result, got)
		})
	}
}

// TestSet tests that Set replaces a value set under another spelling of the
// same Vitess flag, and only that.
func TestSet(t *testing.T) {
	tests := []struct {
		name   string
		flags  Flags
		key    string
		value  interface{}
		result Flags
	}{
		{
			name:   "underscore replaces dashed",
			flags:  Flags{"db-charset": "utf8mb4", "port": 15000},
			key:    "db_charset",
			value:  "latin1",
			result: Flags{"db_charset": "latin1", "port": 15000},
		},
		{
			name:   "dashed replaces underscore",
			flags:  Flags{"db_charset": "utf8mb4"},
			key:    "db-charset",
			value:  "latin1",
			result: Flags{"db-charset": "latin1"},
		},
		{
			name:   "leading dashes are removed",
			flags:  Flags{"db-charset": "utf8mb4"},
			key:    "--db_charset",
			value:  "latin1",
			result: Flags{"db_charset": "latin1"},
		},
		{
			// Vitess keeps glog's underscores, so these are two different flags.
			name:   "glog flags keep their underscores",
			flags:  Flags{"log_dir": "/a"},
			key:    "log-dir",
			value:  "/b",
			result: Flags{"log_dir": "/a", "log-dir": "/b"},
		},
		{
			// Vitess only normalizes names without any '-'.
			name:   "mixed names are not normalized",
			flags:  Flags{"db-char-set": "utf8mb4"},
			key:    "db_char-set",
			value:  "latin1",
			result: Flags{"db-char-set": "utf8mb4", "db_char-set": "latin1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.flags.Set(tt.key, tt.value)
			assert.Equal(t, tt.result, tt.flags)
		})
	}
}

// TestHas tests that Has finds a flag under any spelling.
func TestHas(t *testing.T) {
	flags := Flags{"builtinbackup_incremental_restore_path": "/vt"}
	assert.True(t, flags.Has("builtinbackup-incremental-restore-path"))
	assert.True(t, flags.Has("--builtinbackup_incremental_restore_path"))
	assert.False(t, flags.Has("builtinbackup-incremental"))
}

// TestMergeSpellings tests that merging flags never leaves two spellings of the
// same flag, and that the outcome doesn't depend on map iteration order when
// the merged flags contain both spellings themselves.
func TestMergeSpellings(t *testing.T) {
	for i := 0; i < 50; i++ {
		flags := Flags{"db-charset": "utf8mb4", "port": 15000}
		flags.Merge(Flags{"db_charset": "latin1", "db-charset": "ascii"})
		assert.Equal(t, Flags{"db_charset": "latin1", "port": 15000}, flags)
	}
}

// TestMergeStrings tests merging string-valued flags such as extraFlags.
func TestMergeStrings(t *testing.T) {
	flags := Flags{"db-charset": "utf8mb4", "port": 15000}
	flags.MergeStrings(map[string]string{"--db_charset": "latin1", "grpc-port": "15999"})
	assert.Equal(t, Flags{"db_charset": "latin1", "port": 15000, "grpc-port": "15999"}, flags)
}
