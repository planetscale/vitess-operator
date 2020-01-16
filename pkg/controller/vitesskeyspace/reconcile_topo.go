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

package vitesskeyspace

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"vitess.io/vitess/go/vt/logutil"
	"vitess.io/vitess/go/vt/wrangler"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/results"
	"planetscale.dev/vitess-operator/pkg/operator/toposerver"
)

const (
	topoReconcileTimeout = 5 * time.Second
	// topoRequeueDelay is how long to wait before retrying when a topology
	// server call failed. We typically return success with a requeue delay
	// instead of returning an error, because it's unlikely that retrying
	// immediately will be worthwhile.
	topoRequeueDelay = 5 * time.Second
)

func (r *ReconcileVitessKeyspace) reconcileTopology(ctx context.Context, vtk *planetscalev2.VitessKeyspace) (reconcile.Result, error) {
	resultBuilder := &results.Builder{}

	// Don't hold our slot in the reconcile work queue for too long.
	ctx, cancel := context.WithTimeout(ctx, topoReconcileTimeout)
	defer cancel()

	ts, err := toposerver.Open(ctx, vtk.Spec.GlobalLockserver)
	if err != nil {
		r.recorder.Eventf(vtk, corev1.EventTypeWarning, "TopoConnectFailed", "failed to connect to global lockserver: %v", err)
		// Give the lockserver some time to come up.
		return resultBuilder.RequeueAfter(topoRequeueDelay)
	}
	defer ts.Close()

	if *vtk.Spec.TopologyReconciliation.PruneShards {
		result, err := r.pruneShards(ctx, vtk, ts)
		resultBuilder.Merge(result, err)
	}

	return resultBuilder.Result()
}

func (r *ReconcileVitessKeyspace) pruneShards(ctx context.Context, vtk *planetscalev2.VitessKeyspace, ts *toposerver.Conn) (reconcile.Result, error) {
	resultBuilder := &results.Builder{}

	// Get list of keyspaces in topo.
	shardNames, err := ts.GetShardNames(ctx, vtk.Spec.Name)
	if err != nil {
		r.recorder.Eventf(vtk, corev1.EventTypeWarning, "TopoListFailed", "failed to list shards in topology: %v", err)
		return resultBuilder.RequeueAfter(topoRequeueDelay)
	}

	// Clean up shards that exist but shouldn't.
	for _, name := range shardNames {
		if vtk.Status.Shards[name] == nil && vtk.Status.OrphanedShards[name] == nil {
			// The shard exists in topo, but not in the VitessKeyspace spec.
			// It's also not being kept around by a blocked turn-down.
			// We use the Vitess wrangler (multi-step command executor) to recursively delete the shard.
			// This is equivalent to `vtctl DeleteShard -recursive`.
			wr := wrangler.New(logutil.NewConsoleLogger(), ts.Server, nil)
			if err := wr.DeleteShard(ctx, vtk.Spec.Name, name, true /*recursive*/, false /* evenIfServing */); err != nil {
				r.recorder.Eventf(vtk, corev1.EventTypeWarning, "TopoCleanupFailed", "unable to remove shard %s from topology: %v", name, err)
				resultBuilder.RequeueAfter(topoRequeueDelay)
			} else {
				r.recorder.Eventf(vtk, corev1.EventTypeNormal, "TopoCleanup", "removed unwanted shard %s from topology", name)
			}
		}
	}
	return resultBuilder.Result()
}
