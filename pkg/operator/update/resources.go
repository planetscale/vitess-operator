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

func KeyspaceDiskSize(dst *planetscalev2.VitessKeyspaceTemplate, src planetscalev2.VitessKeyspaceTemplate) {
	// Check that the keyspace definitions line up.
	if !validateKeyspacePartitionings(*dst,src) {
		return
	}

	updateDiskSize(dst, src)
}

func updateDiskSize(dst *planetscalev2.VitessKeyspaceTemplate, src planetscalev2.VitessKeyspaceTemplate) {
	for i := range dst.Partitionings {
		dstPartitioning := dst.Partitionings[i]
		if dstPartitioning.Equal != nil {
			updatePartitioningDiskSize(dstPartitioning.Equal, *src.Partitionings[i].Equal)
		}
	}
}

func updatePartitioningDiskSize(dst *planetscalev2.VitessKeyspaceEqualPartitioning, src planetscalev2.VitessKeyspaceEqualPartitioning) {
	srcLoop:
		for i := range dst.ShardTemplate.TabletPools {
			dstTablet := dst.ShardTemplate.TabletPools[i]
			var requestedTablet *planetscalev2.VitessShardTabletPool

			for j := range src.ShardTemplate.TabletPools {
				srcTablet := src.ShardTemplate.TabletPools[j]
				// Match each dst tablet pool with its unique Type, Cell tuple in src.
				if srcTablet.Type == dstTablet.Type && srcTablet.Cell == dstTablet.Cell {
					requestedTablet = &srcTablet
				}

				// If we can't find a match, continue and try to find matches for other tablet pools.
				if requestedTablet == nil {
					continue srcLoop
				}

				if requestedTablet.DataVolumeClaimTemplate == nil {
					continue srcLoop
				}
			}

			srcRequests := &requestedTablet.DataVolumeClaimTemplate.Resources.Requests
			dstRequests := &dstTablet.DataVolumeClaimTemplate.Resources.Requests

			DiskResource(dstRequests, srcRequests)
		}
}

func validateKeyspacePartitionings(dst planetscalev2.VitessKeyspaceTemplate, src planetscalev2.VitessKeyspaceTemplate) bool {
	// Check that the list of partitionings are the same length.
	if len(dst.Partitionings) != len(src.Partitionings) {
		return false
	}

	for i := range dst.Partitionings {
		// Validate that partitionings have the same type and shard count.
		if !validatePartitionings(&dst.Partitionings[i], &src.Partitionings[i]) {
			return false
		}
	}

	return true
}

func validatePartitionings(dst *planetscalev2.VitessKeyspacePartitioning, src *planetscalev2.VitessKeyspacePartitioning) bool {
	if !validatePartitioningTypes(dst, src) {
		return false
	}

	valid := false
	if dst.Equal != nil {
		valid = validateEqualPartitioning(dst.Equal, src.Equal)
	} else {
		valid = validateCustomPartitioning(dst.Custom, src.Custom)
	}

	return valid
}

func validatePartitioningTypes(dst *planetscalev2.VitessKeyspacePartitioning, src *planetscalev2.VitessKeyspacePartitioning) bool {
	if dst.Equal != nil && src.Equal == nil {
		return false
	} else if dst.Equal == nil && src.Equal != nil {
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

// DiskResource updates disk size entries in 'dst' based on the values in 'src'.
func DiskResource(dst, src *corev1.ResourceList) {
	if *dst == nil {
		if len(*src) > 0 {
			*dst = *src
		}
		return
	}
	for srcKey, srcVal := range *src {
		if srcKey != corev1.ResourceStorage {
			continue
		}
		(*dst)[srcKey] = srcVal
	}
}
