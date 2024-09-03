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
Package resync helps controllers specify custom resync behavior,
which is important when they need to check state that lives outside
the Kubernetes API server, such as in an app-specific system.

The controller-runtime library supports a form of resync, but you have to use
the same setting for all controllers in the controller manager, and it tends
to dump everything in the queue at once, which is spiky.

This package allows each controller to set its own resync period, and tries to
spread out the requeuing of objects over time.
*/
package resync

import (
	"container/heap"
	"math/rand"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// Periodic will trigger periodic resyncs of objects.
type Periodic struct {
	name   string
	period time.Duration

	// trigger is the channel we send to when we want to trigger a reconcile.
	trigger chan event.GenericEvent
	// enqueue is the channel that the controller uses (via Enqueue()) to send
	// us objects to enqueue for periodic resync.
	enqueue chan client.ObjectKey
}

// NewPeriodic returns a new periodic resync.
func NewPeriodic(name string, period time.Duration) *Periodic {
	p := &Periodic{
		name:    name,
		period:  period,
		trigger: make(chan event.GenericEvent),
		enqueue: make(chan client.ObjectKey),
	}
	go p.run()
	return p
}

// WatchSource returns the source.Source that can be passed to Controller.Watch()
// to plug this resync into the controller.
func (p *Periodic) WatchSource() source.Source {
	return source.Channel(p.trigger, &handler.EnqueueRequestForObject{})
}

// Enqueue adds the given object to the periodic resync queue.
// If the object was already in the queue, this resets the timer on it since
// it's assumed that this is called after a reconcile pass has just finished.
func (p *Periodic) Enqueue(objKey client.ObjectKey) {
	p.enqueue <- objKey
}

func (p *Periodic) run() {
	// Maintain a priority queue of timers in expiration order.
	queue := timerQueue{}
	// Also remember how to map from ObjectKey to timer.
	objTimers := make(map[client.ObjectKey]*timer)

	var tick <-chan time.Time
	var nextTimer *time.Timer

	for {
		periodicResyncQueueSize.WithLabelValues(p.name).Set(float64(len(queue)))

		select {
		case objKey := <-p.enqueue:
			// Stop the timer, if there was one.
			if nextTimer != nil && !nextTimer.Stop() {
				// We need to drain the channel since we didn't stop it in time.
				<-nextTimer.C
			}

			// Update or add the queue entry for this object.
			// Add some jitter (+/-25%) to spread out initial spikes.
			delay := time.Duration(float64(p.period) * (0.75 + 0.5*rand.Float64()))
			expiration := time.Now().Add(delay)

			if t := objTimers[objKey]; t != nil {
				t.expiration = expiration
				heap.Fix(&queue, t.index)
			} else {
				t := &timer{objectKey: objKey, expiration: expiration}
				heap.Push(&queue, t)
				objTimers[objKey] = t
			}
		case <-tick:
		}

		// Trigger reconciles for any expired timers.
		now := time.Now()
		for len(queue) > 0 && now.After(queue[0].expiration) {
			objKey := queue[0].objectKey
			heap.Pop(&queue)
			delete(objTimers, objKey)

			// GenericEvent takes a real client.Object, so need to give it one
			// handler.EnqueueRequestForObject will only grab NamespacedName off of it though
			obj := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Namespace: objKey.Namespace, Name: objKey.Name},
			}

			p.trigger <- event.GenericEvent{
				Object: obj,
			}
		}

		if len(queue) == 0 {
			// Go to sleep until something is enqueued.
			nextTimer = nil
			tick = nil
			continue
		}

		// Set a timer to wait for the next soonest expiration.
		duration := queue[0].expiration.Sub(now)
		if nextTimer == nil {
			nextTimer = time.NewTimer(duration)
			tick = nextTimer.C
		} else {
			nextTimer.Reset(duration)
		}
	}
}

type timer struct {
	objectKey  client.ObjectKey
	expiration time.Time

	// index is needed by update and is maintained by the heap.Interface methods.
	index int
}

// timerQueue is pretty much a copy of the PriorityQueue example from the
// "container/heap" package docs.
type timerQueue []*timer

func (q timerQueue) Len() int { return len(q) }

func (q timerQueue) Less(i, j int) bool {
	return q[i].expiration.Before(q[j].expiration)
}

func (q timerQueue) Swap(i, j int) {
	q[i], q[j] = q[j], q[i]
	q[i].index = i
	q[j].index = j
}

func (q *timerQueue) Push(x interface{}) {
	n := len(*q)
	item := x.(*timer)
	item.index = n
	*q = append(*q, item)
}

func (q *timerQueue) Pop() interface{} {
	old := *q
	n := len(old)
	item := old[n-1]
	item.index = -1 // for safety
	*q = old[0 : n-1]
	return item
}
