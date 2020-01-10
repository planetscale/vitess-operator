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
