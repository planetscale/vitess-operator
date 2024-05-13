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

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// VitessBackupSchedule is the Schema for the cronjobs API
type VitessBackupSchedule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VitessBackupScheduleSpec   `json:"spec,omitempty"`
	Status VitessBackupScheduleStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// VitessBackupScheduleList contains a list of VitessBackupSchedule
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

	// TODO: add non-user-aware fields below (image, etc...)

	// // +kubebuilder:validation:Minimum=0
	// // Optional deadlines in seconds for starting the job if it misses scheduled
	// // time for any reason. Missed jobs executions will be counted as failed ones.
	// // +optional
	// StartingDeadlineSeconds *int64 `json:"startingDeadlineSeconds,omitempty"`
	//
	// // Specifies ho to treat concurrent executions of a Job.
	// // Valid values are:
	// // - "Allow" (default): allows CronJobs to run concurrently;
	// // - "Forbid": forbids concurrent runs, skipping next run if previous run hasn't finished yet;
	// // - "Replace": cancels currently running job and replaces it with a new one.
	// // +optional
	// ConcurrencyPolicy ConcurrencyPolicy `json:"concurrencyPolicy,omitempty"`
	//
	// // This flag tells the controller to suspend subsequent executions, it does not apply to already
	// // started executions. Defaults to false.
	// // +optional
	// Suspend *bool `json:"suspend,omitempty"`
	//
	// // Specifies the job that will be created when executing a VitessBackupSchedule.
	// JobTemplate v1beta1.JobTemplateSpec `json:"jobTemplate"`
	//
	// // +kubebuilder:validation:Minimum=0
	//
	// // The number of successful finished jobs to retain. This is a pointer to distinguish between
	// // explicit zero and not specified.
	// // +optional
	// SuccessfulJobsHistoryLimit *int32 `json:"successfulJobsHistoryLimit,omitempty"`
	//
	// // +kubebuilder:validation:Minimum=0
	//
	// // The number of failed finished jobs to retain. This is a pointer to distinguish between
	// // explicit zero and not specified.
	// // +optional
	// FailedJobsHistoryLimit *int32 `json:"failedJobsHistoryLimit,omitempty"`
}

type VitessBackupScheduleTemplate struct {
	// The schedule in Cron format, see https://en.wikipedia.org/wiki/Cron.
	// +kubebuilder:validation:MinLength=0
	Schedule string `json:"schedule"`

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
