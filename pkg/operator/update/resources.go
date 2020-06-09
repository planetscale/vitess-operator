/*
Copyright 2020 PlanetScale Inc.

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

package update

import (
	"strconv"

	corev1 "k8s.io/api/core/v1"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
)

// ShardDiskSize updates values in 'dst' based on values in 'src'.
// It does not update any other values besides disk sizes.
// It leaves extra entries (found in 'dst' but not in 'src') untouched.
func ShardDiskSize(dst []planetscalev2.VitessShardTabletPool, src []planetscalev2.VitessShardTabletPool) {
	updateTabletPoolDiskSize(dst, src)
}

// KeyspaceDiskSize updates disk sizes in 'dst' based on values in 'src'.
// It does not update any other values besides disk sizes.
// It leaves extra entries (found in 'dst' but not in 'src') untouched.
func KeyspaceDiskSize(dst, src *planetscalev2.VitessKeyspaceTemplate) {
	// Check that the keyspace definitions line up.
	if !keyspacePartitioningsAreValid(dst.Partitionings, src.Partitionings) {
		return
	}

	updateDiskSize(dst, src)
}

func updateDiskSize(dst, src *planetscalev2.VitessKeyspaceTemplate) {
	for i := range dst.Partitionings {
		dstPartitioning := &dst.Partitionings[i]
		srcPartitioning := &src.Partitionings[i]

		if dstPartitioning.Equal != nil {
			updateTabletPoolDiskSize(dstPartitioning.Equal.ShardTemplate.TabletPools, srcPartitioning.Equal.ShardTemplate.TabletPools)
		}
		if dstPartitioning.Custom != nil {
			for j := range dstPartitioning.Custom.Shards {
				dstShard := dstPartitioning.Custom.Shards[j]
				srcShard := srcPartitioning.Custom.Shards[j]

				updateTabletPoolDiskSize(dstShard.TabletPools, srcShard.TabletPools)
			}
		}
	}
}

func updateTabletPoolDiskSize(dst, src []planetscalev2.VitessShardTabletPool) {
	for i := range dst {
		dstTablet := &dst[i]

		requestedTablet := matchingTabletPool(dstTablet, src)
		if requestedTablet == nil || requestedTablet.DataVolumeClaimTemplate == nil {
			continue
		}

		dstRequests := &dstTablet.DataVolumeClaimTemplate.Resources.Requests
		srcRequests := requestedTablet.DataVolumeClaimTemplate.Resources.Requests

		// Apply the disk resource changes.
		StorageResource(dstRequests, srcRequests)
	}
}

func matchingTabletPool(dst *planetscalev2.VitessShardTabletPool, src []planetscalev2.VitessShardTabletPool) *planetscalev2.VitessShardTabletPool {
	for i := range src {
		srcTablet := &src[i]
		if srcTablet.IsMatch(dst) {
			return srcTablet
		}
	}

	return nil
}

func keyspacePartitioningsAreValid(dst, src []planetscalev2.VitessKeyspacePartitioning) bool {
	// Check that the list of partitionings are the same length.
	if len(dst) != len(src) {
		return false
	}

	for i := range dst {
		// Validate that partitionings have the same type and shard count.
		if !partitioningsMatch(&dst[i], &src[i]) {
			return false
		}
	}

	return true
}

func partitioningsMatch(dst *planetscalev2.VitessKeyspacePartitioning, src *planetscalev2.VitessKeyspacePartitioning) bool {
	if !partitioningTypesMatch(dst, src) {
		return false
	}

	if dst.Equal != nil {
		return equalPartitioningsMatch(dst.Equal, src.Equal)
	}
	if dst.Custom != nil {
		return customPartitioningsMatch(dst.Custom, src.Custom)
	}
	return false
}

func partitioningTypesMatch(dst *planetscalev2.VitessKeyspacePartitioning, src *planetscalev2.VitessKeyspacePartitioning) bool {
	if dst.Equal != nil && src.Equal == nil {
		return false
	} else if dst.Equal == nil && src.Equal != nil {
		return false
	}

	if dst.Custom != nil && src.Custom == nil {
		return false
	} else if dst.Custom == nil && src.Custom != nil {
		return false
	}

	return true
}

func equalPartitioningsMatch(dst *planetscalev2.VitessKeyspaceEqualPartitioning, src *planetscalev2.VitessKeyspaceEqualPartitioning) bool {
	// Validate that the number of shards is the same.
	if dst.Parts != src.Parts {
		return false
	}

	return true
}

func customPartitioningsMatch(dst *planetscalev2.VitessKeyspaceCustomPartitioning, src *planetscalev2.VitessKeyspaceCustomPartitioning) bool {
	// Validate that the number of shards is the same.
	if len(dst.Shards) != len(src.Shards) {
		return false
	}

	for i := range dst.Shards {
		// For a custom partitioning, we must make sure that the key ranges match.
		if dst.Shards[i].KeyRange != src.Shards[i].KeyRange {
			return false
		}
	}

	return true
}

// StorageResource updates disk size entries in 'dst' based on the values in 'src'.
func StorageResource(dst *corev1.ResourceList, src corev1.ResourceList) {
	srcVal, ok := src[corev1.ResourceStorage]
	if !ok {
		return
	}
	if *dst == nil {
		*dst = make(corev1.ResourceList)
	}
	(*dst)[corev1.ResourceStorage] = srcVal
}

// GOMAXPROCS sets the GOMAXPROCS env var if CPU resource limits are configured.
func GOMAXPROCS(dst *[]corev1.EnvVar, resources corev1.ResourceRequirements) {
	// We only set GOMAXPROCS if a CPU limit is set.
	// Requests are irrelevant because they don't trigger CPU throttling.
	cpu, ok := resources.Limits[corev1.ResourceCPU]
	if !ok {
		return
	}
	// Value() rounds up to the nearest integer.
	gomaxprocs := cpu.Value()

	Env(dst, []corev1.EnvVar{
		{
			Name:  "GOMAXPROCS",
			Value: strconv.FormatInt(gomaxprocs, 10),
		},
	})
}
