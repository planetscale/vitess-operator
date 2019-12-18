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
Package results has functions to work with controller-runtime reconcile.Result objects.
*/
package results

import (
	"time"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Builder aggregates an overall reconcile.Result as you go, so you don't have
// to quit on the first error, in case there are multiple things you want to
// attempt that don't depend on each other.
type Builder struct {
	result   reconcile.Result
	firstErr error
}

// Error sets the resulting error if an error has not yet been set.
// It has no effect if an error has already been set.
// In either case, after this is called, the ultimate reconcile
// result will be failure.
// It returns the aggregated result and error so far.
func (a *Builder) Error(err error) (reconcile.Result, error) {
	if a.firstErr == nil {
		a.firstErr = err
	}
	return a.Result()
}

// Requeue marks that an immediate requeue should be requested,
// even if the ultimate result is success.
// It returns the aggregated result and error so far.
func (a *Builder) Requeue() (reconcile.Result, error) {
	// Clear the time delay, if any, since someone asked for immediate requeue.
	a.result.RequeueAfter = 0
	a.result.Requeue = true
	return a.Result()
}

// RequeueAfter marks that a delayed requeue should be requested,
// even if the ultimate result is success.
// It has no effect if an immediate requeue or a less-delayed (sooner)
// requeue has already been requested.
// It returns the aggregated result and error so far.
func (a *Builder) RequeueAfter(delay time.Duration) (reconcile.Result, error) {
	if a.result.Requeue {
		// We're already requesting immediate requeue.
		return a.Result()
	}
	if a.result.RequeueAfter > 0 && a.result.RequeueAfter < delay {
		// We're already requesting a sooner requeue.
		return a.Result()
	}
	a.result.RequeueAfter = delay
	return a.Result()
}

// Merge aggregates another result into this one.
// It returns the aggregated result and error so far.
func (a *Builder) Merge(otherResult reconcile.Result, otherErr error) (reconcile.Result, error) {
	if otherErr != nil {
		a.Error(otherErr)
	}
	if otherResult.Requeue {
		a.Requeue()
	}
	if otherResult.RequeueAfter > 0 {
		a.RequeueAfter(otherResult.RequeueAfter)
	}
	return a.Result()
}

// Result returns the aggregated result and error.
func (a *Builder) Result() (reconcile.Result, error) {
	return a.result, a.firstErr
}
