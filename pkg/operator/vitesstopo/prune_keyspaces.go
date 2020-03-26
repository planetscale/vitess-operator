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

type PruneKeyspacesParams struct {
	// EventObj holds the object type that the recorder will use when writing events.
	EventObj   runtime.Object
	TopoServer *topo.Server
	Recorder   record.EventRecorder
	// Keyspaces is a current list of KeySpaceTemplates from the cluster spec.
	Keyspaces []planetscalev2.VitessKeyspaceTemplate
	// OrphanedKeyspaces is a list of unwanted keyspaces that could not be turned down.
	OrphanedKeyspaces map[string]*planetscalev2.OrphanStatus
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
func KeyspacesToPrune(keyspaceNames []string, desiredKeyspaces sets.String, orphanedKeyspaces map[string]*planetscalev2.OrphanStatus) []string {
	var candidates []string

	for _, name := range keyspaceNames {
		if !desiredKeyspaces.Has(name) && orphanedKeyspaces[name] == nil {
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

	for _, name := range keyspaceNames {
		// The keyspace exists in topo, but not in the VT spec.
		// It's also not being kept around by a blocked turn-down.
		// We use the Vitess wrangler (multi-step command executor) to recursively delete the keyspace.
		// This is equivalent to `vtctl DeleteKeyspace -recursive`.
		wr := wrangler.New(logutil.NewConsoleLogger(), ts, nil)

		// topo.NoNode is the error type returned if we can't find the keyspace when deleting. This ensures that this operation is idempotent.
		if err := wr.DeleteKeyspace(ctx, name, true); err != nil && !topo.IsErrType(err, topo.NoNode) {
			recorder.Eventf(eventObj, corev1.EventTypeWarning, "TopoCleanupFailed", "unable to remove keyspace %s from topology: %v", name, err)
			resultBuilder.RequeueAfter(topoRequeueDelay)
		} else {
			recorder.Eventf(eventObj, corev1.EventTypeNormal, "TopoCleanup", "removed unwanted keyspace %s from topology", name)
		}
	}

	return resultBuilder.Result()
}
