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
