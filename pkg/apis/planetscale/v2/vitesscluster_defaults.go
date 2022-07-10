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
	DefaultVitessImages(&vt.Spec.Images, DefaultImages)
	DefaultVitessDashboard(&vt.Spec.VitessDashboard)
	DefaultVtAdmin(&vt.Spec.VtAdmin)
	DefaultVitessKeyspaceTemplates(vt.Spec.Keyspaces)
	defaultClusterBackup(vt.Spec.Backup)
	DefaultTopoReconcileConfig(&vt.Spec.TopologyReconciliation)
	DefaultUpdateStrategy(&vt.Spec.UpdateStrategy)
	DefaultServiceOverrides(&vt.Spec.GatewayService)
	DefaultServiceOverrides(&vt.Spec.TabletService)
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
	if dst.Vtadmin == "" {
		dst.Vtadmin = src.Vtadmin
	}
	if dst.Vtorc == "" {
		dst.Vtorc = src.Vtorc
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
	DefaultServiceOverrides(&(*dashboard).Service)
}

func DefaultVtAdmin(dashboard **VtAdminSpec) {
	// Do not deploy vtadmin if not specified.
	if *dashboard == nil {
		return
	}
	if (*dashboard).Replicas == nil {
		(*dashboard).Replicas = pointer.Int32Ptr(defaultVtadminReplicas)
	}
	if len((*dashboard).WebResources.Requests) == 0 {
		(*dashboard).WebResources.Requests = corev1.ResourceList{
			corev1.ResourceCPU:    *resource.NewMilliQuantity(defaultVtadminCPUMillis, resource.DecimalSI),
			corev1.ResourceMemory: *resource.NewQuantity(defaultVtadminMemoryBytes, resource.BinarySI),
		}
	}
	if len((*dashboard).WebResources.Limits) == 0 {
		(*dashboard).WebResources.Limits = corev1.ResourceList{
			corev1.ResourceMemory: *resource.NewQuantity(defaultVtadminMemoryBytes, resource.BinarySI),
		}
	}
	if len((*dashboard).APIResources.Requests) == 0 {
		(*dashboard).APIResources.Requests = corev1.ResourceList{
			corev1.ResourceCPU:    *resource.NewMilliQuantity(defaultVtadminCPUMillis, resource.DecimalSI),
			corev1.ResourceMemory: *resource.NewQuantity(defaultVtadminMemoryBytes, resource.BinarySI),
		}
	}
	if len((*dashboard).APIResources.Limits) == 0 {
		(*dashboard).APIResources.Limits = corev1.ResourceList{
			corev1.ResourceMemory: *resource.NewQuantity(defaultVtadminMemoryBytes, resource.BinarySI),
		}
	}
	DefaultServiceOverrides(&(*dashboard).Service)
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

	if keyspace.DurabilityPolicy == "" {
		keyspace.DurabilityPolicy = "none"
	}

	for i := range keyspace.Partitionings {
		partition := &keyspace.Partitionings[i]
		defaultCustomPartitioning(partition.Custom)
		defaultEqualPartitioning(partition.Equal)
	}
}

func defaultCustomPartitioning(customPartition *VitessKeyspaceCustomPartitioning) {
	if customPartition == nil {
		return
	}

	for i := range customPartition.Shards {
		DefaultVitessShardTemplate(&customPartition.Shards[i].VitessShardTemplate)
	}
}

func defaultEqualPartitioning(equalPartition *VitessKeyspaceEqualPartitioning) {
	if equalPartition == nil {
		return
	}

	DefaultVitessShardTemplate(&equalPartition.ShardTemplate)
}

func defaultClusterBackup(backup *ClusterBackupSpec) {
	if backup == nil {
		return
	}
	if backup.Engine == "" {
		backup.Engine = defaultBackupEngine
	}
}

func DefaultTopoReconcileConfig(confPtr **TopoReconcileConfig) {
	if *confPtr == nil {
		*confPtr = &TopoReconcileConfig{}
	}
	conf := *confPtr

	// Defaulting registration code.
	if conf.RegisterCells == nil {
		conf.RegisterCells = pointer.BoolPtr(true)
	}
	if conf.RegisterCellsAliases == nil {
		conf.RegisterCellsAliases = pointer.BoolPtr(true)
	}

	// Defaulting pruning code.
	if conf.PruneCells == nil {
		conf.PruneCells = pointer.BoolPtr(true)
	}
	if conf.PruneKeyspaces == nil {
		conf.PruneKeyspaces = pointer.BoolPtr(true)
	}
	if conf.PruneShards == nil {
		conf.PruneShards = pointer.BoolPtr(true)
	}
	if conf.PruneShardCells == nil {
		conf.PruneShardCells = pointer.BoolPtr(true)
	}
	if conf.PruneTablets == nil {
		conf.PruneTablets = pointer.BoolPtr(true)
	}
	if conf.PruneSrvKeyspaces == nil {
		conf.PruneSrvKeyspaces = pointer.BoolPtr(true)
	}
}

func DefaultUpdateStrategy(updateStratPtr **VitessClusterUpdateStrategy) {
	if *updateStratPtr == nil {
		*updateStratPtr = &VitessClusterUpdateStrategy{}
	}

	updateStrat := *updateStratPtr

	defaultUpdateStrategyType := ExternalVitessClusterUpdateStrategyType

	if updateStrat.Type == nil {
		updateStrat.Type = &defaultUpdateStrategyType
	}

	if *updateStrat.Type == ExternalVitessClusterUpdateStrategyType {
		if updateStrat.External == nil {
			updateStrat.External = &ExternalVitessClusterUpdateStrategyOptions{}
		}
	}
}

// DefaultServiceOverrides applies defaults to a ServiceOverrides field.
func DefaultServiceOverrides(so **ServiceOverrides) {
	if *so == nil {
		*so = &ServiceOverrides{}
	}
}
