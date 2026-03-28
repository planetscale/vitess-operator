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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	kbatch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
)

func vtctldService() *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-zone1-vtctld",
			Namespace: "default",
			Labels: map[string]string{
				planetscalev2.ClusterLabel:   "example",
				planetscalev2.ComponentLabel: planetscalev2.VtctldComponentName,
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name: planetscalev2.DefaultGrpcPortName,
					Port: 15999,
				},
			},
		},
	}
}

func vtctldCluster() *planetscalev2.VitessCluster {
	return &planetscalev2.VitessCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example",
			Namespace: "default",
		},
		Spec: planetscalev2.VitessClusterSpec{
			Images: planetscalev2.VitessImages{
				Vtctld: "vitess/operator:mysql80",
			},
			ImagePullPolicies: planetscalev2.VitessImagePullPolicies{
				Vtctld: corev1.PullAlways,
			},
			ImagePullSecrets: []corev1.LocalObjectReference{
				{Name: "regcred"},
			},
			VitessDashboard: &planetscalev2.VitessDashboardSpec{
				Tolerations: []corev1.Toleration{
					{
						Key:      "dedicated",
						Operator: corev1.TolerationOpExists,
					},
				},
			},
		},
	}
}

func vtctldclientVBSC() planetscalev2.VitessBackupSchedule {
	return planetscalev2.VitessBackupSchedule{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "daily",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Now().Add(-2 * time.Hour)),
			UID:               types.UID("test-uid"),
			ResourceVersion:   "1",
			Generation:        1,
		},
		Spec: planetscalev2.VitessBackupScheduleSpec{
			Cluster:         "example",
			Image:           "vitess/lite:mysql80",
			ImagePullPolicy: corev1.PullIfNotPresent,
			VitessBackupScheduleTemplate: planetscalev2.VitessBackupScheduleTemplate{
				Name:         "daily",
				Schedule:     "0 0 * * *",
				BackupMethod: planetscalev2.BackupMethodVtctldclient,
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("256Mi"),
					},
				},
				Strategy: []planetscalev2.VitessBackupScheduleStrategy{
					{
						Name:     "commerce-x",
						Keyspace: "commerce",
						Shard:    "-",
					},
				},
			},
		},
		Status: planetscalev2.NewVitessBackupScheduleStatus(planetscalev2.VitessBackupScheduleStatus{}),
	}
}

func TestCreateVtctldclientJobPod_BasicPodSpec(t *testing.T) {
	r := &ReconcileVitessBackupsSchedule{
		client: fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(vtctldService(), vtctldCluster()).Build(),
	}

	vbsc := vtctldclientVBSC()
	strategy := vbsc.Spec.Strategy[0]

	pod, err := r.createVtctldclientJobPod(t.Context(), vbsc, strategy)
	require.NoError(t, err)
	require.NotNil(t, pod)

	// Single container, no init containers
	require.Len(t, pod.Spec.Containers, 1)
	require.Empty(t, pod.Spec.InitContainers)

	container := pod.Spec.Containers[0]
	assert.Equal(t, "vtctldclient", container.Name)
	assert.Equal(t, "vitess/lite:mysql80", container.Image)
	assert.Equal(t, corev1.PullIfNotPresent, container.ImagePullPolicy)
	assert.Equal(t, []string{"/vt/bin/vtctldclient"}, container.Command)
	assert.Equal(t, vbsc.Spec.Resources, container.Resources)

	// RestartPolicy should be Never
	assert.Equal(t, corev1.RestartPolicyNever, pod.Spec.RestartPolicy)

	// No volumes (vtctldclient doesn't need local storage)
	assert.Empty(t, pod.Spec.Volumes)
}

func TestCreateVtctldclientJobPod_InheritsClusterDefaults(t *testing.T) {
	cluster := vtctldCluster()
	r := &ReconcileVitessBackupsSchedule{
		client: fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(vtctldService(), cluster).Build(),
	}

	vbsc := vtctldclientVBSC()
	vbsc.Spec.Image = ""
	vbsc.Spec.ImagePullPolicy = ""
	strategy := vbsc.Spec.Strategy[0]

	pod, err := r.createVtctldclientJobPod(t.Context(), vbsc, strategy)
	require.NoError(t, err)
	require.NotNil(t, pod)

	container := pod.Spec.Containers[0]
	assert.Equal(t, cluster.Spec.Images.Vtctld, container.Image)
	assert.Equal(t, cluster.Spec.ImagePullPolicies.Vtctld, container.ImagePullPolicy)
	assert.Equal(t, cluster.Spec.ImagePullSecrets, pod.Spec.ImagePullSecrets)
	assert.Equal(t, cluster.Spec.VitessDashboard.Tolerations, pod.Spec.Tolerations)
	assert.Equal(t, planetscalev2.DefaultVitessPriorityClass, pod.Spec.PriorityClassName)
}

func TestCreateVtctldclientJobPod_CorrectArgs(t *testing.T) {
	r := &ReconcileVitessBackupsSchedule{
		client: fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(vtctldService(), vtctldCluster()).Build(),
	}

	vbsc := vtctldclientVBSC()
	strategy := vbsc.Spec.Strategy[0]

	pod, err := r.createVtctldclientJobPod(t.Context(), vbsc, strategy)
	require.NoError(t, err)

	args := pod.Spec.Containers[0].Args
	require.Len(t, args, 3)
	assert.Equal(t, "--server=example-zone1-vtctld:15999", args[0])
	assert.Equal(t, "BackupShard", args[1])
	assert.Equal(t, "commerce/-", args[2])
}

func TestCreateVtctldclientJobPod_ExtraFlags(t *testing.T) {
	r := &ReconcileVitessBackupsSchedule{
		client: fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(vtctldService(), vtctldCluster()).Build(),
	}

	vbsc := vtctldclientVBSC()
	strategy := planetscalev2.VitessBackupScheduleStrategy{
		Name:     "commerce-x",
		Keyspace: "commerce",
		Shard:    "-",
		ExtraFlags: map[string]string{
			"concurrency":   "4",
			"allow-primary": "true",
		},
	}

	pod, err := r.createVtctldclientJobPod(t.Context(), vbsc, strategy)
	require.NoError(t, err)

	args := pod.Spec.Containers[0].Args
	// --server, BackupShard, 2 extra flags (sorted), keyspace/shard
	require.Len(t, args, 5)
	assert.Equal(t, "--server=example-zone1-vtctld:15999", args[0])
	assert.Equal(t, "BackupShard", args[1])
	// Extra flags should be sorted alphabetically
	assert.Equal(t, "--allow-primary=true", args[2])
	assert.Equal(t, "--concurrency=4", args[3])
	assert.Equal(t, "commerce/-", args[4])
}

func TestCreateVtctldclientJobPod_NoVtctldService(t *testing.T) {
	// No vtctld service in the fake client
	r := &ReconcileVitessBackupsSchedule{
		client: fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(vtctldCluster()).Build(),
	}

	vbsc := vtctldclientVBSC()
	strategy := vbsc.Spec.Strategy[0]

	pod, err := r.createVtctldclientJobPod(t.Context(), vbsc, strategy)
	require.Error(t, err)
	require.Nil(t, pod)
	require.Contains(t, err.Error(), "no vtctld service found")
}

func TestCreateJob_VtctldclientMethodNoPVC(t *testing.T) {
	scheme := newScheme()
	r := &ReconcileVitessBackupsSchedule{
		client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(vtctldService(), vtctldCluster()).Build(),
		scheme: scheme,
	}

	vbsc := vtctldclientVBSC()
	strategy := vbsc.Spec.Strategy[0]
	vkr := planetscalev2.VitessKeyRange{}

	job, err := r.createJob(t.Context(), vbsc, strategy, time.Now(), vkr)
	require.NoError(t, err)
	require.NotNil(t, job)

	// Verify the backup-method label is set
	assert.Equal(t, string(planetscalev2.BackupMethodVtctldclient), job.Labels[planetscalev2.BackupMethodLabel])

	// Verify no PVC was created
	pvcList := &corev1.PersistentVolumeClaimList{}
	err = r.client.List(t.Context(), pvcList, client.InNamespace("default"))
	require.NoError(t, err)
	assert.Empty(t, pvcList.Items)
}

func TestCreateJob_VtbackupMethodCreatesPVC(t *testing.T) {
	initBackup := true
	vts := &planetscalev2.VitessShard{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-commerce-x-x",
			Namespace: "default",
			Labels: map[string]string{
				planetscalev2.ClusterLabel:  "example",
				planetscalev2.KeyspaceLabel: "commerce",
			},
		},
		Spec: planetscalev2.VitessShardSpec{
			VitessShardTemplate: planetscalev2.VitessShardTemplate{
				TabletPools: []planetscalev2.VitessShardTabletPool{{
					Cell:     "zone1",
					Type:     planetscalev2.ReplicaPoolType,
					Replicas: 1,
					Vttablet: planetscalev2.VttabletSpec{},
					Mysqld:   &planetscalev2.MysqldSpec{},
					DataVolumeClaimTemplate: &corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse("10Gi"),
							},
						},
					},
				}},
				Replication: planetscalev2.VitessReplicationSpec{
					InitializeBackup: &initBackup,
				},
			},
			Images: planetscalev2.VitessKeyspaceImages{
				Vtbackup: "vitess/lite:mysql80",
				Vttablet: "vitess/lite:mysql80",
				Vtorc:    "vitess/lite:mysql80",
				Mysqld: &planetscalev2.MysqldImage{
					Mysql80Compatible: "vitess/lite:mysql80",
				},
			},
			KeyRange:        planetscalev2.VitessKeyRange{},
			BackupLocations: []planetscalev2.VitessBackupLocation{{Name: ""}},
		},
		Status: readyShardStatus(),
	}

	scheme := newScheme()
	r := &ReconcileVitessBackupsSchedule{
		client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(vts).Build(),
		scheme: scheme,
	}

	vbsc := planetscalev2.VitessBackupSchedule{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "daily",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Now().Add(-2 * time.Hour)),
			UID:               types.UID("test-uid"),
			ResourceVersion:   "1",
		},
		Spec: planetscalev2.VitessBackupScheduleSpec{
			Cluster: "example",
			VitessBackupScheduleTemplate: planetscalev2.VitessBackupScheduleTemplate{
				Name:         "daily",
				Schedule:     "0 0 * * *",
				BackupMethod: planetscalev2.BackupMethodVtbackup,
				Resources:    corev1.ResourceRequirements{},
				Strategy: []planetscalev2.VitessBackupScheduleStrategy{
					{Name: "commerce-x", Keyspace: "commerce", Shard: "-"},
				},
			},
		},
		Status: planetscalev2.NewVitessBackupScheduleStatus(planetscalev2.VitessBackupScheduleStatus{}),
	}
	strategy := vbsc.Spec.Strategy[0]
	vkr := planetscalev2.VitessKeyRange{}

	job, err := r.createJob(t.Context(), vbsc, strategy, time.Now(), vkr)
	require.NoError(t, err)
	require.NotNil(t, job)

	// Verify the backup-method label is set
	assert.Equal(t, string(planetscalev2.BackupMethodVtbackup), job.Labels[planetscalev2.BackupMethodLabel])

	// Verify a PVC was created
	pvcList := &corev1.PersistentVolumeClaimList{}
	err = r.client.List(t.Context(), pvcList, client.InNamespace("default"))
	require.NoError(t, err)
	assert.Len(t, pvcList.Items, 1)
}

func TestCreateJob_DefaultMethodIsVtbackup(t *testing.T) {
	// When BackupMethod is empty, it should default to vtbackup.
	initBackup := true
	vts := &planetscalev2.VitessShard{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-commerce-x-x",
			Namespace: "default",
			Labels: map[string]string{
				planetscalev2.ClusterLabel:  "example",
				planetscalev2.KeyspaceLabel: "commerce",
			},
		},
		Spec: planetscalev2.VitessShardSpec{
			VitessShardTemplate: planetscalev2.VitessShardTemplate{
				TabletPools: []planetscalev2.VitessShardTabletPool{{
					Cell:     "zone1",
					Type:     planetscalev2.ReplicaPoolType,
					Replicas: 1,
					Vttablet: planetscalev2.VttabletSpec{},
					Mysqld:   &planetscalev2.MysqldSpec{},
					DataVolumeClaimTemplate: &corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse("10Gi"),
							},
						},
					},
				}},
				Replication: planetscalev2.VitessReplicationSpec{
					InitializeBackup: &initBackup,
				},
			},
			Images: planetscalev2.VitessKeyspaceImages{
				Vtbackup: "vitess/lite:mysql80",
				Vttablet: "vitess/lite:mysql80",
				Vtorc:    "vitess/lite:mysql80",
				Mysqld: &planetscalev2.MysqldImage{
					Mysql80Compatible: "vitess/lite:mysql80",
				},
			},
			KeyRange:        planetscalev2.VitessKeyRange{},
			BackupLocations: []planetscalev2.VitessBackupLocation{{Name: ""}},
		},
		Status: readyShardStatus(),
	}

	scheme := newScheme()
	r := &ReconcileVitessBackupsSchedule{
		client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(vts).Build(),
		scheme: scheme,
	}

	vbsc := planetscalev2.VitessBackupSchedule{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "daily",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Now().Add(-2 * time.Hour)),
			UID:               types.UID("test-uid"),
			ResourceVersion:   "1",
		},
		Spec: planetscalev2.VitessBackupScheduleSpec{
			Cluster: "example",
			VitessBackupScheduleTemplate: planetscalev2.VitessBackupScheduleTemplate{
				Name:         "daily",
				Schedule:     "0 0 * * *",
				BackupMethod: "", // empty — should default to vtbackup
				Resources:    corev1.ResourceRequirements{},
				Strategy: []planetscalev2.VitessBackupScheduleStrategy{
					{Name: "commerce-x", Keyspace: "commerce", Shard: "-"},
				},
			},
		},
		Status: planetscalev2.NewVitessBackupScheduleStatus(planetscalev2.VitessBackupScheduleStatus{}),
	}
	strategy := vbsc.Spec.Strategy[0]
	vkr := planetscalev2.VitessKeyRange{}

	job, err := r.createJob(t.Context(), vbsc, strategy, time.Now(), vkr)
	require.NoError(t, err)
	require.NotNil(t, job)

	// Should have defaulted to vtbackup
	assert.Equal(t, string(planetscalev2.BackupMethodVtbackup), job.Labels[planetscalev2.BackupMethodLabel])

	// Verify a PVC was created (vtbackup behavior)
	pvcList := &corev1.PersistentVolumeClaimList{}
	err = r.client.List(t.Context(), pvcList, client.InNamespace("default"))
	require.NoError(t, err)
	assert.Len(t, pvcList.Items, 1)
}

func TestCreateJob_ControllerOwnedLabelsCannotBeOverridden(t *testing.T) {
	scheme := newScheme()
	r := &ReconcileVitessBackupsSchedule{
		client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(vtctldService(), vtctldCluster()).Build(),
		scheme: scheme,
	}

	vbsc := vtctldclientVBSC()
	vbsc.Labels = map[string]string{
		planetscalev2.BackupMethodLabel: "vtbackup",
		planetscalev2.ClusterLabel:      "wrong-cluster",
		"custom":                        "kept",
	}
	strategy := vbsc.Spec.Strategy[0]
	vkr := planetscalev2.VitessKeyRange{}

	job, err := r.createJob(t.Context(), vbsc, strategy, time.Now(), vkr)
	require.NoError(t, err)
	require.NotNil(t, job)

	assert.Equal(t, string(planetscalev2.BackupMethodVtctldclient), job.Labels[planetscalev2.BackupMethodLabel])
	assert.Equal(t, vbsc.Spec.Cluster, job.Labels[planetscalev2.ClusterLabel])
	assert.Equal(t, vbsc.Name, job.Labels[planetscalev2.BackupScheduleLabel])
	assert.Equal(t, "kept", job.Labels["custom"])
}

func TestCleanupJobsWithLimit_SkipsPVCForVtctldclientJobs(t *testing.T) {
	vtctldJob := &kbatch.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vtctldclient-job",
			Namespace: "default",
			Labels: map[string]string{
				planetscalev2.BackupMethodLabel: string(planetscalev2.BackupMethodVtctldclient),
			},
		},
		Status: kbatch.JobStatus{
			StartTime: &metav1.Time{Time: time.Now().Add(-1 * time.Hour)},
		},
	}
	vtbackupJob := &kbatch.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vtbackup-job",
			Namespace: "default",
			Labels: map[string]string{
				planetscalev2.BackupMethodLabel: string(planetscalev2.BackupMethodVtbackup),
			},
		},
		Status: kbatch.JobStatus{
			StartTime: &metav1.Time{Time: time.Now().Add(-2 * time.Hour)},
		},
	}
	// Create a PVC only for the vtbackup job (vtctldclient jobs don't have PVCs)
	vtbackupPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vtbackup-job",
			Namespace: "default",
		},
	}

	scheme := newScheme()
	r := &ReconcileVitessBackupsSchedule{
		client: fake.NewClientBuilder().WithScheme(scheme).
			WithObjects(vtctldJob, vtbackupJob, vtbackupPVC).Build(),
	}

	jobs := []*kbatch.Job{vtbackupJob, vtctldJob}

	// Clean up with limit 0 (delete all)
	r.cleanupJobsWithLimit(t.Context(), jobs, 0)

	// Both jobs should be deleted
	jobList := &kbatch.JobList{}
	err := r.client.List(t.Context(), jobList, client.InNamespace("default"))
	require.NoError(t, err)
	assert.Empty(t, jobList.Items)

	// The vtbackup PVC should also be deleted
	pvcList := &corev1.PersistentVolumeClaimList{}
	err = r.client.List(t.Context(), pvcList, client.InNamespace("default"))
	require.NoError(t, err)
	assert.Empty(t, pvcList.Items)
}

func TestRemoveTimeoutJobs_SkipsPVCForVtctldclientJobs(t *testing.T) {
	scheduledAt := time.Now().Add(-30 * time.Minute)
	vtctldJob := &kbatch.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vtctldclient-timeout",
			Namespace: "default",
			Labels: map[string]string{
				planetscalev2.BackupMethodLabel: string(planetscalev2.BackupMethodVtctldclient),
			},
			Annotations: map[string]string{
				scheduledTimeAnnotation: scheduledAt.Format(time.RFC3339),
			},
		},
	}

	scheme := newScheme()
	r := &ReconcileVitessBackupsSchedule{
		client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(vtctldJob).Build(),
	}

	// Timeout of 10 minutes — the job scheduled 30min ago should be timed out
	err := r.removeTimeoutJobs(t.Context(), []*kbatch.Job{vtctldJob}, "test-vbsc", 10)
	require.NoError(t, err)

	// The job should be deleted
	jobList := &kbatch.JobList{}
	err = r.client.List(t.Context(), jobList, client.InNamespace("default"))
	require.NoError(t, err)
	assert.Empty(t, jobList.Items)

	// No PVC should exist (and no error from trying to delete a non-existent one)
	pvcList := &corev1.PersistentVolumeClaimList{}
	err = r.client.List(t.Context(), pvcList, client.InNamespace("default"))
	require.NoError(t, err)
	assert.Empty(t, pvcList.Items)
}

func TestCreateVtctldclientJobPod_SecurityContext(t *testing.T) {
	r := &ReconcileVitessBackupsSchedule{
		client: fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(vtctldService(), vtctldCluster()).Build(),
	}

	vbsc := vtctldclientVBSC()
	strategy := vbsc.Spec.Strategy[0]

	pod, err := r.createVtctldclientJobPod(t.Context(), vbsc, strategy)
	require.NoError(t, err)

	// Pod security context should be set if defaults are configured
	if planetscalev2.DefaultVitessFSGroup >= 0 {
		require.NotNil(t, pod.Spec.SecurityContext)
		require.NotNil(t, pod.Spec.SecurityContext.FSGroup)
		assert.Equal(t, planetscalev2.DefaultVitessFSGroup, *pod.Spec.SecurityContext.FSGroup)
	}

	// Container security context
	container := pod.Spec.Containers[0]
	if planetscalev2.DefaultVitessRunAsUser >= 0 {
		require.NotNil(t, container.SecurityContext)
		require.NotNil(t, container.SecurityContext.RunAsUser)
		assert.Equal(t, planetscalev2.DefaultVitessRunAsUser, *container.SecurityContext.RunAsUser)
	}
}

func TestCreateVtctldclientJobPod_Affinity(t *testing.T) {
	r := &ReconcileVitessBackupsSchedule{
		client: fake.NewClientBuilder().WithScheme(newScheme()).WithObjects(vtctldService(), vtctldCluster()).Build(),
	}

	vbsc := vtctldclientVBSC()
	vbsc.Spec.Affinity = &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{Key: "disktype", Operator: corev1.NodeSelectorOpIn, Values: []string{"ssd"}},
						},
					},
				},
			},
		},
	}
	strategy := vbsc.Spec.Strategy[0]

	pod, err := r.createVtctldclientJobPod(t.Context(), vbsc, strategy)
	require.NoError(t, err)
	require.NotNil(t, pod.Spec.Affinity)
	assert.Equal(t, vbsc.Spec.Affinity, pod.Spec.Affinity)
}
