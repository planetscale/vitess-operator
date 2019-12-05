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

package stringsets

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStringSetsEqual(t *testing.T) {
	assert.True(t, Equal([]string{"a", "b", "a"}, []string{"a", "a", "b"}))
	assert.True(t, Equal([]string{"a", "a", "a"}, []string{"a", "a", "a"}))
	assert.False(t, Equal([]string{"x", "y", "z"}, []string{"a", "b", "c"}))
	assert.True(t, Equal([]string{"a", "b", "c"}, []string{"c", "a", "b"}))
	assert.False(t, Equal([]string{"c", "b", "b"}, []string{"c", "c", "b"}))
	assert.True(t, Equal([]string{"c", "b1", "b"}, []string{"c", "b1", "b"}))
	assert.True(t, Equal([]string{"awsuseast1b", "awsuseast1c", "awsuseast1d"},
		[]string{"awsuseast1d", "awsuseast1b", "awsuseast1c"}))
	assert.False(t, Equal([]string{"c", "c", "c"}, []string{"c", "c"}))
	assert.False(t, Equal([]string{"c", "c", "c"}, []string{}))
	assert.True(t, Equal([]string{}, []string{}))
}
