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
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/results"
	"planetscale.dev/vitess-operator/pkg/operator/toposerver"
	"planetscale.dev/vitess-operator/pkg/operator/vitesstopo"
)

const (
	topoReconcileTimeout = 10 * time.Second
	// topoRequeueDelay is how long to wait before retrying when a topology
	// server call failed. We typically return success with a requeue delay
	// instead of returning an error, because it's unlikely that retrying
	// immediately will be worthwhile.
	topoRequeueDelay = 5 * time.Second
)

func (r *ReconcileVitessKeyspace) reconcileTopology(ctx context.Context, vtk *planetscalev2.VitessKeyspace) (reconcile.Result, error) {
	resultBuilder := &results.Builder{}

	ts, err := toposerver.Open(ctx, vtk.Spec.GlobalLockserver)
	if err != nil {
		r.recorder.Eventf(vtk, corev1.EventTypeWarning, "TopoConnectFailed", "failed to connect to global lockserver: %v", err)
		// Give the lockserver some time to come up.
		return resultBuilder.RequeueAfter(topoRequeueDelay)
	}
	defer ts.Close()

	if *vtk.Spec.TopologyReconciliation.PruneShards {
		// Don't hold our slot in the reconcile work queue for too long.
		ctx, cancel := context.WithTimeout(ctx, topoReconcileTimeout)
		defer cancel()

		desiredShards := make(sets.String, len(vtk.Status.Shards))
		for k := range vtk.Status.Shards {
			desiredShards.Insert(k)
		}

		result, err := vitesstopo.PruneShards(ctx, vitesstopo.PruneShardsParams{
			EventObj:       vtk,
			TopoServer:     ts.Server,
			Recorder:       r.recorder,
			KeyspaceName:   vtk.Spec.Name,
			DesiredShards:  desiredShards,
			OrphanedShards: vtk.Status.OrphanedShards,
		})
		resultBuilder.Merge(result, err)
	}

	return resultBuilder.Result()
}
