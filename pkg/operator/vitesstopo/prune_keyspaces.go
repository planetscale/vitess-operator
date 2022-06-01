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

	vtctldatapb "vitess.io/vitess/go/vt/proto/vtctldata"
)

type PruneKeyspacesParams struct {
	// EventObj holds the object type that the recorder will use when writing events.
	EventObj   runtime.Object
	TopoServer *topo.Server
	Recorder   record.EventRecorder
	// Keyspaces is a current list of KeySpaceTemplates from the cluster spec.
	Keyspaces []planetscalev2.VitessKeyspaceTemplate
	// OrphanedKeyspaces is a list of unwanted keyspaces that could not be turned down.
	OrphanedKeyspaces map[string]planetscalev2.OrphanStatus
}

// PruneKeyspaces will prune keyspaces that exist but shouldn't anymore.
func PruneKeyspaces(ctx context.Context, p PruneKeyspacesParams) (reconcile.Result, error) {
	resultBuilder := &results.Builder{}

	// Make a map from keyspace name (as Vitess calls them) back to the keyspace spec.
	desiredKeyspaces := make(sets.String, len(p.Keyspaces))
	for i := range p.Keyspaces {
		desiredKeyspaces.Insert(p.Keyspaces[i].Name)
	}

	// Get list of keyspaces in topo.
	keyspaceNames, err := p.TopoServer.GetKeyspaces(ctx)
	if err != nil {
		p.Recorder.Eventf(p.EventObj, corev1.EventTypeWarning, "TopoListFailed", "failed to list keyspaces in topology: %v", err)
		return resultBuilder.RequeueAfter(topoRequeueDelay)
	}

	candidates := KeyspacesToPrune(keyspaceNames, desiredKeyspaces, p.OrphanedKeyspaces)

	result, err := DeleteKeyspaces(ctx, p.TopoServer, p.Recorder, p.EventObj, candidates)
	resultBuilder.Merge(result, err)

	return resultBuilder.Result()
}

// KeyspacesToPrune returns a list of keyspace candidates for pruning, based on a provided list of keyspaces to consider.
func KeyspacesToPrune(keyspaceNames []string, desiredKeyspaces sets.String, orphanedKeyspaces map[string]planetscalev2.OrphanStatus) []string {
	var candidates []string

	for _, name := range keyspaceNames {
		_, orphaned := orphanedKeyspaces[name]
		if !desiredKeyspaces.Has(name) && !orphaned {
			// The keyspace exists in topo, but not in the VT spec.
			// It's also not being kept around by a blocked turn-down.
			// We should add it to the list of candidates to prune.
			candidates = append(candidates, name)
		}
	}

	return candidates
}

// DeleteKeyspaces takes in a list of keyspace names and deletes their records from topology.
func DeleteKeyspaces(ctx context.Context, ts *topo.Server, recorder record.EventRecorder, eventObj runtime.Object, keyspaceNames []string) (reconcile.Result, error) {
	resultBuilder := &results.Builder{}

	// We use the Vitess wrangler (multi-step command executor) to recursively delete the keyspace.
	// This is equivalent to `vtctl DeleteKeyspace -recursive`.
	wr := wrangler.New(logutil.NewConsoleLogger(), ts, nil)

	for _, name := range keyspaceNames {
		// Before we delete a keyspace, we must delete vschema for this operation to be idempotent.
		if err := ts.DeleteVSchema(ctx, name); err != nil && !topo.IsErrType(err, topo.NoNode) {
			recorder.Eventf(eventObj, corev1.EventTypeWarning, "TopoCleanupFailed", "unable to remove keyspace %s vschema from topology: %v", name, err)
			resultBuilder.RequeueAfter(topoRequeueDelay)
			// If we can't delete the vschema for this keyspace, then we shouldn't try to delete the keyspace.
			// We'll retry later.
			continue
		}
		recorder.Eventf(eventObj, corev1.EventTypeNormal, "TopoCleanup", "removed unwanted keyspace %s vschema from topology", name)

		// topo.NoNode is the error type returned if we can't find the keyspace when deleting. This ensures that this operation is idempotent.
		if _, err := wr.VtctldServer().DeleteKeyspace(ctx, &vtctldatapb.DeleteKeyspaceRequest{
			Keyspace:  name,
			Recursive: true,
		}); err != nil && !topo.IsErrType(err, topo.NoNode) {
			recorder.Eventf(eventObj, corev1.EventTypeWarning, "TopoCleanupFailed", "unable to remove keyspace %s from topology: %v", name, err)
			resultBuilder.RequeueAfter(topoRequeueDelay)
		} else {
			recorder.Eventf(eventObj, corev1.EventTypeNormal, "TopoCleanup", "removed unwanted keyspace %s from topology", name)
		}
	}

	return resultBuilder.Result()
}
