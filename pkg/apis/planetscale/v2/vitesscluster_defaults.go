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
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/pointer"
)

// DefaultVitessCluster fills in default values for unspecified fields.
func DefaultVitessCluster(vt *VitessCluster) {
	defaultGlobalLockserver(vt)
	DefaultVitessImages(&vt.Spec.Images, defaultVitessImages)
	DefaultVitessDashboard(&vt.Spec.VitessDashboard)
	DefaultVitessKeyspaceTemplates(vt.Spec.Keyspaces)
	defaultClusterBackup(vt.Spec.Backup)
	defaultTopoReconcileConfig(vt)
}

func defaultGlobalLockserver(vt *VitessCluster) {
	gls := &vt.Spec.GlobalLockserver
	if gls.External != nil {
		// There's nothing to do if an external lockserver was specified.
		// We'll just pass those params to Vitess.
		return
	}
	if gls.Etcd == nil {
		// By default, deploy our own etcd cluster with default settings.
		gls.Etcd = &EtcdLockserverTemplate{}
	}
	DefaultEtcdLockserverTemplate(gls.Etcd)
}

// DefaultVitessImages copies images from src to dst to fill any
// unspecified values in dst.
func DefaultVitessImages(dst *VitessImages, src *VitessImages) {
	if dst.Vtctld == "" {
		dst.Vtctld = src.Vtctld
	}
	if dst.Vtgate == "" {
		dst.Vtgate = src.Vtgate
	}
	if dst.Vttablet == "" {
		dst.Vttablet = src.Vttablet
	}
	if dst.Vtbackup == "" {
		dst.Vtbackup = src.Vtbackup
	}
	if dst.Mysqld == nil {
		dst.Mysqld = src.Mysqld
	}
	if dst.MysqldExporter == "" {
		dst.MysqldExporter = src.MysqldExporter
	}
}

func DefaultVitessDashboard(dashboard **VitessDashboardSpec) {
	if *dashboard == nil {
		*dashboard = &VitessDashboardSpec{}
	}
	if (*dashboard).Replicas == nil {
		(*dashboard).Replicas = pointer.Int32Ptr(defaultVtctldReplicas)
	}
	if len((*dashboard).Resources.Requests) == 0 {
		(*dashboard).Resources.Requests = corev1.ResourceList{
			corev1.ResourceCPU:    *resource.NewMilliQuantity(defaultVtctldCPUMillis, resource.DecimalSI),
			corev1.ResourceMemory: *resource.NewQuantity(defaultVtctldMemoryBytes, resource.BinarySI),
		}
	}
	if len((*dashboard).Resources.Limits) == 0 {
		(*dashboard).Resources.Limits = corev1.ResourceList{
			corev1.ResourceMemory: *resource.NewQuantity(defaultVtctldMemoryBytes, resource.BinarySI),
		}
	}
}

func DefaultVitessKeyspaceTemplates(keyspaces []VitessKeyspaceTemplate) {
	for i := range keyspaces {
		DefaultVitessKeyspaceTemplate(&keyspaces[i])
	}
}

func DefaultVitessKeyspaceTemplate(keyspace *VitessKeyspaceTemplate) {
	if keyspace.TurndownPolicy == "" {
		keyspace.TurndownPolicy = VitessKeyspaceTurndownPolicyRequireIdle
	}
}

func defaultClusterBackup(backup *ClusterBackupSpec) {
	if backup == nil {
		return
	}
	if backup.Engine == "" {
		backup.Engine = defaultBackupEngine
	}
}

func defaultTopoReconcileConfig(vt *VitessCluster) {
	// Defaulting registration code.
	if vt.Spec.TopoReconciliation.RegisterCells == nil {
		registerCells := true
		vt.Spec.TopoReconciliation.RegisterCells = &registerCells
	}
	if vt.Spec.TopoReconciliation.RegisterCellsAliases == nil {
		registerCellsAliases := true
		vt.Spec.TopoReconciliation.RegisterCellsAliases = &registerCellsAliases
	}

	// Defaulting pruning code.
	if vt.Spec.TopoReconciliation.PruneCells == nil {
		pruneCells := true
		vt.Spec.TopoReconciliation.PruneCells = &pruneCells
	}
	if vt.Spec.TopoReconciliation.PruneKeyspaces == nil {
		pruneKeyspaces := true
		vt.Spec.TopoReconciliation.PruneKeyspaces = &pruneKeyspaces
	}
	if vt.Spec.TopoReconciliation.PruneShards == nil {
		pruneShards := true
		vt.Spec.TopoReconciliation.PruneShards = &pruneShards
	}
	if vt.Spec.TopoReconciliation.PruneShardCells == nil {
		pruneShardCells := true
		vt.Spec.TopoReconciliation.PruneShardCells = &pruneShardCells
	}
	if vt.Spec.TopoReconciliation.PruneTablets == nil {
		pruneTablets := true
		vt.Spec.TopoReconciliation.PruneTablets = &pruneTablets
	}
	if vt.Spec.TopoReconciliation.PruneSrvKeyspaces == nil {
		pruneSrvingKeyspaces := true
		vt.Spec.TopoReconciliation.PruneSrvKeyspaces = &pruneSrvingKeyspaces
	}
}
