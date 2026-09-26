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
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/vttablet"
)

func TestTabletAvailableRequeueAfterAtBoundary(t *testing.T) {
	window := 2 * time.Minute
	now := time.Date(2026, time.August, 3, 12, 0, 0, 0, time.UTC)
	readySince := now.Add(-window)

	assert.Equal(t, time.Millisecond, tabletAvailableRequeueAfter(readySince, window, now))
}

func newVitessShard(keyspace string, pools []planetscalev2.VitessShardTabletPool) *planetscalev2.VitessShard {
	return &planetscalev2.VitessShard{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				planetscalev2.KeyspaceLabel: keyspace,
			},
		},
		Spec: planetscalev2.VitessShardSpec{
			KeyRange: planetscalev2.VitessKeyRange{},
			VitessShardTemplate: planetscalev2.VitessShardTemplate{
				TabletPools: pools,
			},
		},
	}
}

func TestTabletUidLabelZeroPadded(t *testing.T) {
	// This specific combination generates a UID with leading zero(s)
	cluster := "default"
	cell := "zone1"
	keyspace := "commerce"
	keyRange := planetscalev2.VitessKeyRange{Start: "", End: ""}
	tabletType := planetscalev2.ReplicaPoolType
	tabletIdx := uint32(3)
	wantUID := vttablet.UID(cell, keyspace, keyRange, tabletType, tabletIdx)
	wantUIDStr := vttablet.UIDString(wantUID)

	shard := newVitessShard(keyspace, []planetscalev2.VitessShardTabletPool{
		{
			Cell:     cell,
			Type:     planetscalev2.ReplicaPoolType,
			Replicas: 3,
		},
	})

	parentLabels := map[string]string{
		planetscalev2.ComponentLabel: planetscalev2.VttabletComponentName,
		planetscalev2.ClusterLabel:   cluster,
		planetscalev2.KeyspaceLabel:  keyspace,
		planetscalev2.ShardLabel:     shard.Spec.KeyRange.SafeName(),
	}

	tablets := vttabletSpecs(shard, parentLabels)

	for _, tablet := range tablets {
		uid := tablet.Labels[planetscalev2.TabletUidLabel]
		idx := tablet.Labels[planetscalev2.TabletIndexLabel]

		if len(uid) != 10 {
			t.Errorf("expected uid label for tablet %s to be 10 characters, got %d (%s)", idx, len(uid), uid)
		}

		if uint32(tablet.Index) != tabletIdx {
			continue
		}

		if uid != wantUIDStr {
			t.Errorf("expected tablet with index %d to have uid %q, got %q", tabletIdx, wantUIDStr, uid)
		}
	}
}

func TestNamedTabletPoolsGetDistinctUIDs(t *testing.T) {
	// Multiple pools of one tablet type in one cell, distinguished only by name, with local MySQL
	cluster := "default"
	cell := "zone1"
	keyspace := "commerce"
	poolNames := []string{"node-a", "node-b", "node-c"}

	pools := make([]planetscalev2.VitessShardTabletPool, 0, len(poolNames))
	for _, name := range poolNames {
		pools = append(pools, planetscalev2.VitessShardTabletPool{
			Cell:     cell,
			Type:     planetscalev2.ReplicaPoolType,
			Name:     name,
			Replicas: 1,
		})
	}
	shard := newVitessShard(keyspace, pools)
	parentLabels := map[string]string{
		planetscalev2.ComponentLabel: planetscalev2.VttabletComponentName,
		planetscalev2.ClusterLabel:   cluster,
		planetscalev2.KeyspaceLabel:  keyspace,
		planetscalev2.ShardLabel:     shard.Spec.KeyRange.SafeName(),
	}

	tablets := vttabletSpecs(shard, parentLabels)
	if len(tablets) != len(poolNames) {
		t.Fatalf("expected %d tablets, got %d", len(poolNames), len(tablets))
	}

	seen := make(map[uint32]string, len(tablets))
	for i, tablet := range tablets {
		wantUID := vttablet.UIDWithPoolName(cell, keyspace, shard.Spec.KeyRange, planetscalev2.ReplicaPoolType, uint32(tablet.Index), poolNames[i])
		if tablet.Alias.Uid != wantUID {
			t.Errorf("pool %q: expected uid %v, got %v", poolNames[i], wantUID, tablet.Alias.Uid)
		}
		if other, ok := seen[tablet.Alias.Uid]; ok {
			t.Errorf("pool %q has the same uid as pool %q (%v), so both would produce one Pod and one PVC",
				poolNames[i], other, tablet.Alias.Uid)
		}
		seen[tablet.Alias.Uid] = poolNames[i]

		if got := tablet.Labels[planetscalev2.TabletPoolNameLabel]; got != poolNames[i] {
			t.Errorf("pool %q: expected pool name label %q, got %q", poolNames[i], poolNames[i], got)
		}
	}
}

func TestUnnamedTabletPoolUIDIsUnchanged(t *testing.T) {
	// A pool with no name must keep the UID it has always had
	cluster := "default"
	cell := "zone1"
	keyspace := "commerce"

	shard := newVitessShard(keyspace, []planetscalev2.VitessShardTabletPool{
		{
			Cell:     cell,
			Type:     planetscalev2.ReplicaPoolType,
			Replicas: 3,
		},
	})
	parentLabels := map[string]string{
		planetscalev2.ComponentLabel: planetscalev2.VttabletComponentName,
		planetscalev2.ClusterLabel:   cluster,
		planetscalev2.KeyspaceLabel:  keyspace,
		planetscalev2.ShardLabel:     shard.Spec.KeyRange.SafeName(),
	}

	for _, tablet := range vttabletSpecs(shard, parentLabels) {
		wantUID := vttablet.UID(cell, keyspace, shard.Spec.KeyRange, planetscalev2.ReplicaPoolType, uint32(tablet.Index))
		if tablet.Alias.Uid != wantUID {
			t.Errorf("tablet index %d: expected uid %v, got %v", tablet.Index, wantUID, tablet.Alias.Uid)
		}
		if _, ok := tablet.Labels[planetscalev2.TabletPoolNameLabel]; ok {
			t.Errorf("tablet index %d: unnamed pool should not carry a pool name label", tablet.Index)
		}
	}
}

func TestExternalDatastorePoolUIDIsUnchanged(t *testing.T) {
	// An ExternalDatastore pool with no name has always used the plain UID, so it has to keep using it
	// UIDWithPoolName hashes an empty name differently, so calling it here would rename the tablet
	cluster := "default"
	cell := "zone1"
	keyspace := "commerce"

	shard := newVitessShard(keyspace, []planetscalev2.VitessShardTabletPool{
		{
			Cell:     cell,
			Type:     planetscalev2.ExternalReplicaPoolType,
			Replicas: 2,
			ExternalDatastore: &planetscalev2.ExternalDatastore{
				User:     "vitess",
				Host:     "mysql.example.com",
				Port:     3306,
				Database: keyspace,
			},
		},
	})
	parentLabels := map[string]string{
		planetscalev2.ComponentLabel: planetscalev2.VttabletComponentName,
		planetscalev2.ClusterLabel:   cluster,
		planetscalev2.KeyspaceLabel:  keyspace,
		planetscalev2.ShardLabel:     shard.Spec.KeyRange.SafeName(),
	}

	for _, tablet := range vttabletSpecs(shard, parentLabels) {
		wantUID := vttablet.UID(cell, keyspace, shard.Spec.KeyRange, planetscalev2.ExternalReplicaPoolType, uint32(tablet.Index))
		if tablet.Alias.Uid != wantUID {
			t.Errorf("tablet index %d: expected uid %v, got %v", tablet.Index, wantUID, tablet.Alias.Uid)
		}
		if got, ok := tablet.Labels[planetscalev2.TabletPoolNameLabel]; !ok || got != "" {
			t.Errorf("tablet index %d: expected an empty pool name label, got %q (present %v)", tablet.Index, got, ok)
		}
	}
}
