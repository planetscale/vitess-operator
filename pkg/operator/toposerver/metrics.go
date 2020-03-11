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

package toposerver

import (
	"github.com/prometheus/client_golang/prometheus"
	"planetscale.dev/vitess-operator/pkg/operator/metrics"
)

const (
	subsystemName = "toposerver"

	connStateLabel  = "conn_state"
	connStateActive = "active"
	connStateDead   = "dead"

	reasonLabel = "reason"
	reasonIdle  = "idle"
	reasonDead  = "dead"
)

var (
	connCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metrics.Namespace,
		Subsystem: subsystemName,
		Name:      "conn_count",
		Help:      "Number of connections in the topology server connection cache",
	}, []string{connStateLabel})
	connRefCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metrics.Namespace,
		Subsystem: subsystemName,
		Name:      "conn_ref_count",
		Help:      "Number of outstanding references to a cached connection",
	}, []string{connStateLabel})

	cacheHits = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: metrics.Namespace,
		Subsystem: subsystemName,
		Name:      "cache_hits",
		Help:      "Requests for a topology server connection served from the cache",
	})
	cacheMisses = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: metrics.Namespace,
		Subsystem: subsystemName,
		Name:      "cache_misses",
		Help:      "Requests for a topology server connection that missed the cache",
	})
	openLatency = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: metrics.Namespace,
		Subsystem: subsystemName,
		Name:      "open_latency_seconds",
		Help:      "Time spent trying to open a connection, possibly returned from the cache",
	})

	connectSuccesses = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: metrics.Namespace,
		Subsystem: subsystemName,
		Name:      "connect_successes",
		Help:      "Successful attempts to make a new topology server connection",
	})
	connectErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: metrics.Namespace,
		Subsystem: subsystemName,
		Name:      "connect_errors",
		Help:      "Failed attempts to make a new topology server connection",
	})
	connectLatency = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: metrics.Namespace,
		Subsystem: subsystemName,
		Name:      "connect_latency_seconds",
		Help:      "Time spent trying to make a new connection when one was not cached",
	})

	checkSuccesses = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: metrics.Namespace,
		Subsystem: subsystemName,
		Name:      "check_successes",
		Help:      "Successful liveness checks on cached connections",
	})
	checkErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: metrics.Namespace,
		Subsystem: subsystemName,
		Name:      "check_errors",
		Help:      "Failed liveness checks on cached connections",
	})

	disconnects = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metrics.Namespace,
		Subsystem: subsystemName,
		Name:      "disconnects",
		Help:      "Closed connections",
	}, []string{reasonLabel})
)

func init() {
	metrics.Registry.MustRegister(
		connCount,
		connRefCount,
		cacheHits,
		cacheMisses,
		openLatency,
		connectSuccesses,
		connectErrors,
		connectLatency,
		checkSuccesses,
		checkErrors,
		disconnects,
	)
}
