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
	"encoding/hex"

	"planetscale.dev/vitess-operator/pkg/operator/partitioning"
	topodatapb "vitess.io/vitess/go/vt/proto/topodata"
)

/*
KeyRanges returns the set of ranges for an equal partitioning.

The business logic for figuring out the key ranges happens in `pkg/operator/partitioning`, and this method simply
calls that function, which returns the key ranges as an array of raw vitess KeyRange type (start and end as []byte).
This function then translates that into VitessKeyRanges (start and end as hex strings) and returns the []VitessKeyRange.
*/
func (p *VitessKeyspaceEqualPartitioning) KeyRanges() []VitessKeyRange {
	byteKeyRanges := partitioning.EqualKeyRanges(uint64(p.Parts))
	vitessKeyRanges := make([]VitessKeyRange, len(byteKeyRanges))

	for i, byteKeyRange := range byteKeyRanges {
		vitessKeyRanges[i].FillFromByteKeyRange(byteKeyRange)
	}

	return vitessKeyRanges
}

// FillFromByteKeyRange is a method to fill a VitessKeyRange from a topodatapb.KeyRange (encoding from []byte to
// hex string).
func (v *VitessKeyRange) FillFromByteKeyRange(kr topodatapb.KeyRange) {
	v.Start = hex.EncodeToString(kr.Start)
	v.End = hex.EncodeToString(kr.End)
}

// ShardTemplates returns a list of shards to satisfy all partitionings defined in the keyspace.
// The list is returned in sorted order for determinism.
func (spec *VitessKeyspaceTemplate) ShardTemplates() []*VitessKeyspaceKeyRangeShard {
	// Iterate over all partitionings and build a deduplicated set of shards.
	//
	// Note that in order to be equal keyranges, the bounds must be exactly equal
	// byte-by-byte, and we enforce through validation that hex digits a-f must
	// be lowercase, so it's safe to directly compare two VitessKeyRange objects
	// for equality. In particular, ranges like "10-20" and "1000-2000" are *not*
	// equivalent in the general case, so if both are defined, they really are
	// different shards.
	//
	// Also, shards from latter partitionings (in the order listed by the user)
	// always overwrite shards with the same keyrange from earlier partitionings.
	shardMap := make(map[VitessKeyRange]*VitessKeyspaceKeyRangeShard)

	for i := range spec.Partitionings {
		partitioning := &spec.Partitionings[i]

		switch {
		case partitioning.Equal != nil:
			for _, kr := range partitioning.Equal.KeyRanges() {
				shardMap[kr] = &VitessKeyspaceKeyRangeShard{
					KeyRange:            kr,
					VitessShardTemplate: partitioning.Equal.ShardTemplate,
				}
			}
		case partitioning.Custom != nil:
			for i := range partitioning.Custom.Shards {
				krShard := &partitioning.Custom.Shards[i]
				shardMap[krShard.KeyRange] = krShard
			}
		default:
			// Someday we'll have cross-field validation that prevents invalid config.
			// For now, we just ignore partitionings that are empty.
		}
	}

	// Sort the map keys so the order is deterministic.
	keyRanges := make([]VitessKeyRange, 0, len(shardMap))
	for kr := range shardMap {
		keyRanges = append(keyRanges, kr)
	}
	SortKeyRanges(keyRanges)

	// Turn the deduplicated map back into a list.
	shards := make([]*VitessKeyspaceKeyRangeShard, 0, len(shardMap))
	for _, kr := range keyRanges {
		shards = append(shards, shardMap[kr])
	}
	return shards
}
