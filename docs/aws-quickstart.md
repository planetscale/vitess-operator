## Prerequisites

This guide assumes you have the following components and services:

- A [Elastic Kubernetes Service](https://aws.amazon.com/eks/) (EKS) cluster;
- A local `kubectl` client [configured to access the EKS cluster](https://aws.amazon.com/premiumsupport/knowledge-center/eks-cluster-connection/) where you wish to install the operator;
- An [S3 storage bucket](https://docs.aws.amazon.com/AmazonS3/latest/user-guide/create-bucket.html);
- An [AWS IAM role and policy](https://docs.aws.amazon.com/eks/latest/userguide/enable-iam-roles-for-service-accounts.html) with access to the S3 storage bucket;
- A Kubernetes secret matching your service account; or alternatively a Kubernetes service account mapped to your S3 IAM role above;
- A local [installation of vtctlclient](https://vitess.io/docs/get-started/kubernetes/#prerequisites).

## Overview

To deploy a Vitess cluster on EKS using the Vitess Operator, follow these steps:

1. Download the operator installation and example database configuration files.
1. Apply the operator installation file against your Kubernetes cluster.
1. Edit the name your S3 bucket and region in the database configuration file.
1. Apply the database configuration file to your cluster.
1. Port-forward the `vtctld` service to your Kubernetes cluster.
1. Apply the VSchema to your Vitess database.
1. Apply the SQL schema to your Vitess database.
1. Expose the Vitess service.
1. Connect to your Vitess database using a MySQL client.

## Step 1. Download the operator and example database configuration files.

Download the following files:

- [Operator installation file](https://storage.googleapis.com/vitess-operator/install/operator.yaml)
- [Database configuration file](https://storage.googleapis.com/vitess-operator/examples/exampledb_aws.yaml)
- [Example VSchema](https://storage.googleapis.com/vitess-operator/examples/vschema.json)
- [Example SQL schema](https://storage.googleapis.com/vitess-operator/examples/schema.sql)

This guide will assume that the above files are in your working directory.

## Step 2. Apply the operator installation file against your Kubernetes cluster.

This step assumes that `kubectl` is configured to access the GKE cluster.

Enter the following command:

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
NAME                               READY   STATUS    RESTARTS   AGE
vitess-operator-5f64b6fb65-pnvjl   1/1     Running   0          35s
```

## Step 3. Edit the name of the Kubernetes secret in the database configuration file.

This step is only necessary if you want to backup your database; for a quick test deployment, you can skip this step. If skipping this step, you need to remove the 'spec.backup' section of your `exampledb_aws.yaml` file.

The `exampledb_aws.yaml` file contains the name of the Kubernetes secret for your database:

```yaml
# Version: 20200113
apiVersion: planetscale.com/v2
kind: VitessCluster
metadata:
  name: example
spec:
  backup:
    locations:
      - s3:
          bucket: mybucketname1
          region: us-west-2
```

Edit the values of 'spec.backup.locations.s3.bucket' and 'spec.backup.locations.s3.region' to reflect the name and region for your storage bucket. This assumes that your EKS cluster default Kubernetes service account has permissions to access (list, read & write) the bucket. If not, you may need to add an 'spec.backup.locations.s3.authSecret' section, and make sure you have a matching Kubernetes secret created for your AWS service account with access to that bucket.

## Step 4. Apply the database configuration file to your cluster.

Apply the example database configuration to your Kubernetes cluster using the following command:

```shell
$ kubectl apply -f exampledb_aws.yaml
------
vitesscluster.planetscale.com/example created
secret/example-cluster-config created
```

After a few minutes, you should see the pods for your keyspace using the following command:

```shell
$ kubectl get pods
------
NAME                                                 READY   STATUS      RESTARTS   AGE
example-90089e05-vitessbackupstorage-subcontroller   1/1     Running     0          44s
example-etcd-faf13de3-1                              1/1     Running     0          44s
example-etcd-faf13de3-2                              1/1     Running     0          44s
example-etcd-faf13de3-3                              1/1     Running     0          43s
example-main-80-x-vtbackup-init-f09214a2             0/1     Completed   0          42s
example-main-x-80-vtbackup-init-6f097fa4             0/1     Completed   0          42s
example-uswest2a-vtctld-e9a472d4-68f7844c65-6n48p    1/1     Running     2          44s
example-uswest2a-vtgate-dc57c24e-66b7fd464-v67n8     0/1     Running     2          43s
example-uswest2a-vtgate-dc57c24e-66b7fd464-x776t     0/1     Running     2          43s
example-vttablet-uswest2a-0433486107-2adb076e        2/3     Running     1          44s
example-vttablet-uswest2a-1016938354-18161672        2/3     Running     1          43s
example-vttablet-uswest2a-1990736494-c589ebc7        2/3     Running     1          44s
example-vttablet-uswest2a-3169804963-d4c380c8        2/3     Running     1          44s
vitess-operator-5f64b6fb65-pnvjl                     1/1     Running     0          23m
```

## Step 5. Port-forward the vtctld service to your Kubernetes cluster.

Use the following command:

```shell
kubectl port-forward --address localhost deployment/$(kubectl get deployment --selector="planetscale.com/component=vtctld" -o=jsonpath="{.items..metadata.name}") 15999:15999
```

You should now be able to see all of your tablets using the following command:

```shell
$ vtctlclient -server localhost:15999 ListAllTablets
------
uswest2a-0433486107 main -80 master 192.168.5.111:15000 192.168.5.111:3306 []
uswest2a-1016938354 main -80 replica 192.168.0.234:15000 192.168.0.234:3306 []
uswest2a-1990736494 main 80- replica 192.168.17.236:15000 192.168.17.236:3306 []
uswest2a-3169804963 main 80- master 192.168.3.15:15000 192.168.3.15:3306 []
```

## Step 6. Apply the VSchema to your Vitess database.

Apply the example [VSchema](https://vitess.io/docs/reference/vschema/) using the following command:

```shell
$ vtctlclient -server localhost:15999 ApplyVSchema -vschema "$(cat ./vschema.json)" main
```

## Step 7. Apply the SQL schema to your Vitess database.

Apply the example SQL schema using the following command:

```shell
$ vtctlclient -server localhost:15999 ApplySchema -sql "$(cat ./schema.sql)" main
```

## Step 8. Expose the Vitess service.

Expose the service using the following command:

```shell
$ kubectl expose deployment $( kubectl get deployment --selector="planetscale.com/component=vtgate" -o=jsonpath="{.items..metadata.name}" ) --type=LoadBalancer --name=test-vtgate --port 3306 --target-port 3306
```

Use the following command to find the external ELB name for your LoadBalancer service:

```shell
$ kubectl get service test-vtgate
------
NAME          TYPE           CLUSTER-IP      EXTERNAL-IP     PORT(S)          AGE
test-vtgate   LoadBalancer   [cluster_ip]    [elb_dns_name]  3306:30481/TCP   59s
```

It may take a few minutes for the load balancer to become available.

## Step 9. Connect to your Vitess database using a MySQL client.

Use the IP from the previous step to connect to your Vitess database using a command like the following:

```shell
$ mysql -u user -h [elb_dns_name] -p
```

After entering your password (the default is `password` from the `exampledb_aws.yaml` file), you can now submit queries against your Vitess database from your MySQL client.

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
