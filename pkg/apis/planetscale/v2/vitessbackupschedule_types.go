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

package v2

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const SupportedBackupFrequencyExamples = "1m, 5m, 15m, 30m, 1h, 2h, 4h, 6h, 12h, 24h"

// BackupScope defines the scope at which a backup strategy operates.
// +kubebuilder:validation:Enum=Shard;Keyspace;Cluster
type BackupScope string

const (
	// BackupScopeShard targets a single specific shard (default, original behavior).
	BackupScopeShard BackupScope = "Shard"
	// BackupScopeKeyspace dynamically discovers all shards in the specified keyspace.
	BackupScopeKeyspace BackupScope = "Keyspace"
	// BackupScopeCluster dynamically discovers all shards across all keyspaces in the cluster.
	BackupScopeCluster BackupScope = "Cluster"
)

// BackupMethod defines the method used to take scheduled backups.
// +kubebuilder:validation:Enum=vtbackup;vtctldclient
type BackupMethod string

const (
	// BackupMethodVtbackup uses vtbackup to take backups. A Kubernetes Job runs
	// a vtbackup pod (with a PVC) that starts mysqld locally, restores the latest
	// backup, catches up on replication, and then uploads a new backup.
	BackupMethodVtbackup BackupMethod = "vtbackup"

	// BackupMethodVtctldclient uses vtctldclient to take backups. A lightweight
	// Kubernetes Job sends a BackupShard command to vtctld, which tells a running
	// serving replica to take the backup. No PVC is needed.
	BackupMethodVtctldclient BackupMethod = "vtctldclient"
)

// ConcurrencyPolicy describes how the concurrency of new jobs created by VitessBackupSchedule
// is handled, the default is set to AllowConcurrent.
// +kubebuilder:validation:Enum=Allow;Forbid
type ConcurrencyPolicy string

const (
	// AllowConcurrent allows CronJobs to run concurrently.
	AllowConcurrent ConcurrencyPolicy = "Allow"

	// ForbidConcurrent forbids concurrent runs, skipping next run if previous hasn't finished yet.
	ForbidConcurrent ConcurrencyPolicy = "Forbid"
)

// VitessBackupSchedule is the Schema for the VitessBackupSchedule API.
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type VitessBackupSchedule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VitessBackupScheduleSpec   `json:"spec,omitempty"`
	Status VitessBackupScheduleStatus `json:"status,omitempty"`
}

// VitessBackupScheduleList contains a list of VitessBackupSchedule.
// +kubebuilder:object:root=true
type VitessBackupScheduleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VitessBackupSchedule `json:"items"`
}

// VitessBackupScheduleSpec defines the desired state of VitessBackupSchedule.
type VitessBackupScheduleSpec struct {
	// VitessBackupScheduleTemplate contains the user-specific parts of VitessBackupScheduleSpec.
	// These are the parts that are configurable through the VitessCluster CRD.
	VitessBackupScheduleTemplate `json:",inline"`

	// Cluster on which this schedule runs.
	Cluster string `json:"cluster"`

	// Image is the container image used by vtctldclient backup jobs.
	// The controller re-uses the vtctld image by default.
	// This field is only used when backupMethod is set to "vtctldclient".
	Image string `json:"image,omitempty"`

	// ImagePullPolicy defines the policy to pull the Docker image in the job's pod.
	// The PullPolicy used will be the same as the one used to pull the vtctld image.
	// This field is only used when backupMethod is set to "vtctldclient".
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`
}

// VitessBackupScheduleTemplate contains all the user-specific fields that the user will be
// able to define when writing their YAML file.
type VitessBackupScheduleTemplate struct {
	// Name is the schedule name, this name must be unique across all the different VitessBackupSchedule
	// objects in the cluster.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:Pattern=^[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?$
	// +kubebuilder:example="every-day"
	Name string `json:"name"`

	// The schedule in Cron format, see https://en.wikipedia.org/wiki/Cron.
	// Mutually exclusive with Frequency.
	// +optional
	// +kubebuilder:validation:MinLength=0
	// +kubebuilder:example="0 0 * * *"
	Schedule string `json:"schedule,omitempty"`

	// Frequency is a Go duration string that defines how often backups should run.
	// Since schedules are executed via cron, only frequencies that can be represented exactly
	// in cron are supported. Examples include 1m, 5m, 15m, 30m, 1h, 2h, 4h, 6h, 12h, and 24h.
	// When set, the controller generates deterministic per-shard cron schedules staggered
	// across the interval to avoid bandwidth spikes.
	// Mutually exclusive with Schedule.
	// +optional
	Frequency string `json:"frequency,omitempty"`

	// BackupMethod defines the method used to take scheduled backups.
	// "vtbackup" (default) runs a dedicated vtbackup pod with a local mysqld that
	// restores from the latest backup, catches up on replication, and takes a new backup.
	// "vtctldclient" sends a BackupShard command to vtctld, which tells a running serving
	// replica to take the backup directly. No PVC is needed for vtctldclient.
	// +optional
	// +kubebuilder:default="vtbackup"
	BackupMethod BackupMethod `json:"backupMethod,omitempty"`

	// Strategy defines how we are going to take a backup.
	// If you want to take several backups within the same schedule you can add more items
	// to the Strategy list. Each VitessBackupScheduleStrategy will be executed within different
	// kubernetes jobs. This is useful if you want to have a single schedule backing up multiple shards
	// at the same time.
	// +kubebuilder:validation:MinItems=1
	// +patchMergeKey=name
	// +patchStrategy=merge
	Strategy []VitessBackupScheduleStrategy `json:"strategies"`

	// Resources specify the compute resources to allocate for every Jobs's pod.
	Resources corev1.ResourceRequirements `json:"resources"`

	// SuccessfulJobsHistoryLimit defines how many successful jobs will be kept around.
	// +optional
	// +kubebuilder:validation:Minimum=0
	SuccessfulJobsHistoryLimit *int32 `json:"successfulJobsHistoryLimit,omitempty"`

	// FailedJobsHistoryLimit defines how many failed jobs will be kept around.
	// +optional
	// +kubebuilder:validation:Minimum=0
	FailedJobsHistoryLimit *int32 `json:"failedJobsHistoryLimit,omitempty"`

	// Suspend pause the associated backup schedule. Pausing any further scheduled
	// runs until Suspend is set to false again. This is useful if you want to pause backup without
	// having to remove the entire VitessBackupSchedule object from the cluster.
	// +optional
	Suspend *bool `json:"suspend,omitempty"`

	// StartingDeadlineSeconds enables the VitessBackupSchedule to start a job even though it is late by
	// the given amount of seconds. Let's say for some reason the controller process a schedule run on
	// second after its scheduled time, if StartingDeadlineSeconds is set to 0, the job will be skipped
	// as it's too late, but on the other hand, if StartingDeadlineSeconds is greater than one second,
	// the job will be processed as usual.
	// +optional
	// +kubebuilder:validation:Minimum=0
	StartingDeadlineSeconds *int64 `json:"startingDeadlineSeconds,omitempty"`

	// ConcurrencyPolicy specifies ho to treat concurrent executions of a Job.
	// Valid values are:
	// - "Allow": allows CronJobs to run concurrently;
	// - "Forbid" (default): forbids concurrent runs, skipping next run if previous run hasn't finished yet;
	// - "Replace": cancels currently running job and replaces it with a new one.
	// +optional
	// +kubebuilder:example="Forbid"
	// +kubebuilder:default="Forbid"
	ConcurrencyPolicy ConcurrencyPolicy `json:"concurrencyPolicy,omitempty"`

	// AllowedMissedRuns defines how many missed run of the schedule will be allowed before giving up on finding the last job.
	// If the operator's clock is skewed and we end-up missing a certain number of jobs, finding the last
	// job might be very time-consuming, depending on the frequency of the schedule and the duration during which
	// the operator's clock was misbehaving. Also depending on how laggy the clock is, we can end-up with thousands
	// of missed runs. For this reason, AllowedMissedRun, which is set to 100 by default, will short circuit the search
	// and simply wait for the next job on the schedule.
	// Unless you are experiencing issue with missed runs due to a misconfiguration of the clock, we recommend leaving
	// this field to its default value.
	// +optional
	// +kubebuilder:validation:Minimum=0
	AllowedMissedRuns *int `json:"allowedMissedRun,omitempty"`

	// JobTimeoutMinutes defines after how many minutes a job that has not yet finished should be stopped and removed.
	// Default value is 10 minutes.
	// +optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=10
	JobTimeoutMinutes int32 `json:"jobTimeoutMinute,omitempty"`

	// Annotations are the set of annotations that will be attached to the pods created by VitessBackupSchedule.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// Affinity allows you to set rules that constrain the scheduling of the pods that take backups.
	// WARNING: These affinity rules will override all default affinities that we set; in turn, we can't
	// guarantee optimal scheduling of your pods if you choose to set this field.
	// +optional
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:pruning:PreserveUnknownFields
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// Tolerations allow you to schedule backup pods onto nodes with matching taints.
	// If omitted, the controller uses the default tolerations for the selected backup method.
	// To explicitly clear inherited tolerations, set this field to an empty list.
	// +optional
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:pruning:PreserveUnknownFields
	Tolerations *[]corev1.Toleration `json:"tolerations,omitempty"`
}

// VitessBackupScheduleStrategy defines how we are going to take a backup.
// The VitessBackupSchedule controller uses this data to build either a vtbackup
// pod or a vtctldclient command, depending on the configured BackupMethod.
type VitessBackupScheduleStrategy struct {
	// Name of the backup strategy.
	Name string `json:"name"`

	// Scope defines whether this strategy targets a single Shard, all shards in a Keyspace,
	// or all shards in the Cluster. Defaults to "Shard" for backward compatibility.
	// +optional
	Scope BackupScope `json:"scope,omitempty"`

	// Keyspace defines the keyspace on which we want to take the backup.
	// Required for Shard and Keyspace scopes.
	// +optional
	// +kubebuilder:example="commerce"
	Keyspace string `json:"keyspace,omitempty"`

	// Shard defines the shard on which we want to take a backup.
	// Required only for Shard scope.
	// +optional
	// +kubebuilder:example="-"
	Shard string `json:"shard,omitempty"`

	// ExtraFlags is a map of additional flags passed to vtctldclient's BackupShard command.
	// This field is only used when backupMethod is "vtctldclient"; it is ignored for "vtbackup".
	// +optional
	ExtraFlags map[string]string `json:"extraFlags,omitempty"`
}

// VitessBackupScheduleStatus defines the observed state of VitessBackupSchedule
type VitessBackupScheduleStatus struct {
	// A list of pointers to currently running jobs.
	// This field is deprecated and no longer used in versions >= v2.15. It will be removed in a future release.
	// +optional
	Active []corev1.ObjectReference `json:"active,omitempty"`

	// Information when was the last time the job was successfully scheduled.
	// This field is deprecated and no longer used in versions >= v2.15. It will be removed in a future release.
	// Please use lastScheduledTimes instead which maps the last schedule time to each VitessBackupScheduleStrategy
	// in the VitessBackupSchedule.
	// +optional
	LastScheduledTime *metav1.Time `json:"lastScheduledTime,omitempty"`

	// A list of the last schedule we executed for each VitessBackupScheduleStrategy.
	// Note that these are not the times when the last execution started, only the scheduled times.
	// +optional
	LastScheduledTimes map[string]*metav1.Time `json:"lastScheduledTimes,omitempty"`

	// GeneratedSchedules maps expanded strategy names to their generated cron expressions.
	// This is populated when Frequency is used instead of Schedule, providing observability
	// into the deterministic per-shard cron schedules.
	// +optional
	GeneratedSchedules map[string]string `json:"generatedSchedules,omitempty"`

	// NextScheduledTimes maps expanded strategy names to the next scheduled execution time.
	// This is populated for both cron-based and frequency-based schedules.
	// +optional
	NextScheduledTimes map[string]*metav1.Time `json:"nextScheduledTimes,omitempty"`
}

// NewVitessBackupScheduleStatus creates a new status with default values.
func NewVitessBackupScheduleStatus(status VitessBackupScheduleStatus) VitessBackupScheduleStatus {
	newStatus := VitessBackupScheduleStatus{
		LastScheduledTimes: status.LastScheduledTimes,
		GeneratedSchedules: status.GeneratedSchedules,
		NextScheduledTimes: status.NextScheduledTimes,
	}
	if status.LastScheduledTimes == nil {
		newStatus.LastScheduledTimes = make(map[string]*metav1.Time)
	}
	if status.GeneratedSchedules == nil {
		newStatus.GeneratedSchedules = make(map[string]string)
	}
	if status.NextScheduledTimes == nil {
		newStatus.NextScheduledTimes = make(map[string]*metav1.Time)
	}
	return newStatus
}

func init() {
	SchemeBuilder.Register(&VitessBackupSchedule{}, &VitessBackupScheduleList{})
}
