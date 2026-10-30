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

package vitessshard

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/reconciler"
	"planetscale.dev/vitess-operator/pkg/operator/rollout"
	"planetscale.dev/vitess-operator/pkg/operator/vttablet"
)

func TestTabletAvailableRequeueAfterAtBoundary(t *testing.T) {
	window := 2 * time.Minute
	now := time.Date(2026, time.August, 3, 12, 0, 0, 0, time.UTC)
	readySince := now.Add(-window)

	assert.Equal(t, time.Millisecond, tabletAvailableRequeueAfter(readySince, window, now))
}

func newVitessShard(keyspace string, pools []planetscalev2.VitessShardTabletPool) *planetscalev2.VitessShard {
	return &planetscalev2.VitessShard{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				planetscalev2.KeyspaceLabel: keyspace,
			},
		},
		Spec: planetscalev2.VitessShardSpec{
			KeyRange: planetscalev2.VitessKeyRange{},
			VitessShardTemplate: planetscalev2.VitessShardTemplate{
				TabletPools: pools,
			},
		},
	}
}

func TestTabletUidLabelZeroPadded(t *testing.T) {
	// This specific combination generates a UID with leading zero(s)
	cluster := "default"
	cell := "zone1"
	keyspace := "commerce"
	keyRange := planetscalev2.VitessKeyRange{Start: "", End: ""}
	tabletType := planetscalev2.ReplicaPoolType
	tabletIdx := uint32(3)
	wantUID := vttablet.UID(cell, keyspace, keyRange, tabletType, tabletIdx)
	wantUIDStr := vttablet.UIDString(wantUID)

	shard := newVitessShard(keyspace, []planetscalev2.VitessShardTabletPool{
		{
			Cell:     cell,
			Type:     planetscalev2.ReplicaPoolType,
			Replicas: 3,
		},
	})

	parentLabels := map[string]string{
		planetscalev2.ComponentLabel: planetscalev2.VttabletComponentName,
		planetscalev2.ClusterLabel:   cluster,
		planetscalev2.KeyspaceLabel:  keyspace,
		planetscalev2.ShardLabel:     shard.Spec.KeyRange.SafeName(),
	}

	tablets := vttabletSpecs(shard, parentLabels)

	for _, tablet := range tablets {
		uid := tablet.Labels[planetscalev2.TabletUidLabel]
		idx := tablet.Labels[planetscalev2.TabletIndexLabel]

		if len(uid) != 10 {
			t.Errorf("expected uid label for tablet %s to be 10 characters, got %d (%s)", idx, len(uid), uid)
		}

		if uint32(tablet.Index) != tabletIdx {
			continue
		}

		if uid != wantUIDStr {
			t.Errorf("expected tablet with index %d to have uid %q, got %q", tabletIdx, wantUIDStr, uid)
		}
	}
}

// TestReconcileTabletsDoesNotRollRespelledArgs drives the real tablet Pod
// reconciliation against a Pod whose command lines only differ from the
// desired ones in flag spelling, as written by an older operator. Upgrading the
// operator must not schedule a rollout for it, while a new image still must,
// bringing the new command line with it.
func TestReconcileTabletsDoesNotRollRespelledArgs(t *testing.T) {
	vts := newVitessShard("commerce", []planetscalev2.VitessShardTabletPool{{
		Cell:     "zone1",
		Type:     planetscalev2.ReplicaPoolType,
		Replicas: 1,
		Mysqld:   &planetscalev2.MysqldSpec{},
	}})
	vts.Name = "example-commerce-x-x"
	vts.Namespace = "default"
	vts.Labels[planetscalev2.ClusterLabel] = "example"
	vts.Spec.Images = planetscalev2.VitessKeyspaceImages{
		Vttablet: "vitess/lite:v24.0.5",
		Mysqld:   &planetscalev2.MysqldImage{Mysql80Compatible: "vitess/lite:v24.0.5"},
	}
	vts.Spec.GlobalLockserver = planetscalev2.VitessLockserverParams{Implementation: "etcd2", Address: "etcd:2379", RootPath: "/vitess/example/global"}
	planetscalev2.DefaultVitessShard(vts)

	scheme := backupTestScheme(t)
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(vts.DeepCopy()).Build()
	recorder := record.NewFakeRecorder(100)
	r := &ReconcileVitessShard{
		client:     k8sClient,
		scheme:     scheme,
		recorder:   recorder,
		reconciler: reconciler.New(k8sClient, scheme, recorder),
	}
	reconcileTablets := func() *corev1.Pod {
		t.Helper()
		shard := vts.DeepCopy()
		shard.Status = planetscalev2.NewVitessShardStatus()
		_, err := r.reconcileTablets(t.Context(), shard)
		require.NoError(t, err)
		pods := &corev1.PodList{}
		require.NoError(t, k8sClient.List(t.Context(), pods))
		require.Len(t, pods.Items, 1)
		return &pods.Items[0]
	}

	// Create the Pod, then respell its command lines the way an older operator
	// wrote them.
	pod := reconcileTablets()
	for i := range pod.Spec.Containers {
		if name := pod.Spec.Containers[i].Name; name == "vttablet" || name == vttablet.MysqldContainerName {
			pod.Spec.Containers[i].Args = oldOperatorArgs(pod.Spec.Containers[i].Args)
		}
	}
	require.NoError(t, k8sClient.Update(t.Context(), pod))

	live := reconcileTablets()
	assert.NotContains(t, live.Annotations, rollout.ScheduledAnnotation)

	// A new vttablet image schedules a rollout that brings the new command
	// lines, for vttablet and for mysqlctld, which runs a binary copied from
	// that image.
	vts.Spec.Images.Vttablet = "vitess/lite:v25.0.0"
	live = reconcileTablets()
	require.Contains(t, live.Annotations, rollout.ScheduledAnnotation)
	pendingChanges := live.Annotations[rollout.ScheduledAnnotation]
	assert.Contains(t, pendingChanges, "--init-keyspace=commerce")
	assert.Contains(t, pendingChanges, "--tablet-uid=")
}

// oldOperatorArgs respells a command line the way older operator versions
// wrote it: underscore flag names, plus --logtostderr.
func oldOperatorArgs(args []string) []string {
	old := []string{"--logtostderr=true"}
	for _, arg := range args {
		name, value, _ := strings.Cut(strings.TrimPrefix(arg, "--"), "=")
		old = append(old, "--"+strings.ReplaceAll(name, "-", "_")+"="+value)
	}
	return old
}
