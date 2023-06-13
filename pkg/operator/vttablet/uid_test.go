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

package vttablet

import (
	"testing"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
)

// TestUIDHash checks that nobody changed the hash function for UID().
func TestUIDHash(t *testing.T) {
	cell := "cell"
	keyspace := "keyspace"
	keyRange := planetscalev2.VitessKeyRange{Start: "10", End: "20"}
	tabletType := planetscalev2.ReplicaPoolType
	tabletIndex := uint32(1)

	// DO NOT CHANGE THIS VALUE!
	// This is intentionally a change-detection test. If it breaks, you messed up.
	want := uint32(3376898362)

	if got := UID(cell, keyspace, keyRange, tabletType, tabletIndex); got != want {
		t.Fatalf("UID() = %v, want %v", got, want)
	}
}

// TestUIDWithPoolIndexHash checks that nobody changed the hash function for UIDWithPoolIndex().
func TestUIDWithPoolIndexHash(t *testing.T) {
	cell := "cell"
	keyspace := "keyspace"
	keyRange := planetscalev2.VitessKeyRange{Start: "10", End: "20"}
	tabletType := planetscalev2.ReplicaPoolType
	tabletIndex := uint32(1)
	poolIndex := uint32(1)

	// DO NOT CHANGE THIS VALUE!
	// This is intentionally a change-detection test. If it breaks, you messed up.
	want := uint32(3840445776)

	if got := UIDWithPoolIndex(cell, keyspace, keyRange, tabletType, tabletIndex, poolIndex); got != want {
		t.Fatalf("UIDWithPoolIndex() = %v, want %v", got, want)
	}
}
