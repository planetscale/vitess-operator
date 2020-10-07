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
)

// DefaultVitessKeyspace fills in VitessKeyspace defaults for unspecified fields.
// Note: This should only be used for nillable fields passed down from a parent because controllers run in parallel,
// and the defaulting code for a parent object may not have been run yet, meaning the values passed down from that parent
// might not be safe to deref.
func DefaultVitessKeyspace(dst *VitessKeyspace) {
	DefaultVitessOrchestrator(&dst.Spec.VitessOrchestrator)
	DefaultTopoReconcileConfig(&dst.Spec.TopologyReconciliation)
	DefaultUpdateStrategy(&dst.Spec.UpdateStrategy)
}

func DefaultVitessOrchestrator(vtorc **VitessOrchestratorSpec) {
	// If no vtorc is specified, we don't launch any.
	if *vtorc == nil {
		return
	}
	if len((*vtorc).Resources.Requests) == 0 {
		(*vtorc).Resources.Requests = corev1.ResourceList{
			corev1.ResourceCPU:    *resource.NewMilliQuantity(defaultVtorcCPUMillis, resource.DecimalSI),
			corev1.ResourceMemory: *resource.NewQuantity(defaultVtorcMemoryBytes, resource.BinarySI),
		}
	}
	if len((*vtorc).Resources.Limits) == 0 {
		(*vtorc).Resources.Limits = corev1.ResourceList{
			corev1.ResourceMemory: *resource.NewQuantity(defaultVtorcMemoryBytes, resource.BinarySI),
		}
	}
	DefaultServiceOverrides(&(*vtorc).Service)
}

// DefaultVitessKeyspaceImages fills in unspecified keyspace-level images from cluster-level defaults.
// The clusterDefaults should have already had its unspecified fields filled in with operator defaults.
func DefaultVitessKeyspaceImages(dst *VitessKeyspaceImages, clusterDefaults *VitessImages) {
	if dst.Vttablet == "" {
		dst.Vttablet = clusterDefaults.Vttablet
	}
	if dst.Vtorc == "" {
		dst.Vtorc = clusterDefaults.Vtorc
	}
	if dst.Vtbackup == "" {
		dst.Vtbackup = clusterDefaults.Vtbackup
	}
	if dst.Mysqld == nil {
		dst.Mysqld = clusterDefaults.Mysqld
	}
	if dst.MysqldExporter == "" {
		dst.MysqldExporter = clusterDefaults.MysqldExporter
	}
}
