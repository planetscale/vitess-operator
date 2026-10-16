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
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/drain"
	"planetscale.dev/vitess-operator/pkg/operator/environment"
	"planetscale.dev/vitess-operator/pkg/operator/reconciler"
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

func TestTabletUID(t *testing.T) {
	cell := "zone1"
	keyspace := "commerce"
	keyRange := planetscalev2.VitessKeyRange{}
	tabletType := planetscalev2.ReplicaPoolType
	external := &planetscalev2.ExternalDatastore{}

	tests := []struct {
		name            string
		defaultPoolName string
		poolName        string
		external        *planetscalev2.ExternalDatastore
		wantLegacyUID   bool
	}{
		{name: "unnamed pool", poolName: "", wantLegacyUID: true},
		{name: "unnamed pool with flag set", defaultPoolName: "default", poolName: "", wantLegacyUID: true},
		{name: "unnamed external pool", poolName: "", external: external, wantLegacyUID: true},
		{name: "unnamed external pool with flag set", defaultPoolName: "default", poolName: "", external: external, wantLegacyUID: true},
		{name: "named pool", poolName: "default", wantLegacyUID: false},
		{name: "named pool matching flag", defaultPoolName: "default", poolName: "default", wantLegacyUID: true},
		{name: "named pool not matching flag", defaultPoolName: "default", poolName: "fast-storage", wantLegacyUID: false},
		{name: "named external pool", poolName: "default", external: external, wantLegacyUID: false},
		{name: "named external pool matching flag", defaultPoolName: "default", poolName: "default", external: external, wantLegacyUID: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			environment.SetDefaultPoolName(tt.defaultPoolName)
			defer environment.SetDefaultPoolName("")

			pool := &planetscalev2.VitessShardTabletPool{
				Cell:              cell,
				Type:              tabletType,
				Name:              tt.poolName,
				ExternalDatastore: tt.external,
			}

			for _, tabletIndex := range []uint32{1, 2} {
				want := vttablet.UIDWithPoolName(cell, keyspace, keyRange, tabletType, tabletIndex, tt.poolName)
				if tt.wantLegacyUID {
					want = vttablet.UID(cell, keyspace, keyRange, tabletType, tabletIndex)
				}
				assert.Equal(t, want, tabletUID(pool, keyspace, keyRange, tabletIndex), "tablet index %d", tabletIndex)
			}
		})
	}
}

func TestVttabletSpecsMultiplePoolsSameCellType(t *testing.T) {
	environment.SetDefaultPoolName("default")
	defer environment.SetDefaultPoolName("")

	shard := newVitessShard("commerce", []planetscalev2.VitessShardTabletPool{
		{Cell: "zone1", Type: planetscalev2.ReplicaPoolType, Name: "default", Replicas: 2},
		{Cell: "zone1", Type: planetscalev2.ReplicaPoolType, Name: "fast-storage", Replicas: 2},
	})
	require.NoError(t, validateTabletPools(shard))

	tablets := vttabletSpecs(shard, map[string]string{})
	require.Len(t, tablets, 4)

	seen := make(map[string]bool)
	for _, tablet := range tablets {
		assert.False(t, seen[tablet.AliasStr], "duplicate tablet alias %s", tablet.AliasStr)
		seen[tablet.AliasStr] = true
	}
}

func TestValidateTabletPools(t *testing.T) {
	external := &planetscalev2.ExternalDatastore{}

	tests := []struct {
		name    string
		pools   []planetscalev2.VitessShardTabletPool
		wantErr bool
	}{
		{
			name: "single unnamed pool",
			pools: []planetscalev2.VitessShardTabletPool{
				{Cell: "zone1", Type: planetscalev2.ReplicaPoolType, Replicas: 3},
			},
		},
		{
			name: "unnamed and named pools in the same cell and type",
			pools: []planetscalev2.VitessShardTabletPool{
				{Cell: "zone1", Type: planetscalev2.ReplicaPoolType, Replicas: 2},
				{Cell: "zone1", Type: planetscalev2.ReplicaPoolType, Name: "fast-storage", Replicas: 2},
			},
		},
		{
			name: "unnamed pool and pool matching flag in the same cell and type",
			pools: []planetscalev2.VitessShardTabletPool{
				{Cell: "zone1", Type: planetscalev2.ReplicaPoolType, Replicas: 1},
				{Cell: "zone1", Type: planetscalev2.ReplicaPoolType, Name: "default", Replicas: 1},
			},
			wantErr: true,
		},
		{
			name: "unnamed external pool and pool matching flag in the same cell and type",
			pools: []planetscalev2.VitessShardTabletPool{
				{Cell: "zone1", Type: planetscalev2.ReplicaPoolType, Replicas: 1, ExternalDatastore: external},
				{Cell: "zone1", Type: planetscalev2.ReplicaPoolType, Name: "default", Replicas: 1},
			},
			wantErr: true,
		},
		{
			name: "unnamed pool and pool matching flag in different cells",
			pools: []planetscalev2.VitessShardTabletPool{
				{Cell: "zone1", Type: planetscalev2.ReplicaPoolType, Replicas: 1},
				{Cell: "zone2", Type: planetscalev2.ReplicaPoolType, Name: "default", Replicas: 1},
			},
		},
		{
			name: "unnamed pool and pool matching flag with different types",
			pools: []planetscalev2.VitessShardTabletPool{
				{Cell: "zone1", Type: planetscalev2.ReplicaPoolType, Replicas: 1},
				{Cell: "zone1", Type: planetscalev2.RdonlyPoolType, Name: "default", Replicas: 1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			environment.SetDefaultPoolName("default")
			defer environment.SetDefaultPoolName("")

			err := validateTabletPools(newVitessShard("commerce", tt.pools))
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestVttabletSpecsPoolNameLabel(t *testing.T) {
	tests := []struct {
		name      string
		pool      planetscalev2.VitessShardTabletPool
		wantLabel bool
		wantValue string
	}{
		{
			name: "unnamed pool has no label",
			pool: planetscalev2.VitessShardTabletPool{Cell: "zone1", Type: planetscalev2.ReplicaPoolType, Replicas: 1},
		},
		{
			name:      "unnamed external pool has an empty label",
			pool:      planetscalev2.VitessShardTabletPool{Cell: "zone1", Type: planetscalev2.ReplicaPoolType, Replicas: 1, ExternalDatastore: &planetscalev2.ExternalDatastore{}},
			wantLabel: true,
			wantValue: "",
		},
		{
			name:      "named pool is labelled with its name",
			pool:      planetscalev2.VitessShardTabletPool{Cell: "zone1", Type: planetscalev2.ReplicaPoolType, Name: "fast-storage", Replicas: 1},
			wantLabel: true,
			wantValue: "fast-storage",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tablets := vttabletSpecs(newVitessShard("commerce", []planetscalev2.VitessShardTabletPool{tt.pool}), map[string]string{})
			require.Len(t, tablets, 1)

			value, ok := tablets[0].Labels[planetscalev2.TabletPoolNameLabel]
			assert.Equal(t, tt.wantLabel, ok)
			assert.Equal(t, tt.wantValue, value)
		})
	}
}

func TestReconcileLeavesTabletsAloneWhenPoolsCollide(t *testing.T) {
	environment.SetDefaultPoolName("default")
	defer environment.SetDefaultPoolName("")

	ctx := context.Background()
	shardLabels := map[string]string{
		planetscalev2.ClusterLabel:  "example",
		planetscalev2.KeyspaceLabel: "commerce",
	}

	vts := newVitessShard("commerce", []planetscalev2.VitessShardTabletPool{
		{Cell: "zone1", Type: planetscalev2.ReplicaPoolType, Replicas: 2},
		{Cell: "zone1", Type: planetscalev2.ReplicaPoolType, Name: "default", Replicas: 2},
	})
	vts.Name = "example-commerce-x-x"
	vts.Namespace = "default"
	vts.Labels = shardLabels
	vts.Status.Cells = []string{"zone1"}

	// Existing tablets from before the colliding pool was added
	existing := newVitessShard("commerce", vts.Spec.TabletPools[:1])
	existing.Labels = shardLabels
	tabletLabels := map[string]string{
		planetscalev2.ComponentLabel: planetscalev2.VttabletComponentName,
		planetscalev2.ClusterLabel:   "example",
		planetscalev2.KeyspaceLabel:  "commerce",
		planetscalev2.ShardLabel:     vts.Spec.KeyRange.SafeName(),
	}
	var podNames []string
	var objects []client.Object
	for _, tablet := range vttabletSpecs(existing, tabletLabels) {
		name := vttablet.PodName("example", tablet.Alias)
		podNames = append(podNames, name)
		objects = append(objects,
			&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: vts.Namespace, Labels: tablet.Labels}},
			&corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: vts.Namespace, Labels: tablet.Labels}},
		)
	}
	objects = append(objects, vts)

	scheme := backupTestScheme(t)
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).WithStatusSubresource(vts).Build()
	recorder := record.NewFakeRecorder(10)
	r := &ReconcileVitessShard{
		client:     k8sClient,
		scheme:     scheme,
		recorder:   recorder,
		reconciler: reconciler.New(k8sClient, scheme, recorder),
	}

	_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: vts.Namespace, Name: vts.Name}})
	require.Error(t, err)
	require.Len(t, recorder.Events, 1)
	assert.Contains(t, <-recorder.Events, "InvalidTabletPools")

	for _, name := range podNames {
		key := types.NamespacedName{Namespace: vts.Namespace, Name: name}

		pod := &corev1.Pod{}
		require.NoError(t, k8sClient.Get(ctx, key, pod))
		assert.Nil(t, pod.DeletionTimestamp)
		assert.NotContains(t, pod.Annotations, drain.StartedAnnotation)

		pvc := &corev1.PersistentVolumeClaim{}
		require.NoError(t, k8sClient.Get(ctx, key, pvc))
		assert.Nil(t, pvc.DeletionTimestamp)
	}

	got := &planetscalev2.VitessShard{}
	require.NoError(t, k8sClient.Get(ctx, types.NamespacedName{Namespace: vts.Namespace, Name: vts.Name}, got))
	assert.Equal(t, []string{"zone1"}, got.Status.Cells)
}
