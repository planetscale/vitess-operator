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

package vitesscluster

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	apilabels "k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/lockserver"
	"planetscale.dev/vitess-operator/pkg/operator/names"
	"planetscale.dev/vitess-operator/test/integration/framework"
)

const (
	basicVitessCluster = `
spec:
  cells:
  - name: cell1
  - name: cell2
  - name: cell3
    gateway:
      autoscaler:
        minReplicas: 1
        maxReplicas: 2
        metrics:
          - type: Resource
            resource:
              name: cpu
              target:
                type: Utilization
                averageUtilization: 80
  keyspaces:
  - name: keyspace1
    partitionings:
    - equal:
        parts: 2
        shardTemplate:
          databaseInitScriptSecret:
            key: init_db.sql
            name: init-script-secret
          tabletPools:
          - cell: cell1
            type: replica
            replicas: 3
            vttablet:
              terminationGracePeriodSeconds: 60
              vtbackupExtraFlags:
                foo: bar
                baz: qux
            mysqld: {}
            dataVolumeClaimTemplate:
              accessModes: [ReadWriteOnce]
              resources:
                requests:
                  storage: 1Gi
          - cell: cell2
            type: rdonly
            replicas: 3
            mysqld: {}
            dataVolumeClaimTemplate:
              accessModes: [ReadWriteOnce]
              resources:
                requests:
                  storage: 1Gi
    - equal:
        parts: 1
        shardTemplate:
          databaseInitScriptSecret:
            key: init_db.sql
            name: init-script-secret
          tabletPools:
          - cell: cell3
            type: replica
            replicas: 3
            mysqld: {}
            externalDatastore:
              port: 3306
              credentialsSecret:
                name: cluster-config
                key: db_credentials.json
          - cell: cell3
            type: rdonly
            replicas: 3
            externalDatastore:
              port: 3307
              credentialsSecret:
                name: cluster-config
                key: db_credentials.json
          - cell: cell3
            type: rdonly
            replicas: 3
            name: unmanaged-replica-2
            externalDatastore:
              port: 3308
              credentialsSecret:
                name: cluster-config
                key: db_credentials.json
  backup:
    locations:
    - name: vbs1
    - name: ""
`
)

func TestMain(m *testing.M) {
	framework.TestMain(m.Run)
}

func TestBasicVitessCluster(t *testing.T) {
	ctx := context.Background()

	f := framework.NewFixture(ctx, t)
	defer f.TearDown()

	ns := "default"
	cluster := "test-basic-vitess-cluster"

	f.CreateVitessClusterYAML(ns, cluster, basicVitessCluster)
	verifyBasicVitessCluster(f, ns, cluster)
}

func verifyBasicVitessCluster(f *framework.Fixture, ns, cluster string) {
	// Check that each controller creates its children.
	// This at least verifies that the generated objects are accepted by the
	// apiserver (i.e. they pass validation).

	// VitessCluster creates VitessCells.
	verifyBasicVitessCell(f, ns, cluster, "cell1")
	verifyBasicVitessCell(f, ns, cluster, "cell2")
	verifyBasicVitessCell(f, ns, cluster, "cell3")
	f.MustGet(ns, names.JoinWithConstraints(names.DefaultConstraints, cluster, "cell3"), &autoscalingv2.HorizontalPodAutoscaler{})

	// VitessCluster creates VitessKeyspaces.
	verifyBasicVitessKeyspace(f, ns, cluster, "keyspace1")

	// VitessCluster creates VitessBackupStorages.
	verifyBasicVitessBackupStorage(f, ns, cluster, "")
	verifyBasicVitessBackupStorage(f, ns, cluster, "vbs1")

	// VitessCluster creates global EtcdLockserver.
	verifyBasicEtcdLockserver(f, ns, cluster)

	// VitessCluster creates global Services.
	f.MustGet(ns, names.JoinWithConstraints(names.ServiceConstraints, cluster, "vtctld"), &corev1.Service{})
	f.MustGet(ns, names.JoinWithConstraints(names.ServiceConstraints, cluster, "vtgate"), &corev1.Service{})
	f.MustGet(ns, names.JoinWithConstraints(names.ServiceConstraints, cluster, "vttablet"), &corev1.Service{})

	// VitessCluster creates vtctld Deployments.
	f.MustGet(ns, names.JoinWithConstraints(names.DefaultConstraints, cluster, "cell1", "vtctld"), &appsv1.Deployment{})
	f.MustGet(ns, names.JoinWithConstraints(names.DefaultConstraints, cluster, "cell2", "vtctld"), &appsv1.Deployment{})
	f.MustGet(ns, names.JoinWithConstraints(names.DefaultConstraints, cluster, "cell3", "vtctld"), &appsv1.Deployment{})
}

func verifyBasicVitessBackupStorage(f *framework.Fixture, ns, cluster, location string) {
	var vbsName string
	if location == "" {
		vbsName = names.JoinWithConstraints(names.DefaultConstraints, cluster)
	} else {
		vbsName = names.JoinWithConstraints(names.DefaultConstraints, cluster, location)
	}

	f.MustGet(ns, vbsName, &planetscalev2.VitessBackupStorage{})

	// VitessBackupStorage creates vbs-subcontroller Pod.
	f.MustGet(ns, vbsName+"-vitessbackupstorage-subcontroller", &corev1.Pod{})
}

func verifyBasicEtcdLockserver(f *framework.Fixture, ns, cluster string) {
	etcdName := names.JoinWithConstraints(lockserver.EtcdLockserverNameConstraints, cluster, "etcd")

	f.MustGet(ns, etcdName, &planetscalev2.EtcdLockserver{})

	// EtcdLockserver creates client/peer Services.
	f.MustGet(ns, etcdName+"-client", &corev1.Service{})
	f.MustGet(ns, etcdName+"-peer", &corev1.Service{})

	// EtcdLockserver creates etcd Pods and PVCs.
	f.MustGet(ns, etcdName+"-1", &corev1.Pod{})
	f.MustGet(ns, etcdName+"-2", &corev1.Pod{})
	f.MustGet(ns, etcdName+"-3", &corev1.Pod{})
	f.MustGet(ns, etcdName+"-1", &corev1.PersistentVolumeClaim{})
	f.MustGet(ns, etcdName+"-2", &corev1.PersistentVolumeClaim{})
	f.MustGet(ns, etcdName+"-3", &corev1.PersistentVolumeClaim{})
}

func verifyBasicVitessCell(f *framework.Fixture, ns, cluster, cell string) {
	f.MustGet(ns, names.JoinWithConstraints(names.DefaultConstraints, cluster, cell), &planetscalev2.VitessCell{})

	// VitessCell creates vtgate Service/Deployment.
	f.MustGet(ns, names.JoinWithConstraints(names.ServiceConstraints, cluster, cell, "vtgate"), &corev1.Service{})
	f.MustGet(ns, names.JoinWithConstraints(names.DefaultConstraints, cluster, cell, "vtgate"), &appsv1.Deployment{})
}

func verifyBasicVitessKeyspace(f *framework.Fixture, ns, cluster, keyspace string) {
	f.MustGet(ns, names.JoinWithConstraints(names.DefaultConstraints, cluster, keyspace), &planetscalev2.VitessKeyspace{})

	// VitessKeyspaces create VitessShards.
	verifyBasicVitessShard(f, ns, cluster, keyspace, "x-80", []int{3, 3})
	verifyBasicVitessShard(f, ns, cluster, keyspace, "80-x", []int{3, 3})
	verifyBasicVitessShardExternal(f, ns, cluster, keyspace, "x-x", []int{3, 3, 3})
}

func verifyBasicVitessShard(f *framework.Fixture, ns, cluster, keyspace, shard string, expectedTabletCount []int) {
	f.MustGet(ns, names.JoinWithConstraints(names.DefaultConstraints, cluster, keyspace, shard), &planetscalev2.VitessShard{})

	// VitessShard creates vttablet Pods.
	cell1Pods := f.ExpectPods(&client.ListOptions{
		Namespace:     ns,
		LabelSelector: tabletPodSelector(cluster, keyspace, shard, "cell1", "replica"),
	}, expectedTabletCount[0])
	cell2Pods := f.ExpectPods(&client.ListOptions{
		Namespace:     ns,
		LabelSelector: tabletPodSelector(cluster, keyspace, shard, "cell2", "rdonly"),
	}, expectedTabletCount[1])

	// Each vttablet Pod should have a PVC.
	for i := range cell1Pods.Items {
		f.MustGet(ns, cell1Pods.Items[i].Name, &corev1.PersistentVolumeClaim{})
		if *cell1Pods.Items[i].Spec.TerminationGracePeriodSeconds != 60 {
			f.Fatalf("TerminationGracePeriodSeconds should be 60, but got %d", *cell1Pods.Items[i].Spec.TerminationGracePeriodSeconds)
		}
	}
	for i := range cell2Pods.Items {
		f.MustGet(ns, cell2Pods.Items[i].Name, &corev1.PersistentVolumeClaim{})
		if *cell2Pods.Items[i].Spec.TerminationGracePeriodSeconds != 1800 {
			f.Fatalf("TerminationGracePeriodSeconds should be 1800, but got %d", *cell2Pods.Items[i].Spec.TerminationGracePeriodSeconds)
		}
	}

	// VitessShard creates vtbackup-init Pod/PVC.
	var pod corev1.Pod
	var args []string
	var found bool
	f.MustGet(ns, names.JoinWithConstraints(names.DefaultConstraints, cluster, keyspace, shard, "vtbackup", "init"), &pod)
	f.MustGet(ns, names.JoinWithConstraints(names.DefaultConstraints, cluster, keyspace, shard, "vtbackup", "init"), &corev1.PersistentVolumeClaim{})
        containerNames := make([]string, len(pod.Spec.Containers))
	for i, c := range pod.Spec.Containers {
		containerNames[i] = c.Name
		if c.Name == "vtbackup" {
			args = c.Args
			found = true
			break
		}
	}
	require.True(f.T, found, "vtbackup container not found in pod. Containers: %v", containerNames)
	joined := strings.Join(args, " ")
	require.Contains(f.T, joined, "--foo=bar", "vtbackup args missing --foo=bar. Args: %v", args)
	require.Contains(f.T, joined, "--baz=qux", "vtbackup args missing --baz=qux. Args: %v", args)
}

func verifyBasicVitessShardExternal(f *framework.Fixture, ns, cluster, keyspace, shard string, expectedTabletCount []int) {
	f.MustGet(ns, names.JoinWithConstraints(names.DefaultConstraints, cluster, keyspace, shard), &planetscalev2.VitessShard{})

	// VitessShard creates vttablet Pods.
	f.ExpectPods(&client.ListOptions{
		Namespace:     ns,
		LabelSelector: tabletPodExternalSelector(cluster, keyspace, shard, "cell3", "replica", ""),
	}, expectedTabletCount[0])
	f.ExpectPods(&client.ListOptions{
		Namespace:     ns,
		LabelSelector: tabletPodExternalSelector(cluster, keyspace, shard, "cell3", "rdonly", ""),
	}, expectedTabletCount[1])
	f.ExpectPods(&client.ListOptions{
		Namespace:     ns,
		LabelSelector: tabletPodExternalSelector(cluster, keyspace, shard, "cell3", "rdonly", "unmanaged-replica-2"),
	}, expectedTabletCount[2])
}

func tabletPodSelector(cluster, keyspace, shard, cell, tabletType string) apilabels.Selector {
	// This intentionally does NOT use any shared constants because we want the
	// test to fail if the labels change, since that's a breaking change.
	return apilabels.Set{
		"planetscale.com/cluster":     cluster,
		"planetscale.com/keyspace":    keyspace,
		"planetscale.com/shard":       shard,
		"planetscale.com/cell":        cell,
		"planetscale.com/tablet-type": tabletType,
	}.AsSelector()
}

func tabletPodExternalSelector(cluster, keyspace, shard, cell, tabletType, poolName string) apilabels.Selector {
	// This intentionally does NOT use any shared constants because we want the
	// test to fail if the labels change, since that's a breaking change.
	return apilabels.Set{
		"planetscale.com/cluster":     cluster,
		"planetscale.com/keyspace":    keyspace,
		"planetscale.com/shard":       shard,
		"planetscale.com/cell":        cell,
		"planetscale.com/tablet-type": tabletType,
		"planetscale.com/pool-name":   poolName,
	}.AsSelector()
}
