package vitesscluster

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apilabels "k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/names"
	"planetscale.dev/vitess-operator/test/integration/framework"
)

const (
	basicVitessCluster = `
spec:
  cells:
  - name: cell1
  - name: cell2
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

	f := framework.NewFixture(t)
	defer f.TearDown(ctx)

	ns := "default"
	cluster := "cluster1"

	f.CreateVitessClusterYAML(ctx, ns, cluster, basicVitessCluster)
	verifyBasicVitessCluster(f, ns, cluster)
}

func verifyBasicVitessCluster(f *framework.Fixture, ns, cluster string) {
	// Check that each controller creates its children.
	// This at least verifies that the generated objects are accepted by the
	// apiserver (i.e. they pass validation).

	// VitessCluster creates VitessCells.
	verifyBasicVitessCell(f, ns, cluster, "cell1")
	verifyBasicVitessCell(f, ns, cluster, "cell2")

	// VitessCluster creates VitessKeyspaces.
	verifyBasicVitessKeyspace(f, ns, cluster, "keyspace1")

	// VitessCluster creates VitessBackupStorages.
	verifyBasicVitessBackupStorage(f, ns, cluster, "")
	verifyBasicVitessBackupStorage(f, ns, cluster, "vbs1")

	// VitessCluster creates global EtcdLockserver.
	verifyBasicEtcdLockserver(f, ns, cluster)

	// VitessCluster creates global Services.
	f.MustGet(ns, names.Join(cluster, "vtctld"), &corev1.Service{})
	f.MustGet(ns, names.Join(cluster, "vtgate"), &corev1.Service{})
	f.MustGet(ns, names.Join(cluster, "vttablet"), &corev1.Service{})

	// VitessCluster creates vtctld Deployments.
	f.MustGet(ns, names.Join(cluster, "cell1", "vtctld"), &appsv1.Deployment{})
	f.MustGet(ns, names.Join(cluster, "cell2", "vtctld"), &appsv1.Deployment{})
}

func verifyBasicVitessBackupStorage(f *framework.Fixture, ns, cluster, location string) {
	var vbsName string
	if location == "" {
		vbsName = names.Join(cluster)
	} else {
		vbsName = names.Join(cluster, location)
	}

	f.MustGet(ns, vbsName, &planetscalev2.VitessBackupStorage{})

	// VitessBackupStorage creates vbs-subcontroller Pod.
	f.MustGet(ns, vbsName+"-vitessbackupstorage-subcontroller", &corev1.Pod{})
}

func verifyBasicEtcdLockserver(f *framework.Fixture, ns, cluster string) {
	etcdName := names.Join(cluster, "etcd")

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
	f.MustGet(ns, names.Join(cluster, cell), &planetscalev2.VitessCell{})

	// VitessCell creates vtgate Service/Deployment.
	f.MustGet(ns, names.Join(cluster, cell, "vtgate"), &corev1.Service{})
	f.MustGet(ns, names.Join(cluster, cell, "vtgate"), &appsv1.Deployment{})
}

func verifyBasicVitessKeyspace(f *framework.Fixture, ns, cluster, keyspace string) {
	f.MustGet(ns, names.Join(cluster, keyspace), &planetscalev2.VitessKeyspace{})

	// VitessKeyspaces create VitessShards.
	verifyBasicVitessShard(f, ns, cluster, keyspace, "x-80")
	verifyBasicVitessShard(f, ns, cluster, keyspace, "80-x")
}

func verifyBasicVitessShard(f *framework.Fixture, ns, cluster, keyspace, shard string) {
	f.MustGet(ns, names.Join(cluster, keyspace, shard), &planetscalev2.VitessShard{})

	// VitessShard creates vttablet Pods.
	cell1Pods := f.ExpectPods(&client.ListOptions{
		Namespace:     ns,
		LabelSelector: tabletPodSelector(cluster, keyspace, shard, "cell1", "replica"),
	}, 3)
	cell2Pods := f.ExpectPods(&client.ListOptions{
		Namespace:     ns,
		LabelSelector: tabletPodSelector(cluster, keyspace, shard, "cell2", "rdonly"),
	}, 3)

	// Each vttablet Pod should have a PVC.
	for i := range cell1Pods.Items {
		f.MustGet(ns, cell1Pods.Items[i].Name, &corev1.PersistentVolumeClaim{})
	}
	for i := range cell2Pods.Items {
		f.MustGet(ns, cell2Pods.Items[i].Name, &corev1.PersistentVolumeClaim{})
	}

	// VitessShard creates vtbackup-init Pod/PVC.
	f.MustGet(ns, names.Join(cluster, keyspace, shard, "vtbackup", "init"), &corev1.Pod{})
	f.MustGet(ns, names.Join(cluster, keyspace, shard, "vtbackup", "init"), &corev1.PersistentVolumeClaim{})
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
