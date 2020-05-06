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

// VitessCluster is the top-level interface for configuring a cluster.
//
// Although the VitessCluster controller creates various secondary objects
// like VitessCells, all the user-accessible configuration ultimately lives here.
// The other objects should be considered read-only representations of subsets of
// the dynamic cluster status. For example, you can examine a specific VitessCell
// object to get more details on the status of that cell than are summarized in the
// VitessCluster status, but any configuration changes should only be made in
// the VitessCluster object.
// +k8s:openapi-gen=true
// +kubebuilder:resource:path=vitessclusters,shortName=vt
// +kubebuilder:subresource:status
type VitessCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VitessClusterSpec   `json:"spec,omitempty"`
	Status VitessClusterStatus `json:"status,omitempty"`
}

// VitessClusterSpec defines the desired state of VitessCluster.
// +k8s:openapi-gen=true
type VitessClusterSpec struct {
	// Images specifies the container images (including version tag) to use
	// in the cluster.
	// Default: Let the operator choose.
	Images VitessImages `json:"images,omitempty"`

	// ImagePullPolicies specifies the container image pull policies to use for
	// images defined in the 'images' field.
	ImagePullPolicies VitessImagePullPolicies `json:"imagePullPolicies,omitempty"`

	// Backup specifies how to take and store Vitess backups.
	// This is optional but strongly recommended. In addition to disaster
	// recovery, Vitess currently depends on backups to support provisioning
	// of a new tablet in a shard with existing data, as an implementation detail.
	Backup *ClusterBackupSpec `json:"backup,omitempty"`

	// GlobalLockserver specifies either a deployed or external lockserver
	// to be used as the Vitess global topology store.
	// Default: Deploy an etcd cluster as the global lockserver.
	GlobalLockserver LockserverSpec `json:"globalLockserver,omitempty"`

	// Dashboard deploys a set of Vitess Dashboard servers (vtctld) for the Vitess cluster.
	VitessDashboard *VitessDashboardSpec `json:"vitessDashboard,omitempty"`

	// Cells is a list of templates for VitessCells to create for this cluster.
	//
	// Each VitessCell represents a set of Nodes in a given failure domain,
	// to which VitessKeyspaces can be deployed. The VitessCell also deploys
	// cell-local services that any keyspaces deployed there will need.
	//
	// This field is required, but it may be set to an empty list: [].
	// Before removing any cell from this list, you should first ensure
	// that no keyspaces are set to deploy to this cell.
	// +patchMergeKey=name
	// +patchStrategy=merge
	Cells []VitessCellTemplate `json:"cells" patchStrategy:"merge" patchMergeKey:"name"`

	// Keyspaces defines the logical databases to deploy.
	//
	// A VitessKeyspace can deploy to multiple VitessCells.
	//
	// This field is required, but it may be set to an empty list: [].
	// Before removing any keyspace from this list, you should first ensure
	// that it is undeployed from all cells by clearing the keyspace's list
	// of target cells.
	// +patchMergeKey=name
	// +patchStrategy=merge
	Keyspaces []VitessKeyspaceTemplate `json:"keyspaces,omitempty" patchStrategy:"merge" patchMergeKey:"name"`

	// ExtraVitessFlags can optionally be used to pass flags to all Vitess components.
	// WARNING: Any flags passed here must be flags that can be accepted by vtgate, vtctld and vttablet.
	// An example use-case would be topo flags.
	//
	// All entries must be key-value string pairs of the form "flag": "value". The flag name should
	// not have any prefix (just "flag", not "-flag"). To set a boolean flag,
	// set the string value to either "true" or "false".
	ExtraVitessFlags map[string]string `json:"extraVitessFlags,omitempty"`

	// TopologyReconciliation can be used to enable or disable registration or pruning of various vitess components to and from topo records.
	TopologyReconciliation *TopoReconcileConfig `json:"topologyReconciliation,omitempty"`

	// UpdateStrategy specifies how components in the Vitess cluster will be updated
	// when a revision is made to the VitessCluster spec.
	UpdateStrategy *VitessClusterUpdateStrategy `json:"updateStrategy,omitempty"`
}

// VitessClusterUpdateStrategy indicates the strategy that the operator
// will use to perform updates. It includes any additional parameters
// necessary to perform the update for the indicated strategy.
type VitessClusterUpdateStrategy struct {
	// Type selects the overall update strategy.
	//
	// Supported options are:
	//
	// - External: Schedule updates on objects that should be updated,
	//   but wait for an external tool to release them by adding the
	//   'rollout.planetscale.com/released' annotation.
	// - Immediate: Release updates to all cells, keyspaces, and shards
	//   as soon as the VitessCluster spec is changed. Perform rolling
	//   restart of one tablet Pod per shard at a time, with automatic
	//   planned reparents whenever possible to avoid master downtime.
	//
	// Default: External
	// +kubebuilder:validation:Enum=External,Immediate
	Type *VitessClusterUpdateStrategyType `json:"type,omitempty"`
}

// VitessClusterUpdateStrategyType is a string enumeration type that enumerates
// all possible update strategies for the VitessCluster.
type VitessClusterUpdateStrategyType string

const (
	// ExternalVitessClusterUpdateStrategyType relies on an external actor to release pending updates.
	ExternalVitessClusterUpdateStrategyType VitessClusterUpdateStrategyType = "External"
	// ImmediateVitessClusterUpdateStrategyType will immediately release pending updates.
	ImmediateVitessClusterUpdateStrategyType VitessClusterUpdateStrategyType = "Immediate"
)

// TopoReconcileConfig can be used to turn on or off registration or pruning of specific vitess components from topo records.
// This should only be necessary if you need to override defaults, and shouldn't be required for the vast majority of use cases.
type TopoReconcileConfig struct {
	// RegisterCellsAliases can be used to enable or disable registering cells aliases into topo records.
	// Default: true
	RegisterCellsAliases *bool `json:"registerCellsAliases,omitempty"`

	// RegisterCells can be used to enable or disable registering cells into topo records.
	// Default: true
	RegisterCells *bool `json:"registerCells,omitempty"`

	// PruneCells can be used to enable or disable pruning of extraneous cells from topo records.
	// Default: true
	PruneCells *bool `json:"pruneCells,omitempty"`

	// PruneKeyspaces can be used to enable or disable pruning of extraneous keyspaces from topo records.
	// Default: true
	PruneKeyspaces *bool `json:"pruneKeyspaces,omitempty"`

	// PruneSrvKeyspaces can be used to enable or disable pruning of extraneous serving keyspaces from topo records.
	// Default: true
	PruneSrvKeyspaces *bool `json:"pruneSrvKeyspaces,omitempty"`

	// PruneShards can be used to enable or disable pruning of extraneous shards from topo records.
	// Default: true
	PruneShards *bool `json:"pruneShards,omitempty"`

	// PruneShardCells can be used to enable or disable pruning of extraneous shard cells from topo records.
	// Default: true
	PruneShardCells *bool `json:"pruneShardCells,omitempty"`

	// PruneTablets can be used to enable or disable pruning of extraneous tablets from topo records.
	// Default: true
	PruneTablets *bool `json:"pruneTablets,omitempty"`
}

// VitessImages specifies container images to use for Vitess components.
type VitessImages struct {
	/*
		***ATTENTION***

		Make sure to keep the following up to date if you add fields here:
		  * defaultVitessImages in defaults.go
		  * DefaultVitessImages() in vitesscluster_defaults.go
	*/

	// Vtctld is the container image (including version tag) to use for Vitess Dashboard instances.
	Vtctld string `json:"vtctld,omitempty"`
	// Vtgate is the container image (including version tag) to use for Vitess Gateway instances.
	Vtgate string `json:"vtgate,omitempty"`
	// Vttablet is the container image (including version tag) to use for Vitess Tablet instances.
	Vttablet string `json:"vttablet,omitempty"`
	// Vtbackup is the container image (including version tag) to use for Vitess Backup jobs.
	Vtbackup string `json:"vtbackup,omitempty"`

	// Mysqld specifies the container image to use for mysqld, as well as
	// declaring which MySQL flavor setting in Vitess the image is
	// compatible with. Only one flavor image may be provided at a time.
	// mysqld running alongside each tablet.
	Mysqld *MysqldImage `json:"mysqld,omitempty"`
	// MysqldExporter specifies the container image to use for mysqld-exporter.
	MysqldExporter string `json:"mysqldExporter,omitempty"`
}

// MysqldImage specifies the container image to use for mysqld,
// as well as declaring which MySQL flavor setting in Vitess the
// image is compatible with.
type MysqldImage struct {
	// Mysql56Compatible is a container image (including version tag) for mysqld
	// that's compatible with the Vitess "MySQL56" flavor setting.
	Mysql56Compatible string `json:"mysql56Compatible,omitempty"`
	// Mysql80Compatible is a container image (including version tag) for mysqld
	// that's compatible with the Vitess "MySQL80" flavor setting.
	Mysql80Compatible string `json:"mysql80Compatible,omitempty"`
	// MariadbCompatible is a container image (including version tag) for mysqld
	// that's compatible with the Vitess "MariaDB" flavor setting.
	MariadbCompatible string `json:"mariadbCompatible,omitempty"`
	// Mariadb103Compatible is a container image (including version tag) for mysqld
	// that's compatible with the Vitess "MariaDB103" flavor setting.
	Mariadb103Compatible string `json:"mariadb103Compatible,omitempty"`
}

// VitessImagePullPolicies specifies container image pull policies to use for Vitess components.
type VitessImagePullPolicies struct {
	// Vtctld is the container image pull policy to use for Vitess Dashboard instances.
	Vtctld corev1.PullPolicy `json:"vtctld,omitempty"`
	// Vtgate is the container image pull policy to use for Vitess Gateway instances.
	Vtgate corev1.PullPolicy `json:"vtgate,omitempty"`
	// Vttablet is the container image pull policy to use for Vitess Tablet instances.
	Vttablet corev1.PullPolicy `json:"vttablet,omitempty"`
	// Vtbackup is the container image pull policy to use for Vitess Backup jobs.
	Vtbackup corev1.PullPolicy `json:"vtbackup,omitempty"`

	// Mysqld is the container image pull policy to use for mysqld.
	Mysqld corev1.PullPolicy `json:"mysqld,omitempty"`
	// MysqldExporter is the container image pull policy to use for mysqld-exporter.
	MysqldExporter corev1.PullPolicy `json:"mysqldExporter,omitempty"`
}

// ClusterBackupSpec configures backups for a cluster.
type ClusterBackupSpec struct {
	// Locations is a list of places where Vitess backup data for the cluster
	// can be stored. At least one storage location must be specified.
	// Within each storage location, there are multiple fields for various
	// location types (gcs, s3, etc.); exactly one such field must be populated.
	//
	// Multiple storage locations may be desired if, for example, the cluster
	// spans multiple regions. Each storage location is independent of the others;
	// backups can only be restored from the same storage location in which they
	// were originally taken.
	// +kubebuilder:validation:MinItems=1
	Locations []VitessBackupLocation `json:"locations"`
	// Engine specifies the Vitess backup engine to use, either "builtin" or "xtrabackup".
	// Note that if you change this after a Vitess cluster is already deployed,
	// you must roll the change out to all tablets and then take a new backup
	// from one tablet in each shard. Otherwise, new tablets trying to restore
	// will find that the latest backup was created with the wrong engine.
	// Default: builtin
	// +kubebuilder:validation:Enum=builtin,xtrabackup
	Engine VitessBackupEngine `json:"engine,omitempty"`
}

// VitessBackupEngine is the backup implementation to use.
type VitessBackupEngine string

const (
	// VitessBackupEngineBuiltIn uses the built-in Vitess backup engine.
	VitessBackupEngineBuiltIn VitessBackupEngine = "builtin"
	// VitessBackupEngineXtraBackup uses Percona XtraBackup for backups.
	VitessBackupEngineXtraBackup VitessBackupEngine = "xtrabackup"
)

// LockserverSpec specifies either a deployed or external lockserver,
// which can be either global or local.
type LockserverSpec struct {
	// External specifies that we should connect to an existing
	// lockserver, instead of deploying our own.
	// If this is set, all other Lockserver fields are ignored.
	External *VitessLockserverParams `json:"external,omitempty"`

	// Etcd deploys our own etcd cluster as a lockserver.
	Etcd *EtcdLockserverTemplate `json:"etcd,omitempty"`
}

// LockserverStatus is the lockserver component of status.
type LockserverStatus struct {
	// Etcd is the status of the EtcdCluster, if we were asked to deploy one.
	Etcd *EtcdLockserverStatus `json:"etcd,omitempty"`
}

// VitessLockserverParams contains only the values that Vitess needs
// to connect to a given lockserver.
type VitessLockserverParams struct {
	// Implementation specifies which Vitess "topo" plugin to use.
	Implementation string `json:"implementation"`
	// Address is the host:port of the lockserver client endpoint.
	Address string `json:"address"`
	// RootPath is a path prefix for all lockserver data belonging to a given Vitess cluster.
	// Multiple Vitess clusters can share a lockserver as long as they have unique root paths.
	RootPath string `json:"rootPath"`
}

// VitessDashboardSpec specifies deployment parameters for vtctld.
type VitessDashboardSpec struct {
	// Cells is a list of cell names (as defined in the Cells list)
	// in which to deploy vtctld.
	// Default: Deploy to all defined cells.
	Cells []string `json:"cells,omitempty"`

	// Replicas is the number of vtctld instances to deploy in each cell.
	Replicas *int32 `json:"replicas,omitempty"`

	// Resources determines the compute resources reserved for each vtctld replica.
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// ExtraFlags can optionally be used to override default flags set by the
	// operator, or pass additional flags to vtctld. All entries must be
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

	// InitContainers can optionally be used to supply extra init containers
	// that will be run to completion one after another before any app containers are started.
	InitContainers []corev1.Container `json:"initContainers,omitempty"`

	// SidecarContainers can optionally be used to supply extra containers
	// that run alongside the main containers.
	SidecarContainers []corev1.Container `json:"sidecarContainers,omitempty"`

	// Affinity allows you to set rules that constrain the scheduling of
	// your vtctld pods. WARNING: These affinity rules will override all default affinities
	// that we set; in turn, we can't guarantee optimal scheduling of your pods if you
	// choose to set this field.
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// Annotations can optionally be used to attach custom annotations to Pods
	// created for this component. These will be attached to the underlying
	// Pods that the vtctld Deployment creates.
	Annotations map[string]string `json:"annotations,omitempty"`

	// ExtraLabels can optionally be used to attach custom labels to Pods
	// created for this component. These will be attached to the underlying
	// Pods that the vtctld Deployment creates.
	ExtraLabels map[string]string `json:"extraLabels,omitempty"`
}

// VitessDashboardStatus is a summary of the status of the vtctld deployment.
type VitessDashboardStatus struct {
	// Available indicates whether the vtctld service has available endpoints.
	Available corev1.ConditionStatus `json:"available,omitempty"`
	// ServiceName is the name of the Service for this cluster's vtctld.
	ServiceName string `json:"serviceName,omitempty"`
}

// VitessClusterStatus defines the observed state of VitessCluster
// +k8s:openapi-gen=true
type VitessClusterStatus struct {
	// The generation observed by the controller.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// GlobalLockserver is the status of the global lockserver.
	GlobalLockserver LockserverStatus `json:"globalLockserver,omitempty"`

	// GatewayServiceName is the name of the cluster-wide vtgate Service.
	GatewayServiceName string `json:"gatewayServiceName,omitempty"`

	// VitessDashboard is a summary of the status of the vtctld deployment.
	VitessDashboard VitessDashboardStatus `json:"vitessDashboard,omitempty"`

	// Cells is a summary of the status of desired cells.
	Cells map[string]*VitessClusterCellStatus `json:"cells,omitempty"`
	// Keyspaces is a summary of the status of desired keyspaces.
	Keyspaces map[string]*VitessClusterKeyspaceStatus `json:"keyspaces,omitempty"`

	// OrphanedCells is a list of unwanted cells that could not be turned down.
	OrphanedCells map[string]*OrphanStatus `json:"orphanedCells,omitempty"`
	// OrphanedKeyspaces is a list of unwanted keyspaces that could not be turned down.
	OrphanedKeyspaces map[string]*OrphanStatus `json:"orphanedKeyspaces,omitempty"`
}

// NewVitessClusterStatus creates a new status object with default values.
func NewVitessClusterStatus() VitessClusterStatus {
	return VitessClusterStatus{
		VitessDashboard: VitessDashboardStatus{
			Available: corev1.ConditionUnknown,
		},
		Cells:             make(map[string]*VitessClusterCellStatus),
		Keyspaces:         make(map[string]*VitessClusterKeyspaceStatus),
		OrphanedCells:     make(map[string]*OrphanStatus),
		OrphanedKeyspaces: make(map[string]*OrphanStatus),
	}
}

// VitessClusterCellStatus is the status of a cell within a VitessCluster.
type VitessClusterCellStatus struct {
	// PendingChanges describes changes to the cell that will be
	// applied the next time a rolling update allows.
	PendingChanges string `json:"pendingChanges,omitempty"`
	// GatewayAvailable indicates whether the vtgate service is fully available.
	GatewayAvailable corev1.ConditionStatus `json:"gatewayAvailable,omitempty"`
}

// NewVitessClusterCellStatus creates a new status object with default values.
func NewVitessClusterCellStatus() *VitessClusterCellStatus {
	return &VitessClusterCellStatus{
		GatewayAvailable: corev1.ConditionUnknown,
	}
}

// VitessClusterKeyspaceStatus is the status of a keyspace within a VitessCluster.
type VitessClusterKeyspaceStatus struct {
	// PendingChanges describes changes to the keyspace that will be
	// applied the next time a rolling update allows.
	PendingChanges string `json:"pendingChanges,omitempty"`
	// DesiredShards is the number of desired shards. This is computed from
	// information that's already available in the spec, but clients should
	// use this value instead of trying to compute shard partitionings on their
	// own.
	DesiredShards int32 `json:"desiredShards,omitempty"`
	// Shards is the number of observed shards. This could be higher or lower
	// than desiredShards if the state has not yet converged.
	Shards int32 `json:"shards,omitempty"`
	// ReadyShards is the number of desired shards that are Ready.
	ReadyShards int32 `json:"readyShards,omitempty"`
	// UpdatedShards is the number of desired shards that are up-to-date
	// (have no pending changes).
	UpdatedShards int32 `json:"updatedShards,omitempty"`
	// DesiredTablets is the total number of desired tablets across all shards.
	// This is computed from information that's already available in the spec,
	// but clients should use this value instead of trying to compute shard
	// partitionings on their own.
	DesiredTablets int32 `json:"desiredTablets,omitempty"`
	// Tablets is the total number of observed tablets across all shards.
	// This could be higher or lower than desiredTablets if the state has not
	// yet converged.
	Tablets int32 `json:"tablets,omitempty"`
	// ReadyTablets is the number of desired tablets that are Ready.
	ReadyTablets int32 `json:"readyTablets,omitempty"`
	// UpdatedTablets is the number of desired tablets that are up-to-date
	// (have no pending changes).
	UpdatedTablets int32 `json:"updatedTablets,omitempty"`
	// Cells is a list of cells in which any observed tablets for this keyspace
	// are deployed.
	Cells []string `json:"cells,omitempty"`
}

// NewVitessClusterKeyspaceStatus creates a new status object with default values.
func NewVitessClusterKeyspaceStatus(spec *VitessKeyspaceTemplate) *VitessClusterKeyspaceStatus {
	// Fill in the parts of keyspace status that express desired states, which
	// we can compute from the template spec without waiting to observe
	// anything. Typically, clients would look directly at the spec to find
	// desired states, but these counts are non-trival to compute because it
	// requires computing shard partitionings. We offer this roll-up
	// information in status so clients don't need to do that.
	shards := spec.ShardTemplates()
	desiredShards := int32(len(shards))
	desiredTablets := int32(0)
	for _, shard := range shards {
		for tpIndex := range shard.TabletPools {
			desiredTablets += shard.TabletPools[tpIndex].Replicas
		}
	}

	return &VitessClusterKeyspaceStatus{
		DesiredShards:  desiredShards,
		DesiredTablets: desiredTablets,
	}
}

// OrphanStatus indiciates why a secondary object is orphaned.
type OrphanStatus struct {
	// Reason is a CamelCase token for programmatic reasoning about why the object is orphaned.
	Reason string `json:"reason"`
	// Message is a human-readable explanation for why the object is orphaned.
	Message string `json:"message"`
}

// NewOrphanStatus creates a new OrphanStatus.
func NewOrphanStatus(reason, message string) *OrphanStatus {
	return &OrphanStatus{Reason: reason, Message: message}
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VitessClusterList contains a list of VitessCluster
type VitessClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VitessCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VitessCluster{}, &VitessClusterList{})
}
