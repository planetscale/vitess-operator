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

// VitessShard represents a group of Vitess instances (tablets) that store a subset
// of the data in a logical database (keyspace).
//
// The tablets belonging to one VitessShard can ultimately be deployed across
// various VitessCells. All the tablets in a given shard, across all cells,
// use MySQL replication to stay eventually consistent with the MySQL master
// for that shard.
// +kubebuilder:resource:path=vitessshards,shortName=vts
// +kubebuilder:subresource:status
type VitessShard struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VitessShardSpec   `json:"spec,omitempty"`
	Status VitessShardStatus `json:"status,omitempty"`
}

// VitessShardSpec defines the desired state of a VitessShard.
type VitessShardSpec struct {
	// VitessShardTemplate contains the user-specified parts of VitessShardSpec.
	// These are the parts that are configurable inside VitessCluster.
	// The rest of the fields below are filled in by the parent controller.
	VitessShardTemplate `json:",inline"`

	// Name is the shard name as it's known to Vitess.
	Name string `json:"name"`

	// DatabaseName is the name to use for the underlying MySQL database.
	// It is inherited from the parent keyspace, so it can only be configured at
	// the keyspace level.
	DatabaseName string `json:"databaseName,omitempty"`

	// ZoneMap is a map from Vitess cell name to zone (failure domain) name
	// for all cells defined in the VitessCluster.
	ZoneMap map[string]string `json:"zoneMap"`

	// Images are not customizable by users at the shard level because version
	// skew across the shard is discouraged except during rolling updates,
	// in which case this field is automatically managed by the VitessKeyspace
	// controller that owns this VitessShard.
	Images VitessKeyspaceImages `json:"images"`

	// ImagePullPolicies are inherited from the VitessCluster spec.
	ImagePullPolicies VitessImagePullPolicies `json:"imagePullPolicies,omitempty"`

	// ImagePullSecrets are inherited from the VitessCluster spec.
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	// KeyRange is the range of keyspace IDs served by this shard.
	KeyRange VitessKeyRange `json:"keyRange"`

	// GlobalLockserver are the params to connect to the global lockserver.
	GlobalLockserver VitessLockserverParams `json:"globalLockserver"`

	// VitessOrchestrator is inherited from the parent's VitessKeyspace.
	VitessOrchestrator *VitessOrchestratorSpec `json:"vitessOrchestrator,omitempty"`

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

// VitessShardTemplate contains only the user-specified parts of a VitessShard object.
type VitessShardTemplate struct {
	// TabletPools specify groups of tablets in a given cell with a certain
	// tablet type and a shared configuration template.
	//
	// There must be at most one pool in this list for each (cell,type,name) set.
	// Each shard must have at least one "replica" pool (in at least one cell)
	// in order to be able to serve.
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	// +listMapKey=cell
	// +listMapKey=name
	TabletPools []VitessShardTabletPool `json:"tabletPools,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// DatabaseInitScriptSecret specifies the init_db.sql script file to use for this shard.
	// This SQL script file is executed immediately after bootstrapping an empty database
	// to set up initial tables and other MySQL-level entities needed by Vitess.
	DatabaseInitScriptSecret SecretSource `json:"databaseInitScriptSecret"`

	// Replication configures Vitess replication settings for the shard.
	Replication VitessReplicationSpec `json:"replication,omitempty"`

	// Annotations can optionally be used to attach custom annotations to the VitessShard object.
	Annotations map[string]string `json:"annotations,omitempty"`
}

// VitessReplicationSpec specifies how Vitess will set up MySQL replication.
type VitessReplicationSpec struct {
	// InitializeMaster specifies whether to choose an initial master for a
	// new or restored shard that has no master yet.
	//
	// Default: true.
	InitializeMaster *bool `json:"initializeMaster,omitempty"`

	// InitializeBackup specifies whether to take an initial placeholder backup
	// as part of preparing tablets to begin replication. This only takes effect
	// if a backup location is defined in the VitessCluster.
	//
	// Default: true.
	InitializeBackup *bool `json:"initializeBackup,omitempty"`

	// RecoverRestartedMaster specifies whether the operator attempts to repair
	// replication when the master MySQL restarts in-place (due to a crash) or
	// its Pod gets deleted and recreated, causing the Pod IP to change.
	//
	// Default: true.
	RecoverRestartedMaster *bool `json:"recoverRestartedMaster,omitempty"`
}

// VitessShardTabletPool defines a pool of tablets with a similar purpose.
type VitessShardTabletPool struct {
	// Cell is the name of the Vitess cell in which to deploy this pool.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=^[A-Za-z0-9]([_.A-Za-z0-9]*[A-Za-z0-9])?$
	Cell string `json:"cell"`

	// Type is the type of tablet contained in this tablet pool.
	//
	// The allowed types are:
	//
	//   * replica - master-eligible tablets that serve transactional (OLTP) workloads
	//   * rdonly - master-ineligible tablets (can never be promoted to master) that serve batch/analytical (OLAP) workloads
	//   * externalmaster - tablets pointed at an external, read-write MySQL endpoint
	//   * externalreplica - tablets pointed at an external, read-only MySQL endpoint that serve transactional (OLTP) workloads
	//   * externalrdonly - tablets pointed at an external, read-only MySQL endpoint that serve batch/analytical (OLAP) workloads
	// +kubebuilder:validation:Enum=replica;rdonly;externalmaster;externalreplica;externalrdonly
	Type VitessTabletPoolType `json:"type"`

	// Name is the pool's unique name within the (cell,type) pair.
	// This field is optional, and defaults to an empty.
	// Assigning different names to this field enables the existence of multiple pools with a specific tablet type in a given cell,
	// which can be beneficial for unmanaged tablets.
	// Hence, you must specify ExternalDatastore when assigning a name to this field.
	// +kubebuilder:default=""
	Name string `json:"name,omitempty"`

	// Replicas is the number of tablets to deploy in this pool.
	// This field is required, although it may be set to 0,
	// which will scale the pool down to 0 tablets.
	// +kubebuilder:validation:Minimum=0
	Replicas int32 `json:"replicas"`

	// DataVolumeClaimTemplate configures the PersistentVolumeClaims that will be created
	// for each tablet to store its database files.
	// This field is required for local MySQL, but should be omitted in the case of externally
	// managed MySQL.
	//
	// IMPORTANT: For a tablet pool in a Kubernetes cluster that spans multiple
	// zones, you should ensure that `volumeBindingMode: WaitForFirstConsumer`
	// is set on the StorageClass specified in the storageClassName field here.
	DataVolumeClaimTemplate *corev1.PersistentVolumeClaimSpec `json:"dataVolumeClaimTemplate,omitempty"`

	// BackupLocationName is the name of the backup location to use for this
	// tablet pool. It must match the name of one of the backup locations
	// defined in the VitessCluster.
	// Default: Use the backup location whose name is empty.
	BackupLocationName string `json:"backupLocationName,omitempty"`

	// Vttablet configures the vttablet server within each tablet.
	Vttablet VttabletSpec `json:"vttablet"`

	// Mysqld configures a local MySQL running inside each tablet Pod.
	// You must specify either Mysqld or ExternalDatastore, but not both.
	Mysqld *MysqldSpec `json:"mysqld,omitempty"`

	// MysqldExporter configures a MySQL exporter running inside each tablet Pod.
	MysqldExporter *MysqldExporterSpec `json:"mysqldExporter,omitempty"`

	// ExternalDatastore provides information for an externally managed MySQL.
	// You must specify either Mysqld or ExternalDatastore, but not both.
	ExternalDatastore *ExternalDatastore `json:"externalDatastore,omitempty"`

	// Affinity allows you to set rules that constrain the scheduling of
	// your vttablet pods. Affinity rules will affect all underlying
	// tablets in the specified tablet pool the same way. WARNING: These affinity rules
	// will override all default affinities that we set; in turn, we can't guarantee
	// optimal scheduling of your pods if you choose to set this field.
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:pruning:PreserveUnknownFields
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// Annotations can optionally be used to attach custom annotations to Pods
	// created for this component.
	Annotations map[string]string `json:"annotations,omitempty"`

	// ExtraLabels can optionally be used to attach custom labels to Pods
	// created for this component.
	ExtraLabels map[string]string `json:"extraLabels,omitempty"`

	// ExtraEnv can optionally be used to override default environment variables
	// set by the operator, or pass additional environment variables.
	// These values are applied to both the vttablet and mysqld containers.
	ExtraEnv []corev1.EnvVar `json:"extraEnv,omitempty"`

	// ExtraVolumes can optionally be used to override default Pod volumes
	// defined by the operator, or provide additional volumes to the Pod.
	// Note that when adding a new volume, you should usually also add a
	// volumeMount to specify where in each container's filesystem the volume
	// should be mounted.
	// These volumes are available to be mounted by both vttablet and mysqld.
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:pruning:PreserveUnknownFields
	ExtraVolumes []corev1.Volume `json:"extraVolumes,omitempty"`

	// ExtraVolumeMounts can optionally be used to override default Pod
	// volumeMounts defined by the operator, or specify additional mounts.
	// Typically, these are used to mount volumes defined through extraVolumes.
	// These values are applied to both the vttablet and mysqld containers.
	ExtraVolumeMounts []corev1.VolumeMount `json:"extraVolumeMounts,omitempty"`

	// InitContainers can optionally be used to supply extra init containers
	// that will be run to completion one after another before any app containers are started.
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:pruning:PreserveUnknownFields
	InitContainers []corev1.Container `json:"initContainers,omitempty"`

	// SidecarContainers can optionally be used to supply extra containers
	// that run alongside the main containers.
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:pruning:PreserveUnknownFields
	SidecarContainers []corev1.Container `json:"sidecarContainers,omitempty"`

	// Tolerations allow you to schedule pods onto nodes with matching taints.
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:pruning:PreserveUnknownFields
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// TopologySpreadConstraint can optionally be used to
	// specify how to spread vttablet pods among the given topology
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:pruning:PreserveUnknownFields
	TopologySpreadConstraints []corev1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`
}

// VttabletSpec configures the vttablet server within a tablet.
type VttabletSpec struct {
	// Resources specify the compute resources to allocate for just the vttablet
	// process (the Vitess query server that sits in front of MySQL).
	// This field is required.
	Resources corev1.ResourceRequirements `json:"resources"`

	// ExtraFlags can optionally be used to override default flags set by the
	// operator, or pass additional flags to vttablet. All entries must be
	// key-value string pairs of the form "flag": "value". The flag name should
	// not have any prefix (just "flag", not "-flag"). To set a boolean flag,
	// set the string value to either "true" or "false".
	ExtraFlags map[string]string `json:"extraFlags,omitempty"`

	// Lifecycle can optionally be used to add container lifecycle hooks
	// to vttablet container
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:pruning:PreserveUnknownFields
	Lifecycle corev1.Lifecycle `json:"lifecycle,omitempty"`

	// TerminationGracePeriodSeconds can optionally be used to customize
	// terminationGracePeriodSeconds of the vttablet pod.
	TerminationGracePeriodSeconds *int64 `json:"terminationGracePeriodSeconds,omitempty"`
}

// MysqldSpec configures the local MySQL server within a tablet.
type MysqldSpec struct {
	// Resources specify the compute resources to allocate for just the MySQL
	// process (the underlying local datastore).
	// This field is required.
	Resources corev1.ResourceRequirements `json:"resources"`

	// ConfigOverrides can optionally be used to provide a my.cnf snippet
	// to override default my.cnf values (included with Vitess) for this
	// particular MySQL instance.
	ConfigOverrides string `json:"configOverrides,omitempty"`
}

// MysqldExporterSpec configures the local MySQL exporter within a tablet.
type MysqldExporterSpec struct {
	// ExtraFlags can optionally be used to override default flags set by the
	// operator, or pass additional flags to mysqld_exporter. All entries must be
	// key-value string pairs of the form "flag": "value". The flag name should
	// not have any prefix (just "flag", not "-flag"). To set a boolean flag,
	// set the string value to either "true" or "false"; the flag will be
	// automatically converted to the format expected by mysqld_exporter.
	ExtraFlags map[string]string `json:"extraFlags,omitempty"`

	// Resources specify the compute resources to allocate for just the MySQL Exporter.
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

// VitessTabletPoolType represents the tablet types for which it makes sense
// to deploy a dedicated pool. Tablet types that indicate temporary or
// transient states are not valid pool types.
type VitessTabletPoolType string

const (
	// ReplicaPoolType is hte VitessTabletPoolType for master-eligible tablets.
	ReplicaPoolType VitessTabletPoolType = "replica"
	// RdonlyPoolType is the VitessTabletPoolType for master-ineligible tablets.
	RdonlyPoolType VitessTabletPoolType = "rdonly"
	// ExternalMasterPoolType is the VitessTabletPoolType for connecting a master
	// tablet to externally managed MySQL.
	ExternalMasterPoolType VitessTabletPoolType = "externalmaster"
	// ExternalReplicaPoolType is the VitessTabletPoolType for connecting a replica
	// tablet to externally managed MySQL.
	ExternalReplicaPoolType VitessTabletPoolType = "externalreplica"
	// ExternalRdonlyPoolType is the VitessTabletPoolType for connecting a rdonly
	// tablet to externally managed MySQL.
	ExternalRdonlyPoolType VitessTabletPoolType = "externalrdonly"
)

// ExternalDatastore defines information that vttablet needs to connect to an
// externally managed MySQL.
type ExternalDatastore struct {
	// User is a provided database user from an externally managed MySQL that Vitess can use to
	// carry out necessary actions.  Password for this user must be supplied in the CredentialsSecret.
	User string `json:"user"`
	// Host is the endpoint string to an externally managed MySQL, without any port.
	Host string `json:"host"`
	// Port specifies the port for the externally managed MySQL endpoint.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port"`
	// Database is the name of the database.
	Database string `json:"database"`
	// CredentialsSecret should link to a JSON credentials file used to connect to the externally managed
	// MySQL endpoint. The credentials file is understood and parsed by Vitess and must be in the format:
	// {
	//   "username": [
	//     "password"
	//   ]
	// }
	// Vitess always uses the first password in the password array.
	CredentialsSecret SecretSource `json:"credentialsSecret"`

	// ServerCACertSecret should link to a certificate authority file if one is required by your externally managed MySQL endpoint.
	ServerCACertSecret *SecretSource `json:"serverCACertSecret,omitempty"`
}

// VitessShardStatus defines the observed state of a VitessShard.
type VitessShardStatus struct {
	// The generation observed by the controller.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Tablets is a summary of the status of all desired tablets in the shard.
	Tablets map[string]VitessTabletStatus `json:"tablets,omitempty"`
	// OrphanedTablets is a list of unwanted tablets that could not be turned down.
	OrphanedTablets map[string]OrphanStatus `json:"orphanedTablets,omitempty"`

	// Cells is a list of cells in which any tablets for this shard are deployed.
	Cells []string `json:"cells,omitempty"`

	// VitessOrchestrator is a summary of the status of the vtorc deployment.
	VitessOrchestrator VitessOrchestratorStatus `json:"vitessOrchestrator,omitempty"`

	// HasMaster is a condition indicating whether the Vitess topology
	// reflects a master for this shard.
	HasMaster corev1.ConditionStatus `json:"hasMaster,omitempty"`

	// HasInitialBackup is a condition indicating whether the initial backup
	// has been seeded for the shard.
	HasInitialBackup corev1.ConditionStatus `json:"hasInitialBackup,omitempty"`

	// ServingWrites is a condition indicating whether this shard is the one
	// that serves writes for its key range, according to Vitess topology.
	// A shard might be deployed without serving writes if, for example, it is
	// the target of a resharding operation that is still in progress.
	ServingWrites corev1.ConditionStatus `json:"servingWrites,omitempty"`

	// Idle is a condition indicating whether the shard can be turned down.
	// If Idle is True, the shard is not part of the active shard set
	// (partitioning) for any tablet type in any cell, so it should be safe
	// to turn down the shard.
	Idle corev1.ConditionStatus `json:"idle,omitempty"`

	// Conditions is a map of all VitessShard specific conditions we want to set and monitor.
	// It's ok for multiple controllers to add conditions here, and those conditions will be preserved.
	Conditions map[VitessShardConditionType]VitessShardCondition `json:"conditions,omitempty"`

	// MasterAlias is the tablet alias of the master according to the global
	// shard record. This could be empty either because there is no master,
	// or because the shard record could not be read. Check the HasMaster
	// condition whenever the distinction is important.
	MasterAlias string `json:"masterAlias,omitempty"`

	// BackupLocations reports information about the backups for this shard in
	// each backup location.
	BackupLocations []*ShardBackupLocationStatus `json:"backupLocations,omitempty"`

	// LowestPodGeneration is the oldest VitessShard object generation seen across
	// all child Pods. The tablet information in VitessShard status is guaranteed to be
	// at least as up-to-date as this VitessShard generation. Changes made in
	// subsequent generations that affect tablets may not be reflected in status yet.
	LowestPodGeneration int64 `json:"lowestPodGeneration,omitempty"`
}

// VitessOrchestratorStatus is a summary of the status of the vtorc deployment.
type VitessOrchestratorStatus struct {
	// Available indicates whether the vtctld service has available endpoints.
	Available corev1.ConditionStatus `json:"available,omitempty"`
	// ServiceName is the name of the Service for this cluster's vtorc.
	ServiceName string `json:"serviceName,omitempty"`
}

// VitessShardConditionType is a valid value for the key of a VitessShardCondition map where the key is a
// VitessShardConditionType and the value is a VitessShardCondition.
type VitessShardConditionType string

// VitessShardCondition contains details for the current condition of this VitessShard.
type VitessShardCondition struct {
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

// NewVitessShardStatus creates a new status object with default values.
func NewVitessShardStatus() VitessShardStatus {
	return VitessShardStatus{
		Tablets:         make(map[string]VitessTabletStatus),
		OrphanedTablets: make(map[string]OrphanStatus),
		VitessOrchestrator: VitessOrchestratorStatus{
			Available: corev1.ConditionUnknown,
		},
		HasMaster:        corev1.ConditionUnknown,
		HasInitialBackup: corev1.ConditionUnknown,
		ServingWrites:    corev1.ConditionUnknown,
		Idle:             corev1.ConditionUnknown,
		Conditions:       make(map[VitessShardConditionType]VitessShardCondition),
	}
}

// VitessTabletStatus is the status of one tablet in a shard.
type VitessTabletStatus struct {
	// PoolType is the target tablet type for the tablet pool.
	PoolType string `json:"poolType,omitempty"`
	// Index is the tablet's index within its tablet pool.
	Index int32 `json:"index,omitempty"`
	// Running indicates whether the vttablet Pod is running.
	Running corev1.ConditionStatus `json:"running,omitempty"`
	// Ready indicates whether the vttablet Pod is passing health checks,
	// meaning it's ready to serve queries.
	Ready corev1.ConditionStatus `json:"ready,omitempty"`
	// Available indicates whether the vttablet Pod has been consistently Ready
	// for long enough to be considered stable.
	Available corev1.ConditionStatus `json:"available,omitempty"`
	// DataVolumeBound indicates whether the main PersistentVolumeClaim has been
	// matched up with a PersistentVolume and bound to it.
	DataVolumeBound corev1.ConditionStatus `json:"dataVolumeBound,omitempty"`
	// Type is the observed tablet type as reflected in topology.
	Type string `json:"type,omitempty"`
	// PendingChanges describes changes to the tablet Pod that will be applied
	// the next time a rolling update allows.
	PendingChanges string `json:"pendingChanges,omitempty"`
}

// NewVitessTabletStatus creates a new status object with default values.
func NewVitessTabletStatus(poolType VitessTabletPoolType, index int32) VitessTabletStatus {
	return VitessTabletStatus{
		PoolType:        string(poolType),
		Index:           index,
		Running:         corev1.ConditionUnknown,
		Ready:           corev1.ConditionUnknown,
		Available:       corev1.ConditionUnknown,
		DataVolumeBound: corev1.ConditionUnknown,
	}
}

// ShardBackupLocationStatus reports status for the backups of a given shard in
// a given backup location.
type ShardBackupLocationStatus struct {
	// Name is the backup location name.
	Name string `json:"name,omitempty"`
	// CompleteBackups is the number of complete backups observed.
	CompleteBackups int32 `json:"completeBackups"`
	// IncompleteBackups is the number of incomplete backups observed.
	IncompleteBackups int32 `json:"incompleteBackups"`
	// LatestCompleteBackupTime is the timestamp of the most recent complete backup.
	LatestCompleteBackupTime *metav1.Time `json:"latestCompleteBackupTime,omitempty"`
}

// NewShardBackupLocationStatus creates a new status object with default values.
func NewShardBackupLocationStatus(name string) *ShardBackupLocationStatus {
	return &ShardBackupLocationStatus{
		Name: name,
	}
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VitessShardList contains a list of VitessShard
type VitessShardList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VitessShard `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VitessShard{}, &VitessShardList{})
}
