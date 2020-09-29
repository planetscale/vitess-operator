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

// VitessKeyspace represents the deployment of a logical database in Vitess.
// Each keyspace consists of a number of shards, which then consist of tablets.
// The tablets belonging to one VitessKeyspace can ultimately be deployed across
// various VitessCells.
// +kubebuilder:resource:path=vitesskeyspaces,shortName=vtk
// +kubebuilder:subresource:status
type VitessKeyspace struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VitessKeyspaceSpec   `json:"spec,omitempty"`
	Status VitessKeyspaceStatus `json:"status,omitempty"`
}

// VitessKeyspaceSpec defines the desired state of a VitessKeyspace.
type VitessKeyspaceSpec struct {
	// VitessKeyspaceTemplate contains the user-specified parts of VitessKeyspaceSpec.
	// These are the parts that are configurable inside VitessCluster.
	// The rest of the fields below are filled in by the parent controller.
	VitessKeyspaceTemplate `json:",inline"`

	// GlobalLockserver are the params to connect to the global lockserver.
	GlobalLockserver VitessLockserverParams `json:"globalLockserver"`

	// Images are not customizable by users at the keyspace level because version
	// skew across the cluster is discouraged except during rolling updates,
	// in which case this field is automatically managed by the VitessCluster
	// controller that owns this VitessKeyspace.
	Images VitessKeyspaceImages `json:"images,omitempty"`

	// ImagePullPolicies are inherited from the VitessCluster spec.
	ImagePullPolicies VitessImagePullPolicies `json:"imagePullPolicies,omitempty"`

	// ImagePullSecrets are inherited from the VitessCluster spec.
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	// ZoneMap is a map from Vitess cell name to zone (failure domain) name
	// for all cells defined in the VitessCluster.
	ZoneMap map[string]string `json:"zoneMap"`

	// BackupLocations are the backup locations defined in the VitessCluster.
	BackupLocations []VitessBackupLocation `json:"backupLocations,omitempty"`

	// BackupEngine specifies the Vitess backup engine to use, either "builtin" or "xtrabackup".
	BackupEngine VitessBackupEngine `json:"backupEngine,omitempty"`

	// ExtraVitessFlags is inherited from the parent's VitessClusterSpec.
	ExtraVitessFlags map[string]string `json:"extraVitessFlags,omitempty"`

	// TopologyReconciliation is inherited from the parent's VitessClusterSpec.
	TopologyReconciliation *TopoReconcileConfig `json:"topologyReconciliation,omitempty"`

	// UpdateStrategy is inherited from the parent's VitessClusterSpec.
	UpdateStrategy *VitessClusterUpdateStrategy `json:"updateStrategy,omitempty"`
}

// VitessKeyspaceTemplate contains only the user-specified parts of a VitessKeyspace object.
type VitessKeyspaceTemplate struct {
	// Name is the keyspace name as it should be provided to Vitess.
	// Note that this is different from the VitessKeyspace object's
	// metadata.name, which is generated by the operator.
	//
	// WARNING: DO NOT change the name of a keyspace that was already deployed.
	// Keyspaces cannot be renamed, so this will be interpreted as an
	// instruction to delete the old keyspace and create a new one.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=25
	// +kubebuilder:validation:Pattern=^[a-z0-9]([a-z0-9]*[a-z0-9])?$
	Name string `json:"name"`

	// DatabaseName is the name to use for the underlying, physical MySQL
	// database created to hold data for the keyspace.
	//
	// This name is mostly hidden from Vitess clients, which should see and use
	// only the keyspace name as a logical database. However, you may want to
	// set this to control the name used by clients that bypass Vitess and
	// connect directly to the underlying MySQL, such as certain DBA tools.
	//
	// The default, when the field is either left unset or set to empty string,
	// is to add a "vt_" prefix to the keyspace name since that has historically
	// been the default in Vitess itself. However, it's often preferable to set
	// this to be the same as the keyspace name to reduce confusion.
	//
	// Default: Add a "vt_" prefix to the keyspace name.
	DatabaseName string `json:"databaseName,omitempty"`

	// Partitionings specify how to divide the keyspace up into shards by
	// defining the range of keyspace IDs that each shard contains.
	// For example, you might divide the keyspace into N equal-sized key ranges.
	//
	// Note that this is distinct from defining how each row maps to a keyspace ID,
	// which is done in the VSchema. Partitioning is purely an operational concern
	// (scaling the infrastructure), while VSchema is an application-level concern
	// (modeling relationships between data). This separation of concerns allows
	// resharding to occur generically at the infrastructure level without any
	// knowledge of the data model.
	//
	// Each partitioning must define a set of shards that fully covers the
	// space of all possible keyspace IDs; there can be no gaps between ranges.
	// There's usually only one partitioning present at a time, but during
	// resharding, it's necessary to launch the destination shards alongside
	// the source shards. When the resharding is complete, the old partitioning
	// can be removed, which will turn down (undeploy) any unneeded shards.
	//
	// If only some shards are being split or joined during resharding,
	// the shards that aren't changing must be specified in both partitionings,
	// although the common shards will be shared (only deployed once).
	// If the per-shard configuration differs, the configuration in the latter
	// partitioning (in the order listed in this field) will be used.
	// For this reason, it's recommended to add new partitionings at the end,
	// and only remove partitionings from the beginning.
	//
	// This field is required. An unsharded keyspace may be specified as a
	// partitioning into 1 part.
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=2
	Partitionings []VitessKeyspacePartitioning `json:"partitionings"`

	// TurndownPolicy specifies what should happen if this keyspace is ever
	// removed from the VitessCluster spec. By default, removing a keyspace
	// entry from the VitessCluster spec will NOT actually turn down the
	// deployed resources, unless it can be verified that the keyspace was
	// previously set to have 0 total desired tablets across all shards.
	//
	// With this default policy (RequireIdle), before removing the keyspace
	// entry from the spec, you must first edit the keyspace entry to remove
	// all tablet pools from all shards, and wait for that change to roll out.
	// If a keyspace entry is removed too soon, the keyspace resources will
	// remain deployed indefinitely, and the keyspace will be listed in the
	// orphanedKeyspaces field of VitessCluster status.
	//
	// This is a safety mechanism to prevent accidental edits to the cluster
	// object from having immediate, destructive consequences. If the cluster
	// spec is only ever edited by automation whose edits you trust to be safe,
	// you can set the policy to Immediate to skip these checks.
	//
	// Default: RequireIdle
	// +kubebuilder:validation:Enum=RequireIdle;Immediate
	TurndownPolicy VitessKeyspaceTurndownPolicy `json:"turndownPolicy,omitempty"`

	// Annotations can optionally be used to attach custom annotations to the VitessKeyspace object.
	Annotations map[string]string `json:"annotations,omitempty"`
}

// VitessKeyspaceTurndownPolicy is the policy for turning down a keyspace.
type VitessKeyspaceTurndownPolicy string

const (
	// VitessKeyspaceTurndownPolicyRequireIdle specifies that keyspace may only
	// be turned down if they are idle; that is, if they have no resources
	// deployed.
	VitessKeyspaceTurndownPolicyRequireIdle VitessKeyspaceTurndownPolicy = "RequireIdle"
	// VitessKeyspaceTurndownPolicyImmediate specifies that removing a keyspace
	// from the VitessCluster spec should immediately trigger turn-down of
	// all resources previously deployed for the keyspace.
	VitessKeyspaceTurndownPolicyImmediate VitessKeyspaceTurndownPolicy = "Immediate"
)

// VitessKeyspaceImages specifies container images to use for this keyspace.
type VitessKeyspaceImages struct {
	/*
		***ATTENTION***

		Make sure to keep the following up to date if you add fields here:
		  * DefaultVitessKeyspaceImages() in vitesskeyspace_defaults.go
	*/

	// Vttablet is the container image (including version tag) to use for Vitess Tablet instances.
	Vttablet string `json:"vttablet,omitempty"`
	// Vtbackup is the container image (including version tag) to use for Vitess Backup jobs.
	Vtbackup string `json:"vtbackup,omitempty"`
	// Mysqld specifies the container image to use for mysqld, as well as
	// declaring which MySQL flavor setting in Vitess the image is
	// compatible with. Only one flavor image may be provided at a time.
	// mysqld running alongside each tablet.
	Mysqld *MysqldImage `json:"mysqld,omitempty"`
	// MysqldExporter specifies the container image for mysqld-exporter.
	MysqldExporter string `json:"mysqldExporter,omitempty"`
}

// VitessKeyspacePartitioning defines a set of shards by dividing the keyspace into key ranges.
// Each field is a different method of dividing the keyspace. Only one field should be set on
// a given partitioning.
type VitessKeyspacePartitioning struct {
	// Equal partitioning splits the keyspace into some number of equal parts,
	// assuming that the keyspace IDs are uniformly distributed, for example
	// because they're generated by a hash vindex.
	Equal *VitessKeyspaceEqualPartitioning `json:"equal,omitempty"`

	// Custom partitioning lets you explicitly specify the key range of every shard,
	// in case you don't want them to be divided equally.
	Custom *VitessKeyspaceCustomPartitioning `json:"custom,omitempty"`
}

// VitessKeyspaceEqualPartitioning splits the keyspace into some number of equal parts.
type VitessKeyspaceEqualPartitioning struct {
	// Parts is the number of equal parts to split the keyspace into.
	// If you need shards that are not equal-sized, use custom partitioning instead.
	//
	// Note that if the number of parts is not a power of 2, the key ranges will
	// only be roughly equal in size.
	//
	// WARNING: DO NOT change the number of parts in a partitioning after deploying.
	//          That's effectively deleting the old partitioning and adding a new one,
	//          which can lead to downtime or data loss. Instead, add an additional
	//          partitioning with the desired number of parts, perform a resharding
	//          migration, and then remove the old partitioning.
	// +kubebuilder:validation:Minimum=1
	Parts int32 `json:"parts"`

	// ShardTemplate is the configuration used for each equal-sized shard.
	// If you need shards that don't all share the same configuration,
	// use custom partitioning instead.
	ShardTemplate VitessShardTemplate `json:"shardTemplate,omitempty"`
}

// VitessKeyspaceCustomPartitioning lets you explicitly specify the key range of every shard.
type VitessKeyspaceCustomPartitioning struct {
	// Shards is a list of explicit shard specifications.
	// +patchMergeKey=keyRange
	// +patchStrategy=merge
	Shards []VitessKeyspaceKeyRangeShard `json:"shards" patchStrategy:"merge" patchMergeKey:"keyRange"`
}

// VitessKeyspaceKeyRangeShard defines a shard based on a key range.
type VitessKeyspaceKeyRangeShard struct {
	// KeyRange is the range of keys that this shard serves.
	//
	// WARNING: DO NOT change the key range of a shard after deploying.
	//          That's effectively deleting the old shard and adding a new one,
	//          which can lead to downtime or data loss. Instead, add an additional
	//          partitioning with the desired set of shards, perform a resharding
	//          migration, and then remove the old partitioning.
	KeyRange VitessKeyRange `json:"keyRange"`

	// VitessShardTemplate is the configuration for the shard.
	VitessShardTemplate `json:",inline"`
}

// VitessKeyRange specifies a range of keyspace IDs.
type VitessKeyRange struct {
	// Start is a lowercase hexadecimal string representation of an arbitrary-length sequence of bytes.
	// If Start is the empty string, the key range is unbounded at the bottom.
	// If Start is not empty, the bytes of a keyspace ID must compare greater
	// than or equal to Start in lexicographical order to be in the range.
	// +kubebuilder:validation:Pattern=^([0-9a-f][0-9a-f])*$
	Start string `json:"start,omitempty"`
	// End is a lowercase hexadecimal string representation of an arbitrary-length sequence of bytes.
	// If End is the empty string, the key range is unbounded at the top.
	// If End is not empty, the bytes of a keyspace ID must compare strictly less than End in
	// lexicographical order to be in the range.
	// +kubebuilder:validation:Pattern=^([0-9a-f][0-9a-f])*$
	End string `json:"end,omitempty"`
}

// VitessKeyspaceStatus defines the observed state of a VitessKeyspace.
type VitessKeyspaceStatus struct {
	// The generation observed by the controller.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Shards is a summary of the status of all desired shards.
	Shards map[string]VitessKeyspaceShardStatus `json:"shards,omitempty"`
	// Partitionings is an aggregation of status for all shards in each partitioning.
	Partitionings []VitessKeyspacePartitioningStatus `json:"partitionings,omitempty"`
	// OrphanedShards is a list of unwanted shards that could not be turned down.
	OrphanedShards map[string]OrphanStatus `json:"orphanedShards,omitempty"`
	// Idle is a condition indicating whether the keyspace can be turned down.
	// If Idle is True, the keyspace is not deployed in any cells, so it should
	// be safe to turn down the keyspace.
	Idle corev1.ConditionStatus `json:"idle,omitempty"`
	// ReshardingStatus provides information about an active resharding operation, if any.
	// This field is only present if the ReshardingActive condition is True. If that condition is Unknown,
	// it means the operator was unable to query resharding status from Vitess.
	Resharding *ReshardingStatus `json:"resharding,omitempty"`
	// Conditions is a list of all VitessKeyspace specific conditions we want to set and monitor.
	// It's ok for multiple controllers to add conditions here, and those conditions will be preserved.
	Conditions []VitessKeyspaceCondition `json:"conditions,omitempty"`
}

// ReshardingStatus defines some of the workflow related status information.
type ReshardingStatus struct {
	// Workflow represents the name of the active vreplication workflow for resharding.
	Workflow string `json:"workflow"`
	// State is either 'Running', 'Copying', 'Error' or 'Unknown'.
	State WorkflowState `json:"state"`
	// SourceShards is a list of source shards for the current resharding operation.
	SourceShards []string `json:"sourceShards,omitempty"`
	// TargetShards is a list of target shards for the current resharding operation.
	TargetShards []string `json:"targetShards,omitempty"`
	// CopyProgress will indicate the percentage completion ranging from 0-100 as integer values.
	// Once we are past the copy phase, this value will always be 100, and will never be 100 while we
	// are still within the copy phase.
	// If we can not compute the copy progress in a timely fashion, we will report -1 to indicate the progress is unknown.
	CopyProgress int `json:"copyProgress,omitempty"`
}

// WorkflowState represents the current state for the given Workflow.
type WorkflowState string

const (
	// WorkflowRunning indicates that the workflow is currently in the Running state. This state
	// indicates that vreplication is ongoing, but we have moved passed the copying phase.
	WorkflowRunning WorkflowState = "Running"
	// WorkflowCopying indicates that the workflow is currently in the Copying state.
	WorkflowCopying WorkflowState = "Copying"
	// WorkflowError indicates that the workflow is currently experiencing some kind of error.
	WorkflowError WorkflowState = "Error"
	// WorkflowUnknown indicates that we could not discover the state for the given workflow.
	WorkflowUnknown WorkflowState = "Unknown"
)

// NewVitessKeyspaceStatus creates a new status object with default values.
func NewVitessKeyspaceStatus() VitessKeyspaceStatus {
	return VitessKeyspaceStatus{
		Shards:         make(map[string]VitessKeyspaceShardStatus),
		OrphanedShards: make(map[string]OrphanStatus),
		Idle:           corev1.ConditionUnknown,
	}
}

// VitessKeyspaceShardStatus is the status of a shard within a keyspace.
type VitessKeyspaceShardStatus struct {
	// HasMaster is a condition indicating whether the Vitess topology
	// reflects a master for this shard.
	HasMaster corev1.ConditionStatus `json:"hasMaster,omitempty"`
	// ServingWrites is a condition indicating whether this shard is the one
	// that serves writes for its key range, according to Vitess topology.
	// A shard might be deployed without serving writes if, for example, it is
	// the target of a resharding operation that is still in progress.
	ServingWrites corev1.ConditionStatus `json:"servingWrites,omitempty"`
	// DesiredTablets is the number of desired tablets. This is computed from
	// information that's already available in the spec, but clients should
	// use this value instead of trying to compute shard partitionings on their
	// own.
	DesiredTablets int32 `json:"desiredTablets,omitempty"`
	// Tablets is the number of observed tablets. This could be higher or
	// lower than desiredTablets if the state has not yet converged.
	Tablets int32 `json:"tablets,omitempty"`
	// ReadyTablets is the number of desired tablets that are Ready.
	ReadyTablets int32 `json:"readyTablets,omitempty"`
	// UpdatedTablets is the number of desired tablets that are up-to-date
	// (have no pending changes).
	UpdatedTablets int32 `json:"updatedTablets,omitempty"`
	// PendingChanges describes changes to the shard that will be applied
	// the next time a rolling update allows.
	PendingChanges string `json:"pendingChanges,omitempty"`
	// Cells is a list of cells in which any tablets for this shard are deployed.
	Cells []string `json:"cells,omitempty"`
}

// NewVitessKeyspaceShardStatus creates a new status object with default values.
func NewVitessKeyspaceShardStatus(spec *VitessKeyspaceKeyRangeShard) VitessKeyspaceShardStatus {
	desiredTablets := int32(0)
	for tpIndex := range spec.TabletPools {
		desiredTablets += spec.TabletPools[tpIndex].Replicas
	}

	return VitessKeyspaceShardStatus{
		HasMaster:      corev1.ConditionUnknown,
		ServingWrites:  corev1.ConditionUnknown,
		DesiredTablets: desiredTablets,
	}
}

// VitessKeyspacePartitioningStatus aggregates status for all shards in a given partitioning.
type VitessKeyspacePartitioningStatus struct {
	// ShardNames is a sorted list of shards in this partitioning,
	// in the format Vitess uses for shard names.
	ShardNames []string `json:"shardNames,omitempty"`
	// ServingWrites is a condition indicating whether all shards in this
	// partitioning are serving writes for their key ranges.
	// Note that False only means not all shards are serving writes; it's still
	// possible that some shards in this partitioning are serving writes.
	// Check the per-shard status for full details.
	ServingWrites corev1.ConditionStatus `json:"servingWrites,omitempty"`
}

// NewVitessKeyspacePartitioningStatus creates a new status object with default values.
func NewVitessKeyspacePartitioningStatus(partitioning *VitessKeyspacePartitioning) VitessKeyspacePartitioningStatus {
	return VitessKeyspacePartitioningStatus{
		ShardNames:    partitioning.ShardNameSet().List(),
		ServingWrites: corev1.ConditionUnknown,
	}
}

// VitessKeyspaceCondition contains details for the current condition of this VitessKeyspace.
type VitessKeyspaceCondition struct {
	// Type is the type of the condition.
	Type VitessKeyspaceConditionType `json:"type"`
	// Status is the status of the condition.
	// Can be True, False, Unknown.
	// +kubebuilder:validation:Enum=True;False;Unknown
	Status corev1.ConditionStatus `json:"status"`
	// Last time the condition transitioned from one status to another.
	// Optional.
	LastTransitionTime *metav1.Time `json:"lastTransitionTime,omitempty"`
	// Unique, one-word, PascalCase reason for the condition's last transition.
	// Optional.
	Reason string `json:"reason,omitempty"`
	// Human-readable message indicating details about last transition.
	// Optional.
	Message string `json:"message,omitempty"`
}

// VitessKeyspaceConditionType is a valid value for the key of a VitessKeyspaceCondition map where the key is a
// VitessKeyspaceConditionType and the value is a VitessKeyspaceCondition.
type VitessKeyspaceConditionType string

// These are valid conditions of VitessKeyspace.
const (
	// VitessKeyspaceReshardingActive indicates whether the keyspace has an active resharding operation,
	VitessKeyspaceReshardingActive VitessKeyspaceConditionType = "ReshardingActive"
	// VitessKeyspaceReshardingInSync indicates whether the keyspace has an active
	// resharding operation whose target shards are ready to serve if traffic is switched.
	VitessKeyspaceReshardingInSync VitessKeyspaceConditionType = "ReshardingInSync"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VitessKeyspaceList contains a list of VitessKeyspace
type VitessKeyspaceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VitessKeyspace `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VitessKeyspace{}, &VitessKeyspaceList{})
}
