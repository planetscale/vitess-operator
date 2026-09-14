/*
Copyright 2026 PlanetScale Inc.

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

func TestMajorVersionFromImage(t *testing.T) {
	tests := []struct {
		name   string
		image  string
		want   int
		wantOK bool
	}{
		{
			name:   "clean v24 tag",
			image:  "vitess/lite:v24.0.0",
			want:   24,
			wantOK: true,
		},
		{
			name:   "v24 rc with mysql suffix",
			image:  "vitess/lite:v24.0.0-rc1-mysql80",
			want:   24,
			wantOK: true,
		},
		{
			name:   "v23 with mysql suffix",
			image:  "vitess/lite:v23.0.5-mysql80",
			want:   23,
			wantOK: true,
		},
		{
			name:   "large major version",
			image:  "vitess/lite:v100.2.3",
			want:   100,
			wantOK: true,
		},
		{
			name:   "tag and digest both present",
			image:  "vitess/lite:v24.0.0@sha256:abc",
			want:   24,
			wantOK: true,
		},
		{
			name:   "registry prefix with version",
			image:  "registry.example.com/vitess/lite:v25.0.0-mysql80",
			want:   25,
			wantOK: true,
		},
		{
			name:   "rolling mysql tag",
			image:  "vitess/lite:mysql80",
			want:   0,
			wantOK: false,
		},
		{
			name:   "latest tag",
			image:  "vitess/lite:latest",
			want:   0,
			wantOK: false,
		},
		{
			name:   "digest only",
			image:  "vitess/lite@sha256:abc123",
			want:   0,
			wantOK: false,
		},
		{
			name:   "no tag",
			image:  "vitess/lite",
			want:   0,
			wantOK: false,
		},
		{
			name:   "empty",
			image:  "",
			want:   0,
			wantOK: false,
		},
		{
			name:   "tag without v prefix still parses as semver",
			image:  "vitess/lite:24.0.0",
			want:   24,
			wantOK: true,
		},
		{
			name:   "tag with only major still parses (tolerant)",
			image:  "vitess/lite:v24",
			want:   24,
			wantOK: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := MajorVersionFromImage(tc.image)
			assert.Equal(t, tc.wantOK, ok)
			assert.Equal(t, tc.want, got)
		})
	}
}
