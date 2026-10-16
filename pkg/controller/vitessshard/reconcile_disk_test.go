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

package vitessshard

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/reconciler"
	"planetscale.dev/vitess-operator/pkg/operator/rollout"
	"planetscale.dev/vitess-operator/pkg/operator/vttablet"
)

// TestReconcileDiskIgnoresOtherPoolsInSameCellType checks that a disk resize in
// one pool is not blocked by another pool of the same type with a different size.
func TestReconcileDiskIgnoresOtherPoolsInSameCellType(t *testing.T) {
	ctx := context.Background()

	pvcSpec := func(size string) *corev1.PersistentVolumeClaimSpec {
		return &corev1.PersistentVolumeClaimSpec{
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(size)},
			},
		}
	}

	vts := newVitessShard("commerce", []planetscalev2.VitessShardTabletPool{
		// The unnamed pool is being resized from 10Gi to 20Gi
		{Cell: "zone1", Type: planetscalev2.ReplicaPoolType, Replicas: 2, DataVolumeClaimTemplate: pvcSpec("20Gi")},
		// The named pool is already at its requested size
		{Cell: "zone1", Type: planetscalev2.ReplicaPoolType, Name: "big", Replicas: 2, DataVolumeClaimTemplate: pvcSpec("50Gi")},
	})
	vts.Name = "example-commerce-x-x"
	vts.Namespace = "default"
	vts.Labels[planetscalev2.ClusterLabel] = "example"
	vts.Spec.UpdateStrategy = &planetscalev2.VitessClusterUpdateStrategy{
		Type: ptr.To(planetscalev2.ImmediateVitessClusterUpdateStrategyType),
	}
	vts.Status = planetscalev2.NewVitessShardStatus()

	tabletLabels := map[string]string{
		planetscalev2.ComponentLabel: planetscalev2.VttabletComponentName,
		planetscalev2.ClusterLabel:   "example",
		planetscalev2.KeyspaceLabel:  "commerce",
		planetscalev2.ShardLabel:     vts.Spec.KeyRange.SafeName(),
	}

	objects := []client.Object{vts}
	for _, tablet := range vttabletSpecs(vts, tabletLabels) {
		vts.Status.Tablets[tablet.AliasStr] = planetscalev2.NewVitessTabletStatus(tablet.Type, tablet.Index)
		name := vttablet.PodName("example", tablet.Alias)

		pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: vts.Namespace, Labels: tablet.Labels}}
		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: vts.Namespace, Labels: tablet.Labels},
			Spec:       *tablet.DataVolumePVCSpec,
		}

		if tablet.Labels[planetscalev2.TabletPoolNameLabel] == "big" {
			pvc.Status.Capacity = corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("50Gi")}
		} else {
			pvc.Status.Capacity = corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("10Gi")}
			pvc.Status.Conditions = []corev1.PersistentVolumeClaimCondition{{
				Type:   corev1.PersistentVolumeClaimFileSystemResizePending,
				Status: corev1.ConditionTrue,
			}}
			rollout.Schedule(pod, "resize")
		}

		objects = append(objects, pod, pvc)
	}

	scheme := backupTestScheme(t)
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
	recorder := record.NewFakeRecorder(10)
	r := &ReconcileVitessShard{
		client:     k8sClient,
		scheme:     scheme,
		recorder:   recorder,
		reconciler: reconciler.New(k8sClient, scheme, recorder),
	}

	_, err := r.reconcileDisk(ctx, vts)
	require.NoError(t, err)

	got := &planetscalev2.VitessShard{}
	require.NoError(t, k8sClient.Get(ctx, types.NamespacedName{Namespace: vts.Namespace, Name: vts.Name}, got))
	assert.True(t, rollout.Cascading(got), "expected the resize to cascade, events: %v", drainEvents(recorder))
}
