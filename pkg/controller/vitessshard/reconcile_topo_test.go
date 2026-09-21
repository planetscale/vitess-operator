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

package vitessshard

import (
	"context"
	"errors"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"

	"vitess.io/vitess/go/vt/topo/memorytopo"
	// The wrangler needs a registered tablet manager client.
	_ "vitess.io/vitess/go/vt/vttablet/grpctmclient"

	topodatapb "vitess.io/vitess/go/vt/proto/topodata"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
)

// TestReconcileTopologyIdleWithoutShardRecord covers the cleanup path after a
// Reshard: `Reshard complete` deletes the source shard records, and the old
// partitioning is removed from the VitessKeyspace afterwards. The shard
// controller must still be able to prove the shard idle so the keyspace
// controller can turn it down, without ever treating an unverifiable state as
// idle.
func TestReconcileTopologyIdleWithoutShardRecord(t *testing.T) {
	const (
		keyspace = "customer"
		oldShard = "-"     // source shard of the reshard
		newA     = "-8000" // replacement shards, the only ones in the serving partitions
		newB     = "8000-"
		cellA    = "zone1"
		cellB    = "zone2"
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ts, factory := memorytopo.NewServerAndFactory(ctx, cellA, cellB)
	defer ts.Close()

	if err := ts.CreateKeyspace(ctx, keyspace, &topodatapb.Keyspace{}); err != nil {
		t.Fatalf("CreateKeyspace: %v", err)
	}
	for _, shard := range []string{oldShard, newA, newB} {
		if err := ts.CreateShard(ctx, keyspace, shard); err != nil {
			t.Fatalf("CreateShard %s: %v", shard, err)
		}
	}
	// Traffic has been switched: every cell serves only the new shards.
	for _, cell := range []string{cellA, cellB} {
		if err := ts.UpdateSrvKeyspace(ctx, cell, keyspace, srvKeyspaceServing(newA, newB)); err != nil {
			t.Fatalf("UpdateSrvKeyspace %s: %v", cell, err)
		}
	}

	vts := &planetscalev2.VitessShard{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-customer-x-x-0",
			Namespace: "default",
			Labels: map[string]string{
				planetscalev2.KeyspaceLabel: keyspace,
			},
		},
		Spec: planetscalev2.VitessShardSpec{
			Name: oldShard,
		},
	}
	planetscalev2.DefaultVitessShard(vts)
	// Pruning needs tablets and cluster cells this test doesn't model.
	vts.Spec.TopologyReconciliation.PruneShardCells = ptr.To(false)
	vts.Spec.TopologyReconciliation.PruneTablets = ptr.To(false)

	recorder := record.NewFakeRecorder(100)
	r := &ReconcileVitessShard{recorder: recorder}

	reconcile := func(t *testing.T) []string {
		t.Helper()
		// The controller resets status at the start of every Reconcile, so a
		// previously observed Idle value is never carried over.
		vts.Status = planetscalev2.NewVitessShardStatus()
		if _, err := r.reconcileTopologyWithServer(ctx, vts, ts); err != nil {
			t.Fatalf("reconcileTopologyWithServer: %v", err)
		}
		return drainEvents(recorder)
	}

	t.Run("record present and not serving", func(t *testing.T) {
		reconcile(t)
		if vts.Status.Idle != corev1.ConditionTrue {
			t.Fatalf("Idle = %q, want True", vts.Status.Idle)
		}
	})

	// Reshard complete removes the source shard record.
	if err := ts.DeleteShard(ctx, keyspace, oldShard); err != nil {
		t.Fatalf("DeleteShard: %v", err)
	}

	t.Run("record missing and not serving", func(t *testing.T) {
		// Repeated reconciles must all converge on True; none may depend on a
		// value observed while the record still existed.
		for i := 0; i < 3; i++ {
			events := reconcile(t)
			if vts.Status.Idle != corev1.ConditionTrue {
				t.Fatalf("pass %d: Idle = %q, want True", i, vts.Status.Idle)
			}
			// HasMaster must stay Unknown, not False: the replication
			// controller reads False as "initialize a primary now".
			if vts.Status.HasMaster != corev1.ConditionUnknown || vts.Status.ServingWrites != corev1.ConditionUnknown {
				t.Fatalf("pass %d: HasMaster = %q, ServingWrites = %q, want Unknown/Unknown", i, vts.Status.HasMaster, vts.Status.ServingWrites)
			}
			for _, e := range events {
				if strings.Contains(e, "TopoGetFailed") {
					t.Fatalf("pass %d: unexpected event %q", i, e)
				}
			}
		}
	})

	t.Run("record missing but still referenced in one cell", func(t *testing.T) {
		// Only zone2's RDONLY partition still points at the old shard.
		sk := srvKeyspaceServing(newA, newB)
		sk.Partitions[2].ShardReferences = append(sk.Partitions[2].ShardReferences, &topodatapb.ShardReference{Name: oldShard})
		if err := ts.UpdateSrvKeyspace(ctx, cellB, keyspace, sk); err != nil {
			t.Fatalf("UpdateSrvKeyspace: %v", err)
		}
		reconcile(t)
		if vts.Status.Idle != corev1.ConditionFalse {
			t.Fatalf("Idle = %q, want False", vts.Status.Idle)
		}
		if err := ts.UpdateSrvKeyspace(ctx, cellB, keyspace, srvKeyspaceServing(newA, newB)); err != nil {
			t.Fatalf("UpdateSrvKeyspace: %v", err)
		}
	})

	t.Run("record missing and topo unreadable", func(t *testing.T) {
		factory.SetError(errors.New("topo unavailable"))
		defer factory.SetError(nil)
		events := reconcile(t)
		if vts.Status.Idle != corev1.ConditionUnknown {
			t.Fatalf("Idle = %q, want Unknown", vts.Status.Idle)
		}
		found := false
		for _, e := range events {
			if strings.Contains(e, "TopoGetFailed") {
				found = true
			}
		}
		if !found {
			t.Fatalf("expected a TopoGetFailed event, got %v", events)
		}
	})
}

func srvKeyspaceServing(shards ...string) *topodatapb.SrvKeyspace {
	refs := func() []*topodatapb.ShardReference {
		out := make([]*topodatapb.ShardReference, 0, len(shards))
		for _, s := range shards {
			out = append(out, &topodatapb.ShardReference{Name: s})
		}
		return out
	}
	return &topodatapb.SrvKeyspace{
		Partitions: []*topodatapb.SrvKeyspace_KeyspacePartition{
			{ServedType: topodatapb.TabletType_PRIMARY, ShardReferences: refs()},
			{ServedType: topodatapb.TabletType_REPLICA, ShardReferences: refs()},
			{ServedType: topodatapb.TabletType_RDONLY, ShardReferences: refs()},
		},
	}
}

func drainEvents(rec *record.FakeRecorder) []string {
	var out []string
	for {
		select {
		case e := <-rec.Events:
			out = append(out, e)
		default:
			return out
		}
	}
}
