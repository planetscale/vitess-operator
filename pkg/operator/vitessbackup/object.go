/*
Copyright 2019 PlanetScale.

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

package vitessbackup

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	topodatapb "vitess.io/vitess/go/vt/proto/topodata"
	"vitess.io/vitess/go/vt/topo/topoproto"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/names"
)

const (
	// TimestampFormat is the format used by Vitess for the timestamp in a backup name.
	TimestampFormat = "2006-01-02.150405"

	// objectNameTimeFormat is the format used for the timestamp in VitessBackup
	// object names. We use a different timestamp format than Vitess because our
	// ames have to follow Kubernetes object naming conventions.
	objectNameTimeFormat = "20060102-150405"
)

// ObjectName returns the name for a VitessBackup object.
func ObjectName(clusterName, backupLocationName, keyspaceName string, shardkeyRange planetscalev2.VitessKeyRange, backupTime time.Time, tabletAlias *topodatapb.TabletAlias) string {
	// The tablet alias is only in the original Vitess backup name to avoid name
	// collisions if two tablets start taking a backup at the same time. We also
	// need to avoid collisions for VitessBackup object names, but we don't
	// include the whole tablet alias because the UID is enough and including
	// the whole tablet alias tends to mislead people to think it matters which
	// tablet took the backup.
	uidStr := strconv.FormatInt(int64(tabletAlias.Uid), 16)
	timestamp := backupTime.Format(objectNameTimeFormat)

	if backupLocationName == "" {
		return names.Join(clusterName, keyspaceName, shardkeyRange.SafeName(), timestamp, uidStr)
	}
	return names.Join(clusterName, backupLocationName, keyspaceName, shardkeyRange.SafeName(), timestamp, uidStr)
}

// StorageObjectName returns the name for a VitessBackupStorage object.
func StorageObjectName(clusterName, backupLocationName string) string {
	if backupLocationName == "" {
		return names.Join(clusterName)
	}
	return names.Join(clusterName, backupLocationName)
}

// ParseBackupName parses the name given by Vitess to each backup.
func ParseBackupName(name string) (time.Time, *topodatapb.TabletAlias, error) {
	// Backup names are formatted as "date.time.tablet-alias".
	lastDot := strings.LastIndex(name, ".")
	if lastDot < 0 {
		return time.Time{}, nil, fmt.Errorf("invalid backup name %q; expected format: date.time.tablet-alias", name)
	}
	timestamp, alias := name[:lastDot], name[lastDot+1:]
	backupTime, err := time.Parse(TimestampFormat, timestamp)
	if err != nil {
		return time.Time{}, nil, fmt.Errorf("invalid backup timestamp %q; expected format: %v", timestamp, TimestampFormat)
	}
	tabletAlias, err := topoproto.ParseTabletAlias(alias)
	if err != nil {
		return time.Time{}, nil, fmt.Errorf("invalid tablet alias %q: %v", alias, err)
	}
	return backupTime, tabletAlias, nil
}

// LatestForLocation returns the latest backup from the given list that's in the
// specified storage location. It returns nil if there are no backups in the
// storage location.
func LatestForLocation(locationName string, backups []*planetscalev2.VitessBackup) *planetscalev2.VitessBackup {
	var latest *planetscalev2.VitessBackup
	for _, backup := range backups {
		if backup.Labels[LocationLabel] != locationName {
			continue
		}
		if latest == nil || backup.Status.StartTime.After(latest.Status.StartTime.Time) {
			latest = backup
		}
	}
	return latest
}

// CompleteBackups returns a list of only the complete backups from the input.
func CompleteBackups(backups []planetscalev2.VitessBackup) []*planetscalev2.VitessBackup {
	completeBackups := []*planetscalev2.VitessBackup{}
	for i := range backups {
		if backups[i].Status.Complete {
			completeBackups = append(completeBackups, &backups[i])
		}
	}
	return completeBackups
}
