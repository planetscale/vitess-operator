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

package vitessshard

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/environment"
	"planetscale.dev/vitess-operator/pkg/operator/vttablet"
)

func newVitessShard(keyspace string, keyRange planetscalev2.VitessKeyRange, pools []planetscalev2.VitessShardTabletPool) *planetscalev2.VitessShard {
	return &planetscalev2.VitessShard{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				planetscalev2.KeyspaceLabel: keyspace,
			},
		},
		Spec: planetscalev2.VitessShardSpec{
			KeyRange: keyRange,
			VitessShardTemplate: planetscalev2.VitessShardTemplate{
				TabletPools: pools,
			},
		},
	}
}

// tabletsForPool returns the tablet specs belonging to the named pool, using
// the pool-name label that vttabletSpecs stamps on every tablet.
func tabletsForPool(tablets []*vttablet.Spec, poolName string) []*vttablet.Spec {
	var out []*vttablet.Spec
	for _, tablet := range tablets {
		if tablet.Labels[planetscalev2.TabletPoolNameLabel] == poolName {
			out = append(out, tablet)
		}
	}
	return out
}

func TestDefaultPoolNamePreservesUID(t *testing.T) {
	cell := "zone1"
	keyspace := "commerce"
	keyRange := planetscalev2.VitessKeyRange{Start: "", End: ""}
	tabletType := planetscalev2.ReplicaPoolType

	shard := newVitessShard(keyspace, keyRange, []planetscalev2.VitessShardTabletPool{
		{
			Cell:     cell,
			Type:     tabletType,
			Name:     "",
			Replicas: 2,
			Vttablet: planetscalev2.VttabletSpec{},
		},
	})

	tablets, err := vttabletSpecs(shard, map[string]string{})
	if err != nil {
		t.Fatalf("vttabletSpecs returned unexpected error: %v", err)
	}

	if len(tablets) != 2 {
		t.Fatalf("expected 1 tablet, got %d", len(tablets))
	}

	for i, tabletIdx := range []uint32{1, 2} {
		expectedUID := vttablet.UID(cell, keyspace, keyRange, tabletType, tabletIdx)
		namedUID := vttablet.UIDWithPoolName(cell, keyspace, keyRange, tabletType, tabletIdx, "")

		if expectedUID == namedUID {
			t.Fatal("expected UID() and UIDWithPoolName() to produce different values")
		}

		if tablets[i].Alias.Uid != expectedUID {
			t.Errorf("got %d, want %d", tablets[i].Alias.Uid, expectedUID)
		}
	}
}

func TestOverriddenDefaultPoolNamePreservesUID(t *testing.T) {
	cell := "zone1"
	keyspace := "commerce"
	keyRange := planetscalev2.VitessKeyRange{Start: "", End: ""}
	tabletType := planetscalev2.ReplicaPoolType

	environment.SetDefaultPoolName("default")
	defer environment.SetDefaultPoolName("")

	shard := newVitessShard(keyspace, keyRange, []planetscalev2.VitessShardTabletPool{
		{
			Cell:     cell,
			Type:     tabletType,
			Name:     "default",
			Replicas: 2,
			Vttablet: planetscalev2.VttabletSpec{},
		},
	})

	tablets, err := vttabletSpecs(shard, map[string]string{})
	if err != nil {
		t.Fatalf("vttabletSpecs returned unexpected error: %v", err)
	}

	if len(tablets) != 2 {
		t.Fatalf("expected 1 tablet, got %d", len(tablets))
	}

	for i, tabletIdx := range []uint32{1, 2} {
		expectedUID := vttablet.UID(cell, keyspace, keyRange, tabletType, tabletIdx)
		namedUID := vttablet.UIDWithPoolName(cell, keyspace, keyRange, tabletType, tabletIdx, "default")

		if expectedUID == namedUID {
			t.Fatal("expected UID() and UIDWithPoolName() to produce different values")
		}

		if tablets[i].Alias.Uid != expectedUID {
			t.Fatalf("default pool name should produce same UID as unnamed pool: got %d, want %d", tablets[i].Alias.Uid, expectedUID)
		}
	}
}

func TestNonDefaultPoolNameGetsUniqueUID(t *testing.T) {
	cell := "zone1"
	keyspace := "commerce"
	keyRange := planetscalev2.VitessKeyRange{Start: "", End: ""}
	tabletType := planetscalev2.ReplicaPoolType

	unnamedUID := vttablet.UID(cell, keyspace, keyRange, tabletType, 1)
	namedUID := vttablet.UIDWithPoolName(cell, keyspace, keyRange, tabletType, 1, "fast-storage")

	environment.SetDefaultPoolName("default")
	defer environment.SetDefaultPoolName("")

	shard := newVitessShard(keyspace, keyRange, []planetscalev2.VitessShardTabletPool{
		{
			Cell:     cell,
			Type:     tabletType,
			Name:     "fast-storage",
			Replicas: 1,
			Vttablet: planetscalev2.VttabletSpec{},
		},
	})

	tablets, err := vttabletSpecs(shard, map[string]string{})
	if err != nil {
		t.Fatalf("vttabletSpecs returned unexpected error: %v", err)
	}

	if len(tablets) != 1 {
		t.Fatalf("expected 1 tablet, got %d", len(tablets))
	}

	if tablets[0].Alias.Uid == unnamedUID {
		t.Error("non-default pool name should not produce the same UID as an unnamed pool")
	}
	if tablets[0].Alias.Uid != namedUID {
		t.Errorf("non-default pool name should use UIDWithPoolName: got %d, want %d", tablets[0].Alias.Uid, namedUID)
	}
}

func TestMultiplePoolsSameCellType(t *testing.T) {
	cell := "zone1"
	keyspace := "commerce"
	keyRange := planetscalev2.VitessKeyRange{Start: "", End: ""}
	tabletType := planetscalev2.ReplicaPoolType

	environment.SetDefaultPoolName("default")
	defer environment.SetDefaultPoolName("")

	shard := newVitessShard(keyspace, keyRange, []planetscalev2.VitessShardTabletPool{
		{
			Cell:     cell,
			Type:     tabletType,
			Name:     "default",
			Replicas: 2,
			Vttablet: planetscalev2.VttabletSpec{},
		},
		{
			Cell:     cell,
			Type:     tabletType,
			Name:     "fast-storage",
			Replicas: 2,
			Vttablet: planetscalev2.VttabletSpec{},
		},
	})

	tablets, err := vttabletSpecs(shard, map[string]string{})
	if err != nil {
		t.Fatalf("vttabletSpecs returned unexpected error: %v", err)
	}

	if len(tablets) != 4 {
		t.Fatalf("expected 4 tablets, got %d", len(tablets))
	}

	defaultTablets := tabletsForPool(tablets, "default")
	if len(defaultTablets) != 2 {
		t.Fatalf("expected 2 tablets in default pool, got %d", len(defaultTablets))
	}

	for _, tablet := range defaultTablets {
		expectedUID := vttablet.UID(cell, keyspace, keyRange, tabletType, uint32(tablet.Index))
		if tablet.Alias.Uid != expectedUID {
			t.Errorf("default pool tablet %d: got UID %d, want %d (same as unnamed)", tablet.Index, tablet.Alias.Uid, expectedUID)
		}
	}

	// Verify "fast-storage" pool tablets use UIDWithPoolName()
	fastTablets := tabletsForPool(tablets, "fast-storage")
	if len(fastTablets) != 2 {
		t.Fatalf("expected 2 tablets in fast-storage pool, got %d", len(fastTablets))
	}

	for _, tablet := range fastTablets {
		expectedUID := vttablet.UIDWithPoolName(cell, keyspace, keyRange, tabletType, uint32(tablet.Index), "fast-storage")
		if tablet.Alias.Uid != expectedUID {
			t.Errorf("fast-storage pool tablet %d: got UID %d, want %d", tablet.Index, tablet.Alias.Uid, expectedUID)
		}
	}
}

func TestMultipleDefaultPoolsReturnsError(t *testing.T) {
	cell := "zone1"
	keyspace := "commerce"
	keyRange := planetscalev2.VitessKeyRange{Start: "", End: ""}
	tabletType := planetscalev2.ReplicaPoolType

	environment.SetDefaultPoolName("default")
	defer environment.SetDefaultPoolName("")

	shard := newVitessShard(keyspace, keyRange, []planetscalev2.VitessShardTabletPool{
		{
			Cell:     cell,
			Type:     tabletType,
			Name:     "default",
			Replicas: 1,
			Vttablet: planetscalev2.VttabletSpec{},
		},
		{
			Cell:     cell,
			Type:     tabletType,
			Name:     "default",
			Replicas: 1,
			Vttablet: planetscalev2.VttabletSpec{},
		},
	})

	if _, err := vttabletSpecs(shard, map[string]string{}); err == nil {
		t.Fatal("expected an error for multiple default pools, got nil")
	}
}

func TestDuplicatePoolNameReturnsError(t *testing.T) {
	cell := "zone1"
	keyspace := "commerce"
	keyRange := planetscalev2.VitessKeyRange{Start: "", End: ""}
	tabletType := planetscalev2.ReplicaPoolType

	environment.SetDefaultPoolName("default")
	defer environment.SetDefaultPoolName("")

	shard := newVitessShard(keyspace, keyRange, []planetscalev2.VitessShardTabletPool{
		{
			Cell:     cell,
			Type:     tabletType,
			Name:     "fast-storage",
			Replicas: 1,
			Vttablet: planetscalev2.VttabletSpec{},
		},
		{
			Cell:     cell,
			Type:     tabletType,
			Name:     "fast-storage",
			Replicas: 1,
			Vttablet: planetscalev2.VttabletSpec{},
		},
	})

	if _, err := vttabletSpecs(shard, map[string]string{}); err == nil {
		t.Fatal("expected an error for duplicate pool names, got nil")
	}
}

func TestPoolNameLabelAlwaysSet(t *testing.T) {
	cell := "zone1"
	keyspace := "commerce"
	keyRange := planetscalev2.VitessKeyRange{Start: "", End: ""}
	tabletType := planetscalev2.ReplicaPoolType

	environment.SetDefaultPoolName("default")
	defer environment.SetDefaultPoolName("")

	shard := newVitessShard(keyspace, keyRange, []planetscalev2.VitessShardTabletPool{
		{
			Cell:     cell,
			Type:     tabletType,
			Name:     "default",
			Replicas: 2,
			Vttablet: planetscalev2.VttabletSpec{},
		},
		{
			Cell:     cell,
			Type:     tabletType,
			Name:     "fast-storage",
			Replicas: 2,
			Vttablet: planetscalev2.VttabletSpec{},
		},
	})

	tablets, err := vttabletSpecs(shard, map[string]string{})
	if err != nil {
		t.Fatalf("vttabletSpecs returned unexpected error: %v", err)
	}

	if len(tablets) != 4 {
		t.Fatalf("expected 4 tablets, got %d", len(tablets))
	}

	for _, i := range []int{0, 1} {
		if got := tablets[i].Labels[planetscalev2.TabletPoolNameLabel]; got != "default" {
			t.Errorf("expected pool-name label to be %q, got %q", "default", got)
		}
	}

	for _, i := range []int{2, 3} {
		if got := tablets[i].Labels[planetscalev2.TabletPoolNameLabel]; got != "fast-storage" {
			t.Errorf("expected pool-name label to be %q, got %q", "fast-storage", got)
		}
	}
}
