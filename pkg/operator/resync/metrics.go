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

package resync

import (
	"planetscale.dev/vitess-operator/pkg/operator/metrics"
	"github.com/prometheus/client_golang/prometheus"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
)

const (
	metricsSubsystemName = "resync"
)

var (
	periodicResyncQueueSize = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metrics.Namespace,
		Subsystem: metricsSubsystemName,
		Name:      "periodic_queue_size",
		Help:      "Number of items queued for periodic resync",
	}, []string{"name"})
)

func init() {
	metrics.Registry.MustRegister(
		periodicResyncQueueSize,
	)
}

func metricLabels(vts *planetscalev2.VitessShard, err error) []string {
	return []string{
		vts.Labels[planetscalev2.ClusterLabel],
		vts.Labels[planetscalev2.KeyspaceLabel],
		vts.Spec.Name,
		metrics.Result(err),
	}
}
