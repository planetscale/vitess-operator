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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
)

// Reconciler abstracts reconciliation logic that's common for any kind of Kubernetes object.
type Reconciler struct {
	// Client is the Kubernetes client.
	client client.Client
	// Scheme is the API type lookup scheme for the controller doing this reconcilitation.
	scheme *runtime.Scheme
	// Recorder is an EventRecorder for the controller doing this reconcilation.
	recorder record.EventRecorder
}

// New returns a new Reconciler.
//
// Generally there is one Reconciler for each controller, even if the controller
// reconciles many different types of secondary objects. The Reconciler can be
// reused for various objects by passing a different Strategy each time.
func New(c client.Client, s *runtime.Scheme, rec record.EventRecorder) *Reconciler {
	return &Reconciler{
		client:   c,
		scheme:   s,
		recorder: rec,
	}
}

// Strategy contains user-specified customization for the reconciler behavior.
type Strategy struct {
	// Kind is a "prototype" of the object kind to reconcile, such as &corev1.Service{}.
	Kind runtime.Object

	/*
		New is called when the the object needs to be created.

		It's ok to set immutable fields here.
		The returned object must be the same type as was given to the Kind field.
	*/
	New func(key client.ObjectKey) runtime.Object

	/*
		UpdateInPlace is called when the object already exists.

		The provided object is a deep copy of the current state, so you can modify it.
		If you change anything in the provided object, the reconciler will update the
		existing object in-place, immediately. If you don't change anything, including
		if you set values that happen to be the same as what they already were, no
		update will be attempted.

		You *must* only try to change mutable fields here. If you need to change any fields
		that require deleting and recreating the object (e.g. most parts of Pod.Spec), use
		UpdateRollingRecreate.

		It should always be safe to cast 'obj' to the same type as the object provided
		in the Kind field (e.g. svc := obj.(*corev1.Service)).
	*/
	UpdateInPlace func(key client.ObjectKey, newObj runtime.Object)

	/*
		UpdateRollingInPlace is called when the object already exists.

		The provided object is a deep copy of the current state, so you can modify it.
		The state will already reflect changes made by UpdateInPlace, if any.

		If you change anything in the provided object, the reconciler will schedule
		an update of the object that will apply the changes in-place. If you don't
		change anything, including if you set values that happen to be the same as
		what they already were, no update will be scheduled.

		Just like with UpdateInPlace, you *must* only try to change mutable fields
		here. If you need to change any fields that require deleting and recreating
		the object (e.g. most parts of Pod.Spec), use UpdateRollingRecreate.

		Updates are scheduled with annotations as defined in the 'rollout' package.

		If you make more changes before the object has been updated, all pending
		changes will be batched into one update to minimize churn.

		It should always be safe to cast 'obj' to the same type as the object provided
		in the Kind field (e.g. svc := obj.(*corev1.Service)).
	*/
	UpdateRollingInPlace func(key client.ObjectKey, newObj runtime.Object)

	/*
		UpdateRecreate is called when the object already exists.

		The provided object is a deep copy of the current state, so you can modify it.
		The state will already reflect changes made by UpdateInPlace, if any.

		If you change anything in the provided object, the reconciler will delete the
		existing object immediately and then recreate it. If you don't change anything,
		including if you set values that happen to be the same as what they already were,
		no deletion will be attempted.

		Unlike with UpdateInPlace or UpdateRollingInPlace, you *can* change fields
		here that are immutable after creation (e.g. most parts of Pod.Spec).
		However, you cannot change the object name or namespace; that should
		instead be thought of as removing one object and adding another one.

		After deletion, the New handler will be called as if the object never existed.
		That means the object will never actually be in the exact state you build here.
		Applying your changes to the current state (which may have changes you didn't
		make and don't care about) is merely a way to determine whether an update is
		necessary without requiring you to write a separate code path for doing diffs.
		Instead, you can write this update code once and then call it from within your
		New handler to avoid repeating yourself.

		It should always be safe to cast 'obj' to the same type as the object provided
		in the Kind field (e.g. svc := obj.(*corev1.Service)).
	*/
	UpdateRecreate func(key client.ObjectKey, newObj runtime.Object)

	/*
		UpdateRollingRecreate is called when the object already exists.

		The provided object is a deep copy of the current state, so you can modify it.
		The state will already reflect changes made by UpdateInPlace and
		UpdateRollingInPlace, if any.

		If you change anything in the provided object, the reconciler will schedule
		an update of the object that will delete and recreate it. If you don't change
		anything, including if you set values that happen to be the same as what they
		already were, no update will be scheduled.

		Unlike with UpdateInPlace or UpdateRollingInPlace, you *can* change fields
		here that are immutable after creation (e.g. most parts of Pod.Spec).
		However, you cannot change the object name or namespace; that should
		instead be thought of as removing one object and adding another one.

		Updates are scheduled with annotations as defined in the 'rollout' package.
		In addition, before deleting the object, the reconciler will check if the
		object supports the protocol defined in the 'drain' package, and will drain
		it if appropriate.

		If you make more changes before the object has been deleted and recreated,
		all pending changes will be batched into one deletion and recreation to
		minimize churn.

		After deletion, the New handler will be called as if the object never existed.
		That means the object will never actually be in the exact state you build here.
		Applying your changes to the current state (which may have changes you didn't
		make and don't care about) is merely a way to determine whether an update is
		necessary without requiring you to write a separate code path for doing diffs.
		Instead, you can write this update code once and then call it from within your
		New handler to avoid repeating yourself.

		It should always be safe to cast 'obj' to the same type as the object provided
		in the Kind field (e.g. svc := obj.(*corev1.Service)).
	*/
	UpdateRollingRecreate func(key client.ObjectKey, newObj runtime.Object)

	/*
		Status is called when the object exists and is desired.

		The provided object is the current state at the beginning of the reconciliation,
		before any changes are attempted. This state is what Status should reflect.
		If any changes are successfully made by other hooks, another reconcile
		will be queued to reflect those changes.

		It should always be safe to cast 'obj' to the same type as the object provided
		in the Kind field (e.g. svc := obj.(*corev1.Service)).
	*/
	Status func(key client.ObjectKey, curObj runtime.Object)

	/*
		OrphanStatus is called when an unwanted object could not be turned down
		because PrepareForTurndown returned a non-nil OrphanStatus.

		It should always be safe to cast 'obj' to the same type as the object provided
		in the Kind field (e.g. svc := obj.(*corev1.Service)).
	*/
	OrphanStatus func(key client.ObjectKey, curObj runtime.Object, orphanStatus *planetscalev2.OrphanStatus)

	/*
		PrepareForTurndown is called to determine if it's ok to delete an object
		that exists but is unwanted (it won't be recreated).

		This callback should return nil if and only if it's ok to delete the object now.
		Otherwise, it should return an OrphanStatus describing why the object should be
		kept around as an orphan.

		The provided object is a deep copy of the current state, so you can modify it.
		If you return an error (not ok to delete now), any modifications you made to
		the object will be sent to the server as if UpdateInPlace had been called.

		It should always be safe to cast 'obj' to the same type as the object provided
		in the Kind field (e.g. svc := obj.(*corev1.Service)).
	*/
	PrepareForTurndown func(key client.ObjectKey, newObj runtime.Object) *planetscalev2.OrphanStatus
}
