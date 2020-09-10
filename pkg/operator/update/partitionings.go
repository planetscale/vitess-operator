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
	"strings"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
)

// PartitioningSet adds, removes, or reorders partitionings in dst as needed to
// keep the set of partitionings in sync with src, without changing the content
// of any partitioning that already existed in dst.
func PartitioningSet(dst *[]planetscalev2.VitessKeyspacePartitioning, src []planetscalev2.VitessKeyspacePartitioning) {
	// Make a map of partitionings that already exist in dst.
	// If there are multiple partitionings with the same key, we take the latest
	// one in the list to be consistent with the deduplication rule used in
	// VitessKeyspaceTemplate.ShardTemplates().
	dstMap := make(map[string]planetscalev2.VitessKeyspacePartitioning, len(*dst))
	for _, partitioning := range *dst {
		dstMap[partitioningKey(partitioning)] = partitioning
	}

	// Make a new slice of partitionings based on src, but replace the content
	// with that from dst for any partitionings that already existed.
	result := make([]planetscalev2.VitessKeyspacePartitioning, len(src))

	for i, partitioning := range src {
		key := partitioningKey(partitioning)

		if dstPartitioning, exists := dstMap[key]; exists {
			partitioning = dstPartitioning
		}

		result[i] = partitioning
	}

	*dst = result
}

// partitioningKey generates a map key to identify a partitioning by the set of
// shards it contains. Any two partitionings that specify the same set of shard
// ranges will be given the same key.
func partitioningKey(partitioning planetscalev2.VitessKeyspacePartitioning) string {
	return strings.Join(partitioning.ShardNameSet().List(), ",")
}
