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
)

// Cell looks up an item in the Cells list by name.
// It returns a pointer to the item, or nil if the specified cell doesn't exist.
func (spec *VitessClusterSpec) Cell(cellName string) *VitessCellTemplate {
	for i := range spec.Cells {
		if spec.Cells[i].Name == cellName {
			return &spec.Cells[i]
		}
	}
	return nil
}

// CellNames returns a sorted list of all cells in which any part of the keyspace
// (any tablet pool of any shard) should be deployed.
func (spec *VitessKeyspaceSpec) CellNames() []string {
	cellMap := make(map[string]struct{})

	for partitionIndex := range spec.Partitionings {
		pools := spec.Partitionings[partitionIndex].TabletPools()
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

// ZoneMap returns a map from cell names to zone names.
func (spec *VitessClusterSpec) ZoneMap() map[string]string {
	zones := make(map[string]string, len(spec.Cells))
	for i := range spec.Cells {
		cell := &spec.Cells[i]
		zones[cell.Name] = cell.Zone
	}
	return zones
}

// Image returns the first mysqld flavor image that's set.
func (image *MysqldImage) Image() string {
	switch {
	case image.Mysql56Compatible != "":
		return image.Mysql56Compatible
	case image.Mysql80Compatible != "":
		return image.Mysql80Compatible
	case image.MariadbCompatible != "":
		return image.MariadbCompatible
	case image.Mariadb103Compatible != "":
		return image.Mariadb103Compatible
	default:
		return ""
	}
}

// Flavor returns Vitess flavor setting value
// for the first flavor that has an image set.
func (image *MysqldImage) Flavor() string {
	switch {
	case image.Mysql56Compatible != "":
		return "MySQL56"
	case image.Mysql80Compatible != "":
		return "MySQL80"
	case image.MariadbCompatible != "":
		return "MariaDB"
	case image.Mariadb103Compatible != "":
		return "MariaDB103"
	default:
		return ""
	}
}

// MasterEligibleTabletCount returns the total number of master-eligible tablets in the shard.
func (shard *VitessShardSpec) MasterEligibleTabletCount() int32 {
	count := int32(0)
	for poolIndex := range shard.TabletPools {
		pool := &shard.TabletPools[poolIndex]
		if pool.Type == ReplicaPoolType || pool.Type == ExternalMasterPoolType {
			count += pool.Replicas
		}
	}
	return count
}

// BackupLocation looks up a backup location in the list by name.
// It returns nil if no location by that name exists.
func (shard *VitessShardSpec) BackupLocation(name string) *VitessBackupLocation {
	// Note that "" is a valid name (commonly used when the user only needs to
	// configure one backup location), so we always check even if 'name' is
	// empty.
	//
	// TODO(enisoc): Use a validating webhook to guarantee uniqueness and referential integrity.
	//               For now, we take the first one with a matching name, if any.
	for i := range shard.BackupLocations {
		if shard.BackupLocations[i].Name == name {
			return &shard.BackupLocations[i]
		}
	}
	// No backup with that name found.
	return nil
}

// BackupsEnabled returns whether at least one tablet pool in the shard has a
// backup location set.
func (shard *VitessShardSpec) BackupsEnabled() bool {
	for i := range shard.TabletPools {
		pool := &shard.TabletPools[i]
		// If any explicit value is set, backups are enabled.
		if pool.BackupLocationName != "" {
			return true
		}
		// If the pool uses the default location, backups are enabled if a
		// default location exists.
		if shard.BackupLocation("") != nil {
			return true
		}
	}
	return false
}
