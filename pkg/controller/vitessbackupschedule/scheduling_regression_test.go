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

package vitessbackupschedule

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	kbatch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/resync"
)

func TestReconcileSkipsStatusUpdateWhenStatusIsUnchanged(t *testing.T) {
	vbsc := vtctldclientVBSC()
	vbsc.Spec.Strategy = nil

	statusUpdates := 0
	scheme := newScheme()
	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&planetscalev2.VitessBackupSchedule{}).
		WithObjects(&vbsc).
		WithInterceptorFuncs(interceptor.Funcs{
			SubResourceUpdate: func(ctx context.Context, c client.Client, subResourceName string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
				if subResourceName == "status" {
					statusUpdates++
				}
				return c.SubResource(subResourceName).Update(ctx, obj, opts...)
			},
		}).
		Build()
	r := &ReconcileVitessBackupsSchedule{
		client: k8sClient,
		resync: resync.NewPeriodic("test-vbsc-status", time.Hour),
	}

	_, err := r.Reconcile(t.Context(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(&vbsc)})
	require.NoError(t, err)
	assert.Zero(t, statusUpdates)
}

func TestReconcileStrategyUsesStatusAfterSuccessfulJobHistoryIsRemoved(t *testing.T) {
	now := time.Now().UTC()
	lastScheduled := now.Truncate(time.Hour)
	zero := int32(0)

	vbsc := vtctldclientVBSC()
	vbsc.CreationTimestamp = metav1.NewTime(now.Add(-2 * time.Hour))
	vbsc.Spec.Schedule = "0 * * * *"
	vbsc.Spec.SuccessfulJobsHistoryLimit = &zero
	strategy := vbsc.Spec.Strategy[0]
	vbsc.Status.LastScheduledTimes[strategy.Name] = &metav1.Time{Time: lastScheduled}

	scheme := newScheme()
	r := &ReconcileVitessBackupsSchedule{
		client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(vtctldService(), vtctldCluster()).Build(),
		scheme: scheme,
	}

	_, err := r.reconcileStrategy(t.Context(), strategy, ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: vbsc.Namespace, Name: vbsc.Name},
	}, vbsc)
	require.NoError(t, err)

	jobs := &kbatch.JobList{}
	require.NoError(t, r.client.List(t.Context(), jobs, client.InNamespace(vbsc.Namespace)))
	assert.Empty(t, jobs.Items, "the persisted scheduling cursor must prevent recreating an already-scheduled run")
}

func TestReconcileStrategyRepairsStatusFromNewerJobAnnotation(t *testing.T) {
	now := time.Now().UTC()
	jobScheduledTime := now.Truncate(time.Hour)
	zero := int32(0)

	vbsc := vtctldclientVBSC()
	vbsc.CreationTimestamp = metav1.NewTime(now.Add(-2 * time.Hour))
	vbsc.Spec.Schedule = "0 * * * *"
	vbsc.Spec.SuccessfulJobsHistoryLimit = &zero
	strategy := vbsc.Spec.Strategy[0]
	vbsc.Status.LastScheduledTimes[strategy.Name] = &metav1.Time{Time: jobScheduledTime.Add(-time.Hour)}

	job := &kbatch.Job{ObjectMeta: metav1.ObjectMeta{
		Name:      "existing-backup",
		Namespace: vbsc.Namespace,
		Labels: map[string]string{
			planetscalev2.BackupScheduleLabel: vbsc.Name,
			planetscalev2.ClusterLabel:        vbsc.Spec.Cluster,
			planetscalev2.KeyspaceLabel:       strategy.Keyspace,
			planetscalev2.ShardLabel:          "x-x",
			planetscalev2.BackupMethodLabel:   string(planetscalev2.BackupMethodVtctldclient),
		},
		Annotations: map[string]string{
			scheduledTimeAnnotation: jobScheduledTime.Format(time.RFC3339),
		},
	}, Status: kbatch.JobStatus{Conditions: []kbatch.JobCondition{{
		Type:   kbatch.JobComplete,
		Status: corev1.ConditionTrue,
	}}}}

	scheme := newScheme()
	r := &ReconcileVitessBackupsSchedule{
		client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(job).Build(),
		scheme: scheme,
	}

	_, err := r.reconcileStrategy(t.Context(), strategy, ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: vbsc.Namespace, Name: vbsc.Name},
	}, vbsc)
	require.NoError(t, err)
	require.NotNil(t, vbsc.Status.LastScheduledTimes[strategy.Name])
	assert.Equal(t, jobScheduledTime, vbsc.Status.LastScheduledTimes[strategy.Name].Time)
	require.NoError(t, r.client.Get(t.Context(), client.ObjectKeyFromObject(job), &kbatch.Job{}),
		"the Job must remain until the repaired status has been persisted")
}

func TestRemoveTimeoutJobsUsesJobStartTime(t *testing.T) {
	now := time.Date(2026, time.August, 28, 12, 0, 0, 0, time.UTC)
	job := timeoutTestJob(
		"recently-started",
		now.Add(-2*time.Hour),
		metav1.NewTime(now.Add(-5*time.Minute)),
		planetscalev2.BackupMethodVtctldclient,
	)

	scheme := newScheme()
	r := &ReconcileVitessBackupsSchedule{
		client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(job).Build(),
	}

	require.NoError(t, r.removeTimeoutJobs(t.Context(), []*kbatch.Job{job}, "test-vbsc", 10, now))

	remaining := &kbatch.Job{}
	require.NoError(t, r.client.Get(t.Context(), client.ObjectKeyFromObject(job), remaining))
}

func TestRemoveTimeoutJobsDeletesJobAfterStartTimeTimeout(t *testing.T) {
	now := time.Date(2026, time.August, 28, 12, 0, 0, 0, time.UTC)
	job := timeoutTestJob(
		"timed-out",
		now.Add(-5*time.Minute),
		metav1.NewTime(now.Add(-30*time.Minute)),
		planetscalev2.BackupMethodVtctldclient,
	)

	var propagationPolicy *metav1.DeletionPropagation
	scheme := newScheme()
	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(job).
		WithInterceptorFuncs(interceptor.Funcs{
			Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
				if _, ok := obj.(*kbatch.Job); ok {
					propagationPolicy = (&client.DeleteOptions{}).ApplyOptions(opts).PropagationPolicy
				}
				return c.Delete(ctx, obj, opts...)
			},
		}).
		Build()
	r := &ReconcileVitessBackupsSchedule{client: k8sClient}

	require.NoError(t, r.removeTimeoutJobs(t.Context(), []*kbatch.Job{job}, "test-vbsc", 10, now))

	err := r.client.Get(t.Context(), client.ObjectKeyFromObject(job), &kbatch.Job{})
	assert.True(t, apierrors.IsNotFound(err), "expected timed-out Job to be deleted, got: %v", err)
	require.NotNil(t, propagationPolicy)
	assert.Equal(t, metav1.DeletePropagationForeground, *propagationPolicy)
}

func TestRemoveTimeoutJobsWaitsForJobStartTime(t *testing.T) {
	now := time.Date(2026, time.August, 28, 12, 0, 0, 0, time.UTC)
	job := timeoutTestJob(
		"not-started",
		now.Add(-2*time.Hour),
		metav1.Time{},
		planetscalev2.BackupMethodVtctldclient,
	)
	job.Status.StartTime = nil

	scheme := newScheme()
	r := &ReconcileVitessBackupsSchedule{
		client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(job).Build(),
	}

	require.NoError(t, r.removeTimeoutJobs(t.Context(), []*kbatch.Job{job}, "test-vbsc", 10, now))

	remaining := &kbatch.Job{}
	require.NoError(t, r.client.Get(t.Context(), client.ObjectKeyFromObject(job), remaining))
}

func TestRemoveTimeoutJobsSkipsJobAlreadyBeingDeleted(t *testing.T) {
	now := time.Date(2026, time.August, 28, 12, 0, 0, 0, time.UTC)
	job := timeoutTestJob(
		"already-deleting",
		now.Add(-30*time.Minute),
		metav1.NewTime(now.Add(-30*time.Minute)),
		planetscalev2.BackupMethodVtctldclient,
	)
	deletionTime := metav1.NewTime(now.Add(-time.Minute))
	job.DeletionTimestamp = &deletionTime
	job.Finalizers = []string{"test.planetscale.com/keep-job"}

	jobDeletes := 0
	scheme := newScheme()
	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(job).
		WithInterceptorFuncs(interceptor.Funcs{
			Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
				if _, ok := obj.(*kbatch.Job); ok {
					jobDeletes++
				}
				return c.Delete(ctx, obj, opts...)
			},
		}).
		Build()
	r := &ReconcileVitessBackupsSchedule{client: k8sClient}

	require.NoError(t, r.removeTimeoutJobs(t.Context(), []*kbatch.Job{job}, "test-vbsc", 10, now))
	assert.Zero(t, jobDeletes)
}

func TestRemoveTimeoutJobsAllowsDisabledTimeout(t *testing.T) {
	now := time.Date(2026, time.August, 28, 12, 0, 0, 0, time.UTC)
	job := timeoutTestJob(
		"timeout-disabled",
		now.Add(-24*time.Hour),
		metav1.NewTime(now.Add(-24*time.Hour)),
		planetscalev2.BackupMethodVtctldclient,
	)

	scheme := newScheme()
	r := &ReconcileVitessBackupsSchedule{
		client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(job).Build(),
	}

	require.NoError(t, r.removeTimeoutJobs(t.Context(), []*kbatch.Job{job}, "test-vbsc", -1, now))

	remaining := &kbatch.Job{}
	require.NoError(t, r.client.Get(t.Context(), client.ObjectKeyFromObject(job), remaining))
}

func TestRemoveTimeoutJobsPreservesPVCWhenJobDeletionFails(t *testing.T) {
	now := time.Date(2026, time.August, 28, 12, 0, 0, 0, time.UTC)
	job := timeoutTestJob(
		"delete-fails",
		now.Add(-30*time.Minute),
		metav1.NewTime(now.Add(-30*time.Minute)),
		planetscalev2.BackupMethodVtbackup,
	)
	pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{
		Name:      job.Name,
		Namespace: job.Namespace,
	}}

	pvcDeleteCalled := false
	scheme := newScheme()
	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(job, pvc).
		WithInterceptorFuncs(interceptor.Funcs{
			Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
				switch obj.(type) {
				case *kbatch.Job:
					return apierrors.NewServiceUnavailable("Job deletion unavailable")
				case *corev1.PersistentVolumeClaim:
					pvcDeleteCalled = true
				}
				return c.Delete(ctx, obj, opts...)
			},
		}).
		Build()
	r := &ReconcileVitessBackupsSchedule{client: k8sClient}

	err := r.removeTimeoutJobs(t.Context(), []*kbatch.Job{job}, "test-vbsc", 10, now)
	require.ErrorContains(t, err, "Job deletion unavailable")
	assert.False(t, pvcDeleteCalled)
	require.NoError(t, r.client.Get(t.Context(), client.ObjectKeyFromObject(pvc), &corev1.PersistentVolumeClaim{}))
}

func timeoutTestJob(name string, scheduledTime time.Time, startTime metav1.Time, method planetscalev2.BackupMethod) *kbatch.Job {
	return &kbatch.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
			Labels: map[string]string{
				planetscalev2.BackupMethodLabel: string(method),
			},
			Annotations: map[string]string{
				scheduledTimeAnnotation: scheduledTime.Format(time.RFC3339),
			},
		},
		Status: kbatch.JobStatus{StartTime: &startTime},
	}
}
