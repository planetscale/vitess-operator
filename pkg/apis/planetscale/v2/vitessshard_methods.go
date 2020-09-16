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
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

// IsExternalMaster indicates whether the tablet is in a pool of type "externalmaster".
func (t *VitessTabletStatus) IsExternalMaster() bool {
	return t.PoolType == ExternalMasterTabletPoolName
}

// IsRunning indicates whether the tablet is known to be Running.
func (t *VitessTabletStatus) IsRunning() bool {
	return t.Running == corev1.ConditionTrue
}

// InitTabletType returns a string representing what the initial tablet
// type should be for a tablet in this type of pool.
func (t *VitessTabletPoolType) InitTabletType() string {
	switch *t {
	case ExternalMasterPoolType:
		// All external master tablets start out SPARE, as opposed to regular masters which start out REPLICA.
		// We don't want external masters to ever serve REPLICA queries because it's not actually possible to
		// convert an external tablet between REPLICA and MASTER at the MySQL level, since we don't control
		// replication in the external case.
		return "spare"
	case ExternalReplicaPoolType:
		// We tell Vitess this is a normal REPLICA because the distinction that it's an external replica only
		// matters for configuration (e.g. other flags) that we handle at the operator level.
		return "replica"
	case ExternalRdonlyPoolType:
		// We tell Vitess this is a normal RDONLY because the distinction that it's an external rdonly only
		// matters for configuration (e.g. other flags) that we handle at the operator level.
		return "rdonly"
	default:
		return string(*t)
	}
}

// IsMatch indicates whether a tablet pool matches another tablet pool's type and cell.
func (t *VitessShardTabletPool) IsMatch(inputPool *VitessShardTabletPool) bool {
	return t.Type == inputPool.Type && t.Cell == inputPool.Cell
}

// UsingExternalDatastore indicates whether the VitessShard Spec is using
// externally managed MySQL for any of its tablet pools.
func (s *VitessShardSpec) UsingExternalDatastore() bool {
	for i := range s.TabletPools {
		p := &s.TabletPools[i]
		if p.ExternalDatastore != nil {
			return true
		}
	}

	return false
}

// AllPoolsUsingMysqld returns a boolean indicating whether the VitessShard Spec is using
// local MySQL for all of it's pools by checking the Mysqld field of all tablet pools.
func (s *VitessShardSpec) AllPoolsUsingMysqld() bool {
	for i := range s.TabletPools {
		p := &s.TabletPools[i]
		if p.Mysqld == nil {
			return false
		}
	}

	return true
}

// MasterEligibleTabletCount returns the total number of master-eligible tablets in the shard.
func (s *VitessShardSpec) MasterEligibleTabletCount() int32 {
	count := int32(0)
	for poolIndex := range s.TabletPools {
		pool := &s.TabletPools[poolIndex]
		if pool.Type == ReplicaPoolType || pool.Type == ExternalMasterPoolType {
			count += pool.Replicas
		}
	}
	return count
}

// BackupLocation looks up a backup location in the list by name.
// It returns nil if no location by that name exists.
func (s *VitessShardSpec) BackupLocation(name string) *VitessBackupLocation {
	// Note that "" is a valid name (commonly used when the user only needs to
	// configure one backup location), so we always check even if 'name' is
	// empty.
	//
	// TODO(enisoc): Use a validating webhook to guarantee uniqueness and referential integrity.
	//               For now, we take the first one with a matching name, if any.
	for i := range s.BackupLocations {
		if s.BackupLocations[i].Name == name {
			return &s.BackupLocations[i]
		}
	}
	// No backup with that name found.
	return nil
}

// BackupsEnabled returns whether at least one tablet pool in the shard has a
// backup location set.
func (s *VitessShardSpec) BackupsEnabled() bool {
	for i := range s.TabletPools {
		pool := &s.TabletPools[i]
		// If any explicit value is set, backups are enabled.
		if pool.BackupLocationName != "" {
			return true
		}
		// If the pool uses the default location, backups are enabled if a
		// default location exists.
		if s.BackupLocation("") != nil {
			return true
		}
	}
	return false
}

// GetCells returns the set of all cells used by any tablet pools
// defined in this VitessShardSpec.
func (s *VitessShardSpec) GetCells() sets.String {
	cells := sets.String{}

	for i := range s.TabletPools {
		cells.Insert(s.TabletPools[i].Cell)
	}

	return cells
}

// CellInCluster returns whether the given cell name is defined in the
// VitessCluster to which this shard ultimately belongs.
func (s *VitessShardSpec) CellInCluster(cellName string) bool {
	// The set of cells defined in the VitessCluster is ultimately passed down
	// to each VitessShard in the form of a map from Vitess cell names to
	// provider-specific zone names (even if zone names are left empty).
	// Therefore the key exists in this map if and only if that cell name is
	// defined in the VitessCluster.
	_, inZoneMap := s.ZoneMap[cellName]
	return inZoneMap
}

// SetConditionStatus first ensures we have allocated a conditions map, and also ensures we have allocated a ShardCondition
// for the VitessShardConditionType key supplied. It then moves onto setting the conditions status.
// For the condition's status, it always updates the reason and message every time. If the current status is the same as the supplied
// newStatus, then we do not update LastTransitionTime. However, if newStatus is different from current status, then
// we update the status and update the transition time.
func (s *VitessShardStatus) SetConditionStatus(condType VitessShardConditionType, newStatus corev1.ConditionStatus, reason, message string) {
	cond, ok := s.getCondition(condType)
	if !ok {
		cond = NewVitessShardCondition()
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

// NewVitessShardCondition returns an init VitessShardCondition object.
func NewVitessShardCondition() *VitessShardCondition {
	now := metav1.NewTime(time.Now())
	return &VitessShardCondition{
		Status:             corev1.ConditionUnknown,
		LastTransitionTime: &now,
		Reason:             "",
		Message:            "",
	}
}

// StatusDuration returns the duration since LastTransitionTime. It represents how long we've been in the current status for
// this condition. If LastTransitionTime is nil, then we return zero to indicate that we have no confidence about the duration
// of the status.
func (c *VitessShardCondition) StatusDuration() time.Duration {
	if c.LastTransitionTime == nil {
		return time.Duration(0)
	}

	return time.Since(c.LastTransitionTime.Time)
}

// DeepCopyConditions deep copies the conditions map for VitessShardStatus.
func (s *VitessShardStatus) DeepCopyConditions() []VitessShardCondition {
	out := make([]VitessShardCondition, 0, len(s.Conditions))

	for _, condition := range s.Conditions {
		out = append(out, *condition.DeepCopy())
	}

	return out
}

// TabletAliases returns a sorted list of desired tablet aliases for the shard.
func (s *VitessShardStatus) TabletAliases() []string {
	tabletKeys := make([]string, 0, len(s.Tablets))
	for key := range s.Tablets {
		tabletKeys = append(tabletKeys, key)
	}
	sort.Strings(tabletKeys)

	return tabletKeys
}

// GetCondition provides map style access to retrieve a condition from the conditions list by it's type
// If the condition doesn't exist, we return false for the exists named return value.
func (s *VitessShardStatus) GetCondition(ty VitessShardConditionType) (value VitessShardCondition, exists bool) {
	cond, exists := s.getCondition(ty)
	if !exists {
		return VitessShardCondition{}, false
	}
	return *cond.DeepCopy(), true
}

// getCondition is used internally for map style access, and returns a pointer to reduce unnecessary copying.
func (s *VitessShardStatus) getCondition(ty VitessShardConditionType) (value *VitessShardCondition, exists bool) {
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
func (s *VitessShardStatus) setCondition(newCondition *VitessShardCondition) {
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
