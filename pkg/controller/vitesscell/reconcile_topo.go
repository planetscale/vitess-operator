/*
Copyright 2019 PlanetScale.

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
	"time"

	corev1 "k8s.io/api/core/v1"

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

func (r *ReconcileVitessCell) reconcileTopology(ctx context.Context, vtc *planetscalev2.VitessCell, keyspaces []*planetscalev2.VitessKeyspace) (topoEntries []string, err error) {
	// Don't hold our slot in the reconcile work queue for too long.
	ctx, cancel := context.WithTimeout(ctx, topoReconcileTimeout)
	defer cancel()

	if vtc.Status.Lockserver.Etcd != nil {
		// We know something about local etcd status.
		// We can use that to avoid trying to connect when we know it won't work.
		if vtc.Status.Lockserver.Etcd.Available != corev1.ConditionTrue {
			r.recorder.Event(vtc, corev1.EventTypeNormal, "TopoWaiting", "waiting for local etcd to become Available")
			// Return success. We don't need to requeue because we'll get queued any time the EtcdCluster status changes.
			return nil, nil
		}
	}

	// We actually know the address of the local lockserver already,
	// but for now we'll follow the same rule as all Vitess components,
	// which is to use the global lockserver to find the local ones.
	ts, err := toposerver.Open(ctx, vtc.Spec.GlobalLockserver)
	if err != nil {
		r.recorder.Eventf(vtc, corev1.EventTypeWarning, "TopoConnectFailed", "failed to connect to global lockserver: %v", err)
		return nil, err
	}
	defer ts.Close()

	// Get the list of keyspaces deployed (served) in this cell.
	srvKeyspaceNames, err := ts.GetSrvKeyspaceNames(ctx, vtc.Spec.Name)
	if err != nil {
		r.recorder.Eventf(vtc, corev1.EventTypeWarning, "TopoListFailed", "failed to list keyspaces in cell-local lockserver: %v", err)
		return nil, err
	}

	// We don't need to create Keyspace entries in topo for keyspaces that should exist.
	// The tablets we deploy take care of that automatically.
	// However, only we know when it's time to remove a keyspace entry from topo,
	// when the keyspace has been undeployed from this cell.
	resultBuilder := &results.Builder{}
	topoEntries = make([]string, 0, len(srvKeyspaceNames))
	wanted := make(map[string]bool, len(keyspaces))
	for _, vtk := range keyspaces {
		wanted[vtk.Spec.Name] = true
	}
	for _, srvKeyspaceName := range srvKeyspaceNames {
		if wanted[srvKeyspaceName] {
			topoEntries = append(topoEntries, srvKeyspaceName)
			continue
		}
		// It's not wanted. Try to delete it.
		if err := ts.DeleteSrvKeyspace(ctx, vtc.Spec.Name, srvKeyspaceName); err != nil {
			// We failed to delete it, so assume the entry still exists.
			topoEntries = append(topoEntries, srvKeyspaceName)

			r.recorder.Eventf(vtc, corev1.EventTypeWarning, "TopoCleanupBlocked", "unable to remove keyspace %s from cell-local topology: %v", srvKeyspaceName, err)
			resultBuilder.Error(err)
		} else {
			r.recorder.Eventf(vtc, corev1.EventTypeNormal, "TopoCleanup", "removed unwanted keyspace %s from cell-local topology", srvKeyspaceName)
		}
	}
	_, err = resultBuilder.Result()
	return topoEntries, err
}
