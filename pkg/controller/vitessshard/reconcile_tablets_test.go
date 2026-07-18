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
	"planetscale.dev/vitess-operator/pkg/operator/vttablet"
)

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
