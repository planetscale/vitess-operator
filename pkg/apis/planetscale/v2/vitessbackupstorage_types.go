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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.
//
// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VitessBackupStorage represents a storage location for Vitess backups.
// It provides access to metadata about Vitess backups inside Kubernetes by
// maintaining a set of VitessBackup objects that represent backups in the given
// storage location. One VitessBackupStorage represents a storage location
// defined at the VitessCluster level, so it provides access to metadata
// about backups stored in that location for any keyspace and any shard in that
// cluster.
// +kubebuilder:resource:path=vitessbackupstorages,shortName=vtbs
// +kubebuilder:subresource:status
type VitessBackupStorage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VitessBackupStorageSpec   `json:"spec,omitempty"`
	Status VitessBackupStorageStatus `json:"status,omitempty"`
}

// VitessBackupStorageSpec defines the desired state of VitessBackupStorage.
type VitessBackupStorageSpec struct {
	// Location specifies the Vitess parameters for connecting to the backup
	// storage location.
	Location VitessBackupLocation `json:"location"`
	// Subcontroller specifies any parameters needed for launching the VitessBackupStorage subcontroller pod.
	Subcontroller *VitessBackupSubcontrollerSpec `json:"subcontroller,omitempty"`
}

type VitessBackupSubcontrollerSpec struct {
	// ServiceAccountName specifies the ServiceAccount used to launch the VitessBackupStorage subcontroller pod in the
	// namespace of the VitessCluster. If empty (the default), the same account as the operator will be reused. If your
	// VitessCluster is in a different namespace than the operator, this account is unlikely to work.
	ServiceAccountName string `json:"serviceAccountName,omitempty"`
}

// VitessBackupLocation defines a location where Vitess backups can be stored.
type VitessBackupLocation struct {
	// Name is used to refer to this backup location from other parts of a
	// VitessCluster object.
	//
	// In particular, the backupLocationName field in each tablet pool within
	// each shard must match one of the names defined by this field.
	//
	// This name must be unique among all backup locations defined in a given
	// cluster. A backup location with an empty name defines the default
	// location used when a tablet pool does not specify a backupLocationName.
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=^[A-Za-z0-9]([A-Za-z0-9-_.]*[A-Za-z0-9])?$
	Name string `json:"name,omitempty"`
	// GCS specifies a backup location in Google Cloud Storage.
	GCS *GCSBackupLocation `json:"gcs,omitempty"`
	// S3 specifies a backup location in Amazon S3.
	S3 *S3BackupLocation `json:"s3,omitempty"`
	// Azblob specifies a backup location in Azure Blob Storage.
	Azblob *AzblobBackupLocation `json:"azblob,omitempty"`
	// Ceph specifies a backup location in Ceph S3.
	Ceph *CephBackupLocation `json:"ceph,omitempty"`
	// Volume specifies a backup location as a Kubernetes Volume Source to mount.
	// This can be used, for example, to store backups on an NFS mount, or on
	// a shared host path for local testing.
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:pruning:PreserveUnknownFields
	Volume *corev1.VolumeSource `json:"volume,omitempty"`
	// VolumeSubPath gives the subpath in the volume to mount to the backups target.
	// Only used for Volume-backed backup storage, ignored otherwise.
	VolumeSubPath string `json:"volumeSubPath,omitempty"`
	// Annotations can optionally be used to attach custom annotations to Pods
	// that need access to this backup storage location.
	Annotations map[string]string `json:"annotations,omitempty"`
}

// GCSBackupLocation specifies a backup location in Google Cloud Storage.
type GCSBackupLocation struct {
	// Bucket is the name of the GCS bucket to use.
	// +kubebuilder:validation:MinLength=1
	Bucket string `json:"bucket"`
	// KeyPrefix is an optional prefix added to all object keys created by Vitess.
	// This is only needed if the same bucket is also used for something other
	// than backups for VitessClusters. Backups from different clusters,
	// keyspaces, or shards will automatically avoid colliding with each other
	// within a bucket, regardless of this setting.
	// +kubebuilder:validation:Pattern=^[^\r\n]*$
	// +kubebuilder:validation:MaxLength=256
	KeyPrefix string `json:"keyPrefix,omitempty"`
	// AuthSecret is a reference to the Secret to use for GCS authentication.
	// If set, this must point to a file in the format expected for the
	// GOOGLE_APPLICATION_CREDENTIALS environment variable.
	// Default: Use the default credentials of the Node.
	AuthSecret *SecretSource `json:"authSecret,omitempty"`
}

// S3BackupLocation specifies a backup location in Amazon S3.
type S3BackupLocation struct {
	// Region is the AWS region in which the bucket is located.
	// +kubebuilder:validation:MinLength=1
	Region string `json:"region"`
	// Bucket is the name of the S3 bucket to use.
	// +kubebuilder:validation:MinLength=1
	Bucket string `json:"bucket"`
	// Endpoint is the `host:port` (port is required) for the S3 backend.
	// Default: Use the endpoint associated with `region` by the driver.
	Endpoint string `json:"endpoint,omitempty"`
	// ForcePathStyle is an optional param to force connection using <endpoint>/<bucket>
	// Default: false By default the s3 client will try to connect to <bucket>.<endpoint>.
	ForcePathStyle bool `json:"forcePathStyle,omitempty"`
	// KeyPrefix is an optional prefix added to all object keys created by Vitess.
	// This is only needed if the same bucket is also used for something other
	// than backups for VitessClusters. Backups from different clusters,
	// keyspaces, or shards will automatically avoid colliding with each other
	// within a bucket, regardless of this setting.
	// +kubebuilder:validation:Pattern=^[^\r\n]*$
	// +kubebuilder:validation:MaxLength=256
	KeyPrefix string `json:"keyPrefix,omitempty"`
	// AuthSecret is a reference to the Secret to use for S3 authentication.
	// If set, this must point to a file in the format expected for the
	// `~/.aws/credentials` file.
	// Default: Use the default credentials of the Node.
	AuthSecret *SecretSource `json:"authSecret,omitempty"`
}

// AzblobBackupLocation specifies a backup location in Azure Blob Storage.
type AzblobBackupLocation struct {
	// Account is the name of the Azure storage account to use.
	// +kubebuilder:validation:MinLength=1
	Account string `json:"account"`
	// Container is the name of the Azure storage account container to use.
	// +kubebuilder:validation:MinLength=1
	Container string `json:"container"`
	// KeyPrefix is an optional prefix added to all object keys created by Vitess.
	// This is only needed if the same container is also used for something other
	// than backups for VitessClusters. Backups from different clusters,
	// keyspaces, or shards will automatically avoid colliding with each other
	// within a container, regardless of this setting.
	// +kubebuilder:validation:Pattern=^[^\r\n]*$
	// +kubebuilder:validation:MaxLength=256
	KeyPrefix string `json:"keyPrefix,omitempty"`
	// AuthSecret is a reference to the Secret to use for Azure authentication.
	AuthSecret SecretSource `json:"authSecret"`
}

// CephBackupLocation specifies a backup location in Ceph S3.
type CephBackupLocation struct {
	// AuthSecret is a reference to the Secret to use for Ceph S3 authentication.
	// If set, this must point to a file in the format expected for the
	// `https://github.com/vitessio/vitess/blob/master/examples/local/ceph_backup_config.json` file.
	AuthSecret SecretSource `json:"authSecret"`
}

// VitessBackupStorageStatus defines the observed state of VitessBackupStorage.
type VitessBackupStorageStatus struct {
	// The generation observed by the controller.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// TotalBackupCount is the total number of backups found in this storage
	// location, across all keyspaces and shards.
	TotalBackupCount int32 `json:"totalBackupCount,omitempty"`
}

// NewVitessBackupStorageStatus creates a new status with default values.
func NewVitessBackupStorageStatus() *VitessBackupStorageStatus {
	return &VitessBackupStorageStatus{}
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VitessBackupStorageList contains a list of VitessBackupStorages.
type VitessBackupStorageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VitessBackupStorage `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VitessBackupStorage{}, &VitessBackupStorageList{})
}
