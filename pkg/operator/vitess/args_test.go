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
	corev1 "k8s.io/api/core/v1"
)

func TestEquivalentArgs(t *testing.T) {
	dashed := []string{"--db-charset=utf8mb4", "--port=15000", "--service-map=grpc-queryservice,grpc-tabletmanager"}
	tests := []struct {
		name string
		live []string
		want bool
	}{
		{
			name: "identical",
			live: dashed,
			want: true,
		},
		{
			name: "underscore spelling, different order, glog flag",
			live: []string{"--db_charset=utf8mb4", "--logtostderr=true", "--port=15000", "--service_map=grpc-queryservice,grpc-tabletmanager"},
			want: true,
		},
		{
			name: "different value",
			live: []string{"--db_charset=latin1", "--port=15000", "--service_map=grpc-queryservice,grpc-tabletmanager"},
			want: false,
		},
		{
			name: "missing flag",
			live: []string{"--db_charset=utf8mb4", "--port=15000"},
			want: false,
		},
		{
			name: "extra flag",
			live: append([]string{"--grpc_port=15999"}, dashed...),
			want: false,
		},
		{
			// Vitess combines the two values of a list flag, so this isn't
			// the same command line as one --service-map.
			name: "same flag under two spellings",
			live: append([]string{"--service_map=grpc-throttler"}, dashed...),
			want: false,
		},
		{
			name: "not a --name=value flag",
			live: []string{"--db_charset=utf8mb4", "--port=15000", "--service_map=grpc-queryservice,grpc-tabletmanager", "-v"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, EquivalentArgs(tt.live, dashed))
		})
	}

	// A dropped flag is only ignored when the operator stopped passing it, not
	// when the desired command line sets it, for example from extraFlags.
	assert.False(t, EquivalentArgs([]string{"--logtostderr=true", "--port=15000"}, []string{"--logtostderr=false", "--port=15000"}))
	assert.False(t, EquivalentArgs([]string{"--port=15000"}, []string{"--alsologtostderr=true", "--port=15000"}))
	assert.True(t, EquivalentArgs([]string{"--alsologtostderr=true", "--port=15000"}, []string{"--alsologtostderr=true", "--port=15000"}))

	// Anything other than --name=value flags must match exactly, even when the
	// flags around it only differ in spelling.
	assert.False(t, EquivalentArgs([]string{"-v=1", "--db_charset=utf8mb4"}, []string{"-v=1", "--db-charset=utf8mb4"}))
}

func TestKeepEquivalentArgs(t *testing.T) {
	desired := []string{"--db-charset=utf8mb4", "--port=15000"}
	respelled := []string{"--db_charset=utf8mb4", "--logtostderr=true", "--port=15000"}
	live := []corev1.Container{
		{Name: "sidecar", Image: "sidecar:1", Args: []string{"--x=1"}},
		{Name: "vttablet", Image: "vitess/lite:v24.0.5", Args: respelled},
	}

	// Same image and only respelled: keep the live command line.
	assert.Equal(t, respelled, KeepEquivalentArgs(live, "vttablet", "vitess/lite:v24.0.5", desired))
	// A new image gets the new command line along with it.
	assert.Equal(t, desired, KeepEquivalentArgs(live, "vttablet", "vitess/lite:v25.0.0", desired))
	// A real flag change still goes out.
	changed := []string{"--db-charset=latin1", "--port=15000"}
	assert.Equal(t, changed, KeepEquivalentArgs(live, "vttablet", "vitess/lite:v24.0.5", changed))
	// No live container, as for a new Pod.
	assert.Equal(t, desired, KeepEquivalentArgs(nil, "vttablet", "vitess/lite:v24.0.5", desired))
}
