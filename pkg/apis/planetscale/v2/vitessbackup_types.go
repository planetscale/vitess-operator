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

package v2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.
//
// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VitessBackup is a one-way mirror of metadata for a Vitess backup.
// These objects are created automatically by the VitessBackupStorage controller
// to provide access to backup metadata from Kubernetes. Each backup found in
// the storage location will be represented by its own VitessBackup object.
// +k8s:openapi-gen=true
// +kubebuilder:resource:path=vitessbackups,shortName=vtb
type VitessBackup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VitessBackupSpec   `json:"spec,omitempty"`
	Status VitessBackupStatus `json:"status,omitempty"`
}

// VitessBackupSpec defines the desired state of the backup.
// +k8s:openapi-gen=true
type VitessBackupSpec struct {
}

// VitessBackupStatus describes the observed state of the backup.
// +k8s:openapi-gen=true
type VitessBackupStatus struct {
	// StartTime is the time when the backup started.
	StartTime metav1.Time `json:"startTime,omitempty"`
	// FinishedTime is the time when the backup finished.
	FinishedTime *metav1.Time `json:"finishedTime,omitempty"`
	// Complete indicates whether the backup ever completed.
	Complete bool `json:"complete,omitempty"`
	// Position is the replication position of the snapshot that was backed up.
	// The position is expressed in the native, GTID-based format of the MySQL
	// flavor that took the backup.
	// This is only available after the backup is complete.
	Position string `json:"position,omitempty"`
	// Engine is the Vitess backup engine implementation that was used.
	Engine string `json:"engine,omitempty"`
	// StorageDirectory is the name of the parent directory in storage that
	// contains this backup.
	StorageDirectory string `json:"storageDirectory,omitempty"`
	// StorageName is the name of the backup in storage. This is different from
	// the name of the VitessBackup object created to represent metadata about
	// the actual backup in storage.
	StorageName string `json:"storageName,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VitessBackupList contains a list of VitessBackups.
type VitessBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VitessBackup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VitessBackup{}, &VitessBackupList{})
}
