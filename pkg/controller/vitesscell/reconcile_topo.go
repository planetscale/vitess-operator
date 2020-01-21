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

package vitesscell

import (
	"context"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"time"

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

func (r *ReconcileVitessCell) reconcileTopology(ctx context.Context, vtc *planetscalev2.VitessCell, ts *toposerver.Conn, keyspaces []*planetscalev2.VitessKeyspace) (reconcile.Result, error) {
	resultBuilder := &results.Builder{}

	// Don't hold our slot in the reconcile work queue for too long.
	ctx, cancel := context.WithTimeout(ctx, topoReconcileTimeout)
	defer cancel()

	if *vtc.Spec.TopologyReconciliation.PruneSrvKeyspaces {
		result, err := r.pruneSrvKeyspaces(ctx, vtc, keyspaces, ts)
		resultBuilder.Merge(result, err)
	}

	return resultBuilder.Result()
}

func (r *ReconcileVitessCell) pruneSrvKeyspaces(ctx context.Context, vtc *planetscalev2.VitessCell, keyspaces []*planetscalev2.VitessKeyspace, ts *toposerver.Conn) (reconcile.Result, error) {
	resultBuilder := &results.Builder{}

	// Get the list of keyspaces deployed (served) in this cell.
	srvKeyspaceNames, err := ts.GetSrvKeyspaceNames(ctx, vtc.Spec.Name)
	if err != nil {
		r.recorder.Eventf(vtc, corev1.EventTypeWarning, "TopoListFailed", "failed to list keyspaces in cell-local lockserver: %v", err)
		return resultBuilder.RequeueAfter(topoRequeueDelay)
	}

	wanted := make(map[string]bool, len(keyspaces))
	for _, vtk := range keyspaces {
		wanted[vtk.Spec.Name] = true
	}
	for _, srvKeyspaceName := range srvKeyspaceNames {
		if wanted[srvKeyspaceName] {
			continue
		}

		// It's not wanted. Try to delete it.
		if err := ts.DeleteSrvKeyspace(ctx, vtc.Spec.Name, srvKeyspaceName); err != nil {
			r.recorder.Eventf(vtc, corev1.EventTypeWarning, "TopoCleanupBlocked", "unable to remove keyspace %s from cell-local topology: %v", srvKeyspaceName, err)
			resultBuilder.RequeueAfter(topoRequeueDelay)
		} else {
			r.recorder.Eventf(vtc, corev1.EventTypeNormal, "TopoCleanup", "removed unwanted keyspace %s from cell-local topology", srvKeyspaceName)
		}
	}

	return resultBuilder.Result()
}
