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
	"k8s.io/utils/ptr"
)

// DefaultVitessShard fills in VitessShard defaults for unspecified fields.
// Note: This should only be used for nillable fields passed down from a parent because controllers run in parallel,
// and the defaulting code for a parent object may not have been run yet, meaning the values passed down from that parent
// might not be safe to deref.
func DefaultVitessShard(dst *VitessShard) {
	DefaultUpdateStrategy(&dst.Spec.UpdateStrategy)
	DefaultTopoReconcileConfig(&dst.Spec.TopologyReconciliation)
	DefaultVitessShardTemplate(&dst.Spec.VitessShardTemplate)
}

func DefaultVitessShardTemplate(shardTemplate *VitessShardTemplate) {
	if shardTemplate == nil {
		return
	}

	DefaultVitessReplicationSpec(&shardTemplate.Replication)
}

func DefaultVitessReplicationSpec(replicationSpec *VitessReplicationSpec) {
	// Enable initialization of replication by default.
	if replicationSpec.InitializeMaster == nil {
		replicationSpec.InitializeMaster = ptr.To(true)
	}

	// Enable initialization of backup by default.
	if replicationSpec.InitializeBackup == nil {
		replicationSpec.InitializeBackup = ptr.To(true)
	}

	// Enable replication repair by default.
	if replicationSpec.RecoverRestartedMaster == nil {
		replicationSpec.RecoverRestartedMaster = ptr.To(true)
	}
}
