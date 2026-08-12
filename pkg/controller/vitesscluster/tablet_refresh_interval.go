/*
Copyright 2026 PlanetScale Inc.

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

package vitesscluster

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
)

// requiredTabletRefreshInterval keeps the shard gate large enough for both
// the desired setting and every interval still running in the vtgate fleet.
func requiredTabletRefreshInterval(desired metav1.Duration, statuses map[string]planetscalev2.VitessClusterCellStatus) metav1.Duration {
	required := desired
	for _, status := range statuses {
		if status.TabletRefreshInterval != nil && status.TabletRefreshInterval.Duration > required.Duration {
			required = *status.TabletRefreshInterval
		}
	}
	return required
}

// tabletRefreshIntervalsReportedByCells prevents a transition from proceeding
// before the operator knows the interval running in every existing cell.
func tabletRefreshIntervalsReportedByCells(cells []planetscalev2.VitessCellTemplate, statuses map[string]planetscalev2.VitessClusterCellStatus) bool {
	for i := range cells {
		status, ok := statuses[cells[i].Name]
		if !ok || status.TabletRefreshInterval == nil {
			return false
		}
	}
	return true
}

// tabletRefreshIntervalObservedByCells allows a shard gate to shrink only
// after every vtgate Deployment has completed the desired rollout.
func tabletRefreshIntervalObservedByCells(desired metav1.Duration, cells []planetscalev2.VitessCellTemplate, statuses map[string]planetscalev2.VitessClusterCellStatus) bool {
	for i := range cells {
		status, ok := statuses[cells[i].Name]
		if !ok || status.TabletRefreshInterval == nil || status.TabletRefreshInterval.Duration != desired.Duration {
			return false
		}
	}
	return true
}

// tabletRefreshIntervalObservedByKeyspaces allows vtgate changes only after
// every keyspace reports that all of its shard controllers use a safe gate.
func tabletRefreshIntervalObservedByKeyspaces(required metav1.Duration, keyspaces []planetscalev2.VitessKeyspaceTemplate, statuses map[string]planetscalev2.VitessClusterKeyspaceStatus) bool {
	for i := range keyspaces {
		status, ok := statuses[keyspaces[i].Name]
		if !ok || status.TabletRefreshInterval == nil || status.TabletRefreshInterval.Duration < required.Duration {
			return false
		}
	}
	return true
}

// stagedTabletRefreshInterval preserves the child setting until the other
// half of the vtgate/shard-gate contract has reached a safe state.
func stagedTabletRefreshInterval(current *metav1.Duration, desired metav1.Duration, ready bool) *metav1.Duration {
	if !ready {
		return current
	}
	return desired.DeepCopy()
}

// stagedTabletRefreshGateInterval allows an unobserved fleet to raise a shard
// gate but never lower the protection already present on an existing keyspace.
func stagedTabletRefreshGateInterval(current *metav1.Duration, target metav1.Duration, cellsReported bool) *metav1.Duration {
	currentInterval := planetscalev2.EffectiveTabletRefreshInterval(current)
	if !cellsReported && currentInterval.Duration > target.Duration {
		return currentInterval.DeepCopy()
	}
	return target.DeepCopy()
}

// observedTabletRefreshInterval provides a safe bootstrap value for a cell
// added while an interval transition is already in progress.
func observedTabletRefreshInterval(statuses map[string]planetscalev2.VitessClusterCellStatus) *metav1.Duration {
	var observed *metav1.Duration
	for _, status := range statuses {
		if status.TabletRefreshInterval == nil {
			continue
		}
		if observed == nil || status.TabletRefreshInterval.Duration > observed.Duration {
			observed = status.TabletRefreshInterval.DeepCopy()
		}
	}
	return observed
}

// observedKeyspaceTabletRefreshInterval prevents stale shard-gate status from
// authorizing a vtgate rollout after the keyspace target has changed.
func observedKeyspaceTabletRefreshInterval(current, observed *metav1.Duration, target metav1.Duration) *metav1.Duration {
	currentInterval := planetscalev2.EffectiveTabletRefreshInterval(current)
	if observed == nil || observed.Duration != currentInterval.Duration || currentInterval.Duration != target.Duration {
		return nil
	}
	return observed
}
