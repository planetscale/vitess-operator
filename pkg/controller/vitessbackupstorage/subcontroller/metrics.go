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

package subcontroller

import (
	"github.com/prometheus/client_golang/prometheus"

	"planetscale.dev/vitess-operator/pkg/operator/metrics"
)

const (
	metricsSubsystemName = "backup_storage_subcontroller"
)

var (
	reconcileCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metrics.Namespace,
		Subsystem: metricsSubsystemName,
		Name:      "reconcile_count",
		Help:      "Reconciliation attempts for a VitessBackupStorage",
	}, []string{metrics.BackupStorageLabel, metrics.ResultLabel})

	// lastCompleteTimestamp uses Subsystem "backup" so the exported name is
	// vitess_operator_backup_last_complete_timestamp_seconds, matching the
	// agreed metric name rather than the subcontroller's usual subsystem.
	lastCompleteTimestamp = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metrics.Namespace,
		Subsystem: "backup",
		Name:      "last_complete_timestamp_seconds",
		Help:      "Unix timestamp of the latest complete backup observed for a shard and storage location. Zero if the shard has been inventoried but has no complete backup.",
	}, []string{metrics.ClusterLabel, metrics.KeyspaceLabel, metrics.ShardLabel, metrics.LocationLabel})
)

func init() {
	metrics.Registry.MustRegister(
		reconcileCount,
		lastCompleteTimestamp,
	)
}
