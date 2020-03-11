/*
Copyright 2020 PlanetScale Inc.

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

package drain

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// checkInvariants will check to see if we broke our one invariant, that at most
// one element is ever marked as "Finished".
func checkInvariants(drainStates map[string]State) error {
	foundFinished := false
	for name, state := range drainStates {
		if state == FinishedState {
			if foundFinished {
				return fmt.Errorf("Found another element marked as Finished: %s", name)
			}
			foundFinished = true
		}
	}
	return nil
}

// checkNoSpontaneousDrains is a sanity check to ensure that our algorithm isn't
// touching any elements that are "NotDraining".
func checkNoSpontaneousDrains(drainStates map[string]State, transitions map[string]State) error {
	for name, state := range transitions {
		if drainStates[name] == NotDrainingState {
			if state != NotDrainingState {
				return fmt.Errorf("Found an element that started draining unprompted: %s", name)
			}
		}
	}
	return nil
}

// generateRandomDrainStates generates a set of elements in random but valid
// drain states.  At most element will be marked as finished, satisfying our
// invariant.
func generateRandomDrainStates() map[string]State {
	drainStates := map[string]State{}
	markFinished := rand.Intn(20)
	for i := 0; i <= 9; i++ {
		startingState := rand.Intn(3)
		if startingState == 0 {
			drainStates[fmt.Sprintf("%d", i)] = NotDrainingState
		} else if startingState == 1 {
			drainStates[fmt.Sprintf("%d", i)] = DrainingState
		} else if startingState == 2 {
			drainStates[fmt.Sprintf("%d", i)] = AcknowledgedState
		}
		// We have a 50% chance of marking one of our 10 tablets as finished
		if i == markFinished {
			drainStates[fmt.Sprintf("%d", i)] = FinishedState
		}
	}
	return drainStates
}

// applyTransitions can either apply all transitions to drainStates or a random
// subset.  Useful for getting the new "real" and "cached" states after the
// transitions are applied.
func applyTransitions(drainStates map[string]State, transitions map[string]State, applyAll bool) map[string]State {
	resultDrainStates := map[string]State{}
	for name, state := range drainStates {
		resultDrainStates[name] = state
	}
	for name, state := range transitions {
		applyTransition := rand.Intn(4)
		if applyAll || applyTransition == 0 {
			resultDrainStates[name] = state
		}
	}
	return resultDrainStates
}

// applyRandomDrains applies some random "Draining" operations to any elements
// in "NotDraining".  A subset of these "Draining" requests will be applied to
// the cached state, but not all of them.
func applyRandomDrains(realDrainStates map[string]State, cachedDrainStates map[string]State) (map[string]State, map[string]State) {
	resultRealDrainStates := map[string]State{}
	resultCachedDrainStates := map[string]State{}
	for name, state := range cachedDrainStates {
		resultCachedDrainStates[name] = state
	}
	for name, state := range realDrainStates {
		resultRealDrainStates[name] = state
		if state == NotDrainingState && rand.Intn(2) == 0 {
			resultRealDrainStates[name] = DrainingState
			if rand.Intn(2) == 0 {
				resultCachedDrainStates[name] = DrainingState
			}
		}
	}
	return resultRealDrainStates, resultCachedDrainStates
}

func TestStateTransitions(t *testing.T) {
	for i := 0; i <= 10000; i++ {

		// 1. Run state transition on initial random valid set of states.
		initial := generateRandomDrainStates()
		trans1 := StateTransitions(initial)

		// 2. Sanity check that we aren't touching anything that isn't draining.
		err := checkNoSpontaneousDrains(initial, trans1)
		if err != nil {
			t.Errorf("Old: %v+, New: %v+, Error: %v", initial, trans1, err)
		}

		// 3. Generate the "real" and "cached" states after the transition.
		current := applyTransitions(initial, trans1, true)
		cached := applyTransitions(initial, trans1, false)

		// 4. Check that we haven't broken any invariants on our "real" state.
		err = checkInvariants(current)
		if err != nil {
			t.Errorf("initial: %v+, current: %v+, trans1: %v+, Error: %v",
				initial, current, trans1, err)
		}

		// 5. Apply random drains to "real" and "cached" states.
		current, cached = applyRandomDrains(current, cached)

		// 6. Run another iteration based on "cached" state.
		trans2 := StateTransitions(cached)

		// 7. Sanity check that we aren't touching anything that isn't draining.
		err = checkNoSpontaneousDrains(cached, trans2)
		if err != nil {
			t.Errorf("Old: %v+, New: %v+, Error: %v", cached, trans2, err)
		}

		// 8. Get the current "real" state with both transitions applied.
		current = applyTransitions(current, trans2, true)

		// 9. Check whether the final "real" state broke our invariants.
		err = checkInvariants(current)
		if err != nil {
			t.Errorf(
				"initial: %v+, current: %v+, cached: %v+, trans1: %v+, trans2: %v+, err: %v",
				initial, current, cached, trans1, trans2, err)
		}
	}
}

// setDrainingAnnotation sets the draining annotation because the library
// doesn't do this (it's the user).
func setDrainingAnnotation(obj metav1.Object) {
	ann := obj.GetAnnotations()
	if _, present := ann[StartedAnnotation]; present {
		return
	}
	if ann == nil {
		ann = make(map[string]string, 1)
	}
	ann[StartedAnnotation] = "Decommissioning Node"
	obj.SetAnnotations(ann)
}

func TestGetDrainState(t *testing.T) {
	goodState := corev1.Pod{}
	Acknowledge(&goodState)
	state, err := GetState(&goodState)
	assert.NoError(t, err, "Error getting drain state")
	assert.Equal(t, state, AcknowledgedState,
		"Draining should be marked acknowledged even if the drain request was removed")

	Finish(&goodState)
	state, err = GetState(&goodState)
	assert.NoError(t, err, "Error getting drain state")
	assert.Equal(t, state, FinishedState,
		"Draining should be marked finished even if the drain request was removed")

	setDrainingAnnotation(&goodState)
	state, err = GetState(&goodState)
	assert.NoError(t, err, "Error getting drain state")
	assert.Equal(t, state, FinishedState,
		"Draining should be marked as finished")

	badState := corev1.Pod{}
	Finish(&badState)
	_, err = GetState(&badState)
	assert.Error(t, err, "Should have failed because this is an invalid state")
}
