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

/*
Package reconciler abstracts reconciliation logic that's common for any kind of Kubernetes object.
*/
package reconciler

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/util/strategicpatch"

	"planetscale.dev/vitess-operator/pkg/operator/drain"
	"planetscale.dev/vitess-operator/pkg/operator/rollout"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

/*
ReconcileObject reconciles a single object by name with the given strategy.

If 'wanted' is true, the object will be created or updated as needed.
If 'wanted' is false, the object will be deleted if it exists.
*/
func (r *Reconciler) ReconcileObject(ctx context.Context, owner runtime.Object, key client.ObjectKey, labels map[string]string, wanted bool, s Strategy) (finalErr error) {
	// Get the name of the Kind, for event log messages.
	gvk, err := apiutil.GVKForObject(s.Kind, r.scheme)
	if err != nil {
		return err
	}
	ownerGVK, err := apiutil.GVKForObject(owner, r.scheme)
	if err != nil {
		return err
	}
	objDesc := fmt.Sprintf("%v %v", gvk.Kind, key.Name)
	ownerMeta, err := meta.Accessor(owner)
	if err != nil {
		return err
	}

	defer func() {
		reconcileCount.With(metricLabels(gvk, ownerGVK, finalErr)).Inc()
	}()

	// Check if the object already exists.
	// Note that this reads from the local cache, so it might be out of date.
	// This is fine, since everything we do should be monotonic and idempotent.
	// If we encounter any inconsistency, we just return an error so the reconcile gets requeued.
	curObj := s.Kind.DeepCopyObject()
	if err := r.client.Get(ctx, key, curObj); err != nil {
		if apierrors.IsNotFound(err) {
			curObj = nil
		} else {
			// It's some other error, meaning we couldn't determine whether it exists.
			r.recorder.Eventf(owner, corev1.EventTypeWarning, "GetFailed", "failed to get %v: %v", objDesc, err)
			return err
		}
	}

	// If it's a Pod, we need to check a special case.
	if pod, ok := curObj.(*corev1.Pod); ok {
		if (pod.Spec.RestartPolicy == corev1.RestartPolicyAlways || pod.Spec.RestartPolicy == corev1.RestartPolicyOnFailure) &&
			pod.Status.Phase == corev1.PodFailed {
			// The Pod was never supposed to enter the permanent Failed phase, but it did.
			// This happens if the Node decides to evict the Pod. Rather than delete
			// the Pod, the Node just marks it with this "tombstone" state and expects
			// anyone who cares about the Pod to create a new one. Since we use stable
			// Pod names like StatefulSet does, we need to delete the Failed Pod to make
			// room for us to reuse the name, just like StatefulSet does.
			p := &metav1.Preconditions{UID: &pod.UID}
			err = r.client.Delete(ctx, curObj, client.PropagationPolicy(metav1.DeletePropagationBackground), client.Preconditions(p))
			deleteCount.With(metricLabels(gvk, ownerGVK, err)).Inc()
			if err != nil {
				r.recorder.Eventf(owner, corev1.EventTypeWarning, "DeleteFailed", "failed to delete evicted Pod %v: %v", pod.Name, err)
				return err
			}
			evictedPodCount.Inc()
			r.recorder.Eventf(owner, corev1.EventTypeNormal, "Deleted", "deleted evicted Pod %v", pod.Name)
			// For the rest of the function, assume the Pod doesn't exist
			// so we'll try to create it if it's wanted.
			curObj = nil
		}
	}

	// See if we need to clean up an unwanted object.
	if !wanted {
		if curObj == nil {
			// The object we don't want already doesn't exist, so there's nothing to do.
			return nil
		}
		curObjMeta, err := meta.Accessor(curObj)
		if err != nil {
			return nil
		}
		if curObjMeta.GetDeletionTimestamp() != nil {
			// It's already on its way down.
			return nil
		}
		curObjDesc := fmt.Sprintf("%v %v", gvk.Kind, curObjMeta.GetName())
		// Verify labels to make sure it's ours.
		if !hasMatchingLabels(curObjMeta, labels) {
			// We don't want one and it's not ours, so we'll just leave it.
			r.recorder.Eventf(owner, corev1.EventTypeWarning, "NameCollision", "not deleting unwanted %v because its labels don't match ours", curObjDesc)
			return nil
		}
		// See if it's ok to delete the unwanted object.
		if s.PrepareForTurndown != nil {
			newObj := curObj.DeepCopyObject()
			if orphanStatus := s.PrepareForTurndown(key, newObj); orphanStatus != nil {
				// Record that we can't clean up, but don't necessarily return an error,
				// because this is not a failure to reconcile; we're just waiting.
				// We will get queued again if the state changes, at which time we'll
				// recheck if it's ok to turn down.
				r.recorder.Eventf(owner, corev1.EventTypeWarning, "TurndownBlocked", "refusing to delete unwanted %v: %v", curObjDesc, orphanStatus.Message)
				if s.OrphanStatus != nil {
					s.OrphanStatus(key, curObj, orphanStatus)
				}
				// Update the object if anything was changed as part of preparing for turndown.
				return r.updateInPlace(ctx, owner, key, s, curObj, newObj)
			}
		}
		// Prepare succeeded. Try to delete the object.
		uid := curObjMeta.GetUID()
		p := &metav1.Preconditions{UID: &uid}
		err = r.client.Delete(ctx, curObj, client.PropagationPolicy(metav1.DeletePropagationBackground), client.Preconditions(p))
		deleteCount.With(metricLabels(gvk, ownerGVK, err)).Inc()
		if err != nil {
			r.recorder.Eventf(owner, corev1.EventTypeWarning, "DeleteFailed", "failed to delete %v: %v", curObjDesc, err)
			return err
		}
		r.recorder.Eventf(owner, corev1.EventTypeNormal, "Deleted", "deleted %v", curObjDesc)
		return nil
	}

	if curObj == nil {
		// The object we want doesn't exist, so create a new one.
		newObj := s.New(key)
		newObjMeta, err := meta.Accessor(newObj)
		if err != nil {
			return err
		}
		newObjMeta.SetNamespace(key.Namespace)
		newObjMeta.SetName(key.Name)
		if err := controllerutil.SetControllerReference(ownerMeta, newObjMeta, r.scheme); err != nil {
			r.recorder.Eventf(owner, corev1.EventTypeWarning, "CreateFailed", "failed to create %v: %v", objDesc, err)
			return err
		}
		err = r.client.Create(ctx, newObj)
		createCount.With(metricLabels(gvk, ownerGVK, err)).Inc()
		if err != nil {
			r.recorder.Eventf(owner, corev1.EventTypeWarning, "CreateFailed", "failed to create %v: %v", objDesc, err)
			return err
		}
		r.recorder.Eventf(owner, corev1.EventTypeNormal, "Created", "created %v", objDesc)
		return nil
	}

	// The object we want already exists. See if we need to update it.
	// First, verify the labels are as expected so we don't accidentally steal.
	curObjMeta, err := meta.Accessor(curObj)
	if err != nil {
		return err
	}
	curObjDesc := fmt.Sprintf("%v %v", gvk.Kind, curObjMeta.GetName())
	if !hasMatchingLabels(curObjMeta, labels) {
		err := fmt.Errorf("%v already exists, but does not have matching labels", curObjDesc)
		r.recorder.Event(owner, corev1.EventTypeWarning, "NameCollision", err.Error())
		return err
	}

	// Update status based on the current object.
	if s.Status != nil {
		s.Status(key, curObj)
	}

	if curObjMeta.GetDeletionTimestamp() != nil {
		// The object is on its way down, so we shouldn't try to update it.
		return nil
	}

	// Now see if we have any changes to apply.
	// Update things that are safe to change immediately in-place.
	updatedObjInPlace := curObj.DeepCopyObject()
	if s.UpdateInPlace != nil {
		s.UpdateInPlace(key, updatedObjInPlace)
	}

	// See if anything else needs to be updated that would trigger an immediate
	// deletion.
	if s.UpdateRecreate != nil {
		updatedObjRecreate := updatedObjInPlace.DeepCopyObject()
		s.UpdateRecreate(key, updatedObjRecreate)
		if !deepEqual(r.scheme, updatedObjInPlace, updatedObjRecreate) {
			// Something changed that triggers an immediate deletion.
			// After deleting, we wait for the next reconciliation to recreate.
			return r.delete(ctx, owner, key, s, curObj)
		}
	}

	// If the object is ready to be rolled out, also apply rolling updates.
	if rollout.Released(curObjMeta) {
		if s.UpdateRollingInPlace != nil {
			s.UpdateRollingInPlace(key, updatedObjInPlace)
		}
		if s.UpdateRollingRecreate != nil {
			updatedObjRecreate := updatedObjInPlace.DeepCopyObject()
			s.UpdateRollingRecreate(key, updatedObjRecreate)
			if !deepEqual(r.scheme, updatedObjInPlace, updatedObjRecreate) {
				// Even if we were to apply the immediate-in-place and rolling-in-place changes,
				// there would still be additional changes that require deletion and recreation
				// anyway, so we can just ignore the in-place changes and delete now.
				// After deleting, we wait for the next reconciliation to recreate.
				return r.drainAndDelete(ctx, owner, key, s, curObj, updatedObjInPlace, updatedObjRecreate)
			}
		}
		// If this update is successful, we'll have no more pending changes, so remove the annotation.
		updatedObjInPlaceMeta, err := meta.Accessor(updatedObjInPlace)
		if err != nil {
			return err
		}
		rollout.Unschedule(updatedObjInPlaceMeta)
		return r.updateInPlace(ctx, owner, key, s, curObj, updatedObjInPlace)
	}

	// The object is not ready to be rolled out. See if we need to schedule pending changes.
	// Check if any additional changes will be needed on top of the immediate, in-place changes.
	updatedObjRollout := updatedObjInPlace.DeepCopyObject()
	if s.UpdateRollingInPlace != nil {
		s.UpdateRollingInPlace(key, updatedObjRollout)
	}
	if s.UpdateRollingRecreate != nil {
		s.UpdateRollingRecreate(key, updatedObjRollout)
	}
	// In either case, we go ahead and do the in-place update.
	// Just set the rollout annotations accordingly.
	updatedObjInPlaceMeta, err := meta.Accessor(updatedObjInPlace)
	if err != nil {
		return err
	}
	if deepEqual(r.scheme, updatedObjInPlace, updatedObjRollout) {
		rollout.Unschedule(updatedObjInPlaceMeta)
	} else {
		rollout.Schedule(updatedObjInPlaceMeta, describeDiff(updatedObjInPlace, updatedObjRollout, s.Kind))
	}
	return r.updateInPlace(ctx, owner, key, s, curObj, updatedObjInPlace)
}

func (r *Reconciler) updateInPlace(ctx context.Context, owner runtime.Object, key client.ObjectKey, s Strategy, curObj, newObj runtime.Object) error {
	gvk, err := apiutil.GVKForObject(s.Kind, r.scheme)
	if err != nil {
		return err
	}
	ownerGVK, err := apiutil.GVKForObject(owner, r.scheme)
	if err != nil {
		return err
	}
	ownerMeta, err := meta.Accessor(owner)
	if err != nil {
		return err
	}
	newObjMeta, err := meta.Accessor(newObj)
	if err != nil {
		return err
	}
	newObjDesc := fmt.Sprintf("%v %v", gvk.Kind, newObjMeta.GetName())

	if err := controllerutil.SetControllerReference(ownerMeta, newObjMeta, r.scheme); err != nil {
		r.recorder.Eventf(owner, corev1.EventTypeWarning, "UpdateFailed", "failed to update %v: %v", newObjDesc, err)
		return err
	}

	if deepEqual(r.scheme, curObj, newObj) {
		// Nothing changed so we're already satisfied.
		return nil
	}

	logrus.WithFields(logrus.Fields{
		"gvk":  gvk.String(),
		"key":  key.String(),
		"diff": describeDiff(curObj, newObj, s.Kind),
	}).Info("Updating object in place")

	err = r.client.Update(ctx, newObj)
	updateCount.With(metricLabels(gvk, ownerGVK, err)).Inc()
	if err != nil {
		r.recorder.Eventf(owner, corev1.EventTypeWarning, "UpdateFailed", "failed to update %v: %v", newObjDesc, err)
		return err
	}
	r.recorder.Eventf(owner, corev1.EventTypeNormal, "Updated", "updated %v", newObjDesc)
	return nil
}

func (r *Reconciler) drainAndDelete(ctx context.Context, owner runtime.Object, key client.ObjectKey, s Strategy, curObj, updatedObjInPlace, updatedObjRecreate runtime.Object) error {
	// If we can't delete because we need to drain first,
	// we'll try to at least do the in-place update.
	newObj := updatedObjInPlace
	newObjMeta, err := meta.Accessor(newObj)
	if err != nil {
		return err
	}
	curObjMeta, err := meta.Accessor(curObj)
	if err != nil {
		return err
	}

	// If the object supports drain, we need to drain first.
	if drain.Supported(curObjMeta) && !drain.Finished(curObjMeta) {
		drain.Start(newObjMeta, "rolling update")
		// We still have changes pending from UpdateRollingRecreate
		// since we didn't get to delete yet.
		rollout.Schedule(newObjMeta, describeDiff(updatedObjInPlace, updatedObjRecreate, s.Kind))
		return r.updateInPlace(ctx, owner, key, s, curObj, newObj)
	}

	// Really delete now.
	return r.delete(ctx, owner, key, s, curObj)
}

func (r *Reconciler) delete(ctx context.Context, owner runtime.Object, key client.ObjectKey, s Strategy, curObj runtime.Object) error {
	gvk, err := apiutil.GVKForObject(s.Kind, r.scheme)
	if err != nil {
		return err
	}
	ownerGVK, err := apiutil.GVKForObject(owner, r.scheme)
	if err != nil {
		return err
	}
	curObjMeta, err := meta.Accessor(curObj)
	if err != nil {
		return err
	}
	curObjDesc := fmt.Sprintf("%v %v", gvk.Kind, curObjMeta.GetName())

	uid := curObjMeta.GetUID()
	p := &metav1.Preconditions{UID: &uid}
	err = r.client.Delete(ctx, curObj, client.PropagationPolicy(metav1.DeletePropagationBackground), client.Preconditions(p))
	deleteCount.With(metricLabels(gvk, ownerGVK, err)).Inc()
	if err != nil {
		r.recorder.Eventf(owner, corev1.EventTypeWarning, "DeleteFailed", "failed to delete %v: %v", curObjDesc, err)
		return err
	}
	r.recorder.Eventf(owner, corev1.EventTypeNormal, "Deleted", "deleted %v", curObjDesc)
	return nil
}

func hasMatchingLabels(obj metav1.Object, expectedLabels map[string]string) bool {
	labels := obj.GetLabels()
	for k, v := range expectedLabels {
		if labels[k] != v {
			return false
		}
	}
	return true
}

func deepEqual(scheme *runtime.Scheme, curObj, newObj runtime.Object) bool {
	// Before comparing with the current object, try to fill in defaults.
	// This should help to avoid fighting with the server, because the values
	// we set tend to not have defaults, but the server will always fill them in.
	//
	// Note that this only works if we actually import the defaulters for the APIs
	// we use. See schemeAddFuncs in main.go for an example of how to do that.
	scheme.Default(curObj)
	scheme.Default(newObj)

	return apiequality.Semantic.DeepEqual(curObj, newObj)
}

func describeDiff(curObj, newObj runtime.Object, kind runtime.Object) string {
	patch, err := describeDiffPatch(curObj, newObj, kind)
	if err != nil {
		// Strategic merge patch might not work for every object.
		// Fall back to a more generic diff mechanism.
		return diff.ObjectReflectDiff(curObj, newObj)
	}
	return patch
}

func describeDiffPatch(curObj, newObj runtime.Object, kind runtime.Object) (string, error) {
	curJSON, err := json.Marshal(curObj)
	if err != nil {
		return "", err
	}
	newJSON, err := json.Marshal(newObj)
	if err != nil {
		return "", err
	}
	patch, err := strategicpatch.CreateTwoWayMergePatch(curJSON, newJSON, kind)
	if err != nil {
		return "", err
	}
	// Unmarshal the JSON patch so we can Marshal into YAML.
	patchMap := map[string]interface{}{}
	if err := json.Unmarshal(patch, &patchMap); err != nil {
		return "", err
	}

	// While we have the patch as a map, clean up some fields we don't need
	// to make it look prettier for humans.
	cleanPatchMap(patchMap)

	patchYAML, err := yaml.Marshal(patchMap)
	if err != nil {
		return "", err
	}
	return string(patchYAML), nil
}

func cleanPatchMap(patch map[string]interface{}) {
	for k, v := range patch {
		if strings.HasPrefix(k, "$setElementOrder") {
			delete(patch, k)
			continue
		}
		if val, ok := v.(map[string]interface{}); ok {
			cleanPatchMap(val)
		}
	}
}
