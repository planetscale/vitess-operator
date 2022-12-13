## Prerequisites

This guide assumes you have the following components and services:

- A [Google Kubernetes Engine](https://cloud.google.com/kubernetes-engine/docs) (GKE) cluster;
- A local `kubectl` client [configured to access the GKE cluster](https://cloud.google.com/kubernetes-engine/docs/how-to/cluster-access-for-kubectl) where you wish to install the operator;
- A [Google Cloud Storage (GCS) storage bucket](https://cloud.google.com/storage/docs/creating-buckets);
- A [GCP service account](https://cloud.google.com/storage/docs/projects#service-accounts) with access to the GCS storage bucket;
- A [Kubernetes secret matching your service account](https://cloud.google.com/kubernetes-engine/docs/tutorials/authenticating-to-cloud-platform#step_3_create_service_account_credentials);
- A local [installation of vtctlclient](https://vitess.io/docs/get-started/kubernetes/#prerequisites).

## Overview

To deploy a Vitess cluster on GCP using the Vitess Operator, follow these steps:

1. Download the operator installation and example database configuration files.
1. Apply the operator installation file against your Kubernetes cluster.
1. Edit the name of the GCS bucket and associated Kubernetes secret in the database configuration file.
1. Apply the database configuration file to your cluster.
1. Port-forward the `vtctld` service to your Kubernetes cluster.
1. Apply the VSchema to your Vitess database.
1. Apply the SQL schema to your Vitess database.
1. Expose the Vitess service.
1. Connect to your Vitess database using a MySQL client.

## Step 1. Download the operator and example database configuration files.

Download the following files:

- [Operator installation file](../test/endtoend/operator/operator-latest.yaml)
- [Database configuration file](../test/endtoend/operator/101_initial_cluster_vtorc_vtadmin.yaml)
- [Example VSchema](../test/endtoend/operator/vschema_commerce_initial.json)
- [Example SQL schema](../test/endtoend/operator/create_commerce_schema.sql)

This guide will assume that the above files are in your working directory.

## Step 2. Apply the operator installation file against your Kubernetes cluster.

This step assumes that `kubectl` is configured to access the GKE cluster.

Enter the following command, you should see a similar output:

```shell
$ kubectl apply -f operator.yaml
------
customresourcedefinition.apiextensions.k8s.io/etcdlockservers.planetscale.com created
customresourcedefinition.apiextensions.k8s.io/vitessbackups.planetscale.com created
customresourcedefinition.apiextensions.k8s.io/vitessbackupstorages.planetscale.com created
customresourcedefinition.apiextensions.k8s.io/vitesscells.planetscale.com created
customresourcedefinition.apiextensions.k8s.io/vitessclusters.planetscale.com created
customresourcedefinition.apiextensions.k8s.io/vitesskeyspaces.planetscale.com created
customresourcedefinition.apiextensions.k8s.io/vitessshards.planetscale.com created
serviceaccount/vitess-operator created
role.rbac.authorization.k8s.io/vitess-operator created
rolebinding.rbac.authorization.k8s.io/vitess-operator created
priorityclass.scheduling.k8s.io/vitess created
priorityclass.scheduling.k8s.io/vitess-operator-control-plane created
deployment.apps/vitess-operator created
```

You can verify the status of the operator pod using the following command:

```shell
$ kubectl get pods
------
NAME							 READY	 STATUS	 RESTARTS AGE
vitess-operator-6f54958746-mr9hp			 1/1	 Running 0	  17m
```

## Step 3. Edit the name of the Kubernetes secret in the database configuration file.

This step is only necessary if you want to backup your database; for a quick test deployment, you can skip this step. If skipping this step, you need to remove the `spec.backup` section of your `exampledb.yaml` file.

The `exampledb.yaml` file contains the name of the Kubernetes secret for your database:

```yaml
# Version: 20200113
apiVersion: planetscale.com/v2
kind: VitessCluster
metadata:
  name: example
spec:
  backup:
    locations:
      - gcs:
          bucket: mybucketname1
          authSecret:
            name: gcs-secret
            key: gcs_key.json
```

Edit the values of 'spec.backup.locations.gcs.bucket', 'spec.backup.locations.gcs.authSecret.name', and 'spec.backup.locations.gcs.authSecret.key' to reflect the values for your storage bucket and the Kubernetes secret for your GCP service account with access to a GCS bucket.

## Step 4. Apply the database configuration file to your cluster.

Apply the example database configuration to your Kubernetes cluster using the following command:

```shell
$ kubectl apply -f exampledb.yaml
------
vitesscluster.planetscale.com/example created
secret/example-cluster-config created
```

After a few minutes, you should see the pods for your keyspace using the following command, you should see a similar output:

```shell
$ kubectl get pods
------
NAME                                                  READY  STATUS   RESTARTS  AGE
example-90089e05-vitessbackupstorage-subcontroller    1/1    Running  0         59s
example-etcd-faf13de3-1                               1/1    Running  0         59s
example-etcd-faf13de3-2                               1/1    Running  0         59s
example-etcd-faf13de3-3                               1/1    Running  0         59s
example-uscentral1a-vtctld-6a268099-56c48bbc89-6r9dp  1/1    Running  2         58s
example-uscentral1a-vtgate-bbffae2f-54d5fdd79-gmwlm   0/1    Running  2         54s
example-uscentral1a-vtgate-bbffae2f-54d5fdd79-jldzg   0/1    Running  2         54s
example-vttablet-uscentral1a-0261268656-d6078140      2/3    Running  2         58s
example-vttablet-uscentral1a-1579720563-f892b0e6      2/3    Running  2         59s
example-vttablet-uscentral1a-2253629440-17557ac0      2/3    Running  2         58s
example-vttablet-uscentral1a-3067826231-d454720e      2/3    Running  2         59s
example-vttablet-uscentral1a-3815197730-f3886a80      2/3    Running  2         58s
example-vttablet-uscentral1a-3876690474-0ed30664      2/3    Running  2         59s
vitess-operator-6f54958746-mr9hp                      1/1    Running  0         17m
```

## Step 5. Port-forward the vtctld service to your Kubernetes cluster.

Use the following command:

```shell
kubectl port-forward --address localhost deployment/$(kubectl get deployment --selector="planetscale.com/component=vtctld" -o=jsonpath="{.items..metadata.name}") 15999:15999
```

You should now be able to see all of your tablets using the following command, you should see a similar output:

```shell
$ vtctldclient --server localhost:15999 GetTablets
------
uscentral1a-0261268656 commerce -80 replica 10.16.1.16:15000 10.16.1.16:3306 []
uscentral1a-1579720563 commerce 80- replica 10.16.1.15:15000 10.16.1.15:3306 []
uscentral1a-2253629440 commerce -80 replica 10.16.0.18:15000 10.16.0.18:3306 []
uscentral1a-3067826231 commerce 80- replica 10.16.0.17:15000 10.16.0.17:3306 []
uscentral1a-3815197730 commerce 80- primary 10.16.2.20:15000 10.16.2.20:3306 []
uscentral1a-3876690474 commerce -80 primary 10.16.2.21:15000 10.16.2.21:3306 []
```

## Step 6. Apply the VSchema to your Vitess database.

Apply the example [VSchema](https://vitess.io/docs/reference/vschema/) using the following command:

```console
$ vtctldclient --server localhost:15999 ApplyVSchema --vschema "$(cat ./vschema.json)" commerce
```

## Step 7. Apply the SQL schema to your Vitess database.

Apply the example SQL schema using the following command:

```shell
$ vtctldclient --server localhost:15999 ApplySchema --sql "$(cat ./schema.sql)" commerce
```

## Step 8. Expose the Vitess service.

Expose the service using the following command:

```shell
$ kubectl expose deployment $( kubectl get deployment --selector="planetscale.com/component=vtgate" -o=jsonpath="{.items..metadata.name}" ) --type=LoadBalancer --name=test-vtgate --port 3306 --target-port 3306
```

Use the following command to find the external IP for your LoadBalancer service:

```shell
$ kubectl get service test-vtgate
------
NAME         TYPE          CLUSTER-IP      EXTERNAL-IP      PORT(S)         AGE
test-vtgate  LoadBalancer  [cluster_ip]    [external_ip]    3306:32157/TCP  90s
```

It may take a few minutes for the load balancer to become available.

## Step 9. Connect to your Vitess database using a MySQL client.

Use the IP from the previous step to connect to your Vitess database using a command like the following:

```shell
$ mysql -u user -h [external_ip] -p
```

After entering your password (the default is `password` from the `exampledb.yaml` file), you can now submit queries against your Vitess database from your MySQL client.

For example, the following query displays the tables in your database with VSchemas:

```sql
> SHOW VSCHEMA TABLES;
```

The above query should return the following output:

```sql
+----------------+
| Tables         |
+----------------+
| dual           |
| users          |
| users_name_idx |
+----------------+
3 rows in set (0.06 sec)
```
