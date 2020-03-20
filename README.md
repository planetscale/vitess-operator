# ALPHA STAGE WARNING

This project was only recently open-sourced and we are still working on making sure it's usable outside our own
infrastructure.

If you want to help develop features or find/fix bugs, you're welcome to give it a try and send
issues or PRs.

# Vitess Operator

## Docs

- [VitessCluster CRD API Reference](https://vitess-operator.planetscale.dev/api/)

## Build

If you only want to deploy vitess-operator from pre-built images, you can skip to the
[Deploy](#deploy) section.

### Prerequisites

Install [Operator SDK](https://github.com/operator-framework/operator-sdk/blob/master/doc/user/install-operator-sdk.md)
version 0.10.0 and rename it to `operator-sdk-v0.10.0` in your path.
We include the version in the file name to ensure that builds will fail if the
installed version is incorrect.

### Build Docker image

From this directory, run:

```
make build IMAGE_NAME=your.registry/vitess/operator
```

## Contributing

If you would like to contribute to this project, please refer to the
[contributing readme](CONTRIBUTING.md)

## Deploy

### Prerequisites

You need to install [kustomize](https://github.com/kubernetes-sigs/kustomize)
on your local machine.

You also need to set up a place for Vitess to store backups, such as an S3 or
GCS bucket.
This backup storage location is used not only for disaster recovery, but also
for cloning MySQL data from one replica to another.
As a result, a backup storage location is required to initialize a new Vitess
keyspace or shard, or to add a new tablet to an existing shard.

### Deploy Operator

Once you have all the prerequisites installed, you should be able to deploy
the operator like this:

```sh
kustomize build deploy | kubectl apply -f -
```

You can inspect the output of `kustomize build deploy` first if you want to see
what will be applied to the cluster.

## Create an example VitessCluster

The file `deploy/example.yaml` contains an example cluster with one keyspace and
two shards.

See the [VitessCluster CRD API Reference](https://vitess-operator.planetscale.dev/api/)
for details on all of the fields.

### Cluster Config Secret

In addition to the VitessCluster spec, the `example.yaml` file also
contains necessary per-cluster config, expressed as a Kubernetes Secret object:

* `users.json` is the authentication config for vtgate's MySQL protocol mode.

  This is where you must set up usernames and passwords to be used by apps
  that connect to Vitess through a MySQL-compatible client.

  Each entry in this JSON object should map from a MySQL username to an object
  that specifies the MySQL password for that user, as well as the name by which
  this MySQL user should be known internally within Vitess (`UserData`), for
  the purposes of the Vitess Caller ID and Table ACL features.

  Note that if you change the contents of this Secret after deploying,
  the operator will automatically begin a rolling restart of vtgates to make
  them reload the authentication configuration.

* `init_db.sql` is a SQL script file that is used to initialize an empty mysqld.

  This sets up the authentication and authorization for the actual underlying MySQL.
  Apps usually don't access the underlying MySQL directly, so most remote connections
  are disallowed.

### Configure backup storage

TODO: Document how to configure the backup storage location.

### Deploy example cluster

After you've customized the example cluster YAML file to configure your backup
storage location, you can deploy it like this:

```sh
kubectl apply -f deploy/example.yaml
```

## CRDs

The main CRD for vitess-operator is called VitessCluster.
You can abbreviate this to `vt` when using kubectl:

```sh
kubectl get vt
```

The operator also manages sub-component CRDs that you won't create or edit
directly, but you can read them to see more detailed status:

* VitessKeyspace (short name `vtk`)
* VitessShard (short name `vts`)
* VitessCell (short name `vtc`)

## Check status

The main VitessCluster object has a status section that summarizes the
overall state of the cluster.

```sh
kubectl describe vt example
```

You can also see more detailed status for each keyspace by checking the status
of the corresponding VitessKeyspace object.
Here's how you could describe the VitessKeyspaces belonging to the VitessCluster
`example`:

```sh
kubectl describe vtk -l planetscale.com/cluster=example
```

You can drill down even further by looking at individual VitessShard objects:

```sh
kubectl describe vts -l planetscale.com/cluster=example
```

From here, you can see the status of individual tablets within the shard,
listed by the Vitess Tablet Alias (`<cell>-<uid>`).

### Events

When the operator takes action or encounters a problem on a VitessCluster,
or any of the sub-component CRDs (e.g. VitessShard), it logs an entry in the
Kubernetes "events" stream for that object. This makes it easy for devs and SREs
maintaining the operator and its clusters to immediately find out what's going
on with one particular object. You should see some events in the output of some
of the `kubectl describe` commands above.

## Connect to vtgate

By default, vitess-operator does not configure the vtgate service to be
accessible from outside the Kubernetes cluster, since in most cases it's best to
run clients (i.e. your app) in the same Kubernetes cluster whenever possible.

You can use port-forwarding to test out vtgate from your workstation outside the
Kubernetes cluster, once all your tablets are ready as shown in the VT status.

```sh
kubectl port-forward "svc/$(kubectl get vt example -o jsonpath='{.status.gatewayServiceName}')" 30000:3306
```

Then while that's running, run the `mysql` CLI in another terminal, with the
username and pasword that you configured in the
[Cluster Config Secret](#cluster-config-secret):

```sh
mysql --host=127.0.0.1 --port=30000 --user=<username> --password
```

## Connect to vtctl

Vtctl is the standard Vitess CLI tool for cluster administration. It allows
you to perform CRUD operations on Vitess objects, run migration workflows,
and much more. The reference for this tool can be located [here](https://vitess.io/docs/reference/vtctl/).
To run vtctl commands, you must first have `vtctlclient` installed.
If you don't have it installed, you can install it by running:

```sh
go get vitess.io/vitess/go/cmd/vtctlclient
```

Then you can use port-forwarding to access vtctld from outside the Kubernetes cluster:

```sh
kubectl port-forward "svc/$(kubectl get vt example -o jsonpath='{.status.vitessDashboard.serviceName}')" 15555:15999
```

Once that port is forwarded, you can connect to it with `vtctlclient`:

```sh
vtctlclient -server localhost:15555 help
```

## Cleaning up

To delete the example VitessCluster and cluster config Secret:

```sh
kubectl delete -f deploy/example.yaml
```

To uninstall vitess-operator:

```sh
kustomize build deploy | kubectl delete -f -
```
