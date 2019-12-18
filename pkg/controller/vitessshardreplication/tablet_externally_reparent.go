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

package vitessshardreplication

import (
	"context"
	"time"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/results"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	topodatapb "vitess.io/vitess/go/vt/proto/topodata"
	"vitess.io/vitess/go/vt/topo/topoproto"
	"vitess.io/vitess/go/vt/wrangler"
)

const (
	externallyReparentTimeout = 30 * time.Second
)

func (r *ReconcileVitessShard) tabletExternallyReparent(ctx context.Context, vts *planetscalev2.VitessShard, wr *wrangler.Wrangler) (reconcile.Result, error) {
	resultBuilder := &results.Builder{}

	// If we're using local MySQL then we should not call externallyReparent,
	// but should instead try to use initShardMaster.
	if !vts.UsingExternalDatastore() {
		return resultBuilder.Result()
	}

	// If we already have a master we can bail early.
	if vts.Status.HasMaster == corev1.ConditionTrue {
		return resultBuilder.Result()
	}

	// Everything we can check through k8s and topology looks good to proceed.
	// Now we start talking to the actual vttablet processes.
	// Don't hold our slot in the reconcile work queue for too long.
	ctx, cancel := context.WithTimeout(ctx, externallyReparentTimeout)
	defer cancel()

	// Check actual shard record in case we are out of sync
	// and bail if shard record says we have a master already.
	keyspaceName := vts.Labels[planetscalev2.KeyspaceLabel]
	shard, err := wr.TopoServer().GetShard(ctx, keyspaceName, vts.Name)
	if err == nil && shard.HasMaster() {
		return resultBuilder.Result()
	}

	// Find the first external master that's running, if any.
	var masterCandidateAlias *topodatapb.TabletAlias
	for name, tablet := range vts.Status.Tablets {
		// If tablet is not of external pool type AND running, we can move on to the next tablet.
		if !(tablet.IsExternalMaster() && tablet.IsRunning()) {
			continue
		}

		tabletAlias, err := topoproto.ParseTabletAlias(name)
		if err != nil {
			r.recorder.Eventf(vts, corev1.EventTypeWarning, "InternalError", "can't parse tablet alias %q: %v", name, err)
			// Return success since there's no point retrying.
			return resultBuilder.Result()
		}

		masterCandidateAlias = tabletAlias
		break
	}

	// We found an external master tablet that's running. It might be ready to be marked as master,
	// but it also might not be yet. For now, we don't bother to check anything else because we believe
	// it's always safe to attempt TER. If it fails, we will try again later.
	if masterCandidateAlias == nil {
		// We didn't find any tablets in the external master pool that are eligible for external reparent.
		// Return success because there's no point retrying this until tablet status changes.
		r.recorder.Eventf(vts, corev1.EventTypeWarning, "ExternalMasterShardBlocked", "can't externally reparent shard: no master-eligible tablets (pool type 'externalmaster') deployed")
		return resultBuilder.Result()
	}

	// Get the tablet record from topology.
	ti, err := wr.TopoServer().GetTablet(ctx, masterCandidateAlias)
	if err != nil {
		r.recorder.Eventf(vts, corev1.EventTypeWarning, "ExternalMasterShardBlocked", "can't externally reparent shard: %v", err)
		return resultBuilder.RequeueAfter(replicationRequeueDelay)
	}

	masterCandidate := ti.Tablet

	// All checks passed. Do TabletExternallyReparented.
	if err := wr.TabletManagerClient().TabletExternallyReparented(ctx, masterCandidate, ""); err != nil {
		r.recorder.Eventf(vts, corev1.EventTypeWarning, "TabletExternallyReparentedFailed", "failed to externally reparent shard: %v", err)
		return resultBuilder.RequeueAfter(replicationRequeueDelay)
	}

	r.recorder.Eventf(vts, corev1.EventTypeNormal, "TabletExternallyReparented", "Externally reparented tablet %v", topoproto.TabletAliasString(masterCandidate.Alias))
	return resultBuilder.Result()
}
