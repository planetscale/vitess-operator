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
Package drain defines a protocol for various agents (humans, controllers, scripts)
to cooperate to avoid disruption due to planned maintenance.

If a Pod or other object is annotated as requiring a drain step, any human or
automated agent that plans to delete it as part of a voluntary disruption
(e.g. config rollouts, app upgrades, Node upgrades, etc.) should first add an
annotation to the object requesting that it be drained. The agent should then
wait for another annotation to appear that confirms the object has been drained,
at which point the object can be safely deleted.

The exact behavior triggered by a drain request varies depending on the object.
The object's controller is responsible for doing whatever is appropriate to
minimize the disruption that would be caused by deleting the object in question.
For example, the VitessShard controller may need to do a planned reparent if the
object was a vttablet Pod that happened to be the master at the time.

By encapsulating this behavior into a drain request, we free all the various
sources of voluntary disruptions from needing to know how to prepare any given
object for deletion. Also, sending the request to the object's controller
instead of directly initiating actions like reparents makes it easier to reason
about how the system will respond under various conditions, since the controller
maintains sole authority over its objects.
*/
package drain

import (
	"fmt"
	"sort"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// AnnotationPrefix is the prefix for annotation keys that are used for
	// coordinating drains through this package.
	AnnotationPrefix = "drain.planetscale.com"

	// SupportedAnnotation is the annotation whose presence indicates that an
	// object's controller implements drains, so this object should be drained
	// before deleting it as part of any voluntary disruption.
	//
	// The string value of the annotation is an optional, human-readable
	// description of what will happen to the object if it's drained.
	//
	// If an object doesn't have the "supported" annotation, none of the other
	// drain-related annotations will have any effect.
	SupportedAnnotation = AnnotationPrefix + "/" + "supported"

	// StartedAnnotation is the annotation whose presence triggers the object's
	// controller to begin draining it, if the controller supports drains.
	StartedAnnotation = AnnotationPrefix + "/" + "started"

	// AcknowledgedAnnotation is necessary in the state machine that ensures
	// only one object at a time is ever marked as finished.  This is entirely
	// internal to the controller and should not be set manually.
	AcknowledgedAnnotation = AnnotationPrefix + "/" + "acknowledged"

	// FinishedAnnotation is the annotation whose presence signals that the object
	// has been successfully drained by its controller. Note that this is not
	// guaranteed to be monotonic; a drain that has finished might become
	// unfinished, for example if some unplanned disruption caused the object
	// to be reassigned its formerly-drained duties to avoid downtime.
	FinishedAnnotation = AnnotationPrefix + "/" + "finished"
)

// Supported returns whether the object's controller supports drains.
func Supported(obj metav1.Object) bool {
	ann := obj.GetAnnotations()
	// We only care that the annotation key is present.
	// An empty annotation value still indicates drains are supported.
	_, present := ann[SupportedAnnotation]
	return present
}

// Started returns whether a drain has been requested on the object.
func Started(obj metav1.Object) bool {
	ann := obj.GetAnnotations()
	// We only care that the annotation key is present.
	// An empty annotation value still triggers a drain.
	_, present := ann[StartedAnnotation]
	return present
}

// Acknowledged returns whether a drain on the object has been acknowledged by
// the controller.
func Acknowledged(obj metav1.Object) bool {
	ann := obj.GetAnnotations()
	// We only care that the annotation key is present.
	// An empty annotation value still triggers a drain.
	_, present := ann[AcknowledgedAnnotation]
	return present
}

// Finished returns whether the object has been successfully drained.
func Finished(obj metav1.Object) bool {
	ann := obj.GetAnnotations()
	// We only care that the annotation key is present.
	// An empty annotation value still means done.
	_, present := ann[FinishedAnnotation]
	return present
}

/*
Start annotates an object to request a drain.

If a drain has already been requested on the object, this has no effect.

Note that this only mutates the provided, in-memory object to add the
annotation; the caller is responsible for sending the updated object to
the server.

'message' is an optional, human-readable description of the reason for the drain.
*/
func Start(obj metav1.Object, message string) {
	ann := obj.GetAnnotations()
	if _, present := ann[StartedAnnotation]; present {
		// A drain has already been requested.
		return
	}
	if ann == nil {
		ann = make(map[string]string, 1)
	}
	ann[StartedAnnotation] = message
	obj.SetAnnotations(ann)
}

/*
Acknowledge annotates an object to signal that the controller has seen the drain
request.

If a drain has already been acknowledged on the object, this has no effect.

Note that this only mutates the provided, in-memory object to add the
annotation; the caller is responsible for sending the updated object to the
server.
*/
func Acknowledge(obj metav1.Object) {
	ann := obj.GetAnnotations()
	if _, present := ann[AcknowledgedAnnotation]; present {
		// A drain has already been acknowledged.
		return
	}
	if ann == nil {
		ann = make(map[string]string, 1)
	}
	ann[AcknowledgedAnnotation] = time.Now().UTC().String()
	obj.SetAnnotations(ann)
}

/*
Unacknowledge removes the "acknowledged" annotation.

If the object does not have the annotation, this has no effect.

Note that this only mutates the provided, in-memory object to add the
annotation; the caller is responsible for sending the updated object to
the server.
*/
func Unacknowledge(obj metav1.Object) {
	ann := obj.GetAnnotations()
	delete(ann, AcknowledgedAnnotation)
	obj.SetAnnotations(ann)
}

/*
Finish annotates an object as having been successfully drained.

If the object has already been marked as drained, this has no effect.

Note that this only mutates the provided, in-memory object to add the
annotation; the caller is responsible for sending the updated object to
the server.
*/
func Finish(obj metav1.Object) {
	ann := obj.GetAnnotations()
	if _, present := ann[FinishedAnnotation]; present {
		// The object has already been drained.
		return
	}
	if ann == nil {
		ann = make(map[string]string, 1)
	}
	ann[FinishedAnnotation] = time.Now().UTC().String()
	obj.SetAnnotations(ann)
}

/*
Unfinish removes the "finished" annotation.

If the object does not have the annotation, this has no effect.

Note that this only mutates the provided, in-memory object to add the
annotation; the caller is responsible for sending the updated object to
the server.
*/
func Unfinish(obj metav1.Object) {
	ann := obj.GetAnnotations()
	delete(ann, FinishedAnnotation)
	obj.SetAnnotations(ann)
}

/*
State is the enum used in the draining state machine algorithm below.
This state machine ensures safety when we are choosing which object is "safe
to delete" (a.k.a. mark as "DrainingFinished") as far as we know.
*/
type State int

const (
	NotDrainingState State = iota
	DrainingState
	AcknowledgedState
	FinishedState
)

func (s State) String() string {
	switch s {
	case NotDrainingState:
		return "NotDraining"
	case DrainingState:
		return "Draining"
	case AcknowledgedState:
		return "DrainingAcknowledged"
	case FinishedState:
		return "DrainingFinished"
	default:
		panic("invalid state")
	}
}

/*
StateTransitions computes state transitions for tablet drains in a given
shard.

The drainStates argument should be a map from your convenient and unique
object identifier (e.g. tablet alias) to the State of that object, which
you can find by calling drain.GetStates.

The return value returns a map populated with only the elements that should
change to a new State, and is keyed by the object identifiers you passed
in.

See below for why this function exists, and a proof of correctness.

# GOALS:

When human administrator wants to drain a node, we want them to have the
ability to safely drain tablets so that the drain can be done without
disrupting vitess clusters.

The workflow we are using now is:

- The administrator annotates tablets that should be drained as "draining".
- The operator annotates one and only one tablet as "finished".
	- Only if other health checks pass and the tablet is not a master.
- The administrator can safely delete the tablet annotated as "finished".

It is actually non trivial to ensure that one and only one tablet will be
marked as finished due to the fact that the operator might be behind in its
understanding of the state of the world, hence this state machine.

# ALGORITHM:

- If a tablet is "NotDraining", take no action.
- If a tablet is "Draining", mark as "DrainingAcknowledged".
- If any tablet is "Draining" but not "DrainingAcknowledged", do not mark any
tablet as "DrainingFinished".
- If all tablets were already either "NotDraining" or "DrainingAcknowledged"
before we made any modifications in this reconcile pass, mark the first
tablet (sorted by tablet alias) as "DrainingFinished".

# PROOF OF CORRECTNESS:

Assume you have two tablets, t1 and t2 that can be in states "not draining",
"draining", "draining-acknowledged", and "finished".

Assume t1 is first in the sort order, which means that if both are
"draining-acknowledged", t1 will always be the tablet set to "finished".

Assume we are marking t1 as "finished", but t2 is already marked as "finished":

- t1 is marked as "finished" only if t2!="finished" and t1="draining-acknowledged" is observed.
- This means t1 was set to "draining-acknowledged" before t2 was set to "finished".
- This means t2 was set to "finished" after t1="draining" was observed.

Step 3 is a contradiction, because t2 would never be set to "finished" at any point after t1="draining" was observed.

Assume we are marking t2 as "finished", but t1 is already marked as "finished":

- t2 is marked as "finished" only if t1!="draining, draining-acknowledged, or
finished" and t2="draining-acknowledged" is observed.
- This means that t1="draining" has not been observed.

Step 2 is a contradiction to the fact that t1 is already marked as "finished"
because t1="draining" has not yet been observed.
*/
func StateTransitions(drainStates map[string]State) map[string]State {
	transitions := map[string]State{}

	// First do the initial scan, acknowledging drains and detecting cases where
	// it is unsafe to mark anything as finished.
	canMarkFinished := true
	for name, state := range drainStates {
		switch state {
		case NotDrainingState:
			continue
		case DrainingState:
			transitions[name] = AcknowledgedState
			canMarkFinished = false
		case AcknowledgedState:
			continue
		case FinishedState:
			canMarkFinished = false
		default:
			panic("Invalid state, should not be possible.")
		}
	}

	if !canMarkFinished {
		return transitions
	}

	// Now iterate in sorted order, and mark the first element in the
	// "DrainingAcknowledged" state as "DrainingFinished".
	var names []string
	for name := range drainStates {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		if drainStates[name] == AcknowledgedState {
			transitions[name] = FinishedState
			return transitions
		}
	}
	return transitions
}

/*
GetState takes an object annotated with the annotations above and
returns the State.  This is the enum that is used in our state machine
algorithm.

It will return an error if the annotations are "impossible", as in they
shouldn't happen if the system is functioning properly.
*/
func GetState(obj metav1.Object) (State, error) {
	if Finished(obj) && Acknowledged(obj) {
		return FinishedState, nil
	}
	if Finished(obj) && !Acknowledged(obj) {
		return FinishedState,
			fmt.Errorf(
				"Invalid annotations on object, this should never happen!  %v", obj)
	}
	if Acknowledged(obj) {
		return AcknowledgedState, nil
	}
	if Started(obj) {
		return DrainingState, nil
	}
	return NotDrainingState, nil
}
