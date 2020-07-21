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
// +genclient

// EtcdLockserver runs an etcd cluster for use as a Vitess lockserver.
// Unlike etcd-operator, it uses static bootstrapping and PVCs, treating members
// as stateful rather the ephemeral. Bringing back existing members instead of
// creating new ones means etcd can recover from loss of quorum without data
// loss, which is important for Vitess because restoring from an etcd backup
// (resetting the lockserver to a point in the past) would violate the
// consistency model that Vitess expects of a lockserver.
// +kubebuilder:resource:path=etcdlockservers,shortName=etcdls
// +kubebuilder:subresource:status
type EtcdLockserver struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EtcdLockserverSpec   `json:"spec,omitempty"`
	Status EtcdLockserverStatus `json:"status,omitempty"`
}

// EtcdLockserverSpec defines the desired state of an EtcdLockserver.
type EtcdLockserverSpec struct {
	// EtcdLockserverTemplate contains the user-specified parts of EtcdLockserverSpec.
	// These are the parts that are configurable inside VitessCluster.
	// The rest of the fields below are filled in by the parent controller.
	EtcdLockserverTemplate `json:",inline"`

	// Zone is the name of the Availability Zone that this lockserver should run in.
	// This value should match the value of the "failure-domain.beta.kubernetes.io/zone"
	// label on the Kubernetes Nodes in that AZ.
	// If the Kubernetes Nodes don't have such a label, leave this empty.
	Zone string `json:"zone,omitempty"`
}

// EtcdLockserverTemplate defines the user-configurable settings for an etcd
// cluster that we deploy (not external), to serve as either a local or global
// lockserver.
type EtcdLockserverTemplate struct {
	// Image is the etcd server image (including version tag) to deploy.
	// Default: Let the operator choose.
	Image string `json:"image,omitempty"`

	// ImagePullPolicy specifies if/when to pull a container image.
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// ImagePullSecrets specifies the container image pull secrets to add to all
	// etcd Pods.
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	// Resources specify the compute resources to allocate for each etcd member.
	// Default: Let the operator choose.
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// DataVolumeClaimTemplate configures the PersistentVolumeClaims that will be created
	// for each etcd instance to store its data files.
	// This field is required.
	//
	// IMPORTANT: For a cell-local lockserver, you must set a storageClassName
	// here for a StorageClass that's configured to only provision volumes in
	// the Availability Zone that corresponds to the Vitess cell.
	// Default: Let the operator choose.
	DataVolumeClaimTemplate corev1.PersistentVolumeClaimSpec `json:"dataVolumeClaimTemplate,omitempty"`

	// ExtraFlags can optionally be used to override default flags set by the
	// operator, or pass additional flags to etcd. All entries must be
	// key-value string pairs of the form "flag": "value". The flag name should
	// not have any prefix (just "flag", not "-flag"). To set a boolean flag,
	// set the string value to either "true" or "false".
	ExtraFlags map[string]string `json:"extraFlags,omitempty"`

	// ExtraEnv can optionally be used to override default environment variables
	// set by the operator, or pass additional environment variables.
	ExtraEnv []corev1.EnvVar `json:"extraEnv,omitempty"`

	// ExtraVolumes can optionally be used to override default Pod volumes
	// defined by the operator, or provide additional volumes to the Pod.
	// Note that when adding a new volume, you should usually also add a
	// volumeMount to specify where in each container's filesystem the volume
	// should be mounted.
	// +kubebuilder:validation:EmbeddedResource
	ExtraVolumes []corev1.Volume `json:"extraVolumes,omitempty"`

	// ExtraVolumeMounts can optionally be used to override default Pod
	// volumeMounts defined by the operator, or specify additional mounts.
	// Typically, these are used to mount volumes defined through extraVolumes.
	ExtraVolumeMounts []corev1.VolumeMount `json:"extraVolumeMounts,omitempty"`

	// InitContainers can optionally be used to supply extra init containers
	// that will be run to completion one after another before any app containers are started.
	// +kubebuilder:validation:EmbeddedResource
	InitContainers []corev1.Container `json:"initContainers,omitempty"`

	// SidecarContainers can optionally be used to supply extra containers
	// that run alongside the main containers.
	// +kubebuilder:validation:EmbeddedResource
	SidecarContainers []corev1.Container `json:"sidecarContainers,omitempty"`

	// Affinity allows you to set rules that constrain the scheduling of
	// your Etcd pods. WARNING: These affinity rules will override all default affinities
	// that we set; in turn, we can't guarantee optimal scheduling of your pods if you
	// choose to set this field.
	// +kubebuilder:validation:EmbeddedResource
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// Annotations can optionally be used to attach custom annotations to Pods
	// created for this component.
	Annotations map[string]string `json:"annotations,omitempty"`

	// ExtraLabels can optionally be used to attach custom labels to Pods
	// created for this component.
	ExtraLabels map[string]string `json:"extraLabels,omitempty"`

	// CreatePDB sets whether to create a PodDisruptionBudget (PDB) for etcd
	// member Pods.
	//
	// Note: Disabling this will NOT delete a PDB that was previously created.
	//
	// Default: true
	CreatePDB *bool `json:"createPDB,omitempty"`

	// CreateClientService sets whether to create a Service for the client port
	// of etcd member Pods.
	//
	// Note: Disabling this will NOT delete a Service that was previously created.
	//
	// Default: true
	CreateClientService *bool `json:"createClientService,omitempty"`

	// CreatePeerService sets whether to create a Service for the peer port
	// of etcd member Pods.
	//
	// Note: Disabling this will NOT delete a Service that was previously created.
	//
	// Default: true
	CreatePeerService *bool `json:"createPeerService,omitempty"`

	// AdvertisePeerURLs can optionally be used to override the URLs that etcd
	// members use to find each other for peer-to-peer connections.
	//
	// If specified, the list must contain exactly 3 entries, one for each etcd
	// member index (1,2,3) respectively.
	//
	// Default: Build peer URLs automatically based on Kubernetes built-in DNS.
	// +kubebuilder:validation:MinItems=3
	// +kubebuilder:validation:MaxItems=3
	AdvertisePeerURLs []string `json:"advertisePeerURLs,omitempty"`

	// LocalMemberIndex can optionally be used to specify that only one etcd
	// member should actually be deployed. This can be used to spread members
	// across multiple Kubernetes clusters by configuring the EtcdLockserver CRD
	// in each cluster to deploy a different member index. If specified, the
	// index must be 1, 2, or 3.
	//
	// Default: Deploy all etcd members locally.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=3
	LocalMemberIndex *int32 `json:"localMemberIndex,omitempty"`

	// ClientService can optionally be used to customize the etcd client Service.
	ClientService *ServiceOverrides `json:"clientService,omitempty"`

	// PeerService can optionally be used to customize the etcd peer Service.
	PeerService *ServiceOverrides `json:"peerService,omitempty"`
}

// EtcdLockserverStatus defines the observed state of an EtcdLockserver.
type EtcdLockserverStatus struct {
	// The generation observed by the controller.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Available is a condition that indicates whether the cluster is able to serve queries.
	Available corev1.ConditionStatus `json:"available,omitempty"`
	// ClientServiceName is the name of the Service for etcd client connections.
	ClientServiceName string `json:"clientServiceName,omitempty"`
}

// NewEtcdLockserverStatus returns a new status with default values.
func NewEtcdLockserverStatus() *EtcdLockserverStatus {
	return &EtcdLockserverStatus{
		Available: corev1.ConditionUnknown,
	}
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// EtcdLockserverList contains a list of EtcdLockserver
type EtcdLockserverList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EtcdLockserver `json:"items"`
}

func init() {
	SchemeBuilder.Register(&EtcdLockserver{}, &EtcdLockserverList{})
}
