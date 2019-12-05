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

package reconciler

import (
	"planetscale.dev/vitess-operator/pkg/operator/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	metricsSubsystemName = "reconciler"
)

var (
	reconcileCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metrics.Namespace,
		Subsystem: metricsSubsystemName,
		Name:      "reconcile_count",
		Help:      "Reconciliation attempts for a Kind",
	}, kindMetricLabels)

	createCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metrics.Namespace,
		Subsystem: metricsSubsystemName,
		Name:      "create_count",
		Help:      "Attempts to create an object of a given Kind",
	}, kindMetricLabels)

	updateCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metrics.Namespace,
		Subsystem: metricsSubsystemName,
		Name:      "update_count",
		Help:      "Attempts to update an object of a given Kind in-place",
	}, kindMetricLabels)

	deleteCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metrics.Namespace,
		Subsystem: metricsSubsystemName,
		Name:      "delete_count",
		Help:      "Attempts to delete an object of a given Kind",
	}, kindMetricLabels)

	evictedPodCount = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: metrics.Namespace,
		Subsystem: metricsSubsystemName,
		Name:      "evicted_pod_count",
		Help:      "Number of evicted Pods that had to be cleaned up",
	})
)

func init() {
	metrics.Registry.MustRegister(
		reconcileCount,
		createCount,
		updateCount,
		deleteCount,
		evictedPodCount,
	)
}

var kindMetricLabels = []string{
	"api_group",
	"api_version",
	"kind",
	"owner_api_group",
	"owner_api_version",
	"owner_kind",
	metrics.ResultLabel,
}

func metricLabels(gvk, ownerGVK schema.GroupVersionKind, err error) prometheus.Labels {
	return prometheus.Labels{
		"api_group":         gvk.Group,
		"api_version":       gvk.Version,
		"kind":              gvk.Kind,
		"owner_api_group":   ownerGVK.Group,
		"owner_api_version": ownerGVK.Version,
		"owner_kind":        ownerGVK.Kind,
		metrics.ResultLabel: metrics.Result(err),
	}
}
