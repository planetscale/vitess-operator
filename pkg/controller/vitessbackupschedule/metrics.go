/*
Copyright 2024 PlanetScale Inc.

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

package vitessbackupschedule

import (
	"github.com/prometheus/client_golang/prometheus"

	"planetscale.dev/vitess-operator/pkg/operator/metrics"
)

const (
	metricsSubsystemName = "backup_schedule"
)

var (
	backupScheduleLabels = []string{
		metrics.BackupScheduleLabel,
		metrics.ResultLabel,
	}

	reconcileCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metrics.Namespace,
		Subsystem: metricsSubsystemName,
		Name:      "reconcile_count",
		Help:      "Reconciliation attempts for a VitessBackupSchedule",
	}, backupScheduleLabels)

	timeoutJobsCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metrics.Namespace,
		Subsystem: metricsSubsystemName,
		Name:      "timeout_jobs_removed_count",
		Help:      "Number of timed out jobs that were removed for a VitessBackupSchedule",
	}, backupScheduleLabels)
)

func init() {
	metrics.Registry.MustRegister(
		reconcileCount,
		timeoutJobsCount,
	)
}
