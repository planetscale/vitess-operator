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

package v2

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.
//
// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VitessCell represents a group of Nodes in a given failure domain (Zone),
// plus Vitess components like the lockserver and gateway that are local
// to each cell. Together, these cell-local components make it possible for
// Vitess instances (tablets) to run on those Nodes, and for clients to reach
// Vitess instances in the cell.
//
// Note that VitessCell does not "own" the VitessKeyspaces deployed in it,
// just like a Node does not own the Pods deployed on it. In addition, each
// VitessKeyspace can deploy Vitess instances in multiple VitessCells,
// just like a Deployment can manage Pods that run on multiple Nodes.
// +k8s:openapi-gen=true
// +kubebuilder:resource:path=vitesscells,shortName=vtc
// +kubebuilder:subresource:status
type VitessCell struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VitessCellSpec   `json:"spec,omitempty"`
	Status VitessCellStatus `json:"status,omitempty"`
}

// VitessCellSpec defines the desired state of a VitessCell.
// +k8s:openapi-gen=true
type VitessCellSpec struct {
	// VitessCellTemplate contains the user-specified parts of VitessCellSpec.
	// These are the parts that are configurable inside VitessCluster.
	// The rest of the fields below are filled in by the parent controller.
	VitessCellTemplate `json:",inline"`

	// GlobalLockserver are the params to connect to the global lockserver.
	GlobalLockserver VitessLockserverParams `json:"globalLockserver"`

	// AllCells is a list of all cells in the Vitess cluster.
	AllCells []string `json:"allCells"`

	// Images are not customizable by users at the cell level because version
	// skew across the cluster is discouraged except during rolling updates,
	// in which case this field is automatically managed by the VitessCluster
	// controller that owns this VitessCell.
	Images VitessCellImages `json:"images,omitempty"`

	// ExtraVitessFlags can optionally be used to override default flags set by the
	// operator, or pass additional flags to vttablet from VitessCellSpec. All entries must be
	// key-value string pairs of the form "flag": "value". The flag name should
	// not have any prefix (just "flag", not "-flag"). To set a boolean flag,
	// set the string value to either "true" or "false".
	ExtraVitessFlags map[string]string `json:"extraVitessFlags,omitempty"`
}

// VitessCellTemplate contains only the user-specified parts of a VitessCell object.
type VitessCellTemplate struct {
	// Name is the cell name as it should be provided to Vitess.
	// Note that this is different from the VitessCell object's
	// metadata.name, which is generated by the operator.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=25
	// +kubebuilder:validation:Pattern=^[a-z0-9]([a-z0-9]*[a-z0-9])?$
	Name string `json:"name"`

	// Zone is the name of the Availability Zone that this Vitess Cell should run in.
	// This value should match the value of the "failure-domain.beta.kubernetes.io/zone"
	// label on the Kubernetes Nodes in that AZ.
	// If the Kubernetes Nodes don't have such a label, leave this empty.
	Zone string `json:"zone,omitempty"`

	// Lockserver specifies either a deployed or external lockserver
	// to be used as the Vitess cell-local topology store.
	// Default: Put this cell's topology data in the global lockserver instead of its own lockserver.
	Lockserver LockserverSpec `json:"lockserver,omitempty"`

	// Gateway configures the Vitess Gateway deployment in this cell.
	Gateway VitessCellGatewaySpec `json:"gateway,omitempty"`
}

// VitessCellImages specifies container images to use for this cell.
type VitessCellImages struct {
	/*
		***ATTENTION***

		Make sure to keep the following up to date if you add fields here:
		  * DefaultVitessCellImages() in vitesscell_defaults.go
	*/

	// Vtgate is the container image (including version tag) to use for Vitess Gateway instances.
	Vtgate string `json:"vtgate,omitempty"`
}

// VitessCellGatewaySpec specifies the per-cell deployment parameters for vtgate.
type VitessCellGatewaySpec struct {
	// Replicas is the number of vtgate instances to deploy in this cell.
	// +kubebuilder:validation:Minimum=0
	Replicas *int32 `json:"replicas,omitempty"`

	// Resources determines the compute resources reserved for each vtgate replica.
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// Authentication configures how Vitess Gateway authenticates MySQL client connections.
	Authentication VitessGatewayAuthentication `json:"authentication,omitempty"`

	// SecureTransport configures secure transport connections for vtgate.
	SecureTransport *VitessGatewaySecureTransport `json:"secureTransport,omitempty"`

	// ExtraFlags can optionally be used to override default flags set by the
	// operator, or pass additional flags to vtgate. All entries must be
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
	ExtraVolumes []corev1.Volume `json:"extraVolumes,omitempty"`

	// ExtraVolumeMounts can optionally be used to override default Pod
	// volumeMounts defined by the operator, or specify additional mounts.
	// Typically, these are used to mount volumes defined through extraVolumes.
	ExtraVolumeMounts []corev1.VolumeMount `json:"extraVolumeMounts,omitempty"`

	// Affinity allows you to set rules that constrain the scheduling of
	// your vtgate pods. WARNING: These affinity rules will override all default affinities
	// that we set; in turn, we can't guarantee optimal scheduling of your pods if you
	// choose to set this field.
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// Annotations can optionally be used to attach custom annotations to Pods
	// created for this component. These will be attached to the underlying
	// pods that the vtgate deployment creates.
	Annotations map[string]string `json:"annotations,omitempty"`
}

// VitessGatewayAuthentication configures authentication for vtgate in this cell.
type VitessGatewayAuthentication struct {
	// Static configures vtgate to use a static file containing usernames and passwords.
	Static *VitessGatewayStaticAuthentication `json:"static,omitempty"`
}

// VitessGatewayStaticAuthentication configures static file authentication for vtgate.
type VitessGatewayStaticAuthentication struct {
	// Secret configures vtgate to load the static auth file from a given key in a given Secret.
	Secret *corev1.SecretKeySelector `json:"secret,omitempty"`
}

// VitessGatewaySecureTransport configures secure transport connections for vtgate.
type VitessGatewaySecureTransport struct {
	// Required configures vtgate to reject non-secure transport connections.
	// Applies only to MySQL protocol connections.
	// All GRPC transport is required to be encrypted when certs are set.
	Required bool `json:"required,omitempty"`

	// TLS configures vtgate to use TLS encrypted transport.
	TLS *VitessGatewayTLSSecureTransport `json:"tls,omitempty"`
}

// VitessGatewayAuthentication configures authentication for vtgate in this cell.
type VitessGatewayTLSSecureTransport struct {
	// ClientCACertSecret configures vtgate to load the TLS certificate authority PEM file from a given key in a given Secret.
	// If specified, checks client certificates are signed by this CA certificate.
	// Optional.
	ClientCACertSecret *corev1.SecretKeySelector `json:"clientCACertSecret,omitempty"`

	// CertSecret configures vtgate to load the TLS cert PEM file from a given key in a given Secret.
	CertSecret *corev1.SecretKeySelector `json:"certSecret,omitempty"`
	// KeySecret configures vtgate to load the TLS key PEM file from a given key in a given Secret.
	KeySecret *corev1.SecretKeySelector `json:"keySecret,omitempty"`
}

// VitessCellGatewayStatus is a summary of the status of vtgate in this cell.
type VitessCellGatewayStatus struct {
	// Available indicates whether the vtgate service is fully available.
	Available corev1.ConditionStatus `json:"available,omitempty"`
	// ServiceName is the name of the Service for this cell's vtgate.
	ServiceName string `json:"serviceName,omitempty"`
}

// VitessCellStatus defines the observed state of VitessCell
// +k8s:openapi-gen=true
type VitessCellStatus struct {
	// The generation observed by the controller.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Lockserver is a summary of the status of the cell-local lockserver.
	Lockserver LockserverStatus `json:"lockserver,omitempty"`
	// Gateway is a summary of the status of vtgate in this cell.
	Gateway VitessCellGatewayStatus `json:"gateway,omitempty"`
	// Keyspaces is a summary of keyspaces deployed in this cell.
	// This summary could be empty either if there are no keyspaces,
	// or if the controller failed to read the current state.
	// Use the Idle condition to distinguish these scenarios
	// when the difference matters.
	Keyspaces map[string]VitessCellKeyspaceStatus `json:"keyspaces,omitempty"`
	// Idle is a condition indicating whether the cell can be turned down.
	// If Idle is True, there are no keyspaces deployed in the cell, so it
	// should be safe to turn down the cell.
	Idle corev1.ConditionStatus `json:"idle,omitempty"`
}

// NewVitessCellStatus creates a new status object with default values.
func NewVitessCellStatus() VitessCellStatus {
	return VitessCellStatus{
		Gateway: VitessCellGatewayStatus{
			Available: corev1.ConditionUnknown,
		},
		Keyspaces: make(map[string]VitessCellKeyspaceStatus),
		Idle:      corev1.ConditionUnknown,
	}
}

// VitessCellKeyspaceStatus summarizes the status of a keyspace deployed in this cell.
type VitessCellKeyspaceStatus struct {
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VitessCellList contains a list of VitessCell
type VitessCellList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VitessCell `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VitessCell{}, &VitessCellList{})
}
