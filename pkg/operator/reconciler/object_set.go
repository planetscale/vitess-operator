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

package reconciler

import (
	"context"

	"planetscale.dev/vitess-operator/pkg/operator/results"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	apilabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

/*
ReconcileObjectSet reconciles a set of multiple objects of a given kind.

The 'keys' argument is a list of objects that should be created or updated in the set.

Any pre-existing objects that match 'selector' but are not listed in 'keys'
are assumed to be unwanted and will be deleted. This will not occur until after
all the desired objects (listed in 'keys') have been reconciled.

Reconciliation of each individual object in the set will use the provided Strategy,
whose fields have the same meaning as for a single call to ReconcileObject.
*/
func (r *Reconciler) ReconcileObjectSet(ctx context.Context, owner runtime.Object, keys []client.ObjectKey, labels map[string]string, s Strategy) error {
	// Get the corresponding GroupVersionKind of the Go type.
	gvk, err := apiutil.GVKForObject(s.Kind, r.scheme)
	if err != nil {
		return err
	}

	// Get a list of objects of that kind.
	listObj, err := r.scheme.New(schema.GroupVersionKind{
		Group:   gvk.Group,
		Version: gvk.Version,
		// This only works if everyone follows the convention of the list kind for "Kind" being named "KindList".
		// TODO(enisoc): See if there's a better way to get the List kind. Might need to look at Discovery info?
		Kind: gvk.Kind + "List",
	})
	if err != nil {
		r.recorder.Eventf(owner, corev1.EventTypeWarning, "ListFailed", "failed to list %v objects: %v", gvk.Kind, err)
		return err
	}
	ownerMeta, err := meta.Accessor(owner)
	if err != nil {
		return err
	}
	listOpts := &client.ListOptions{
		Namespace:     ownerMeta.GetNamespace(),
		LabelSelector: apilabels.SelectorFromSet(apilabels.Set(labels)),
	}
	if err := r.client.List(ctx, listOpts, listObj); err != nil {
		r.recorder.Eventf(owner, corev1.EventTypeWarning, "ListFailed", "failed to list %v objects: %v", gvk.Kind, err)
		return err
	}

	// Remember the first error, but keep trying others.
	// The error is really only used to know whether the overall process should be requeued.
	// Individual things going wrong should be logged as events through the EventRecorder.
	resultBuilder := results.Builder{}

	// Create/update desired objects.
	wanted := make(map[client.ObjectKey]bool, len(keys))
	for _, key := range keys {
		wanted[key] = true

		if err := r.ReconcileObject(ctx, owner, key, labels, true, s); err != nil {
			// Remember the first error, but keep trying others.
			resultBuilder.Error(err)
			continue
		}
	}

	// Delete objects that exist but are unwanted.
	// We guarantee to the caller that we'll only do this *after* reconciling
	// all the desired objects.
	err = meta.EachListItem(listObj, func(obj runtime.Object) error {
		objMeta, err := meta.Accessor(obj)
		if err != nil {
			// Remember the first error, but keep trying others.
			resultBuilder.Error(err)
			return nil
		}
		key := client.ObjectKey{Namespace: objMeta.GetNamespace(), Name: objMeta.GetName()}

		// Skip wanted objects, since we processed them above.
		if wanted[key] {
			return nil
		}

		if err := r.ReconcileObject(ctx, owner, key, labels, false, s); err != nil {
			// Remember the first error, but keep trying others.
			resultBuilder.Error(err)
			return nil
		}

		return nil
	})
	if err != nil {
		resultBuilder.Error(err)
	}

	_, err = resultBuilder.Result()
	return err
}
