package vitessshard

import (
	"context"
	"sort"

	"k8s.io/api/core/v1"
	apilabels "k8s.io/apimachinery/pkg/labels"
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

	if !rollout.Cascaded(vts) {
		// If the shard is not scheduled for a cascading update, silently bail out and do nothing.
		return resultBuilder.Result()
	}

	podList := &v1.PodList{}
	listOpts := &client.ListOptions{
		Namespace: vts.Namespace,
		LabelSelector: apilabels.Set(map[string]string{
			planetscalev2.ComponentLabel: planetscalev2.VttabletComponentName,
			planetscalev2.ClusterLabel:   vts.Labels[planetscalev2.ClusterLabel],
			planetscalev2.KeyspaceLabel:  vts.Labels[planetscalev2.KeyspaceLabel],
			planetscalev2.ShardLabel:     vts.Spec.KeyRange.SafeName(),
		}).AsSelector(),
	}

	if err := r.client.List(ctx, podList, listOpts); err != nil {
		return resultBuilder.Error(err)
	}

	// Safety checks and rolling updates must be deterministically ordered, so we access and sort pods by tablet alias.
	tabletPods := make(map[string]v1.Pod)
	for i := range podList.Items {
		pod := podList.Items[i]
		tabletAlias := vttablet.AliasFromPod(&pod)
		tabletKey := topoproto.TabletAliasString(&tabletAlias)
		tabletPods[tabletKey] = pod
	}

	tabletKeys := make([]string, 0, len(tabletPods))
	for key := range tabletPods {
		tabletKeys = append(tabletKeys, key)
	}
	sort.Strings(tabletKeys)

	for _, tabletKey := range tabletKeys {
		tablet := vts.Status.Tablets[tabletKey]
		if tablet.Available != v1.ConditionTrue {
			// If all tablets aren't healthy, we should bail and not perform a rolling restart.
			r.recorder.Eventf(vts, v1.EventTypeNormal, "RolloutPaused", "Waiting for tablet %v to be Available.", tabletKey)
			return resultBuilder.Result()
		}

		pod := tabletPods[tabletKey]
		if rollout.Released(&pod) {
			// If any tablet has already been released, we should wait until it is finished to release another one.
			r.recorder.Eventf(vts, v1.EventTypeNormal, "RolloutPaused", "Waiting for tablet %v to finish release.", tabletKey)
		}
	}

	scheduledTablets := false
	masterEligibleTablets := vts.Spec.MasterEligibleTabletCount()

	for _, tabletKey := range tabletKeys {
		pod := tabletPods[tabletKey]
		if !rollout.Scheduled(&pod) {
			continue
		}

		scheduledTablets = true
		deletePod := false
		tabletType := pod.Labels[planetscalev2.TabletTypeLabel]
		// These two conditions guarantee that the tablet is a lone master.
		if masterEligibleTablets == 1 && tabletType == string(planetscalev2.ReplicaPoolType) {
			// If we have a lone master, we must delete it since reparenting is impossible.
			deletePod = true
		}

		if err := r.releaseTabletPod(ctx, &pod, deletePod); err != nil {
			r.recorder.Eventf(vts, v1.EventTypeWarning, "RollingRestartFailed", "deletion of pod %v with tablet %v failed", pod.Name, tabletKey)
			return resultBuilder.Error(err)
		} else {
			// We must only release at most one tablet per reconcile loop, so exit if we do.
			return resultBuilder.Result()
		}

	}

	// If we have no more scheduled tablets, uncascade the shard.
	if !scheduledTablets {
		r.uncascadeShard(vts)
	}

	return resultBuilder.Result()
}

func (r *ReconcileVitessShard) releaseTabletPod(ctx context.Context, pod *v1.Pod, deletePod bool) error {
	if deletePod {
		// TODO: Evict pods instead of deleting them directly, to respect PDBs.
		if err := r.client.Delete(ctx, pod); err != nil {
			return err
		}
	} else {
		// Release the pod to be recreated with updates.
		releasePod(pod)
	}

	return nil
}

func releasePod(pod *v1.Pod) {
	if !rollout.Scheduled(pod) || rollout.Released(pod) {
			// If there's nothing scheduled, or already released, we're done.
			return
	}

	rollout.Release(pod)
	return
}

func (r *ReconcileVitessShard) uncascadeShard(vts *planetscalev2.VitessShard) {
	if !rollout.Scheduled(vts) || rollout.Released(vts) {
		// If there's nothing scheduled, or already released, we're done.
		return
	}

	rollout.Uncascade(vts)
}