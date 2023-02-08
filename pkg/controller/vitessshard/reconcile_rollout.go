package vitessshard

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apilabels "k8s.io/apimachinery/pkg/labels"
	"planetscale.dev/vitess-operator/pkg/operator/drain"
	"planetscale.dev/vitess-operator/pkg/operator/toposerver"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"vitess.io/vitess/go/vt/topo/topoproto"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/results"
	"planetscale.dev/vitess-operator/pkg/operator/rollout"
	"planetscale.dev/vitess-operator/pkg/operator/vttablet"
)

func (r *ReconcileVitessShard) reconcileRollout(ctx context.Context, vts *planetscalev2.VitessShard) (reconcile.Result, error) {
	resultBuilder := &results.Builder{}

	if !rollout.Cascading(vts) {
		// If the shard is not scheduled for a cascading update, silently bail out and do nothing.
		return resultBuilder.Result()
	}

	tabletPods, err := r.tabletPodsFromShard(ctx, vts)
	if err != nil {
		return resultBuilder.Error(err)
	}

	tabletKeys := vts.Status.TabletAliases()

	for _, tabletKey := range tabletKeys {
		tablet := vts.Status.Tablets[tabletKey]
		if tablet.Available != corev1.ConditionTrue {
			// If any tablets are unhealthy, we should bail and not perform a rolling restart.
			r.recorder.Eventf(vts, corev1.EventTypeNormal, "RolloutPaused", "Waiting for tablet %v to be Available.", tabletKey)
			return resultBuilder.Result()
		}

		pod, ok := tabletPods[tabletKey]
		if !ok {
			r.recorder.Eventf(vts, corev1.EventTypeNormal, "RolloutPaused", "Waiting for desired tablet %v to be created.", tabletKey)
			return resultBuilder.Result()
		}

		if rollout.Released(pod) {
			// If any tablet has already been released, we should wait until it is finished to release another one.
			r.recorder.Eventf(vts, corev1.EventTypeNormal, "RolloutPaused", "Waiting for tablet %v to finish release.", tabletKey)
			return resultBuilder.Result()
		}
	}

	primaryAlias, err := getPrimaryTabletAlias(ctx, vts)
	if err != nil {
		r.recorder.Eventf(vts, corev1.EventTypeWarning, "RolloutBlocked", "Could not get TabletAlias for the Primary.")
		return resultBuilder.Error(err)
	}

	// Retrieve tablet pod to be released during this reconcile.
	tabletKey, pod := getNextScheduledTablet(tabletKeys, tabletPods, primaryAlias)
	if tabletKey == "" {
		// If we have no more scheduled tablets, uncascade the shard.
		if err := r.uncascadeShard(ctx, vts); err != nil {
			r.recorder.Eventf(vts, corev1.EventTypeWarning, "UncascadeFailed", "Failed to mark cascading shard rollout as complete: %v", err)
			return resultBuilder.Error(err)
		}

		r.recorder.Eventf(vts, corev1.EventTypeNormal, "RollingRestartComplete", "Cascading rollout of tablets is complete.")
		return resultBuilder.Result()
	}

	masterEligibleTablets := vts.Spec.MasterEligibleTabletCount()
	deletePod := false
	tabletType := pod.Labels[planetscalev2.TabletTypeLabel]
	// These two conditions guarantee that the tablet is a lone master.
	if masterEligibleTablets < 2 &&
		(tabletType == string(planetscalev2.ReplicaPoolType) || tabletType == string(planetscalev2.ExternalMasterPoolType)) {
		// If we have a lone master, we must delete it since reparenting is impossible.
		deletePod = true
	}

	if err := r.releaseTabletPod(ctx, pod, deletePod); err != nil {
		r.recorder.Eventf(vts, corev1.EventTypeWarning, "RollingRestartBlocked", "release of Pod %v (tablet %v) failed: %v", pod.Name, tabletKey, err)
		resultBuilder.Error(err)
	}

	return resultBuilder.Result()
}

func (r *ReconcileVitessShard) tabletPodsFromShard(ctx context.Context, vts *planetscalev2.VitessShard) (map[string]*corev1.Pod, error) {
	tabletPods := make(map[string]*corev1.Pod)

	podList := &corev1.PodList{}
	listOpts := &client.ListOptions{
		Namespace: vts.Namespace,
		LabelSelector: apilabels.Set{
			planetscalev2.ComponentLabel: planetscalev2.VttabletComponentName,
			planetscalev2.ClusterLabel:   vts.Labels[planetscalev2.ClusterLabel],
			planetscalev2.KeyspaceLabel:  vts.Labels[planetscalev2.KeyspaceLabel],
			planetscalev2.ShardLabel:     vts.Spec.KeyRange.SafeName(),
		}.AsSelector(),
	}

	if err := r.client.List(ctx, podList, listOpts); err != nil {
		return tabletPods, err
	}

	// Safety checks and rolling updates must be deterministically ordered, so we access and sort pods by tablet alias.
	for i := range podList.Items {
		pod := &podList.Items[i]
		tabletAlias := vttablet.AliasFromPod(pod)
		tabletKey := topoproto.TabletAliasString(&tabletAlias)
		tabletPods[tabletKey] = pod
	}

	return tabletPods, nil
}

func (r *ReconcileVitessShard) releaseTabletPod(ctx context.Context, pod *corev1.Pod, deletePod bool) error {
	if deletePod {
		// TODO: Evict pods instead of deleting them directly, to respect PDBs.
		return r.client.Delete(ctx, pod)
	}

	// Release the pod to be recreated with updates.
	rollout.Release(pod)
	return r.client.Update(ctx, pod)
}

func (r *ReconcileVitessShard) uncascadeShard(ctx context.Context, vts *planetscalev2.VitessShard) error {
	rollout.Uncascade(vts)
	return r.client.Update(ctx, vts)
}

func getNextScheduledTablet(tabletKeys []string, tabletPods map[string]*corev1.Pod, primaryAlias string) (string, *corev1.Pod) {
	scheduledTablets := map[string]bool{}

	for _, tabletKey := range tabletKeys {
		pod := tabletPods[tabletKey]
		if rollout.Scheduled(pod) {
			scheduledTablets[tabletKey] = true

			// If a Pod is scheduled for rollout and it's already drained
			// then it's the next tablet to release since the drain controller
			// will not drain any more tablets in the shard.
			// A tablet may have been drained by something other than a rollout.
			if drain.Finished(pod) {
				return tabletKey, pod
			}
		}
	}

	// Release any scheduled tablet
	for tabletKey := range scheduledTablets {
		if tabletKey != primaryAlias {
			return tabletKey, tabletPods[tabletKey]
		}
	}

	// If there are no remaining scheduled tablets, then release the Primary if its scheduled
	if _, scheduled := scheduledTablets[primaryAlias]; scheduled {
		return primaryAlias, tabletPods[primaryAlias]
	}

	return "", nil
}

func getPrimaryTabletAlias(ctx context.Context, vts *planetscalev2.VitessShard) (string, error) {
	ts, err := toposerver.Open(ctx, vts.Spec.GlobalLockserver)
	if err != nil {
		return "", err
	}
	defer ts.Close()

	keyspaceName := vts.Labels[planetscalev2.KeyspaceLabel]
	shard, err := ts.GetShard(ctx, keyspaceName, vts.Spec.Name)
	if err != nil {
		return "", err
	}

	return topoproto.TabletAliasString(shard.PrimaryAlias), nil
}
