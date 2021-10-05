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
	"fmt"
	"time"

	"vitess.io/vitess/go/mysql"
	topodatapb "vitess.io/vitess/go/vt/proto/topodata"
	"vitess.io/vitess/go/vt/topo"
	"vitess.io/vitess/go/vt/topo/topoproto"
	"vitess.io/vitess/go/vt/wrangler"

	corev1 "k8s.io/api/core/v1"
	apilabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/kubectl/pkg/util/podutils"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/drain"
	"planetscale.dev/vitess-operator/pkg/operator/results"
	"planetscale.dev/vitess-operator/pkg/operator/vttablet"
)

const (
	// reconcileDrainTimeout is the overall timeout for a single drain pass.
	// It should be large enough to include other sub-timeouts below.
	reconcileDrainTimeout = 60 * time.Second
	// reconcileDrainReadTimeout is the timeout for reading state before we
	// decide to do anything. These reads should be fast, so we keep this low to
	// fail fast if topo is down rather than wait until the overall timeout.
	reconcileDrainReadTimeout = 10 * time.Second
	// plannedReparentTimeout is the timeout for executing PlannedReparentShard.
	plannedReparentTimeout = 30 * time.Second
	// candidatePrimaryTimeout is the timeout for contacting candidate primarys to decide which one to choose.
	candidatePrimaryTimeout = 2 * time.Second
)

/*
reconcileDrain prepares tablet Pods to be deleted, in response to drain requests
specified as annotations on the Pods. See the "drain" package for details on how
to initiate drains.

This operates in four phases:

1. Check shard health.  Do not take any action if shard is unhealthy.
2. Load current drain state.  Clear annotations from aborted drains.
3. Handle updating annotations.  Do not mark current primary as finished.
4. Reparent draining primarys only if marked/will be marked as "Finished".

## CAVEATS AND EDGE CASES ##

We guarantee this invariant:

- Only one tablet is marked as finished, and once it is, no other tablet will be
  marked as finished until this tablet is deleted or the drain is aborted
  (aborting the drain is considered an emergency situation and our invariant
  could break here).

This has implications to these situations:

- If the shard becomes unhealthy, anything marked as "finished" will stay
  "finished".
- If the primary is reparented to a "finished" tablet, that tablet will stay
  "finished".

These are necessary because if we ever remove the "finished" annotation we could
then later mark something else as "finished".

If that happened a drainer might see "finished" on two different tablets and
accidentally delete more tablets than can safely be deleted.  This is even worse
than our original situation.

This essentially means that we cannot guarantee that during our planned
decommissioning we won't be racing with an unplanned incident and have the
drainer delete something at a bad time.  However, by deleting only one tablet at
a time we still ensure that for shards with three or more tablets we still have
redundancy during the decommissioning.  Maybe later we can do better.
*/
func (r *ReconcileVitessShard) reconcileDrain(ctx context.Context, vts *planetscalev2.VitessShard, wr *wrangler.Wrangler) (reconcile.Result, error) {
	clusterName := vts.Labels[planetscalev2.ClusterLabel]
	keyspaceName := vts.Labels[planetscalev2.KeyspaceLabel]
	resultBuilder := &results.Builder{}

	// Don't hold our slot in the reconcile work queue for too long.
	ctx, cancel := context.WithTimeout(ctx, reconcileDrainTimeout)
	defer cancel()

	// Put a tighter limit on the initial read phase so we fail fast.
	readCtx, readCancel := context.WithTimeout(ctx, reconcileDrainReadTimeout)
	defer readCancel()

	// Get a list of all our tablet Pods from the cache.
	labels := map[string]string{
		planetscalev2.ComponentLabel: planetscalev2.VttabletComponentName,
		planetscalev2.ClusterLabel:   clusterName,
		planetscalev2.KeyspaceLabel:  keyspaceName,
		planetscalev2.ShardLabel:     vts.Spec.KeyRange.SafeName(),
	}

	podList := &corev1.PodList{}
	listOpts := &client.ListOptions{
		Namespace:     vts.Namespace,
		LabelSelector: apilabels.SelectorFromSet(apilabels.Set(labels)),
	}
	if err := r.client.List(readCtx, podList, listOpts); err != nil {
		r.recorder.Eventf(vts, corev1.EventTypeWarning, "ListFailed", "failed to list Pods: %v", err)
		return resultBuilder.Error(err)
	}

	// Get the shard record to check who the primary is.
	shard, err := wr.TopoServer().GetShard(readCtx, keyspaceName, vts.Spec.Name)
	if err != nil {
		r.recorder.Eventf(vts, corev1.EventTypeWarning, "TopoGetFailed", "failed to get shard record: %v", err)
		return resultBuilder.RequeueAfter(replicationRequeueDelay)
	}

	// Get all the tablet records for the shard, in cells to which we deploy.
	// We ignore tablets in cells we don't deploy, since we assume there's
	// a separate operator instance handling drains on those tablets.
	tablets, err := wr.TopoServer().GetTabletMapForShardByCell(readCtx, keyspaceName, vts.Spec.Name, vts.Spec.GetCells().UnsortedList())
	if err != nil {
		r.recorder.Eventf(vts, corev1.EventTypeWarning, "TopoGetFailed", "failed to get tablet records: %v", err)
		return resultBuilder.RequeueAfter(replicationRequeueDelay)
	}

	// Create a tablet alias to pod map
	pods := make(map[string]*corev1.Pod, len(podList.Items))
	for i := range podList.Items {
		pod := &podList.Items[i]
		tabletAlias := vttablet.AliasFromPod(pod)
		tabletAliasStr := topoproto.TabletAliasString(&tabletAlias)
		pods[tabletAliasStr] = pod
	}

	//
	// 1. Check shard health.  Do not take any action if shard is unhealthy.
	//

	// If the shard is in any way unhealthy, bail out now and do nothing.
	if err := isShardHealthy(vts); err != nil {
		r.recorder.Eventf(vts, corev1.EventTypeWarning,
			"NotReconcilingDrain", "Shard is in an unhealthy state: %v", err)
		return resultBuilder.Result()
	}

	// If this shard does not have a primary, bail out and do nothing.
	if !shard.HasPrimary() {
		r.recorder.Eventf(vts, corev1.EventTypeWarning,
			"NotReconcilingDrain", "Shard does not have a primary")
		return resultBuilder.Result()
	}

	//
	// 2. Get current drain state
	//

	// Keep track of whether drainer is aborting a partially completed drain.
	abortingDrain := false

	// Keep track of whether we've acknowledged any drains this pass.
	acknowledgedDrain := false

	// Get drain state, and clear all annotations from aborted drains.
	drains := map[string]drain.State{}
	for tabletAliasStr, pod := range pods {
		if drain.Started(pod) {
			// If this pod has started the draining process, get its current
			// state from the perspective of our state machine.
			drains[tabletAliasStr], err = drain.GetState(pod)
			if err != nil {
				r.recorder.Eventf(vts, corev1.EventTypeWarning,
					"InvalidDrainState",
					"Found a pod in an invalid drain state: %v, %v", pod.Name, err)
			}
		} else {
			// If we had previously acknowledged or finished drain of this pod,
			// that means we are aborting a drain and should be extra careful to
			// not touch anything.
			if drain.Acknowledged(pod) || drain.Finished(pod) {
				abortingDrain = true
				r.recorder.Eventf(vts, corev1.EventTypeWarning,
					"AbortingDrain",
					"found a partially drained Pod that does not have a drain request: %v", pod.Name)
			}

			// For any Pod that *doesn't* have a drain request, clear out any
			// previous "finished" or "draining-acknowledged"
			// annotations if necessary.
			if err := r.updateDrainStatus(ctx, pod, drain.NotDrainingState); err != nil {
				r.recorder.Eventf(vts, corev1.EventTypeWarning, "UpdateFailed", "failed to update drain annotation on Pod %v: %v", pod.Name, err)
				resultBuilder.Error(err)
			}
		}
	}
	if len(drains) == 0 {
		// Nothing to do.
		return resultBuilder.Result()
	}
	if abortingDrain {
		// If we are aborting the drain, don't bother with state transitions.
		// We are treating this like an emergency situation and it might break
		// our invariants.
		//
		// We expect the drainer to clear all the necessary draining annotations
		// and wait long enough to ensure things have stabilized before trying
		// again.
		r.recorder.Eventf(vts, corev1.EventTypeWarning,
			"AbortingDrain", "detected that we are aborting drain")
		return resultBuilder.Result()
	}

	//
	// 3. Handle updating annotations.  Do not mark current primary as finished.
	//

	// Find our primary so we don't accidentally mark the primary as finished.
	primaryAliasStr := topoproto.TabletAliasString(shard.PrimaryAlias)

	// Update all the new tablet states based on the state machine output.
	transitions := drain.StateTransitions(drains)
	for tabletAliasStr, state := range transitions {
		// Do not mark the primary as finished.
		if state == drain.FinishedState && tabletAliasStr == primaryAliasStr {
			continue
		}

		// If we are acknowledging a drain, do not do any reparents.
		if state == drain.AcknowledgedState {
			acknowledgedDrain = true
		}

		pod := pods[tabletAliasStr]
		if err := r.updateDrainStatus(ctx, pod, state); err != nil {
			r.recorder.Eventf(vts, corev1.EventTypeWarning,
				"UpdateFailed", "failed to update drain annotation on Pod %v: %v", pod.Name, err)
			resultBuilder.Error(err)
		}
	}

	//
	// 4. Reparent draining primarys only if marked/will be marked as "Finished".
	//

	// If we have acknowledged a drain and haven't already marked the primary as
	// finished, don't do a reparent.
	//
	// This is because even if we are trying to mark the primary as "Finished" on
	// this loop, that may change in the next loop because of the tablet we have
	// just marked as acknowledged.  Wait for things to settle before
	// continuing.
	//
	// However, if the primary is already marked as finished, we either messed up
	// or it was reparented by something else, so we should actually do a
	// reparent away from it if we can.
	if acknowledgedDrain && drains[primaryAliasStr] != drain.FinishedState {
		r.recorder.Eventf(vts, corev1.EventTypeNormal,
			"NotReparentingPrimary", "We have acknowledged a drain this loop")
		return resultBuilder.Result()
	}

	// If we haven't already marked the primary as finished and aren't trying to,
	// there is no need to do a reparent.
	if drains[primaryAliasStr] != drain.FinishedState && transitions[primaryAliasStr] != drain.FinishedState {
		r.recorder.Eventf(vts, corev1.EventTypeNormal,
			"NotReparentingPrimary", "We are not marking primary as finished")
		return resultBuilder.Result()
	}

	// See if there's a candidate primary for a planned reparent.
	newPrimary := candidatePrimary(ctx, wr, shard, tablets, pods, vts.Spec.UsingExternalDatastore())
	if newPrimary == nil {
		r.recorder.Eventf(vts, corev1.EventTypeWarning, "DrainBlocked", "unable to drain primary tablet %v: no other tablet is a suitable primary candidate", primaryAliasStr)
		return resultBuilder.RequeueAfter(replicationRequeueDelay)
	}

	// Perform a planned reparent.
	reparentCtx, reparentCancel := context.WithTimeout(ctx, plannedReparentTimeout)
	defer reparentCancel()

	var reparentErr error
	if vts.Spec.UsingExternalDatastore() {
		reparentErr = r.handleExternalReparent(ctx, vts, wr, newPrimary.Alias, shard.PrimaryAlias)
	} else {
		reparentErr = wr.PlannedReparentShard(reparentCtx, keyspaceName, vts.Spec.Name, newPrimary.Alias, nil, plannedReparentTimeout)
	}

	if reparentErr != nil {
		r.recorder.Eventf(vts, corev1.EventTypeWarning, "PlannedReparentFailed", "planned reparent from current primary %v to candidate primary %v failed: %v", primaryAliasStr, newPrimary.AliasString(), reparentErr)
	} else {
		r.recorder.Eventf(vts, corev1.EventTypeNormal, "PlannedReparent", "planned reparent from old primary %v to new primary %v succeeded", primaryAliasStr, newPrimary.AliasString())
	}

	plannedReparentCount.WithLabelValues(metricLabels(vts, reparentErr)...).Inc()

	return resultBuilder.Result()
}

func (r *ReconcileVitessShard) handleExternalReparent(ctx context.Context, vts *planetscalev2.VitessShard, wr *wrangler.Wrangler, newPrimaryAlias, oldPrimaryAlias *topodatapb.TabletAlias) error {
	err := wr.TabletExternallyReparented(ctx, newPrimaryAlias)

	if err == nil {
		// TODO: Remove this after all externalprimary tablets have been updated
		// to set the -demote_primary_type=SPARE flag.
		err = wr.ChangeTabletType(ctx, oldPrimaryAlias, topodatapb.TabletType_SPARE)
	}

	return err
}

func (r *ReconcileVitessShard) updateDrainStatus(ctx context.Context, pod *corev1.Pod, drainStatus drain.State) error {
	hasUpdated := false

	switch drainStatus {
	case drain.FinishedState:
		if !drain.Finished(pod) {
			drain.Finish(pod)
			hasUpdated = true
		}
	case drain.AcknowledgedState:
		if !drain.Acknowledged(pod) {
			drain.Acknowledge(pod)
			hasUpdated = true
		}
	case drain.NotDrainingState:
		if drain.Finished(pod) {
			drain.Unfinish(pod)
			hasUpdated = true
		}
		if drain.Acknowledged(pod) {
			drain.Unacknowledge(pod)
			hasUpdated = true
		}
	case drain.DrainingState:
		// This is set by the drainer
		panic("Programming error, the controller should never set a pod as Draining")
	}

	if !hasUpdated {
		return nil
	}
	return r.client.Update(ctx, pod)
}

func isShardHealthy(vts *planetscalev2.VitessShard) error {
	for name, tablet := range vts.Status.Tablets {
		if tablet.Available != corev1.ConditionTrue {
			return fmt.Errorf("tablet %v is not Available", name)
		}
	}
	return nil
}

// candidatePrimary chooses a candidate tablet to be the new primary in a planned
// reparent (when the current primary is still healthy).
func candidatePrimary(ctx context.Context, wr *wrangler.Wrangler, shard *topo.ShardInfo, tablets map[string]*topo.TabletInfo, pods map[string]*corev1.Pod, usingExternal bool) *topo.TabletInfo {
	candidates := []*topo.TabletInfo{}
	for tabletAliasStr, tablet := range tablets {
		// It must not be the current primary.
		if topoproto.TabletAliasEqual(tablet.Alias, shard.PrimaryAlias) {
			continue
		}

		// The Pod must be Ready.
		pod := pods[tabletAliasStr]
		if pod == nil {
			continue
		}

		// It must be a "replica" type for local MySQL, or any type for external primary pools.
		if usingExternal {
			if pod.Labels[planetscalev2.TabletTypeLabel] != planetscalev2.ExternalMasterTabletPoolName {
				continue
			}
			// Because we aren't handling MySQL replication, if a tablet thinks it's primary then it should be safe.
			if tablet.Type != topodatapb.TabletType_SPARE && tablet.Type != topodatapb.TabletType_PRIMARY {
				continue
			}
		} else {
			if tablet.Type != topodatapb.TabletType_REPLICA {
				continue
			}
		}

		if !podutils.IsPodReady(pod) {
			continue
		}
		// The Pod must not have a drain request, or have already entered the
		// drain state machine.
		if drain.Started(pod) || drain.Acknowledged(pod) || drain.Finished(pod) {
			continue
		}
		// TODO(enisoc): Add other criteria, such as perferred primary cells.
		// For now, this is good enough to be a candidate.
		candidates = append(candidates, tablet)
	}
	if len(candidates) == 0 {
		return nil
	}

	// The last check we do is to look for the candidate whose replication
	// position is farthest ahead, to minimize the time to catch up. We do this
	// on a best-effort basis with a short timeout. Any candidate that doesn't
	// respond in time is disqualified, unless no one responds in time.
	ctx, cancel := context.WithTimeout(ctx, candidatePrimaryTimeout)
	defer cancel()

	// Send results to results channel.
	results := make(chan candidateInfo, len(candidates))
	for _, tablet := range candidates {
		go func(tablet *topo.TabletInfo) {
			status, err := wr.TabletManagerClient().ReplicationStatus(ctx, tablet.Tablet)
			result := candidateInfo{tablet: tablet, err: err}
			if err == nil {
				result.position, result.err = mysql.DecodePosition(status.Position)
			}
			results <- result
		}(tablet)
	}

	// Read results channel and remember the high point so far.
	// No one ever closes the results chan, but we know how many to expect.
	var bestCandidate *topo.TabletInfo
	var highestPosition mysql.Position
	for range candidates {
		result := <-results
		if result.err != nil {
			continue
		}
		if highestPosition.IsZero() || !highestPosition.AtLeast(result.position) {
			bestCandidate = result.tablet
			highestPosition = result.position
		}
	}

	if bestCandidate == nil {
		// None of the candidates responded in time. Just try the first one.
		bestCandidate = candidates[0]
	}

	return bestCandidate
}

type candidateInfo struct {
	tablet   *topo.TabletInfo
	position mysql.Position
	err      error
}
