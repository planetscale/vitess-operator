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

package vitessshard

import (
	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/names"
)

func name(clusterName, keyspaceName, shardSafeName string) string {
	return names.Join(clusterName, keyspaceName, shardSafeName)
}

// Name returns the VitessShard metadata.name for a given shard.
func Name(clusterName, keyspaceName string, keyRange planetscalev2.VitessKeyRange) string {
	return name(clusterName, keyspaceName, keyRange.SafeName())
}

// NameFromLabels returns the VitessShard metadata.name for a given shard,
// based on the standard PlanetScale label values found in the given label set.
func NameFromLabels(labels map[string]string) string {
	clusterName := labels[planetscalev2.ClusterLabel]
	keyspaceName := labels[planetscalev2.KeyspaceLabel]
	shardSafeName := labels[planetscalev2.ShardLabel]
	return name(clusterName, keyspaceName, shardSafeName)
}
