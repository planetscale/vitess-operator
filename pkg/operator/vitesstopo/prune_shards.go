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

package vitesstopo

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"vitess.io/vitess/go/vt/logutil"
	"vitess.io/vitess/go/vt/topo"
	"vitess.io/vitess/go/vt/wrangler"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/results"
)

type PruneShardsParams struct {
	// EventObj holds the object type that the recorder will use when writing events.
	EventObj     runtime.Object
	TopoServer   *topo.Server
	Recorder     record.EventRecorder
	KeyspaceName string
	// DesiredShards is a set of currently desired shard names, usually pulled from the keyspace spec.
	DesiredShards sets.String
	// OrphanedShards is a list of unwanted shards that could not be turned down.
	OrphanedShards map[string]*planetscalev2.OrphanStatus
}

// PruneShards will prune shards that exist but shouldn't anymore.
func PruneShards(ctx context.Context, p PruneShardsParams) (reconcile.Result, error) {
	resultBuilder := &results.Builder{}

	// Get list of shards in topo.
	shardNames, err := p.TopoServer.GetShardNames(ctx, p.KeyspaceName)
	if err != nil {
		p.Recorder.Eventf(p.EventObj, corev1.EventTypeWarning, "TopoListFailed", "failed to list shards in topology: %v", err)
		return resultBuilder.RequeueAfter(topoRequeueDelay)
	}

	candidates := ShardsToPrune(shardNames, p.DesiredShards, p.OrphanedShards)

	result, err := DeleteShards(ctx, p.TopoServer, p.Recorder, p.EventObj, p.KeyspaceName, candidates)
	resultBuilder.Merge(result, err)

	return resultBuilder.Result()
}

// ShardsToPrune returns a list of shard candidates for pruning, based on a provided list of shards to consider.
func ShardsToPrune(currentShards []string, desiredShards sets.String, orphanedShards map[string]*planetscalev2.OrphanStatus) []string {
	var candidates []string

	for _, name := range currentShards {
		if !desiredShards.Has(name) && orphanedShards[name] == nil {
			// The shard exists in topo, but not in the VitessKeyspace spec.
			// It's also not being kept around by a blocked turn-down.
			candidates = append(candidates, name)
		}
	}

	return candidates
}

// DeleteShards takes in a list of shard names and deletes their records from topology.
func DeleteShards(ctx context.Context, ts *topo.Server, recorder record.EventRecorder, eventObj runtime.Object, keyspaceName string, shardNames []string) (reconcile.Result, error) {
	resultBuilder := &results.Builder{}

	for _, name := range shardNames {
		// We use the Vitess wrangler (multi-step command executor) to recursively delete the shard.
		// This is equivalent to `vtctl DeleteShard -recursive`.
		wr := wrangler.New(logutil.NewConsoleLogger(), ts, nil)

		// topo.NoNode is the error type returned if we can't find the shard when deleting. This ensures that this operation is idempotent.
		if err := wr.DeleteShard(ctx, keyspaceName, name, true, false); err != nil && !topo.IsErrType(err, topo.NoNode) {
			recorder.Eventf(eventObj, corev1.EventTypeWarning, "TopoCleanupFailed", "unable to remove shard %s from topology: %v", name, err)
			resultBuilder.RequeueAfter(topoRequeueDelay)
		} else {
			recorder.Eventf(eventObj, corev1.EventTypeNormal, "TopoCleanup", "removed unwanted shard %s from topology", name)
		}
	}

	return resultBuilder.Result()
}
