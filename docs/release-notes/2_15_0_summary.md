## Major Changes

### Table of Contents

- **[Kubernetes Version](#k8s-version)**
- **[Golang Version](#go-version)**
- **[Scheduled Backups now use VTBackup](#vtbackup-scheduled-backups)**
- **[Multiple Namespaces in the Default Examples](#multiple-namespaces)**
- **[Upgrade Path](#upgrade-path)**
  - **[etcd](#etcd-upgrade-path)**
  - **[MySQL](#mysql-upgrade-path)**

### <a id="k8s-version"/>Kubernetes Version</a>

The default supported version of Kubernetes was bumped from `v1.31` to `v1.32`. ([#670](https://github.com/planetscale/vitess-operator/pull/670))

### <a id="go-version"/>Golang Version</a>

The default Golang version was bumped from `v1.23` to `v1.24`. ([#664](https://github.com/planetscale/vitess-operator/pull/664))


### <a id="vtbackup-scheduled-backups"/>Scheduled Backups now use VTBackup</a>

Scheduled backups were previously introduced in `v2.13.0` by [#553](https://github.com/planetscale/vitess-operator/pull/553),
the original design used `vtctldclient` commands to take new backups of the cluster.

This design has changed in `v2.15.0`, the `VitessScheduledBackups` controller now uses `vtbackup` pods to take
new backups. While it uses more resources, the new design is more fault-tolerant and will not take a serving tablet out of the tablet pool.

This change was introduced in [#658](https://github.com/planetscale/vitess-operator/pull/658).

### <a id="multiple-namespaces"/>Multiple Namespaces in the Default Examples</a>

Since [#666](https://github.com/planetscale/vitess-operator/pull/666), end-users can learn how to setup a cluster on multiple namespaces
(e.g. `vitess-operator` in the `default` namespace, and `VitessCluster` in another namespace)
by following the [default K8S user-guide](https://vitess.io/docs/22.0/get-started/operator/).

The default `operator.yaml` provided as an example has been updated to contain all the configuration required to run the examples on multiple namespaces.

All end-to-end CI tests also run with two namespaces.

### <a id="rolling-update-vtgate"/>Rolling Update Settings for VTGate</a>

It is now possible to define the [rolling update settings](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#strategy) of the vtgate deployment.
This enhancement was made via [#676](https://github.com/planetscale/vitess-operator/pull/676).

### <a id="upgrade-path"/>Upgrade Path</a>

#### <a id="etcd-upgrade-path"/>etcd</a>

We have changed the default etcd version to `3.5.17`.

You can upgrade by changing your YAML file to use the new Docker Image (`quay.io/coreos/etcd:v3.5.17`).

#### <a id="mysql-upgrade-path"/>MySQL</a>

With the latest version of Vitess (`v22.0.0`) the default MySQL version changed from `8.0.30` to `8.0.40`.

In order for you to correctly upgrade, there is a certain path to follow:

1. Add `innodb_fast_shutdown=0` to your extra cnf in your YAML file.
2. Apply this file.
3. Wait for all the pods to be healthy.
4. Then change your YAML file to use the new Docker Image (`mysql:8.0.40`).
5. Remove `innodb_fast_shutdown=0` from your extra cnf in your YAML file.
6. Apply this file.
