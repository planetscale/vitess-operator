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
	corev1 "k8s.io/api/core/v1"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
)

func ShardDiskSize(dst []planetscalev2.VitessShardTabletPool, src []planetscalev2.VitessShardTabletPool) {
	for i := range dst {
		if src[i].DataVolumeClaimTemplate == nil {
			continue
		}

		srcTabletDiskSize := src[i].DataVolumeClaimTemplate.Resources.Requests[corev1.ResourceStorage]
		dst[i].DataVolumeClaimTemplate.Resources.Requests[corev1.ResourceStorage] = srcTabletDiskSize
	}
}

// KeyspaceDiskSize updates values in 'dst' based on values in 'src'.
// It leaves extra entries (found in 'dst' but not in 'src') untouched,
// since those might be set by mutating admission webhooks or other controllers.
// TODO: Add unit test to check that various buried disk sizes get updated and
// check that invalid cases (not matching) are left untouched. We can also check
// that nothing besides the disk sizes is touched.
func KeyspaceDiskSize(dst, src *planetscalev2.VitessKeyspaceTemplate) {
	// Check that the keyspace definitions line up.
	if !validateKeyspacePartitionings(dst.Partitionings, src.Partitionings) {
		return
	}

	updateDiskSize(dst, src)
}

func updateDiskSize(dst, src *planetscalev2.VitessKeyspaceTemplate) {
	for i := range dst.Partitionings {
		dstPartitioning := &dst.Partitionings[i]
		if dstPartitioning.Equal != nil {
			updateEqualPartitioningDiskSize(dstPartitioning.Equal, src.Partitionings[i].Equal)
		} else {
			updateCustomPartitioningDiskSize(dstPartitioning.Custom, src.Partitionings[i].Custom)
		}
	}
}

func updateEqualPartitioningDiskSize(dst, src *planetscalev2.VitessKeyspaceEqualPartitioning) {
	for i := range dst.ShardTemplate.TabletPools {
		dstTablet := &dst.ShardTemplate.TabletPools[i]

		requestedTablet := matchingTabletPool(dstTablet, src.ShardTemplate.TabletPools)
		if requestedTablet == nil || requestedTablet.DataVolumeClaimTemplate == nil {
			continue
		}

		dstRequests := &dstTablet.DataVolumeClaimTemplate.Resources.Requests
		srcRequests := requestedTablet.DataVolumeClaimTemplate.Resources.Requests

		// Apply the disk resource changes.
		StorageResource(dstRequests, srcRequests)
	}
}

func updateCustomPartitioningDiskSize(dst, src *planetscalev2.VitessKeyspaceCustomPartitioning) {
	for i := range dst.Shards {
		dstShard := &dst.Shards[i]
		srcShard := &src.Shards[i]

		for j := range dstShard.TabletPools {
			dstTablet := &dstShard.TabletPools[j]

			requestedTablet := matchingTabletPool(dstTablet, srcShard.TabletPools)
			if requestedTablet == nil || requestedTablet.DataVolumeClaimTemplate == nil {
				continue
			}

			dstRequests := &dstTablet.DataVolumeClaimTemplate.Resources.Requests
			srcRequests := requestedTablet.DataVolumeClaimTemplate.Resources.Requests

			// Apply the disk resource changes.
			StorageResource(dstRequests, srcRequests)
		}
	}
}

func matchingTabletPool(dst *planetscalev2.VitessShardTabletPool, src []planetscalev2.VitessShardTabletPool) *planetscalev2.VitessShardTabletPool{
	for i := range src {
		srcTablet := &src[i]
		if srcTablet.IsMatch(dst) {
			return srcTablet
		}
	}

	return nil
}

func validateKeyspacePartitionings(dst, src []planetscalev2.VitessKeyspacePartitioning) bool {
	// Check that the list of partitionings are the same length.
	if len(dst) != len(src) {
		return false
	}

	for i := range dst {
		// Validate that partitionings have the same type and shard count.
		if !validatePartitionings(&dst[i], &src[i]) {
			return false
		}
	}

	return true
}

func validatePartitionings(dst *planetscalev2.VitessKeyspacePartitioning, src *planetscalev2.VitessKeyspacePartitioning) bool {
	if !validatePartitioningTypes(dst, src) {
		return false
	}

	if dst.Equal != nil {
		return validateEqualPartitioning(dst.Equal, src.Equal)
	}
	if dst.Custom != nil {
		return validateCustomPartitioning(dst.Custom, src.Custom)
	}
	return false
}

func validatePartitioningTypes(dst *planetscalev2.VitessKeyspacePartitioning, src *planetscalev2.VitessKeyspacePartitioning) bool {
	if dst.Equal != nil && src.Equal == nil {
		return false
	} else if dst.Equal == nil && src.Equal != nil {
		return false
	}

	if dst.Custom != nil && src.Custom == nil {
		return false
	} else if dst.Custom == nil && src.Custom != nil  {
		return false
	}

	return true
}

func validateEqualPartitioning(dst *planetscalev2.VitessKeyspaceEqualPartitioning, src *planetscalev2.VitessKeyspaceEqualPartitioning) bool {
	// Validate that the number of shards is the same.
	if dst.Parts != src.Parts {
		return false
	}

	return true
 }

func validateCustomPartitioning(dst *planetscalev2.VitessKeyspaceCustomPartitioning, src *planetscalev2.VitessKeyspaceCustomPartitioning) bool {
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