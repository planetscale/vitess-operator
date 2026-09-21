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

package vitessshardreplication

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/results"
)

// TestReadyForPrimaryHasMasterUnknown pins the contract the vitessshard
// controller relies on when the global shard record is missing: as long as
// HasMaster is Unknown, readyForPrimary must keep waiting, even if every
// tablet looks primary-eligible. Only an explicit False means "no primary yet,
// go initialize one".
func TestReadyForPrimaryHasMasterUnknown(t *testing.T) {
	newShard := func(hasMaster corev1.ConditionStatus) *planetscalev2.VitessShard {
		vts := &planetscalev2.VitessShard{}
		vts.Status = planetscalev2.NewVitessShardStatus()
		vts.Status.HasMaster = hasMaster
		// A Running tablet whose observed type is the pool type, which is what
		// the shard controller seeds before topology has been consulted.
		tablet := planetscalev2.NewVitessTabletStatus(planetscalev2.ReplicaPoolType, 0)
		tablet.Running = corev1.ConditionTrue
		tablet.Type = "replica"
		vts.Status.Tablets["zone1-0000000100"] = tablet
		return vts
	}

	r := &ReconcileVitessShard{recorder: record.NewFakeRecorder(10)}

	t.Run("Unknown waits and requeues", func(t *testing.T) {
		rb := &results.Builder{}
		if r.readyForPrimary(newShard(corev1.ConditionUnknown), rb) {
			t.Fatal("readyForPrimary = true with HasMaster=Unknown, want false")
		}
		if res, _ := rb.Result(); res.RequeueAfter != replicationRequeueDelay {
			t.Fatalf("RequeueAfter = %v, want %v", res.RequeueAfter, replicationRequeueDelay)
		}
	})

	t.Run("False proceeds", func(t *testing.T) {
		rb := &results.Builder{}
		if !r.readyForPrimary(newShard(corev1.ConditionFalse), rb) {
			t.Fatal("readyForPrimary = false with HasMaster=False and a running replica, want true")
		}
	})
}
