package v2

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConcurrencyPolicy describes how the job will be handled. Only one of the following concurrent
// policies may be specified. If none of the following policies is specified, the default one
// is AllowConcurrent.
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

// VitessBackupSchedule is the Schema for the cronjobs API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type VitessBackupSchedule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VitessBackupScheduleSpec   `json:"spec,omitempty"`
	Status VitessBackupScheduleStatus `json:"status,omitempty"`
}

// VitessBackupScheduleList contains a list of VitessBackupSchedule
// +kubebuilder:object:root=true
type VitessBackupScheduleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VitessBackupSchedule `json:"items"`
}

// VitessBackupScheduleSpec defines the desired state of VitessBackupSchedule
type VitessBackupScheduleSpec struct {
	// VitessBackupScheduleTemplate contains the user-specific parts of VitessBackupScheduleSpec.
	// These are the parts that are configurable through the VitessCluster CRD.
	// All remaining fields will be handled/filled by the controller.
	VitessBackupScheduleTemplate `json:",inline"`

	// Image should be any image that already contains vtctldclient installed.
	// The controller will re-use the vtctld image by default.
	Image string `json:"image,omitempty"`

	// ImagePullPolicy will be set by the controller to what is set for vtctld.
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`
}

type VitessBackupScheduleTemplate struct {
	// Name is the schedule name.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:Pattern=^[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?$
	Name string `json:"name"`

	// The schedule in Cron format, see https://en.wikipedia.org/wiki/Cron.
	// +kubebuilder:validation:MinLength=0
	Schedule string `json:"schedule"`

	// Strategy defines how we are going to take a backup.
	// There are two options:
	// 		- Using "vtctldclient Backup" for a tablet backup.
	//		- Using "vtctldclient BackupShard" for a shard backup.
	Strategy VitessBackupScheduleStrategy `json:"strategy"`

	// Resources specify the compute resources to allocate for the pod that backups Vitess.
	Resources corev1.ResourceRequirements `json:"resources"`

	// The number of successful finished jobs to retain. This is a pointer to distinguish between
	// explicit zero and not specified.
	// +optional
	// +kubebuilder:validation:Minimum=0
	SuccessfulJobsHistoryLimit *int32 `json:"successfulJobsHistoryLimit,omitempty"`

	// The number of failed finished jobs to retain. This is a pointer to distinguish between
	// explicit zero and not specified.
	// +optional
	// +kubebuilder:validation:Minimum=0
	FailedJobsHistoryLimit *int32 `json:"failedJobsHistoryLimit,omitempty"`

	// This flag tells the controller to suspend subsequent executions, it does not apply to already
	// started executions. Defaults to false.
	// +optional
	Suspend *bool `json:"suspend,omitempty"`

	// Optional deadlines in seconds for starting the job if it misses scheduled
	// time for any reason.
	// +optional
	// +kubebuilder:validation:Minimum=0
	StartingDeadlineSeconds *int64 `json:"startingDeadlineSeconds,omitempty"`

	// Specifies ho to treat concurrent executions of a Job.
	// Valid values are:
	// - "Allow" (default): allows CronJobs to run concurrently;
	// - "Forbid": forbids concurrent runs, skipping next run if previous run hasn't finished yet;
	// - "Replace": cancels currently running job and replaces it with a new one.
	// +optional
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
}

// VitessBackupScheduleStrategy defines how we are going to take a backup.
// There are two options:
//   - Using "vtctldclient Backup" for a tablet backup.
//   - Using "vtctldclient BackupShard" for a shard backup.
type VitessBackupScheduleStrategy struct {
	// BackupTablet defines whether we are going to take the backup using "vtctldclient Backup" or not.
	// This option allows backups of a single tablet at a time.
	// BackupTablet and BackupShard cannot be used at the same time in the same VitessBackupScheduleStrategy.
	// +optional
	BackupTablet *VitessBackupScheduleTablet `json:"backupTablet,omitempty"`

	// BackupShard defines whether we are going to take the backup using "vtctldclient BackupShard" or not.
	// This option allows backups of a single shard at a time.`
	// BackupShard and BackupTablet cannot be used at the same time in the same VitessBackupScheduleStrategy.
	// +optional
	BackupShard *VitessBackupScheduleShard `json:"backupShard,omitempty"`
}

// VitessBackupScheduleTablet uses "vtctldclient Backup" to take backups.
type VitessBackupScheduleTablet struct {
	// Tablet is the tablet alias we want to take a backup on.
	// +kubebuilder:validation:MinLength=1
	Tablet string `json:"tablet"`

	// UpgradeSafe indicates if the backup should be taken with innodb_fast_shutdown=0
	// so that it's a backup that can be used for an upgrade.
	// +optional
	UpgradeSafe bool `json:"upgrade_safe,omitempty"`
}

type VitessBackupScheduleShard struct {
	// Keyspace defines the keyspace in which the shard can be found.
	// +kubebuilder:validation:MinLength=1
	Keyspace string `json:"keyspace"`

	// Shard defines the shard we want to take a backup on.
	// +kubebuilder:validation:MinLength=1
	Shard string `json:"shard"`

	// UpgradeSafe indicates if the backup should be taken with innodb_fast_shutdown=0
	// so that it's a backup that can be used for an upgrade.
	// +optional
	UpgradeSafe bool `json:"upgrade_safe,omitempty"`

	// AllowPrimary allows the backup to occur on a PRIMARY tablet.
	// +optional
	AllowPrimary bool `json:"allow_primary,omitempty"`
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
