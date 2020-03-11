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

package vttablet

import (
	"crypto/md5"
	"encoding/binary"
	"fmt"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
)

/*
UID deterministically generates a 32-bit unsigned integer that should uniquely
identify a given tablet within a Vitess cluster.

The tablet's identity is defined as the tuple (cell,keyspace,shard,pool,index).
Any such tuple must map to only one uint32 value (the same tuple always results
in the same integer), and there must be a negligible probability of accidental
collisions within a given Vitess cluster.

The approach we use here is to take the first 32 bits of a hash of those tuple
elements, which is essentially how YouTube did it. If we assume the first 32
bits of the hash fall in a uniform distribution, the probability of at least
one collision in a cluster with 1000 total tablets is about 0.0001. That means
we should expect to have one cluster experience a collision by the time we reach
10,000 clusters if each cluster has 1000 tablets. This should provide enough
lead time to develop a smarter way to handle tablet identity and MySQL server
IDs in Vitess itself.

WARNING: DO NOT change the behavior of this function, as that may result in
         the deletion and recreation of all tablets.
*/
func UID(cellName, keyspaceName string, shardKeyRange planetscalev2.VitessKeyRange, tabletPoolType planetscalev2.VitessTabletPoolType, tabletIndex uint32) uint32 {
	h := md5.New()
	fmt.Fprintln(h, cellName, keyspaceName, shardKeyRange.String(), string(tabletPoolType), tabletIndex)
	sum := h.Sum(nil)
	return binary.BigEndian.Uint32(sum[:4])
}
