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

// ConcurrencyPolicy describes how the concurrency of new jobs created by VitessBackupSchedule
// is handled, the default is set to AllowConcurrent.
// +kubebuilder:validation:Enum=Allow;Forbid;Replace
type ConcurrencyPolicy string

const (
	// AllowConcurrent allows CronJobs to run concurrently.
	AllowConcurrent ConcurrencyPolicy = "Allow"

	// ForbidConcurrent forbids concurrent runs, skipping next run if previous hasn't finished yet.
	ForbidConcurrent ConcurrencyPolicy = "Forbid"

	// ReplaceConcurrent cancels currently running job and replaces it with a new one.
	ReplaceConcurrent ConcurrencyPolicy = "Replace"
)

// BackupStrategyName describes the vtctldclient command that will be used to take a backup.
// When scheduling a backup, you must specify at least one strategy.
// +kubebuilder:validation:Enum=BackupTablet;BackupShard
type BackupStrategyName string

const (
	// BackupTablet will use the "vtctldclient Backup" command to take a backup
	BackupTablet BackupStrategyName = "BackupTablet"

	// BackupShard will use the "vtctldclient BackupShard" command to take a backup
	BackupShard BackupStrategyName = "BackupShard"
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

	// Image should be any image that already contains vtctldclient installed.
	// The controller will re-use the vtctld image by default.
	Image string `json:"image,omitempty"`

	// ImagePullPolicy defines the policy to pull the Docker image in the job's pod.
	// The PullPolicy used will be the same as the one used to pull the vtctld image.
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`
}

// VitessBackupScheduleTemplate contains all the user-specific fields that the user will be
// able to define when writing their YAML file.
type VitessBackupScheduleTemplate struct {
	// Name is the schedule name, this name must be unique across all the different VitessBackupSchedule
	// objects in the cluster.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:Pattern=^[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?$
	// +kubebuilder:example="every-minute"
	Name string `json:"name"`

	// The schedule in Cron format, see https://en.wikipedia.org/wiki/Cron.
	// +kubebuilder:validation:MinLength=0
	// +kubebuilder:example="* * * * *"
	Schedule string `json:"schedule"`

	// Strategy defines how we are going to take a backup.
	// If you want to take several backups within the same schedule you can add more items
	// to the Strategy list. Each VitessBackupScheduleStrategy will be executed by the same
	// kubernetes job. This is useful if for instance you have one schedule, and you want to
	// take a backup of all shards in a keyspace and don't want to re-create a second schedule.
	// All the VitessBackupScheduleStrategy are concatenated into a single shell command that
	// is executed when the Job's container starts.
	// +kubebuilder:validation:MinItems=1
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

	// Suspend enables the suspension of the VitessBackupSchedule, pausing any further scheduled
	// runs until Suspend is set to false again. This is useful if you want to pause backup without
	// having to remove the entire VitessBackupSchedule object from the cluster.
	// +optional
	Suspend *bool `json:"suspend,omitempty"`

	// StartingDeadlineSeconds allows for the VitessBackupSchedule controller to start jobs that late
	// by the given amount of seconds.
	// +optional
	// +kubebuilder:validation:Minimum=0
	StartingDeadlineSeconds *int64 `json:"startingDeadlineSeconds,omitempty"`

	// ConcurrencyPolicy specifies ho to treat concurrent executions of a Job.
	// Valid values are:
	// - "Allow" (default): allows CronJobs to run concurrently;
	// - "Forbid": forbids concurrent runs, skipping next run if previous run hasn't finished yet;
	// - "Replace": cancels currently running job and replaces it with a new one.
	// +optional
	// +kubebuilder:example="Forbid"
	ConcurrencyPolicy ConcurrencyPolicy `json:"concurrencyPolicy,omitempty"`

	// AllowedMissedRun defines how many missed run of the schedule will be allowed before giving up on finding the last job.
	// If the operator's clocked is skewed and we end-up missing a certain number of jobs, finding the last
	// job might be very consuming, depending on the frequency of the schedule and the duration during which
	// the operator's clock was misbehaving, also depending on how laggy the clock is, we can end-up with thousands
	// or missed runs. For this reason, AllowedMissedRun, which is set to 100 by default, will allow us to give up finding
	// the last job and simply wait for the next job on the schedule.
	// Unless you are experiencing issue with missed runs due to a misconfiguration of the clock, we recommend leaving
	// this field to its default value.
	// +optional
	// +kubebuilder:validation:Minimum=0
	AllowedMissedRun *int `json:"allowedMissedRun,omitempty"`

	// JobTimeoutMinute defines after how many minutes a job that has not yet finished should be stopped and removed.
	// +optional
	// +kubebuilder:validation:Minimum=0
	JobTimeoutMinute *int32 `json:"jobTimeoutMinute,omitempty"`
}

// VitessBackupScheduleStrategy defines how we are going to take a backup.
// The VitessBackupSchedule controller uses this data to build the vtctldclient
// command line that will be executed in the Job's pod.
type VitessBackupScheduleStrategy struct {
	// Name of the backup strategy.
	Name BackupStrategyName `json:"name"`

	// KeyspaceShard defines the keyspace and shard on which we want to take a backup.
	// It has to be formatted as follows: <keyspace>/<shard>
	// This field is mandatory if we have picked the strategy BackupShard.
	// This field will be ignored if we have picked the strategy BackupTablet.
	// +optional
	// +kubebuilder:example="commerce/-"
	KeyspaceShard string `json:"keyspaceShard,omitempty"`

	// TabletAlias is the tablet alias we want to take a backup on.
	// This field is mandatory if we have picked the strategy BackupTablet.
	// This field will be ignored if we have picked the strategy BackupShard.
	// +optional
	// +kubebuilder:example="zone1-0000000102"
	TabletAlias string `json:"tabletAlias"`

	// UpgradeSafe indicates if the backup should be taken with innodb_fast_shutdown=0
	// so that it's a backup that can be used for an upgrade.
	// This will use the flag "--upgrade-safe=true" when calling vtctldclient.
	// +optional
	UpgradeSafe bool `json:"upgradeSafe,omitempty"`

	// AllowPrimary allows the backup to occur on a PRIMARY tablet.
	// This will use the flag "--allow_primary=true" when calling vtctldclient.
	// +optional
	AllowPrimary bool `json:"allowPrimary,omitempty"`
}

// VitessBackupScheduleStatus defines the observed state of VitessBackupSchedule
type VitessBackupScheduleStatus struct {
	// A list of pointers to currently running jobs.
	// +optional
	Active []corev1.ObjectReference `json:"active,omitempty"`

	// Information when was the last time the job was successfully scheduled.
	// +optional
	LastScheduledTime *metav1.Time `json:"lastScheduledTime,omitempty"`
}

func init() {
	SchemeBuilder.Register(&VitessBackupSchedule{}, &VitessBackupScheduleList{})
}
