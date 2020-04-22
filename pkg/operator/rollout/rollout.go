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

/*
Package rollout defines a protocol for automating the gradual rollout of changes
throughout a VitessCluster by splitting rolling update logic into
composable pieces: deciding what changes to make, deciding when and in what
order to apply changes, and then actually applying the changes.

Controllers like VitessShard decide what to change (e.g. new Pod image or args)
based on user input, but rather than applying changes immediately, they merely
annotate objects as having changes pending.

A RolloutPolicy then determines which changes to apply next, based on the user's
configuration, and annotates the objects that are next in line to be updated.

Finally, the Reconciler automatically applies any pending changes that have been
"released" by the RolloutPolicy.

This decomposition makes each piece easier to write correctly, makes the overall
process easier to reason about abstractly, and enables code reuse across
distributed components while maintaining loose coupling to allow composition in
new and unexpected ways without changing the code.
*/
package rollout

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// AnnotationPrefix is the prefix for annotation keys that are used for
	// coordinating drains through this package.
	AnnotationPrefix = "rollout.planetscale.com"

	// ScheduledAnnotation is the annotation whose presence indicates that the
	// object's controller would like to apply pending changes to the object.
	ScheduledAnnotation = AnnotationPrefix + "/" + "scheduled"

	// ReleasedAnnotation is the annotation whose presence indicates that the
	// object's controller may now apply any pending changes to the object.
	ReleasedAnnotation = AnnotationPrefix + "/" + "released"

	// CascadeAnnotation is the annotation whose presence indicates that the
	// object's controller should now release any scheduled changes to its children.
	// The controller will remove the annotation when all children are updated.
	CascadeAnnotation = AnnotationPrefix + "/" + "cascade"
)

// Scheduled returns whether the object has pending changes.
func Scheduled(obj metav1.Object) bool {
	ann := obj.GetAnnotations()
	// We only care that the annotation key is present.
	// An empty annotation value still indicates changes are pending.
	_, present := ann[ScheduledAnnotation]
	return present
}

// Released returns whether it's ok to apply changes to the object.
func Released(obj metav1.Object) bool {
	ann := obj.GetAnnotations()
	// We only care that the annotation key is present.
	// An empty annotation value still triggers a drain.
	_, present := ann[ReleasedAnnotation]
	return present
}

// Cascading returns whether scheduled changes are being applied to an object's children.
func Cascading(obj metav1.Object) bool {
	ann := obj.GetAnnotations()
	// We only care that the annotation key is present.
	// An empty annotation value still indicates that cascading
	// changes will be propagated to the object's children.
	_, present := ann[CascadeAnnotation]
	return present
}

/*
Schedule annotates an object as having pending updates.

If updates are already scheduled, the message will be updated.

Note that this only mutates the provided, in-memory object to add the
annotation; the caller is responsible for sending the updated object to
the server.

'message' is an optional, human-readable description of the pending changes.
*/
func Schedule(obj metav1.Object, message string) {
	ann := obj.GetAnnotations()
	if ann == nil {
		ann = make(map[string]string, 1)
	}
	ann[ScheduledAnnotation] = message
	obj.SetAnnotations(ann)
}

/*
Unschedule removes the "pending" annotation added by Schedule,
as well as the "released" annotation added by Release.

If the object does not have either annotation, this has no effect.

Note that this only mutates the provided, in-memory object to add the
annotation; the caller is responsible for sending the updated object to
the server.
*/
func Unschedule(obj metav1.Object) {
	ann := obj.GetAnnotations()
	delete(ann, ScheduledAnnotation)
	delete(ann, ReleasedAnnotation)
	obj.SetAnnotations(ann)
}

/*
Release annotates an object as being ready to have changes applied.

If the object has already been marked as released, this has no effect.

Note that this only mutates the provided, in-memory object to add the
annotation; the caller is responsible for sending the updated object to
the server.
*/
func Release(obj metav1.Object) {
	ann := obj.GetAnnotations()
	if _, present := ann[ReleasedAnnotation]; present {
		// The object has already been released.
		return
	}
	if ann == nil {
		ann = make(map[string]string, 1)
	}
	ann[ReleasedAnnotation] = ""
	obj.SetAnnotations(ann)
}

/*
Unrelease removes the "released" annotation added by Release.

If the object does not have the annotation, this has no effect.

Note that this only mutates the provided, in-memory object to add the
annotation; the caller is responsible for sending the updated object to
the server.
*/
func Unrelease(obj metav1.Object) {
	ann := obj.GetAnnotations()
	delete(ann, ReleasedAnnotation)
	obj.SetAnnotations(ann)
}

/*
Cascade annotates an object to tell its controller to release
any scheduled changes to its children.

If the object has already been marked as cascading, this has no effect.

Note that this only mutates the provided, in-memory object to add the
annotation; the caller is responsible for sending the updated object to
the server.
*/
func Cascade(obj metav1.Object) {
	ann := obj.GetAnnotations()
	if _, present := ann[CascadeAnnotation]; present {
		// The object has already been marked as cascading.
		return
	}
	if ann == nil {
		ann = make(map[string]string, 1)
	}
	ann[CascadeAnnotation] = ""
	obj.SetAnnotations(ann)
}

/*
Uncascade removes the "cascade" annotation added by Cascade.

If the object does not have the annotation, this has no effect.

Note that this only mutates the provided, in-memory object to add the
annotation; the caller is responsible for sending the updated object to
the server.
 */
func Uncascade(obj metav1.Object) {
	ann := obj.GetAnnotations()
	delete(ann, CascadeAnnotation)
	obj.SetAnnotations(ann)
}