/*
Copyright 2020 PlanetScale Inc.

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

package vitesscluster

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	topodatapb "vitess.io/vitess/go/vt/proto/topodata"
	"vitess.io/vitess/go/vt/topo"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/test/integration/framework"
)

func TestTopology(t *testing.T) {
	ctx := context.Background()

	f := framework.NewFixture(ctx, t)
	defer f.TearDown()

	ns := "default"
	cluster := "test-topology"

	// Start with the same basic VitessCluster as the regular test.
	vt := &planetscalev2.VitessCluster{}
	framework.MustDecodeYAML(basicVitessCluster, vt)

	// Set the cluster to use an external lockserver, namely the same etcd that
	// we use for our test k8s apiserver.
	//
	// This lets us actually test the operator's topology interaction.
	// Otherwise, there is no actual etcd since this integration test
	// environment doesn't actually run any Pods.
	params := &planetscalev2.VitessLockserverParams{
		Implementation: "etcd2",
		Address:        framework.EtcdURL(),
		RootPath:       fmt.Sprintf("/vitess/%s/", cluster),
	}
	vt.Spec.GlobalLockserver.External = params

	f.CreateVitessCluster(ns, cluster, vt)

	ts, err := topo.OpenServer(params.Implementation, params.Address, params.RootPath)
	if err != nil {
		f.Fatalf("Can't open topo server: %v", err)
	}

	// Look for topology entries that are populated by the operator.
	verifyTopoRegistration(f, ts)

	// Create some orphaned topology entries and check that the operator prunes them.
	verifyTopoPruning(f, ts)
}

func verifyTopoRegistration(f *framework.Fixture, ts *topo.Server) {
	f.WaitFor("Cells Alias to be registered", func() error {
		aliases, err := ts.GetCellsAliases(f.Context(), true)
		if err != nil {
			return err
		}
		if aliases["planetscale_operator_default"] == nil {
			return fmt.Errorf("planetscale_operator_default cells alias not found")
		}
		return nil
	})

	f.WaitFor("Cells to be registered", func() error {
		if _, err := ts.GetCellInfo(f.Context(), "cell1", true); err != nil {
			return err
		}
		if _, err := ts.GetCellInfo(f.Context(), "cell2", true); err != nil {
			return err
		}
		return nil
	})
}

func populateTopo(f *framework.Fixture, ts *topo.Server) {
	// Add an orphan SrvKeyspace.
	if err := ts.UpdateSrvKeyspace(f.Context(), "cell1", "delete_me", &topodatapb.SrvKeyspace{}); err != nil {
		f.Fatalf("Can't create SrvKeyspace: %v", err)
	}

	// Add an orphan Cell.
	// We have to use a working address for the cell because Vitess tries to
	// connect to the cell to verify it's empty before letting us delete it.
	if err := ts.CreateCellInfo(f.Context(), "delete_me", &topodatapb.CellInfo{
		ServerAddress: framework.EtcdURL(),
		Root:          "/vitess/delete_me/",
	}); err != nil {
		f.Fatalf("Can't create cell: %v", err)
	}

	// Add an orphan Keyspace.
	if err := ts.CreateKeyspace(f.Context(), "delete_me", &topodatapb.Keyspace{}); err != nil {
		f.Fatalf("Can't create keyspace: %v", err)
	}

	// To test shard and tablet pruning, we have to first create the desired
	// shards (which won't be pruned) because the operator doesn't do that.
	// Normally the vttablets do that, but Vitess is not actually running here.
	if err := ts.CreateShard(f.Context(), "keyspace1", "-80"); err != nil {
		f.Fatalf("Can't create shard: %v", err)
	}
	if err := ts.CreateShard(f.Context(), "keyspace1", "80-"); err != nil {
		f.Fatalf("Can't create shard: %v", err)
	}
	if err := ts.CreateShard(f.Context(), "keyspace1", "-"); err != nil {
		f.Fatalf("Can't create shard: %v", err)
	}

	// Add an orphan Shard.
	if err := ts.CreateShard(f.Context(), "keyspace1", "delete_me"); err != nil {
		f.Fatalf("Can't create shard: %v", err)
	}

	// Add an orphan Tablet.
	if err := ts.CreateTablet(f.Context(), &topodatapb.Tablet{
		Alias:    &topodatapb.TabletAlias{Cell: "cell1", Uid: 12345},
		Keyspace: "keyspace1",
		Shard:    "-80",
	}); err != nil {
		f.Fatalf("Can't create tablet: %v", err)
	}

	// Add an orphan ShardCell.
	// We need to use real cell/keyspace/shards that won't be pruned themselves.
	// It also has to be a cell that some, but not all, of the shards in the
	// keyspace deploy to, so the entire SrvKeyspace won't be pruned.
	ts.UpdateSrvKeyspace(f.Context(), "cell3", "keyspace1", &topodatapb.SrvKeyspace{
		Partitions: []*topodatapb.SrvKeyspace_KeyspacePartition{
			{
				ServedType: topodatapb.TabletType_REPLICA,
				ShardReferences: []*topodatapb.ShardReference{
					{Name: "-80"},
					{Name: "80-"},
					{Name: "-"},
				},
			},
			{
				ServedType: topodatapb.TabletType_RDONLY,
				ShardReferences: []*topodatapb.ShardReference{
					{Name: "-80"},
					{Name: "80-"},
					{Name: "-"},
				},
			},
		},
	})
}

func verifyTopoPruning(f *framework.Fixture, ts *topo.Server) {
	// Create a bunch of topo records that we expect the operator to prune.
	// We create everything all at once because the operator only prunes at a
	// fixed resync period, so if we did them in series it would take much longer.
	populateTopo(f, ts)

	// Now wait for all those records to get pruned.
	f.WaitFor("SrvKeyspace to be pruned", func() error {
		_, err := ts.GetSrvKeyspace(f.Context(), "cell1", "delete_me")
		if topo.IsErrType(err, topo.NoNode) {
			return nil
		}
		return fmt.Errorf("got err = %v; want %q", err, "no node")
	})
	f.WaitFor("Cell to be pruned", func() error {
		_, err := ts.GetCellInfo(f.Context(), "delete_me", true)
		if topo.IsErrType(err, topo.NoNode) {
			return nil
		}
		return fmt.Errorf("got err = %v; want %q", err, "no node")
	})
	f.WaitFor("Keyspace to be pruned", func() error {
		_, err := ts.GetKeyspace(f.Context(), "delete_me")
		if topo.IsErrType(err, topo.NoNode) {
			return nil
		}
		return fmt.Errorf("got err = %v; want %q", err, "no node")
	})
	f.WaitFor("Shard to be pruned", func() error {
		_, err := ts.GetShard(f.Context(), "keyspace1", "delete_me")
		if topo.IsErrType(err, topo.NoNode) {
			return nil
		}
		return fmt.Errorf("got err = %v; want %q", err, "no node")
	})
	f.WaitFor("Tablet to be pruned", func() error {
		_, err := ts.GetTablet(f.Context(), &topodatapb.TabletAlias{Cell: "cell1", Uid: 12345})
		if topo.IsErrType(err, topo.NoNode) {
			return nil
		}
		return fmt.Errorf("got err = %v; want %q", err, "no node")
	})
	f.WaitFor("ShardCell to be pruned", func() error {
		ks, err := ts.GetSrvKeyspace(f.Context(), "cell3", "keyspace1")
		if err != nil {
			return err
		}
		// Only one of the shards actually deploys to cell3, so pruning should
		// remove everything else from each partition's shard list.
		for i := range ks.Partitions {
			shardNames := []string{}
			for _, ref := range ks.Partitions[i].ShardReferences {
				shardNames = append(shardNames, ref.Name)
			}
			if got, want := shardNames, []string{"-"}; !reflect.DeepEqual(got, want) {
				return fmt.Errorf("got Partitions[%v].ShardReferences = %v; want %v", i, got, want)
			}
		}
		return nil
	})
}
