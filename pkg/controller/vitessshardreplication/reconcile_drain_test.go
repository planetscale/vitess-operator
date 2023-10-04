/*
Copyright 2023 PlanetScale Inc.

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

package vitessshardreplication

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSafeMysqldUpgrade(t *testing.T) {
	tests := []struct {
		name      string
		current   string
		desired   string
		needsSafe bool
		err       string
	}{
		{
			name:      "no desired and current",
			needsSafe: false,
		},
		{
			name:      "no desired",
			desired:   "docker.io/vitess/mysql:8.0.23",
			needsSafe: false,
		},
		{
			name:      "no current",
			current:   "docker.io/vitess/mysql:8.0.23",
			needsSafe: false,
		},
		{
			name:      "equal version with same registry",
			current:   "docker.io/vitess/mysql:8.0.23",
			desired:   "docker.io/vitess/mysql:8.0.23",
			needsSafe: false,
		},
		{
			name:      "equal version with different registry",
			current:   "docker.io/vitess/mysql:8.0.23",
			desired:   "docker.io/vitess/mysql:8.0.23",
			needsSafe: false,
		},
		{
			name:      "no explicit current version",
			current:   "docker.io/vitess/mysql:latest",
			desired:   "docker.io/vitess/mysql:8.9.23",
			needsSafe: true,
		},
		{
			name:      "no explicit desired version",
			current:   "docker.io/vitess/mysql:8.0.23",
			desired:   "docker.io/vitess/mysql:latest",
			needsSafe: true,
		},
		{
			name:    "downgrade version",
			current: "docker.io/vitess/mysql:8.0.23",
			desired: "docker.io/vitess/mysql:8.0.22",
			err:     "cannot downgrade patch version from 8.0.23 to 8.0.22",
		},
		{
			name:      "newer version",
			current:   "docker.io/vitess/mysql:8.0.23",
			desired:   "docker.io/vitess/mysql:8.0.24",
			needsSafe: true,
		},
		{
			name:      "newer version until 8.0.34",
			current:   "docker.io/vitess/mysql:8.0.33",
			desired:   "docker.io/vitess/mysql:8.0.34",
			needsSafe: true,
		},
		{
			name:    "downgrade with 8.0.34",
			current: "docker.io/vitess/mysql:8.0.34",
			desired: "docker.io/vitess/mysql:8.0.33",
			err:     "cannot downgrade patch version from 8.0.34 to 8.0.33",
		},
		{
			name:      "newer version skipping 8.0.34",
			current:   "docker.io/vitess/mysql:8.0.33",
			desired:   "docker.io/vitess/mysql:8.0.35",
			needsSafe: true,
		},
		{
			name:    "downgrade skipping 8.0.34",
			current: "docker.io/vitess/mysql:8.0.35",
			desired: "docker.io/vitess/mysql:8.0.33",
			err:     "cannot downgrade patch version from 8.0.35 to 8.0.33",
		},
		{
			name:      "newer version after 8.0.34",
			current:   "docker.io/vitess/mysql:8.0.34",
			desired:   "docker.io/vitess/mysql:8.0.35",
			needsSafe: false,
		},
		{
			name:      "older version after 8.0.35",
			current:   "docker.io/vitess/mysql:8.0.35",
			desired:   "docker.io/vitess/mysql:8.0.34",
			needsSafe: false,
		},
		{
			name:      "early major upgrade",
			current:   "docker.io/vitess/mysql:5.7.35",
			desired:   "docker.io/vitess/mysql:8.0.34",
			needsSafe: true,
		},
		{
			name:      "late major upgrade",
			current:   "docker.io/vitess/mysql:8.2.35",
			desired:   "docker.io/vitess/mysql:9.0.21",
			needsSafe: true,
		},
		{
			name:      "minor upgrade",
			current:   "docker.io/vitess/mysql:8.2.25",
			desired:   "docker.io/vitess/mysql:8.4.12",
			needsSafe: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			needsSafe, err := safeMysqldUpgrade(tt.current, tt.desired)
			if tt.err != "" {
				assert.EqualError(t, err, tt.err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.needsSafe, needsSafe)
			}
		})
	}
}
