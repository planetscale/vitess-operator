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
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
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

// CellNames returns a sorted list of all cells in which any part of the keyspace
// (any tablet pool of any shard) should be deployed.
func (s *VitessKeyspaceSpec) CellNames() []string {
	cellMap := make(map[string]struct{})

	for partitionIndex := range s.Partitionings {
		pools := s.Partitionings[partitionIndex].TabletPools()
		for poolIndex := range pools {
			cellMap[pools[poolIndex].Cell] = struct{}{}
		}
	}

	cells := make([]string, 0, len(cellMap))
	for cellName := range cellMap {
		cells = append(cells, cellName)
	}
	sort.Strings(cells)

	return cells
}

// ShardNameSet returns the set of shard names in this partitioning.
func (p *VitessKeyspacePartitioning) ShardNameSet() sets.String {
	shardNames := sets.NewString()

	switch {
	case p.Equal != nil:
		for _, keyRange := range p.Equal.KeyRanges() {
			shardNames.Insert(keyRange.String())
		}
	case p.Custom != nil:
		for i := range p.Custom.Shards {
			shard := &p.Custom.Shards[i]
			shardNames.Insert(shard.KeyRange.String())
		}
	}

	return shardNames
}

// TabletPools returns the list of tablet pools from whichever paritioning sub-field is defined.
func (p *VitessKeyspacePartitioning) TabletPools() []VitessShardTabletPool {
	if p.Equal != nil {
		return p.Equal.ShardTemplate.TabletPools
	}
	if p.Custom != nil {
		var pools []VitessShardTabletPool
		for i := range p.Custom.Shards {
			pools = append(pools, p.Custom.Shards[i].TabletPools...)
		}
		return pools
	}
	return nil
}

// SetConditionStatus first ensures we have allocated a conditions map, and also ensures we have allocated a ShardCondition
// for the VitessKeyspaceConditionType key supplied. It then moves onto setting the conditions status.
// For the condition's status, it always updates the reason and message every time. If the current status is the same as the supplied
// newStatus, then we do not update LastTransitionTime. However, if newStatus is different from current status, then
// we update the status and update the transition time.
func (s *VitessKeyspaceStatus) SetConditionStatus(condType VitessKeyspaceConditionType, newStatus corev1.ConditionStatus, reason, message string) {
	cond, ok := s.getCondition(condType)
	if !ok {
		cond = NewVitessKeyspaceCondition(condType)
	}

	// We should update reason and message regardless of whether the status type is different.
	cond.Reason = reason
	cond.Message = message

	if cond.Status != newStatus {
		now := metav1.NewTime(time.Now())
		cond.Status = newStatus
		cond.LastTransitionTime = &now
	}

	s.setCondition(cond)
}

// NewVitessKeyspaceCondition returns an init VitessKeyspaceCondition object.
func NewVitessKeyspaceCondition(condType VitessKeyspaceConditionType) *VitessKeyspaceCondition {
	now := metav1.NewTime(time.Now())
	return &VitessKeyspaceCondition{
		Type:               condType,
		Status:             corev1.ConditionUnknown,
		LastTransitionTime: &now,
		Reason:             "",
		Message:            "",
	}
}

// StatusDuration returns the duration since LastTransitionTime. It represents how long we've been in the current status for
// this condition. If LastTransitionTime is nil, then we return zero to indicate that we have no confidence about the duration
// of the status.
func (c *VitessKeyspaceCondition) StatusDuration() time.Duration {
	if c.LastTransitionTime == nil {
		return time.Duration(0)
	}

	return time.Since(c.LastTransitionTime.Time)
}

// DeepCopyConditions deep copies the conditions map for VitessKeyspaceStatus.
func (s *VitessKeyspaceStatus) DeepCopyConditions() []VitessKeyspaceCondition {
	out := make([]VitessKeyspaceCondition, 0, len(s.Conditions))

	for _, condition := range s.Conditions {
		out = append(out, *condition.DeepCopy())
	}

	return out
}

// GetCondition provides map style access to retrieve a condition from the conditions list by it's type
// If the condition doesn't exist, we return false for the exists named return value.
func (s *VitessKeyspaceStatus) GetCondition(ty VitessKeyspaceConditionType) (value VitessKeyspaceCondition, exists bool) {
	cond, exists := s.getCondition(ty)
	if !exists {
		return VitessKeyspaceCondition{}, false
	}
	return *cond.DeepCopy(), true
}

// getCondition is used internally for map style access, and returns a pointer to reduce unnecessary copying.
func (s *VitessKeyspaceStatus) getCondition(ty VitessKeyspaceConditionType) (value *VitessKeyspaceCondition, exists bool) {
	for i := range s.Conditions {
		condition := &s.Conditions[i]
		if condition.Type == ty {
			return condition, true
		}
	}
	return nil, false
}

// setCondition is used internally to provide map style setting of conditions, and will ensure uniqueness by using

// setCondition is used internally to provide map style setting of conditions, and will ensure uniqueness by using
// upsert semantics.
func (s *VitessKeyspaceStatus) setCondition(newCondition *VitessKeyspaceCondition) {
	for i := range s.Conditions {
		condition := &s.Conditions[i]
		if condition.Type == newCondition.Type {
			s.Conditions[i] = *newCondition
			return
		}
	}

	// We got here so we didn't return early by finding the condition already existing. We'll just append to the end.
	s.Conditions = append(s.Conditions, *newCondition)
}
