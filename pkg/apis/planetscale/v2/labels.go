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

const (
	// LabelPrefix is the prefix for label keys that belong to us.
	// We should use this prefix for all our labels to avoid conflicts.
	LabelPrefix = "planetscale.com"

	// ComponentLabel is the key for identifying which component of the operator manages an object.
	ComponentLabel = LabelPrefix + "/" + "component"
	// ClusterLabel is the key for identifying the VitessCluster instance to which an object belongs.
	ClusterLabel = LabelPrefix + "/" + "cluster"
	// CellLabel is the key for identifying the Vitess cell to which an object belongs.
	CellLabel = LabelPrefix + "/" + "cell"
	// KeyspaceLabel is the key for identifying the Vitess keyspace to which an object belongs.
	KeyspaceLabel = LabelPrefix + "/" + "keyspace"
	// ShardLabel is the key for identifying the Vitess shard to which an object belongs.
	ShardLabel = LabelPrefix + "/" + "shard"
	// TabletUidLabel is the key for identifying the Vitess tablet UID for a Pod.
	TabletUidLabel = LabelPrefix + "/" + "tablet-uid"
	// TabletTypeLabel is the key for identifying the Vitess target tablet type for a Pod.
	TabletTypeLabel = LabelPrefix + "/" + "tablet-type"
	// TabletIndexLabel is the key for identifying the index of a Vitess tablet within its pool.
	TabletIndexLabel = LabelPrefix + "/" + "tablet-index"

	// VtctldComponentName is the ComponentLabel value for vtctld.
	VtctldComponentName = "vtctld"
	// VtgateComponentName is the ComponentLabel value for vtgate.
	VtgateComponentName = "vtgate"
	// VttabletComponentName is the ComponentLabel value for vttablet.
	VttabletComponentName = "vttablet"
	// VtbackupComponentName is the ComponentLabel value for vtbackup.
	VtbackupComponentName = "vtbackup"
	// EtcdComponentName is the ComponentLabel value for etcd.
	EtcdComponentName = "etcd"
	// VBSSubcontrollerComponentName is the ComponentLabel value for the vitessbackupstorage subcontroller.
	VBSSubcontrollerComponentName = "vbs-subcontroller"

	// ReplicaTabletPoolName is the TabletPoolLabel value for REPLICA tablets.
	ReplicaTabletPoolName = "replica"
	// RdonlyTabletPoolName is the TabletPoolLabel value for RDONLY tablets.
	RdonlyTabletPoolName = "rdonly"
	// ExternalMasterTabletPoolName is the TabletPoolLabel value for EXTERNALMASTER tablets.
	ExternalMasterTabletPoolName = "externalmaster"
	// ExternalReplicaTabletPoolName is the TabletPoolLabel value for EXTERNALREPLICA tablets.
	ExternalReplicaTabletPoolName = "externalreplica"
	// ExternalRdonlyTabletPoolName is the TabletPoolLabel value for EXTERNALRDONLY tablets.
	ExternalRdonlyTabletPoolName = "externalrdonly"
)

var (
	// VitessComponentNames is a list of all ComponentLabel values that
	// correspond to long-running (non-batch) Vitess components.
	VitessComponentNames = []string{
		VtctldComponentName,
		VtgateComponentName,
		VttabletComponentName,
	}
)
