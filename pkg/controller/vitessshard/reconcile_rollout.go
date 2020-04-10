package vitessshard

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apilabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/results"
	"planetscale.dev/vitess-operator/pkg/operator/rollout"
)

func (r *ReconcileVitessShard) reconcileRollout(ctx context.Context, vts *planetscalev2.VitessShard) (reconcile.Result, error) {
	resultBuilder := &results.Builder{}

	if !rollout.Scheduled(vts) {
		// If the shard is not scheduled for a rolling update, silently bail out and do nothing.
		return resultBuilder.Result()
	}

	for _, tablet := range vts.Status.Tablets {
		if tablet.Ready != corev1.ConditionTrue {
			if tablet.Ready != corev1.ConditionTrue {
				// If all tablets aren't healthy, we should bail and not perform a rolling restart.
				r.recorder.Eventf(vts, corev1.EventTypeWarning, "RollingRestartFailed", "all tablets are not healthy")
				return resultBuilder.Result()
			}
		}
	}

	podList := &corev1.PodList{}
	listOpts := &client.ListOptions{
		Namespace: vts.Namespace,
		LabelSelector: apilabels.Set(map[string]string{
			planetscalev2.ComponentLabel: planetscalev2.VttabletComponentName,
			planetscalev2.ClusterLabel:   vts.Labels[planetscalev2.ClusterLabel],
			planetscalev2.KeyspaceLabel:  vts.Labels[planetscalev2.KeyspaceLabel],
			planetscalev2.ShardLabel:     vts.Spec.KeyRange.SafeName(),
		}).AsSelector(),
	}
	if err := r.client.List(ctx, listOpts, podList); err != nil {
		return resultBuilder.Error(err)
	}

	scheduledTablets := false
	for i := range podList.Items {
		pod := &podList.Items[i]
		if !rollout.Scheduled(pod) {
			continue
		}

		scheduledTablets = true
		if err := r.releaseTabletPod(ctx, log, pod); err != nil {
			r.recorder.Eventf(vts, corev1.EventTypeWarning, "RollingRestartFailed", "tablet deletion failed")
			return resultBuilder.Error(err)
		}
	}

	// If we have no more scheduled tablets, unschedule the shard.
	if !scheduledTablets {
		if err := r.unscheduleObject(ctx, vts); err != nil {
			return resultBuilder.Error(err)
		}
	}

	return resultBuilder.Result()
}

func (r *ReconcileVitessShard) releaseTabletPod(ctx context.Context, log *logrus.Entry, pod *corev1.Pod) error {
	// Release the pod to be recreated with updates.
	if err := r.releaseObject(ctx,pod); err != nil {
		return err
	}

	// TODO: Evict pods before deleting them.
	if err := r.client.Delete(ctx, pod); err != nil {
		return err
	}
	if err := r.waitObjectUpdated(ctx, pod); err != nil {
		return err
	}
	if err := r.waitPodReady(ctx, pod); err != nil {
		return err
	}

	return nil
}

func (r *ReconcileVitessShard) waitObjectUpdated(ctx context.Context, obj runtime.Object) error {
	// Wait for obj to have no scheduled updates.
	for {
		if err := regetObject(ctx, r.client, obj); err != nil {
			// A NotFound error is ok. We wait for it to be recreated.
			if apierrors.IsNotFound(err) {
				time.Sleep(1 * time.Second)
				continue
			} else {
				return err
			}
		}

		if !rollout.Scheduled(objectMeta(obj)) {
			// This is what we're waiting for.
			return nil
		}

		// Keep waiting.
		time.Sleep(1 * time.Second)
	}
}

func (r *ReconcileVitessShard) waitPodReady(ctx context.Context, pod *corev1.Pod) error {
	for {
		if err := regetObject(ctx, r.client, pod); err != nil {
			return err
		}

		if podIsReady(pod) {
			// This is what we're waiting for.
			return nil
		}

		// Keep waiting.
		time.Sleep(1 * time.Second)
	}
}

func (r *ReconcileVitessShard) releaseObject(ctx context.Context, obj runtime.Object) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		if err := regetObject(ctx, r.client, obj); err != nil {
			return err
		}
		objMeta := objectMeta(obj)
		if !rollout.Scheduled(objMeta) || rollout.Released(objMeta) {
			// If there's nothing scheduled, or already released, we're done.
			return nil
		}
		rollout.Release(objMeta)
		return r.client.Update(ctx, obj)
	})
}

func (r *ReconcileVitessShard) unscheduleObject(ctx context.Context, obj runtime.Object) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		if err := regetObject(ctx, r.client, obj); err != nil {
			return err
		}
		objMeta := objectMeta(obj)
		if !rollout.Scheduled(objMeta) || rollout.Released(objMeta) {
			// If there's nothing scheduled, or already released, we're done.
			return nil
		}
		rollout.Unschedule(objMeta)
		return r.client.Update(ctx, obj)
	})
}

// regetObject does a fresh Get from the server to update the contents of an
// existing object.
func regetObject(ctx context.Context, kubeClient client.Client, obj runtime.Object) error {
	// Save the name/namespace.
	objMeta := objectMeta(obj)
	key := client.ObjectKey{
		Namespace: objMeta.GetNamespace(),
		Name:      objMeta.GetName(),
	}

	// Reset all fields in the object.
	resetObject(obj)

	// Restore the name/namespace.
	objMeta = objectMeta(obj)
	objMeta.SetNamespace(key.Namespace)
	objMeta.SetName(key.Name)

	return kubeClient.Get(ctx, key, obj)
}

// resetObject resets an Object to its zero value.
func resetObject(obj runtime.Object) {
	objValue := reflect.Indirect(reflect.ValueOf(obj))
	objValue.Set(reflect.Zero(objValue.Type()))
}

func objectMeta(obj runtime.Object) metav1.Object {
	objMeta, err := meta.Accessor(obj)
	if err != nil {
		// This should never happen.
		panic(fmt.Sprintf("can't get metav1.Object: %v", err))
	}
	return objMeta
}

func podIsReady(pod *corev1.Pod) bool {
	for i := range pod.Status.Conditions {
		cond := &pod.Status.Conditions[i]
		if cond.Type == corev1.PodReady {
			return cond.Status == corev1.ConditionTrue
		}
	}
	return false
}