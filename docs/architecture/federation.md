# Vitess Operator Federation

This document describes how Vitess Operator was designed to support federation
across multiple Kubernetes clusters. Federation allows separate Vitess Operator
instances, in separate Kubernetes clusters, to coordinate to deploy and manage a
single Vitess Cluster that spans multiple Kubernetes clusters.

Note that this support consists of low-level capabilities that must be combined
with additional Kubernetes plug-ins (like some form of cross-cluster LB) and
other capabilities (like federated etcd) to assemble a federated system.
Our commercial product, [PlanetScaleDB Operator][], is an example of such a
holistic system that uses these low-level features to perform federation
automatically, but these features could also be used to build a federated system
manually with Vitess Operator alone.

[PlanetScaleDB Operator]: https://docs.planetscale.com/psdb-operator/overview

## Overview

The basic principle of Vitess Operator federation is to write a set of
VitessCluster object specifications that, when deployed in separate Kubernetes
clusters, each bring up and manage the pieces of the Vitess cluster that live
in that Kubernetes cluster. These pieces should then have some way to discover
each other and connect up to form a single Vitess cluster.

Ordinarily, deploying several VitessCluster CRDs in several different Kubernetes
clusters would result in completely independent Vitess clusters that don't know
about each other. The key to federation is ensuring that all these Vitess
components are pointed at a shared, global Vitess lockserver, which typically
takes the form of an etcd cluster.

Once Vitess components are pointed at a shared, global topology service, they
will use that to find each other's addresses to perform query routing and set up
MySQL replication.

## Requirements

These are the requirements that the underlying infrastructure must meet in order
for Vitess to work across Kubernetes clusters:

- Pod IPs must be directly routable across Kubernetes clusters. That is, any Pod
  can talk to any other Pod in any cluster on its in-cluster IP. Within a cloud
  provider, this is often as simple as putting multiple Kubernetes clusters in
  the same VPC, or peering multiple VPCs. Otherwise, some form of VPN could be
  used.
- You need some form of cross-cluster, internal load-balancing -- that is, a
  private IP (ideally with an associated DNS hostname) that routes connections
  to Ready Pods that may be in different Kubernetes clusters. At PlanetScale,
  for example, we've had good experience with using [Cilium Global Services][]
  for this purpose.
- To truly be able to survive the loss of any given Kubernetes cluster, your
  Vitess global topology service (e.g. etcd) needs to form a quorum across
  multiple Kubernetes clusters, or be otherwise independent somehow, such as by
  being hosted outside Kubernetes. For example, [PlanetScaleDB Operator][] sets
  up a federated etcd cluster consisting of etcd member Pods spread across
  multiple Kubernetes clusters.

[Cilium Global Services]: https://docs.cilium.io/en/v1.7/gettingstarted/clustermesh/#load-balancing-with-global-services

## CRD API

Vitess Operator federation consists of creating a set of [VitessCluster CRD][]
objects that are each customized for a particular Kubernetes cluster that forms
part of the federated system. This section describes how the VitessCluster CRD
API supports federation.

[VitessCluster CRD]: https://docs.planetscale.com/vitess-operator/api

### Lockservers

The most important step to enable federation is to make sure all VitessCluster
CRD objects are configured to talk to the same shared, global lockserver.
Typically this is done by configuring each one to connect to an arbitrary
etcd cluster address, via the `spec.globalLockserver.external` field.

By default, Vitess Operator will also configure cell-local topology data to be
stored in the global lockserver, but this is not recommended in a federated
system because the global lockserver is slow as a result of forming quorum
across large geographical distances.

Instead, the best practice is to deploy a separate etcd cluster for each
Vitess Cell, or at least for each Kubernetes cluster, to store cell-local data.
You can use `spec.cells[].lockserver` to configure these cell-local lockservers
to be either external or deployed by Vitess Operator itself.

Vitess Operator will automatically register the addresses of these cell-local
lockservers into the global lockserver, so Vitess components in any cell can
find and connect to them. Note that this means the address used for a cell-local
lockserver must be resolvable (if it's a hostname) and reachable from any Pod in
any Kubernetes cluster. This is an example of where the requirement for
cross-cluster, internal load-balancing comes in.

### Cell Filtering

The VitessCluster CRD asks you to define a list of Vitess Cells, which are
groups of Vitess components that should be able to operate independently.
In public cloud, for example, it's common to map each Vitess Cell to a
particular Zone or Availability Zone (AZ) within a multi-AZ Kubernetes cluster.

When creating a federated system with Vitess Operator, each VitessCluster CRD
should only define the cells that are to be deployed in that Kubernetes cluster.
For example, if you are federating two Kubernetes clusters in us-east-1 and
us-west-2, respectively, you might deploy a VitessCluster CRD containing the
cells `(useast1a, useast1b, useast1c)` into the first Kubernetes cluster, and a
different VitessCluster CRD containing the cells `(uswest2a, uswest2b, uswest2c)`
into the second.

This allows each Vitess Operator instance to know which cells it should actually
deploy in its local Kubernetes cluster. If the operator sees other cells in the
Vitess cluster when it queries global Vitess topology, it will know that those
cells are handled by some other operator instance.

Similarly, each VitessCluster CRD should contain only the tablet pools that
refer to cells defined in that VitessCluster.

### Topology Reconciliation

Vitess Operator automatically populates and prunes Vitess topology data in the
global and cell-local lockservers in response to edits to the VitessCluster CRD.
However, in a federated system, each independent Vitess Operator instance might
lack the full, global context needed to know with certainty whether a record
should be pruned.

For this reason, it is recommended that certain topology reconciliation features
be disabled when running in a federated system. This can be configured with the
`spec.topologyReconciliation` field in the VitessCluster CRD.

In particular, the following settings should be disabled:

- `registerCellsAliases`
- `pruneCells`
- `pruneKeyspaces`
- `pruneSrvKeyspaces` — if you want to query any keyspace from any cell (most importantly this must be disabled in the VitessCell CRD)
- `pruneShards`

With these features off, you may need to manually clean up Vitess topology when
turning down a cell, keyspace, or shard. Alternatively, it's possible to build
tooling to do automatic clean-up across a federated Vitess cluster, but that
requires global context that Vitess Operator doesn't possess so it's best done
at a higher layer, such as in [PlanetScaleDB Operator][] which builds on top of
Vitess Operator.

### Replication Management

Vitess is designed to make each cell as independent as possible, but MySQL
replication necessarily has to go across cells, which means it has to go across
Kubernetes clusters in a federated system.

Since replication is inherently global, it's important to ensure Vitess Operator
instances that only see part of the state don't disagree with each other about
who is responsible for each step in configuring replication. This can be done
by setting `initializeMaster` and `initializeBackup` (subfields under the
`replication` field within each shard template) to `true` in one and only one
of the VitessCluster CRD objects.

### Drain Controller

The drain controller in Vitess Operator is responsible for performing planned
reparents when the `drain.planetscale.com/started` annotation appears on a
tablet Pod that happens to be the master of its shard.

In a federated Vitess cluster, the drain controller only looks within its local
Kubernetes cluster to find other replicas in the shard that could be promoted to
master to complete the requested drain. This is part of minimizing cross-cluster
dependencies in order to maximize isolation and fault-tolerance.

As a result, the drain controller will only be able to complete a reparent if
there are other healthy, master-eligible tablets for that shard in the same
Kubernetes cluster. Note that this does not prevent you from doing a planned
reparent across clusters yourself; it's just that the drain controller won't do
it automatically in response to a drain request.
