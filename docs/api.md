<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no">
<link rel="stylesheet" href="https://stackpath.bootstrapcdn.com/bootstrap/4.3.1/css/bootstrap.min.css" integrity="sha384-ggOyR0iXCbMQv3Xipma34MD+dH/1fQ784/j6cY/iJTQUOhcWr7x9JvoRxT2MZw1T" crossorigin="anonymous">
<title>Vitess Operator API Reference</title>
</head>
<body>
<nav class="navbar navbar-expand-lg navbar-dark bg-dark">
<a class="navbar-brand" href="#">Vitess Operator API Reference</a>
<ul class="navbar-nav">
<li class="nav-item">
<a class="nav-link" href="#planetscale.com/v2.VitessCluster">VitessCluster</a>
</li>
</ul>
</nav>
<div class="container">
<h1 id="planetscale.com/v2">planetscale.com/v2</h1>
<p>
<p>Package v2 contains API Schema definitions for the planetscale.com/v2 API group.</p>
</p>
<h2>Resource Types</h2>
<ul><li>
<a href="#planetscale.com/v2.EtcdLockserver">EtcdLockserver</a>
</li><li>
<a href="#planetscale.com/v2.VitessBackup">VitessBackup</a>
</li><li>
<a href="#planetscale.com/v2.VitessCluster">VitessCluster</a>
</li></ul>
<h3 id="planetscale.com/v2.EtcdLockserver">EtcdLockserver
</h3>
<p>
<p>EtcdLockserver runs an etcd cluster for use as a Vitess lockserver.
Unlike etcd-operator, it uses static bootstrapping and PVCs, treating members
as stateful rather the ephemeral. Bringing back existing members instead of
creating new ones means etcd can recover from loss of quorum without data
loss, which is important for Vitess because restoring from an etcd backup
(resetting the lockserver to a point in the past) would violate the
consistency model that Vitess expects of a lockserver.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code></br>
string</td>
<td>
<code>
planetscale.com/v2
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code></br>
string
</td>
<td><code>EtcdLockserver</code></td>
</tr>
<tr>
<td>
<code>metadata</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code></br>
<em>
<a href="#planetscale.com/v2.EtcdLockserverSpec">
EtcdLockserverSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>EtcdLockserverTemplate</code></br>
<em>
<a href="#planetscale.com/v2.EtcdLockserverTemplate">
EtcdLockserverTemplate
</a>
</em>
</td>
<td>
<p>
(Members of <code>EtcdLockserverTemplate</code> are embedded into this type.)
</p>
<p>EtcdLockserverTemplate contains the user-specified parts of EtcdLockserverSpec.
These are the parts that are configurable inside VitessCluster.
The rest of the fields below are filled in by the parent controller.</p>
</td>
</tr>
<tr>
<td>
<code>zone</code></br>
<em>
string
</em>
</td>
<td>
<p>Zone is the name of the Availability Zone that this lockserver should run in.
This value should match the value of the &ldquo;failure-domain.beta.kubernetes.io/zone&rdquo;
label on the Kubernetes Nodes in that AZ.
If the Kubernetes Nodes don&rsquo;t have such a label, leave this empty.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#planetscale.com/v2.EtcdLockserverStatus">
EtcdLockserverStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessBackup">VitessBackup
</h3>
<p>
<p>VitessBackup is a one-way mirror of metadata for a Vitess backup.
These objects are created automatically by the VitessBackupStorage controller
to provide access to backup metadata from Kubernetes. Each backup found in
the storage location will be represented by its own VitessBackup object.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code></br>
string</td>
<td>
<code>
planetscale.com/v2
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code></br>
string
</td>
<td><code>VitessBackup</code></td>
</tr>
<tr>
<td>
<code>metadata</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code></br>
<em>
<a href="#planetscale.com/v2.VitessBackupSpec">
VitessBackupSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#planetscale.com/v2.VitessBackupStatus">
VitessBackupStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessCluster">VitessCluster
</h3>
<p>
<p>VitessCluster is the top-level interface for configuring a cluster.</p>
<p>Although the VitessCluster controller creates various secondary objects
like VitessCells, all the user-accessible configuration ultimately lives here.
The other objects should be considered read-only representations of subsets of
the dynamic cluster status. For example, you can examine a specific VitessCell
object to get more details on the status of that cell than are summarized in the
VitessCluster status, but any configuration changes should only be made in
the VitessCluster object.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>apiVersion</code></br>
string</td>
<td>
<code>
planetscale.com/v2
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code></br>
string
</td>
<td><code>VitessCluster</code></td>
</tr>
<tr>
<td>
<code>metadata</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code></br>
<em>
<a href="#planetscale.com/v2.VitessClusterSpec">
VitessClusterSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>images</code></br>
<em>
<a href="#planetscale.com/v2.VitessImages">
VitessImages
</a>
</em>
</td>
<td>
<p>Images specifies the container images (including version tag) to use
in the cluster.
Default: Let the operator choose.</p>
</td>
</tr>
<tr>
<td>
<code>imagePullPolicies</code></br>
<em>
<a href="#planetscale.com/v2.VitessImagePullPolicies">
VitessImagePullPolicies
</a>
</em>
</td>
<td>
<p>ImagePullPolicies specifies the container image pull policies to use for
images defined in the &lsquo;images&rsquo; field.</p>
</td>
</tr>
<tr>
<td>
<code>imagePullSecrets</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#localobjectreference-v1-core">
[]Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
<p>ImagePullSecrets specifies the image pull secrets to add to all Pods that
use the images defined in the &lsquo;images&rsquo; field.</p>
</td>
</tr>
<tr>
<td>
<code>backup</code></br>
<em>
<a href="#planetscale.com/v2.ClusterBackupSpec">
ClusterBackupSpec
</a>
</em>
</td>
<td>
<p>Backup specifies how to take and store Vitess backups.
This is optional but strongly recommended. In addition to disaster
recovery, Vitess currently depends on backups to support provisioning
of a new tablet in a shard with existing data, as an implementation detail.</p>
</td>
</tr>
<tr>
<td>
<code>globalLockserver</code></br>
<em>
<a href="#planetscale.com/v2.LockserverSpec">
LockserverSpec
</a>
</em>
</td>
<td>
<p>GlobalLockserver specifies either a deployed or external lockserver
to be used as the Vitess global topology store.
Default: Deploy an etcd cluster as the global lockserver.</p>
</td>
</tr>
<tr>
<td>
<code>vitessDashboard</code></br>
<em>
<a href="#planetscale.com/v2.VitessDashboardSpec">
VitessDashboardSpec
</a>
</em>
</td>
<td>
<p>Dashboard deploys a set of Vitess Dashboard servers (vtctld) for the Vitess cluster.</p>
</td>
</tr>
<tr>
<td>
<code>vtadmin</code></br>
<em>
<a href="#planetscale.com/v2.VtAdminSpec">
VtAdminSpec
</a>
</em>
</td>
<td>
<p>VtAdmin deploys a set of Vitess Admin servers for the Vitess cluster.</p>
</td>
</tr>
<tr>
<td>
<code>cells</code></br>
<em>
<a href="#planetscale.com/v2.VitessCellTemplate">
[]VitessCellTemplate
</a>
</em>
</td>
<td>
<p>Cells is a list of templates for VitessCells to create for this cluster.</p>
<p>Each VitessCell represents a set of Nodes in a given failure domain,
to which VitessKeyspaces can be deployed. The VitessCell also deploys
cell-local services that any keyspaces deployed there will need.</p>
<p>This field is required, but it may be set to an empty list: [].
Before removing any cell from this list, you should first ensure
that no keyspaces are set to deploy to this cell.</p>
</td>
</tr>
<tr>
<td>
<code>keyspaces</code></br>
<em>
<a href="#planetscale.com/v2.VitessKeyspaceTemplate">
[]VitessKeyspaceTemplate
</a>
</em>
</td>
<td>
<p>Keyspaces defines the logical databases to deploy.</p>
<p>A VitessKeyspace can deploy to multiple VitessCells.</p>
<p>This field is required, but it may be set to an empty list: [].
Before removing any keyspace from this list, you should first ensure
that it is undeployed from all cells by clearing the keyspace&rsquo;s list
of target cells.</p>
</td>
</tr>
<tr>
<td>
<code>extraVitessFlags</code></br>
<em>
map[string]string
</em>
</td>
<td>
<p>ExtraVitessFlags can optionally be used to pass flags to all Vitess components.
WARNING: Any flags passed here must be flags that can be accepted by vtgate, vtctld, vtorc, and vttablet.
An example use-case would be topo flags.</p>
<p>All entries must be key-value string pairs of the form &ldquo;flag&rdquo;: &ldquo;value&rdquo;. The flag name should
not have any prefix (just &ldquo;flag&rdquo;, not &ldquo;-flag&rdquo;). To set a boolean flag,
set the string value to either &ldquo;true&rdquo; or &ldquo;false&rdquo;.</p>
</td>
</tr>
<tr>
<td>
<code>topologyReconciliation</code></br>
<em>
<a href="#planetscale.com/v2.TopoReconcileConfig">
TopoReconcileConfig
</a>
</em>
</td>
<td>
<p>TopologyReconciliation can be used to enable or disable registration or pruning of various vitess components to and from topo records.</p>
</td>
</tr>
<tr>
<td>
<code>updateStrategy</code></br>
<em>
<a href="#planetscale.com/v2.VitessClusterUpdateStrategy">
VitessClusterUpdateStrategy
</a>
</em>
</td>
<td>
<p>UpdateStrategy specifies how components in the Vitess cluster will be updated
when a revision is made to the VitessCluster spec.</p>
</td>
</tr>
<tr>
<td>
<code>gatewayService</code></br>
<em>
<a href="#planetscale.com/v2.ServiceOverrides">
ServiceOverrides
</a>
</em>
</td>
<td>
<p>GatewayService can optionally be used to customize the global vtgate Service.
Note that per-cell vtgate Services can be customized within each cell
definition.</p>
</td>
</tr>
<tr>
<td>
<code>tabletService</code></br>
<em>
<a href="#planetscale.com/v2.ServiceOverrides">
ServiceOverrides
</a>
</em>
</td>
<td>
<p>TabletService can optionally be used to customize the global, headless vttablet Service.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#planetscale.com/v2.VitessClusterStatus">
VitessClusterStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.AutoscalerSpec">AutoscalerSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessCellGatewaySpec">VitessCellGatewaySpec</a>)
</p>
<p>
<p>AutoscalerSpec defines the vtgate&rsquo;s pod autoscaling specification.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>minReplicas</code></br>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>MinReplicas is the minimum number of instances of vtgate to run in
this cell when autoscaling is enabled.</p>
</td>
</tr>
<tr>
<td>
<code>maxReplicas</code></br>
<em>
int32
</em>
</td>
<td>
<p>MaxReplicas is the maximum number of instances of vtgate to run in
this cell when autoscaling is enabled.</p>
</td>
</tr>
<tr>
<td>
<code>metrics</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#metricspec-v2-autoscaling">
[]Kubernetes autoscaling/v2.MetricSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Metrics is meant to provide a customizable way to configure HPA metrics.
currently the only supported custom metrics is type=Pod.
Use TargetCPUUtilization or TargetMemoryUtilization instead if scaling on these common resource metrics.</p>
</td>
</tr>
<tr>
<td>
<code>behavior</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#horizontalpodautoscalerbehavior-v2-autoscaling">
Kubernetes autoscaling/v2.HorizontalPodAutoscalerBehavior
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Behavior specifies the scaling behavior of the target in both Up and Down directions.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.AzblobBackupLocation">AzblobBackupLocation
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessBackupLocation">VitessBackupLocation</a>)
</p>
<p>
<p>AzblobBackupLocation specifies a backup location in Azure Blob Storage.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>account</code></br>
<em>
string
</em>
</td>
<td>
<p>Account is the name of the Azure storage account to use.</p>
</td>
</tr>
<tr>
<td>
<code>container</code></br>
<em>
string
</em>
</td>
<td>
<p>Container is the name of the Azure storage account container to use.</p>
</td>
</tr>
<tr>
<td>
<code>keyPrefix</code></br>
<em>
string
</em>
</td>
<td>
<p>KeyPrefix is an optional prefix added to all object keys created by Vitess.
This is only needed if the same container is also used for something other
than backups for VitessClusters. Backups from different clusters,
keyspaces, or shards will automatically avoid colliding with each other
within a container, regardless of this setting.</p>
</td>
</tr>
<tr>
<td>
<code>authSecret</code></br>
<em>
<a href="#planetscale.com/v2.SecretSource">
SecretSource
</a>
</em>
</td>
<td>
<p>AuthSecret is a reference to the Secret to use for Azure authentication.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.BackupStrategyName">BackupStrategyName
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessBackupScheduleStrategy">VitessBackupScheduleStrategy</a>)
</p>
<p>
<p>BackupStrategyName describes the vtctldclient command that will be used to take a backup.
When scheduling a backup, you must specify at least one strategy.</p>
</p>
<h3 id="planetscale.com/v2.CephBackupLocation">CephBackupLocation
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessBackupLocation">VitessBackupLocation</a>)
</p>
<p>
<p>CephBackupLocation specifies a backup location in Ceph S3.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>authSecret</code></br>
<em>
<a href="#planetscale.com/v2.SecretSource">
SecretSource
</a>
</em>
</td>
<td>
<p>AuthSecret is a reference to the Secret to use for Ceph S3 authentication.
If set, this must point to a file in the format expected for the
<code>https://github.com/vitessio/vitess/blob/master/examples/local/ceph_backup_config.json</code> file.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.ClusterBackupSpec">ClusterBackupSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessClusterSpec">VitessClusterSpec</a>)
</p>
<p>
<p>ClusterBackupSpec configures backups for a cluster.
In addition to disaster recovery, Vitess currently depends on backups to support
provisioning of a new tablet in a shard with existing data, as an implementation detail.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>locations</code></br>
<em>
<a href="#planetscale.com/v2.VitessBackupLocation">
[]VitessBackupLocation
</a>
</em>
</td>
<td>
<p>Locations is a list of places where Vitess backup data for the cluster
can be stored. At least one storage location must be specified.
Within each storage location, there are multiple fields for various
location types (gcs, s3, etc.); exactly one such field must be populated.</p>
<p>Multiple storage locations may be desired if, for example, the cluster
spans multiple regions. Each storage location is independent of the others;
backups can only be restored from the same storage location in which they
were originally taken.</p>
</td>
</tr>
<tr>
<td>
<code>engine</code></br>
<em>
<a href="#planetscale.com/v2.VitessBackupEngine">
VitessBackupEngine
</a>
</em>
</td>
<td>
<p>Engine specifies the Vitess backup engine to use, either &ldquo;builtin&rdquo; or &ldquo;xtrabackup&rdquo;.
Note that if you change this after a Vitess cluster is already deployed,
you must roll the change out to all tablets and then take a new backup
from one tablet in each shard. Otherwise, new tablets trying to restore
will find that the latest backup was created with the wrong engine.
Default: builtin</p>
</td>
</tr>
<tr>
<td>
<code>subcontroller</code></br>
<em>
<a href="#planetscale.com/v2.VitessBackupSubcontrollerSpec">
VitessBackupSubcontrollerSpec
</a>
</em>
</td>
<td>
<p>Subcontroller specifies any parameters needed for launching the VitessBackupStorage subcontroller pod.</p>
</td>
</tr>
<tr>
<td>
<code>schedules</code></br>
<em>
<a href="#planetscale.com/v2.VitessBackupScheduleTemplate">
[]VitessBackupScheduleTemplate
</a>
</em>
</td>
<td>
<p>Schedules defines how often we want to perform a backup and how to perform the backup.
This is a list of VitessBackupScheduleTemplate where the &ldquo;name&rdquo; field has to be unique
across all the items of the list.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.ConcurrencyPolicy">ConcurrencyPolicy
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessBackupScheduleTemplate">VitessBackupScheduleTemplate</a>)
</p>
<p>
<p>ConcurrencyPolicy describes how the concurrency of new jobs created by VitessBackupSchedule
is handled, the default is set to AllowConcurrent.</p>
</p>
<h3 id="planetscale.com/v2.EtcdLockserverSpec">EtcdLockserverSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.EtcdLockserver">EtcdLockserver</a>)
</p>
<p>
<p>EtcdLockserverSpec defines the desired state of an EtcdLockserver.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>EtcdLockserverTemplate</code></br>
<em>
<a href="#planetscale.com/v2.EtcdLockserverTemplate">
EtcdLockserverTemplate
</a>
</em>
</td>
<td>
<p>
(Members of <code>EtcdLockserverTemplate</code> are embedded into this type.)
</p>
<p>EtcdLockserverTemplate contains the user-specified parts of EtcdLockserverSpec.
These are the parts that are configurable inside VitessCluster.
The rest of the fields below are filled in by the parent controller.</p>
</td>
</tr>
<tr>
<td>
<code>zone</code></br>
<em>
string
</em>
</td>
<td>
<p>Zone is the name of the Availability Zone that this lockserver should run in.
This value should match the value of the &ldquo;failure-domain.beta.kubernetes.io/zone&rdquo;
label on the Kubernetes Nodes in that AZ.
If the Kubernetes Nodes don&rsquo;t have such a label, leave this empty.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.EtcdLockserverStatus">EtcdLockserverStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.EtcdLockserver">EtcdLockserver</a>, 
<a href="#planetscale.com/v2.LockserverStatus">LockserverStatus</a>)
</p>
<p>
<p>EtcdLockserverStatus defines the observed state of an EtcdLockserver.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>observedGeneration</code></br>
<em>
int64
</em>
</td>
<td>
<p>The generation observed by the controller.</p>
</td>
</tr>
<tr>
<td>
<code>available</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#conditionstatus-v1-core">
Kubernetes core/v1.ConditionStatus
</a>
</em>
</td>
<td>
<p>Available is a condition that indicates whether the cluster is able to serve queries.</p>
</td>
</tr>
<tr>
<td>
<code>clientServiceName</code></br>
<em>
string
</em>
</td>
<td>
<p>ClientServiceName is the name of the Service for etcd client connections.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.EtcdLockserverTemplate">EtcdLockserverTemplate
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.EtcdLockserverSpec">EtcdLockserverSpec</a>, 
<a href="#planetscale.com/v2.LockserverSpec">LockserverSpec</a>)
</p>
<p>
<p>EtcdLockserverTemplate defines the user-configurable settings for an etcd
cluster that we deploy (not external), to serve as either a local or global
lockserver.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>image</code></br>
<em>
string
</em>
</td>
<td>
<p>Image is the etcd server image (including version tag) to deploy.
Default: Let the operator choose.</p>
</td>
</tr>
<tr>
<td>
<code>imagePullPolicy</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#pullpolicy-v1-core">
Kubernetes core/v1.PullPolicy
</a>
</em>
</td>
<td>
<p>ImagePullPolicy specifies if/when to pull a container image.</p>
</td>
</tr>
<tr>
<td>
<code>imagePullSecrets</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#localobjectreference-v1-core">
[]Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
<p>ImagePullSecrets specifies the container image pull secrets to add to all
etcd Pods.</p>
</td>
</tr>
<tr>
<td>
<code>resources</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#resourcerequirements-v1-core">
Kubernetes core/v1.ResourceRequirements
</a>
</em>
</td>
<td>
<p>Resources specify the compute resources to allocate for each etcd member.
Default: Let the operator choose.</p>
</td>
</tr>
<tr>
<td>
<code>dataVolumeClaimTemplate</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#persistentvolumeclaimspec-v1-core">
Kubernetes core/v1.PersistentVolumeClaimSpec
</a>
</em>
</td>
<td>
<p>DataVolumeClaimTemplate configures the PersistentVolumeClaims that will be created
for each etcd instance to store its data files.
This field is required.</p>
<p>IMPORTANT: For a cell-local lockserver in a Kubernetes cluster that spans
multiple zones, you should ensure that <code>volumeBindingMode: WaitForFirstConsumer</code>
is set on the StorageClass specified in the storageClassName field here.
Default: Let the operator choose.</p>
</td>
</tr>
<tr>
<td>
<code>extraFlags</code></br>
<em>
map[string]string
</em>
</td>
<td>
<p>ExtraFlags can optionally be used to override default flags set by the
operator, or pass additional flags to etcd. All entries must be
key-value string pairs of the form &ldquo;flag&rdquo;: &ldquo;value&rdquo;. The flag name should
not have any prefix (just &ldquo;flag&rdquo;, not &ldquo;-flag&rdquo;). To set a boolean flag,
set the string value to either &ldquo;true&rdquo; or &ldquo;false&rdquo;.</p>
</td>
</tr>
<tr>
<td>
<code>extraEnv</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#envvar-v1-core">
[]Kubernetes core/v1.EnvVar
</a>
</em>
</td>
<td>
<p>ExtraEnv can optionally be used to override default environment variables
set by the operator, or pass additional environment variables.</p>
</td>
</tr>
<tr>
<td>
<code>extraVolumes</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#volume-v1-core">
[]Kubernetes core/v1.Volume
</a>
</em>
</td>
<td>
<p>ExtraVolumes can optionally be used to override default Pod volumes
defined by the operator, or provide additional volumes to the Pod.
Note that when adding a new volume, you should usually also add a
volumeMount to specify where in each container&rsquo;s filesystem the volume
should be mounted.</p>
</td>
</tr>
<tr>
<td>
<code>extraVolumeMounts</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#volumemount-v1-core">
[]Kubernetes core/v1.VolumeMount
</a>
</em>
</td>
<td>
<p>ExtraVolumeMounts can optionally be used to override default Pod
volumeMounts defined by the operator, or specify additional mounts.
Typically, these are used to mount volumes defined through extraVolumes.</p>
</td>
</tr>
<tr>
<td>
<code>initContainers</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#container-v1-core">
[]Kubernetes core/v1.Container
</a>
</em>
</td>
<td>
<p>InitContainers can optionally be used to supply extra init containers
that will be run to completion one after another before any app containers are started.</p>
</td>
</tr>
<tr>
<td>
<code>sidecarContainers</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#container-v1-core">
[]Kubernetes core/v1.Container
</a>
</em>
</td>
<td>
<p>SidecarContainers can optionally be used to supply extra containers
that run alongside the main containers.</p>
</td>
</tr>
<tr>
<td>
<code>affinity</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#affinity-v1-core">
Kubernetes core/v1.Affinity
</a>
</em>
</td>
<td>
<p>Affinity allows you to set rules that constrain the scheduling of
your Etcd pods. WARNING: These affinity rules will override all default affinities
that we set; in turn, we can&rsquo;t guarantee optimal scheduling of your pods if you
choose to set this field.</p>
</td>
</tr>
<tr>
<td>
<code>annotations</code></br>
<em>
map[string]string
</em>
</td>
<td>
<p>Annotations can optionally be used to attach custom annotations to Pods
created for this component.</p>
</td>
</tr>
<tr>
<td>
<code>extraLabels</code></br>
<em>
map[string]string
</em>
</td>
<td>
<p>ExtraLabels can optionally be used to attach custom labels to Pods
created for this component.</p>
</td>
</tr>
<tr>
<td>
<code>createPDB</code></br>
<em>
bool
</em>
</td>
<td>
<p>CreatePDB sets whether to create a PodDisruptionBudget (PDB) for etcd
member Pods.</p>
<p>Note: Disabling this will NOT delete a PDB that was previously created.</p>
<p>Default: true</p>
</td>
</tr>
<tr>
<td>
<code>createClientService</code></br>
<em>
bool
</em>
</td>
<td>
<p>CreateClientService sets whether to create a Service for the client port
of etcd member Pods.</p>
<p>Note: Disabling this will NOT delete a Service that was previously created.</p>
<p>Default: true</p>
</td>
</tr>
<tr>
<td>
<code>createPeerService</code></br>
<em>
bool
</em>
</td>
<td>
<p>CreatePeerService sets whether to create a Service for the peer port
of etcd member Pods.</p>
<p>Note: Disabling this will NOT delete a Service that was previously created.</p>
<p>Default: true</p>
</td>
</tr>
<tr>
<td>
<code>advertisePeerURLs</code></br>
<em>
[]string
</em>
</td>
<td>
<p>AdvertisePeerURLs can optionally be used to override the URLs that etcd
members use to find each other for peer-to-peer connections.</p>
<p>If specified, the list must contain exactly 3 entries, one for each etcd
member index (1,2,3) respectively.</p>
<p>Default: Build peer URLs automatically based on Kubernetes built-in DNS.</p>
</td>
</tr>
<tr>
<td>
<code>localMemberIndex</code></br>
<em>
int32
</em>
</td>
<td>
<p>LocalMemberIndex can optionally be used to specify that only one etcd
member should actually be deployed. This can be used to spread members
across multiple Kubernetes clusters by configuring the EtcdLockserver CRD
in each cluster to deploy a different member index. If specified, the
index must be 1, 2, or 3.</p>
<p>Default: Deploy all etcd members locally.</p>
</td>
</tr>
<tr>
<td>
<code>clientService</code></br>
<em>
<a href="#planetscale.com/v2.ServiceOverrides">
ServiceOverrides
</a>
</em>
</td>
<td>
<p>ClientService can optionally be used to customize the etcd client Service.</p>
</td>
</tr>
<tr>
<td>
<code>peerService</code></br>
<em>
<a href="#planetscale.com/v2.ServiceOverrides">
ServiceOverrides
</a>
</em>
</td>
<td>
<p>PeerService can optionally be used to customize the etcd peer Service.</p>
</td>
</tr>
<tr>
<td>
<code>tolerations</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#toleration-v1-core">
[]Kubernetes core/v1.Toleration
</a>
</em>
</td>
<td>
<p>Tolerations allow you to schedule pods onto nodes with matching taints.</p>
</td>
</tr>
<tr>
<td>
<code>topologySpreadConstraints</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#topologyspreadconstraint-v1-core">
[]Kubernetes core/v1.TopologySpreadConstraint
</a>
</em>
</td>
<td>
<p>TopologySpreadConstraint can optionally be used to
specify how to spread etcd pods among the given topology</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.ExternalDatastore">ExternalDatastore
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessShardTabletPool">VitessShardTabletPool</a>)
</p>
<p>
<p>ExternalDatastore defines information that vttablet needs to connect to an
externally managed MySQL.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>user</code></br>
<em>
string
</em>
</td>
<td>
<p>User is a provided database user from an externally managed MySQL that Vitess can use to
carry out necessary actions.  Password for this user must be supplied in the CredentialsSecret.</p>
</td>
</tr>
<tr>
<td>
<code>host</code></br>
<em>
string
</em>
</td>
<td>
<p>Host is the endpoint string to an externally managed MySQL, without any port.</p>
</td>
</tr>
<tr>
<td>
<code>port</code></br>
<em>
int32
</em>
</td>
<td>
<p>Port specifies the port for the externally managed MySQL endpoint.</p>
</td>
</tr>
<tr>
<td>
<code>database</code></br>
<em>
string
</em>
</td>
<td>
<p>Database is the name of the database.</p>
</td>
</tr>
<tr>
<td>
<code>credentialsSecret</code></br>
<em>
<a href="#planetscale.com/v2.SecretSource">
SecretSource
</a>
</em>
</td>
<td>
<p>CredentialsSecret should link to a JSON credentials file used to connect to the externally managed
MySQL endpoint. The credentials file is understood and parsed by Vitess and must be in the format:
{
&ldquo;username&rdquo;: [
&ldquo;password&rdquo;
]
}
Vitess always uses the first password in the password array.</p>
</td>
</tr>
<tr>
<td>
<code>serverCACertSecret</code></br>
<em>
<a href="#planetscale.com/v2.SecretSource">
SecretSource
</a>
</em>
</td>
<td>
<p>ServerCACertSecret should link to a certificate authority file if one is required by your externally managed MySQL endpoint.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.ExternalVitessClusterUpdateStrategyOptions">ExternalVitessClusterUpdateStrategyOptions
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessClusterUpdateStrategy">VitessClusterUpdateStrategy</a>)
</p>
<p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>allowResourceChanges</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#resourcename-v1-core">
[]Kubernetes core/v1.ResourceName
</a>
</em>
</td>
<td>
<p>AllowResourceChanges can be used to allow changes to certain resource
requests and limits to propagate immediately, bypassing the external rollout tool.</p>
<p>Supported options:
- storage</p>
<p>Default: All resource changes wait to be released by the external rollout tool.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.GCSBackupLocation">GCSBackupLocation
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessBackupLocation">VitessBackupLocation</a>)
</p>
<p>
<p>GCSBackupLocation specifies a backup location in Google Cloud Storage.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>bucket</code></br>
<em>
string
</em>
</td>
<td>
<p>Bucket is the name of the GCS bucket to use.</p>
</td>
</tr>
<tr>
<td>
<code>keyPrefix</code></br>
<em>
string
</em>
</td>
<td>
<p>KeyPrefix is an optional prefix added to all object keys created by Vitess.
This is only needed if the same bucket is also used for something other
than backups for VitessClusters. Backups from different clusters,
keyspaces, or shards will automatically avoid colliding with each other
within a bucket, regardless of this setting.</p>
</td>
</tr>
<tr>
<td>
<code>authSecret</code></br>
<em>
<a href="#planetscale.com/v2.SecretSource">
SecretSource
</a>
</em>
</td>
<td>
<p>AuthSecret is a reference to the Secret to use for GCS authentication.
If set, this must point to a file in the format expected for the
GOOGLE_APPLICATION_CREDENTIALS environment variable.
Default: Use the default credentials of the Node.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.LockserverSpec">LockserverSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessCellTemplate">VitessCellTemplate</a>, 
<a href="#planetscale.com/v2.VitessClusterSpec">VitessClusterSpec</a>)
</p>
<p>
<p>LockserverSpec specifies either a deployed or external lockserver,
which can be either global or local.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>external</code></br>
<em>
<a href="#planetscale.com/v2.VitessLockserverParams">
VitessLockserverParams
</a>
</em>
</td>
<td>
<p>External specifies that we should connect to an existing
lockserver, instead of deploying our own.
If this is set, all other Lockserver fields are ignored.</p>
</td>
</tr>
<tr>
<td>
<code>etcd</code></br>
<em>
<a href="#planetscale.com/v2.EtcdLockserverTemplate">
EtcdLockserverTemplate
</a>
</em>
</td>
<td>
<p>Etcd deploys our own etcd cluster as a lockserver.</p>
</td>
</tr>
<tr>
<td>
<code>cellInfoAddress</code></br>
<em>
string
</em>
</td>
<td>
<p>CellInfoAddress is the host:port of topology service which will be saved to cell info.
Default: etcd client service.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.LockserverStatus">LockserverStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessCellStatus">VitessCellStatus</a>, 
<a href="#planetscale.com/v2.VitessClusterStatus">VitessClusterStatus</a>)
</p>
<p>
<p>LockserverStatus is the lockserver component of status.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>etcd</code></br>
<em>
<a href="#planetscale.com/v2.EtcdLockserverStatus">
EtcdLockserverStatus
</a>
</em>
</td>
<td>
<p>Etcd is the status of the EtcdCluster, if we were asked to deploy one.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.MysqldExporterSpec">MysqldExporterSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessShardTabletPool">VitessShardTabletPool</a>)
</p>
<p>
<p>MysqldExporterSpec configures the local MySQL exporter within a tablet.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>resources</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#resourcerequirements-v1-core">
Kubernetes core/v1.ResourceRequirements
</a>
</em>
</td>
<td>
<p>Resources specify the compute resources to allocate for just the MySQL Exporter.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.MysqldImage">MysqldImage
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessImages">VitessImages</a>, 
<a href="#planetscale.com/v2.VitessKeyspaceImages">VitessKeyspaceImages</a>)
</p>
<p>
<p>TODO: Remove this once everything is migrated to MysqldImageNew.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>mysql56Compatible</code></br>
<em>
string
</em>
</td>
<td>
<p>Mysql56Compatible is a container image (including version tag) for mysqld
that&rsquo;s compatible with the Vitess &ldquo;MySQL56&rdquo; flavor setting.</p>
</td>
</tr>
<tr>
<td>
<code>mysql80Compatible</code></br>
<em>
string
</em>
</td>
<td>
<p>Mysql80Compatible is a container image (including version tag) for mysqld
that&rsquo;s compatible with the Vitess &ldquo;MySQL80&rdquo; flavor setting.</p>
</td>
</tr>
<tr>
<td>
<code>mariadbCompatible</code></br>
<em>
string
</em>
</td>
<td>
<p>MariadbCompatible is a container image (including version tag) for mysqld
that&rsquo;s compatible with the Vitess &ldquo;MariaDB&rdquo; flavor setting.</p>
</td>
</tr>
<tr>
<td>
<code>mariadb103Compatible</code></br>
<em>
string
</em>
</td>
<td>
<p>Mariadb103Compatible is a container image (including version tag) for mysqld
that&rsquo;s compatible with the Vitess &ldquo;MariaDB103&rdquo; flavor setting.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.MysqldImageNew">MysqldImageNew
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessKeyspaceTemplateImages">VitessKeyspaceTemplateImages</a>)
</p>
<p>
<p>MysqldImageNew specifies the container image to use for mysqld,
as well as declaring which MySQL flavor setting in Vitess the
image is compatible with.</p>
<p>TODO: rename this to MysqldImage once MysqldImage is removed.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>mysql56Compatible</code></br>
<em>
string
</em>
</td>
<td>
<p>Mysql56Compatible is a container image (including version tag) for mysqld
that&rsquo;s compatible with the Vitess &ldquo;MySQL56&rdquo; flavor setting.</p>
</td>
</tr>
<tr>
<td>
<code>mysql80Compatible</code></br>
<em>
string
</em>
</td>
<td>
<p>Mysql80Compatible is a container image (including version tag) for mysqld
that&rsquo;s compatible with the Vitess &ldquo;MySQL80&rdquo; flavor setting.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.MysqldSpec">MysqldSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessShardTabletPool">VitessShardTabletPool</a>)
</p>
<p>
<p>MysqldSpec configures the local MySQL server within a tablet.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>resources</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#resourcerequirements-v1-core">
Kubernetes core/v1.ResourceRequirements
</a>
</em>
</td>
<td>
<p>Resources specify the compute resources to allocate for just the MySQL
process (the underlying local datastore).
This field is required.</p>
</td>
</tr>
<tr>
<td>
<code>configOverrides</code></br>
<em>
string
</em>
</td>
<td>
<p>ConfigOverrides can optionally be used to provide a my.cnf snippet
to override default my.cnf values (included with Vitess) for this
particular MySQL instance.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.OrphanStatus">OrphanStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessClusterStatus">VitessClusterStatus</a>, 
<a href="#planetscale.com/v2.VitessKeyspaceStatus">VitessKeyspaceStatus</a>, 
<a href="#planetscale.com/v2.VitessShardStatus">VitessShardStatus</a>)
</p>
<p>
<p>OrphanStatus indiciates why a secondary object is orphaned.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>reason</code></br>
<em>
string
</em>
</td>
<td>
<p>Reason is a CamelCase token for programmatic reasoning about why the object is orphaned.</p>
</td>
</tr>
<tr>
<td>
<code>message</code></br>
<em>
string
</em>
</td>
<td>
<p>Message is a human-readable explanation for why the object is orphaned.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.ReshardingStatus">ReshardingStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessKeyspaceStatus">VitessKeyspaceStatus</a>)
</p>
<p>
<p>ReshardingStatus defines some of the workflow related status information.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>workflow</code></br>
<em>
string
</em>
</td>
<td>
<p>Workflow represents the name of the active vreplication workflow for resharding.</p>
</td>
</tr>
<tr>
<td>
<code>state</code></br>
<em>
<a href="#planetscale.com/v2.WorkflowState">
WorkflowState
</a>
</em>
</td>
<td>
<p>State is either &lsquo;Running&rsquo;, &lsquo;Copying&rsquo;, &lsquo;Error&rsquo; or &lsquo;Unknown&rsquo;.</p>
</td>
</tr>
<tr>
<td>
<code>sourceShards</code></br>
<em>
[]string
</em>
</td>
<td>
<p>SourceShards is a list of source shards for the current resharding operation.</p>
</td>
</tr>
<tr>
<td>
<code>targetShards</code></br>
<em>
[]string
</em>
</td>
<td>
<p>TargetShards is a list of target shards for the current resharding operation.</p>
</td>
</tr>
<tr>
<td>
<code>copyProgress</code></br>
<em>
int
</em>
</td>
<td>
<p>CopyProgress will indicate the percentage completion ranging from 0-100 as integer values.
Once we are past the copy phase, this value will always be 100, and will never be 100 while we
are still within the copy phase.
If we can not compute the copy progress in a timely fashion, we will report -1 to indicate the progress is unknown.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.S3BackupLocation">S3BackupLocation
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessBackupLocation">VitessBackupLocation</a>)
</p>
<p>
<p>S3BackupLocation specifies a backup location in Amazon S3.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>region</code></br>
<em>
string
</em>
</td>
<td>
<p>Region is the AWS region in which the bucket is located.</p>
</td>
</tr>
<tr>
<td>
<code>bucket</code></br>
<em>
string
</em>
</td>
<td>
<p>Bucket is the name of the S3 bucket to use.</p>
</td>
</tr>
<tr>
<td>
<code>endpoint</code></br>
<em>
string
</em>
</td>
<td>
<p>Endpoint is the <code>host:port</code> (port is required) for the S3 backend.
Default: Use the endpoint associated with <code>region</code> by the driver.</p>
</td>
</tr>
<tr>
<td>
<code>forcePathStyle</code></br>
<em>
bool
</em>
</td>
<td>
<p>ForcePathStyle is an optional param to force connection using <endpoint>/<bucket>
Default: false By default the s3 client will try to connect to <bucket>.<endpoint>.</p>
</td>
</tr>
<tr>
<td>
<code>keyPrefix</code></br>
<em>
string
</em>
</td>
<td>
<p>KeyPrefix is an optional prefix added to all object keys created by Vitess.
This is only needed if the same bucket is also used for something other
than backups for VitessClusters. Backups from different clusters,
keyspaces, or shards will automatically avoid colliding with each other
within a bucket, regardless of this setting.</p>
</td>
</tr>
<tr>
<td>
<code>authSecret</code></br>
<em>
<a href="#planetscale.com/v2.SecretSource">
SecretSource
</a>
</em>
</td>
<td>
<p>AuthSecret is a reference to the Secret to use for S3 authentication.
If set, this must point to a file in the format expected for the
<code>~/.aws/credentials</code> file.
Default: Use the default credentials of the Node.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.SecretSource">SecretSource
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.AzblobBackupLocation">AzblobBackupLocation</a>, 
<a href="#planetscale.com/v2.CephBackupLocation">CephBackupLocation</a>, 
<a href="#planetscale.com/v2.ExternalDatastore">ExternalDatastore</a>, 
<a href="#planetscale.com/v2.GCSBackupLocation">GCSBackupLocation</a>, 
<a href="#planetscale.com/v2.S3BackupLocation">S3BackupLocation</a>, 
<a href="#planetscale.com/v2.VitessGatewayStaticAuthentication">VitessGatewayStaticAuthentication</a>, 
<a href="#planetscale.com/v2.VitessGatewayTLSSecureTransport">VitessGatewayTLSSecureTransport</a>, 
<a href="#planetscale.com/v2.VitessShardTemplate">VitessShardTemplate</a>, 
<a href="#planetscale.com/v2.VtAdminSpec">VtAdminSpec</a>)
</p>
<p>
<p>SecretSource specifies where to find the data for a particular secret value.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is the name of a Kubernetes Secret object to use as the data source.
The Secret must be in the same namespace as the VitessCluster.</p>
<p>The &lsquo;key&rsquo; field defines the item to pick from the Secret object&rsquo;s &lsquo;data&rsquo;
map.</p>
<p>If a Secret name is not specified, the data source must be defined
with the &lsquo;volumeName&rsquo; field instead.</p>
</td>
</tr>
<tr>
<td>
<code>volumeName</code></br>
<em>
string
</em>
</td>
<td>
<p>VolumeName directly specifies the name of a Volume in each Pod that
should be mounted. You must ensure a Volume by that name exists in all
relevant Pods, such as by using the appropriate ExtraVolumes fields.
If specified, this takes precedence over the &lsquo;name&rsquo; field.</p>
<p>The &lsquo;key&rsquo; field defines the name of the file to load within this Volume.</p>
</td>
</tr>
<tr>
<td>
<code>key</code></br>
<em>
string
</em>
</td>
<td>
<p>Key is the name of the item within the data source to use as the value.</p>
<p>For a Kubernetes Secret object (specified with the &lsquo;name&rsquo; field),
this is the key within the &lsquo;data&rsquo; map.</p>
<p>When &lsquo;volumeName&rsquo; is used, this specifies the name of the file to load
within that Volume.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.ServiceOverrides">ServiceOverrides
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.EtcdLockserverTemplate">EtcdLockserverTemplate</a>, 
<a href="#planetscale.com/v2.VitessCellGatewaySpec">VitessCellGatewaySpec</a>, 
<a href="#planetscale.com/v2.VitessClusterSpec">VitessClusterSpec</a>, 
<a href="#planetscale.com/v2.VitessDashboardSpec">VitessDashboardSpec</a>, 
<a href="#planetscale.com/v2.VitessOrchestratorSpec">VitessOrchestratorSpec</a>, 
<a href="#planetscale.com/v2.VtAdminSpec">VtAdminSpec</a>)
</p>
<p>
<p>ServiceOverrides allows customization of an arbitrary Service object.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>annotations</code></br>
<em>
map[string]string
</em>
</td>
<td>
<p>Annotations specifies extra annotations to add to the Service object.
Annotations added in this way will NOT be automatically removed from the
Service object if they are removed here.</p>
</td>
</tr>
<tr>
<td>
<code>clusterIP</code></br>
<em>
string
</em>
</td>
<td>
<p>ClusterIP can optionally be used to override the Service&rsquo;s clusterIP.
This field is immutable on Service objects, so changes made after the
initial creation of the Service will only be applied if you manually
delete the Service.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.ShardBackupLocationStatus">ShardBackupLocationStatus
</h3>
<p>
<p>ShardBackupLocationStatus reports status for the backups of a given shard in
a given backup location.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is the backup location name.</p>
</td>
</tr>
<tr>
<td>
<code>completeBackups</code></br>
<em>
int32
</em>
</td>
<td>
<p>CompleteBackups is the number of complete backups observed.</p>
</td>
</tr>
<tr>
<td>
<code>incompleteBackups</code></br>
<em>
int32
</em>
</td>
<td>
<p>IncompleteBackups is the number of incomplete backups observed.</p>
</td>
</tr>
<tr>
<td>
<code>latestCompleteBackupTime</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<p>LatestCompleteBackupTime is the timestamp of the most recent complete backup.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.TopoReconcileConfig">TopoReconcileConfig
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessCellSpec">VitessCellSpec</a>, 
<a href="#planetscale.com/v2.VitessClusterSpec">VitessClusterSpec</a>, 
<a href="#planetscale.com/v2.VitessKeyspaceSpec">VitessKeyspaceSpec</a>, 
<a href="#planetscale.com/v2.VitessShardSpec">VitessShardSpec</a>)
</p>
<p>
<p>TopoReconcileConfig can be used to turn on or off registration or pruning of specific vitess components from topo records.
This should only be necessary if you need to override defaults, and shouldn&rsquo;t be required for the vast majority of use cases.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>registerCellsAliases</code></br>
<em>
bool
</em>
</td>
<td>
<p>RegisterCellsAliases can be used to enable or disable registering cells aliases into topo records.
Default: true</p>
</td>
</tr>
<tr>
<td>
<code>registerCells</code></br>
<em>
bool
</em>
</td>
<td>
<p>RegisterCells can be used to enable or disable registering cells into topo records.
Default: true</p>
</td>
</tr>
<tr>
<td>
<code>pruneCells</code></br>
<em>
bool
</em>
</td>
<td>
<p>PruneCells can be used to enable or disable pruning of extraneous cells from topo records.
Default: true</p>
</td>
</tr>
<tr>
<td>
<code>pruneKeyspaces</code></br>
<em>
bool
</em>
</td>
<td>
<p>PruneKeyspaces can be used to enable or disable pruning of extraneous keyspaces from topo records.
Default: true</p>
</td>
</tr>
<tr>
<td>
<code>pruneSrvKeyspaces</code></br>
<em>
bool
</em>
</td>
<td>
<p>PruneSrvKeyspaces can be used to enable or disable pruning of extraneous serving keyspaces from topo records.
Default: true</p>
</td>
</tr>
<tr>
<td>
<code>pruneShards</code></br>
<em>
bool
</em>
</td>
<td>
<p>PruneShards can be used to enable or disable pruning of extraneous shards from topo records.
Default: true</p>
</td>
</tr>
<tr>
<td>
<code>pruneShardCells</code></br>
<em>
bool
</em>
</td>
<td>
<p>PruneShardCells can be used to enable or disable pruning of extraneous shard cells from topo records.
Default: true</p>
</td>
</tr>
<tr>
<td>
<code>pruneTablets</code></br>
<em>
bool
</em>
</td>
<td>
<p>PruneTablets can be used to enable or disable pruning of extraneous tablets from topo records.
Default: true</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessBackupEngine">VitessBackupEngine
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.ClusterBackupSpec">ClusterBackupSpec</a>, 
<a href="#planetscale.com/v2.VitessKeyspaceSpec">VitessKeyspaceSpec</a>, 
<a href="#planetscale.com/v2.VitessShardSpec">VitessShardSpec</a>)
</p>
<p>
<p>VitessBackupEngine is the backup implementation to use.</p>
</p>
<h3 id="planetscale.com/v2.VitessBackupLocation">VitessBackupLocation
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.ClusterBackupSpec">ClusterBackupSpec</a>, 
<a href="#planetscale.com/v2.VitessBackupStorageSpec">VitessBackupStorageSpec</a>, 
<a href="#planetscale.com/v2.VitessKeyspaceSpec">VitessKeyspaceSpec</a>, 
<a href="#planetscale.com/v2.VitessShardSpec">VitessShardSpec</a>)
</p>
<p>
<p>VitessBackupLocation defines a location where Vitess backups can be stored.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is used to refer to this backup location from other parts of a
VitessCluster object.</p>
<p>In particular, the backupLocationName field in each tablet pool within
each shard must match one of the names defined by this field.</p>
<p>This name must be unique among all backup locations defined in a given
cluster. A backup location with an empty name defines the default
location used when a tablet pool does not specify a backupLocationName.</p>
</td>
</tr>
<tr>
<td>
<code>gcs</code></br>
<em>
<a href="#planetscale.com/v2.GCSBackupLocation">
GCSBackupLocation
</a>
</em>
</td>
<td>
<p>GCS specifies a backup location in Google Cloud Storage.</p>
</td>
</tr>
<tr>
<td>
<code>s3</code></br>
<em>
<a href="#planetscale.com/v2.S3BackupLocation">
S3BackupLocation
</a>
</em>
</td>
<td>
<p>S3 specifies a backup location in Amazon S3.</p>
</td>
</tr>
<tr>
<td>
<code>azblob</code></br>
<em>
<a href="#planetscale.com/v2.AzblobBackupLocation">
AzblobBackupLocation
</a>
</em>
</td>
<td>
<p>Azblob specifies a backup location in Azure Blob Storage.</p>
</td>
</tr>
<tr>
<td>
<code>ceph</code></br>
<em>
<a href="#planetscale.com/v2.CephBackupLocation">
CephBackupLocation
</a>
</em>
</td>
<td>
<p>Ceph specifies a backup location in Ceph S3.</p>
</td>
</tr>
<tr>
<td>
<code>volume</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#volumesource-v1-core">
Kubernetes core/v1.VolumeSource
</a>
</em>
</td>
<td>
<p>Volume specifies a backup location as a Kubernetes Volume Source to mount.
This can be used, for example, to store backups on an NFS mount, or on
a shared host path for local testing.</p>
</td>
</tr>
<tr>
<td>
<code>volumeSubPath</code></br>
<em>
string
</em>
</td>
<td>
<p>VolumeSubPath gives the subpath in the volume to mount to the backups target.
Only used for Volume-backed backup storage, ignored otherwise.</p>
</td>
</tr>
<tr>
<td>
<code>annotations</code></br>
<em>
map[string]string
</em>
</td>
<td>
<p>Annotations can optionally be used to attach custom annotations to Pods
that need access to this backup storage location.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessBackupSchedule">VitessBackupSchedule
</h3>
<p>
<p>VitessBackupSchedule is the Schema for the VitessBackupSchedule API.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code></br>
<em>
<a href="#planetscale.com/v2.VitessBackupScheduleSpec">
VitessBackupScheduleSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>VitessBackupScheduleTemplate</code></br>
<em>
<a href="#planetscale.com/v2.VitessBackupScheduleTemplate">
VitessBackupScheduleTemplate
</a>
</em>
</td>
<td>
<p>
(Members of <code>VitessBackupScheduleTemplate</code> are embedded into this type.)
</p>
<p>VitessBackupScheduleTemplate contains the user-specific parts of VitessBackupScheduleSpec.
These are the parts that are configurable through the VitessCluster CRD.</p>
</td>
</tr>
<tr>
<td>
<code>cluster</code></br>
<em>
string
</em>
</td>
<td>
<p>Cluster on which this schedule runs.</p>
</td>
</tr>
<tr>
<td>
<code>image</code></br>
<em>
string
</em>
</td>
<td>
<p>Image should be any image that already contains vtctldclient installed.
The controller will re-use the vtctld image by default.</p>
</td>
</tr>
<tr>
<td>
<code>imagePullPolicy</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#pullpolicy-v1-core">
Kubernetes core/v1.PullPolicy
</a>
</em>
</td>
<td>
<p>ImagePullPolicy defines the policy to pull the Docker image in the job&rsquo;s pod.
The PullPolicy used will be the same as the one used to pull the vtctld image.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#planetscale.com/v2.VitessBackupScheduleStatus">
VitessBackupScheduleStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessBackupScheduleSpec">VitessBackupScheduleSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessBackupSchedule">VitessBackupSchedule</a>)
</p>
<p>
<p>VitessBackupScheduleSpec defines the desired state of VitessBackupSchedule.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>VitessBackupScheduleTemplate</code></br>
<em>
<a href="#planetscale.com/v2.VitessBackupScheduleTemplate">
VitessBackupScheduleTemplate
</a>
</em>
</td>
<td>
<p>
(Members of <code>VitessBackupScheduleTemplate</code> are embedded into this type.)
</p>
<p>VitessBackupScheduleTemplate contains the user-specific parts of VitessBackupScheduleSpec.
These are the parts that are configurable through the VitessCluster CRD.</p>
</td>
</tr>
<tr>
<td>
<code>cluster</code></br>
<em>
string
</em>
</td>
<td>
<p>Cluster on which this schedule runs.</p>
</td>
</tr>
<tr>
<td>
<code>image</code></br>
<em>
string
</em>
</td>
<td>
<p>Image should be any image that already contains vtctldclient installed.
The controller will re-use the vtctld image by default.</p>
</td>
</tr>
<tr>
<td>
<code>imagePullPolicy</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#pullpolicy-v1-core">
Kubernetes core/v1.PullPolicy
</a>
</em>
</td>
<td>
<p>ImagePullPolicy defines the policy to pull the Docker image in the job&rsquo;s pod.
The PullPolicy used will be the same as the one used to pull the vtctld image.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessBackupScheduleStatus">VitessBackupScheduleStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessBackupSchedule">VitessBackupSchedule</a>)
</p>
<p>
<p>VitessBackupScheduleStatus defines the observed state of VitessBackupSchedule</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>active</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#objectreference-v1-core">
[]Kubernetes core/v1.ObjectReference
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>A list of pointers to currently running jobs.</p>
</td>
</tr>
<tr>
<td>
<code>lastScheduledTime</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Information when was the last time the job was successfully scheduled.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessBackupScheduleStrategy">VitessBackupScheduleStrategy
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessBackupScheduleTemplate">VitessBackupScheduleTemplate</a>)
</p>
<p>
<p>VitessBackupScheduleStrategy defines how we are going to take a backup.
The VitessBackupSchedule controller uses this data to build the vtctldclient
command line that will be executed in the Job&rsquo;s pod.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code></br>
<em>
<a href="#planetscale.com/v2.BackupStrategyName">
BackupStrategyName
</a>
</em>
</td>
<td>
<p>Name of the backup strategy.</p>
</td>
</tr>
<tr>
<td>
<code>keyspace</code></br>
<em>
string
</em>
</td>
<td>
<p>Keyspace defines the keyspace on which we want to take the backup.</p>
</td>
</tr>
<tr>
<td>
<code>shard</code></br>
<em>
string
</em>
</td>
<td>
<p>Shard defines the shard on which we want to take a backup.</p>
</td>
</tr>
<tr>
<td>
<code>extraFlags</code></br>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ExtraFlags is a map of flags that will be sent down to vtctldclient when taking the backup.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessBackupScheduleTemplate">VitessBackupScheduleTemplate
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.ClusterBackupSpec">ClusterBackupSpec</a>, 
<a href="#planetscale.com/v2.VitessBackupScheduleSpec">VitessBackupScheduleSpec</a>)
</p>
<p>
<p>VitessBackupScheduleTemplate contains all the user-specific fields that the user will be
able to define when writing their YAML file.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is the schedule name, this name must be unique across all the different VitessBackupSchedule
objects in the cluster.</p>
</td>
</tr>
<tr>
<td>
<code>schedule</code></br>
<em>
string
</em>
</td>
<td>
<p>The schedule in Cron format, see <a href="https://en.wikipedia.org/wiki/Cron">https://en.wikipedia.org/wiki/Cron</a>.</p>
</td>
</tr>
<tr>
<td>
<code>strategies</code></br>
<em>
<a href="#planetscale.com/v2.VitessBackupScheduleStrategy">
[]VitessBackupScheduleStrategy
</a>
</em>
</td>
<td>
<p>Strategy defines how we are going to take a backup.
If you want to take several backups within the same schedule you can add more items
to the Strategy list. Each VitessBackupScheduleStrategy will be executed by the same
kubernetes job. This is useful if for instance you have one schedule, and you want to
take a backup of all shards in a keyspace and don&rsquo;t want to re-create a second schedule.
All the VitessBackupScheduleStrategy are concatenated into a single shell command that
is executed when the Job&rsquo;s container starts.</p>
</td>
</tr>
<tr>
<td>
<code>resources</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#resourcerequirements-v1-core">
Kubernetes core/v1.ResourceRequirements
</a>
</em>
</td>
<td>
<p>Resources specify the compute resources to allocate for every Jobs&rsquo;s pod.</p>
</td>
</tr>
<tr>
<td>
<code>successfulJobsHistoryLimit</code></br>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>SuccessfulJobsHistoryLimit defines how many successful jobs will be kept around.</p>
</td>
</tr>
<tr>
<td>
<code>failedJobsHistoryLimit</code></br>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>FailedJobsHistoryLimit defines how many failed jobs will be kept around.</p>
</td>
</tr>
<tr>
<td>
<code>suspend</code></br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Suspend pause the associated backup schedule. Pausing any further scheduled
runs until Suspend is set to false again. This is useful if you want to pause backup without
having to remove the entire VitessBackupSchedule object from the cluster.</p>
</td>
</tr>
<tr>
<td>
<code>startingDeadlineSeconds</code></br>
<em>
int64
</em>
</td>
<td>
<em>(Optional)</em>
<p>StartingDeadlineSeconds enables the VitessBackupSchedule to start a job even though it is late by
the given amount of seconds. Let&rsquo;s say for some reason the controller process a schedule run on
second after its scheduled time, if StartingDeadlineSeconds is set to 0, the job will be skipped
as it&rsquo;s too late, but on the other hand, if StartingDeadlineSeconds is greater than one second,
the job will be processed as usual.</p>
</td>
</tr>
<tr>
<td>
<code>concurrencyPolicy</code></br>
<em>
<a href="#planetscale.com/v2.ConcurrencyPolicy">
ConcurrencyPolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ConcurrencyPolicy specifies ho to treat concurrent executions of a Job.
Valid values are:
- &ldquo;Allow&rdquo; (default): allows CronJobs to run concurrently;
- &ldquo;Forbid&rdquo;: forbids concurrent runs, skipping next run if previous run hasn&rsquo;t finished yet;
- &ldquo;Replace&rdquo;: cancels currently running job and replaces it with a new one.</p>
</td>
</tr>
<tr>
<td>
<code>allowedMissedRun</code></br>
<em>
int
</em>
</td>
<td>
<em>(Optional)</em>
<p>AllowedMissedRuns defines how many missed run of the schedule will be allowed before giving up on finding the last job.
If the operator&rsquo;s clock is skewed and we end-up missing a certain number of jobs, finding the last
job might be very time-consuming, depending on the frequency of the schedule and the duration during which
the operator&rsquo;s clock was misbehaving. Also depending on how laggy the clock is, we can end-up with thousands
of missed runs. For this reason, AllowedMissedRun, which is set to 100 by default, will short circuit the search
and simply wait for the next job on the schedule.
Unless you are experiencing issue with missed runs due to a misconfiguration of the clock, we recommend leaving
this field to its default value.</p>
</td>
</tr>
<tr>
<td>
<code>jobTimeoutMinute</code></br>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>JobTimeoutMinutes defines after how many minutes a job that has not yet finished should be stopped and removed.
Default value is 10 minutes.</p>
</td>
</tr>
<tr>
<td>
<code>annotations</code></br>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Annotations are the set of annotations that will be attached to the pods created by VitessBackupSchedule.</p>
</td>
</tr>
<tr>
<td>
<code>affinity</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#affinity-v1-core">
Kubernetes core/v1.Affinity
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Affinity allows you to set rules that constrain the scheduling of the pods that take backups.
WARNING: These affinity rules will override all default affinities that we set; in turn, we can&rsquo;t
guarantee optimal scheduling of your pods if you choose to set this field.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessBackupSpec">VitessBackupSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessBackup">VitessBackup</a>)
</p>
<p>
<p>VitessBackupSpec defines the desired state of the backup.</p>
</p>
<h3 id="planetscale.com/v2.VitessBackupStatus">VitessBackupStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessBackup">VitessBackup</a>)
</p>
<p>
<p>VitessBackupStatus describes the observed state of the backup.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>startTime</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<p>StartTime is the time when the backup started.</p>
</td>
</tr>
<tr>
<td>
<code>finishedTime</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<p>FinishedTime is the time when the backup finished.</p>
</td>
</tr>
<tr>
<td>
<code>complete</code></br>
<em>
bool
</em>
</td>
<td>
<p>Complete indicates whether the backup ever completed.</p>
</td>
</tr>
<tr>
<td>
<code>position</code></br>
<em>
string
</em>
</td>
<td>
<p>Position is the replication position of the snapshot that was backed up.
The position is expressed in the native, GTID-based format of the MySQL
flavor that took the backup.
This is only available after the backup is complete.</p>
</td>
</tr>
<tr>
<td>
<code>engine</code></br>
<em>
string
</em>
</td>
<td>
<p>Engine is the Vitess backup engine implementation that was used.</p>
</td>
</tr>
<tr>
<td>
<code>storageDirectory</code></br>
<em>
string
</em>
</td>
<td>
<p>StorageDirectory is the name of the parent directory in storage that
contains this backup.</p>
</td>
</tr>
<tr>
<td>
<code>storageName</code></br>
<em>
string
</em>
</td>
<td>
<p>StorageName is the name of the backup in storage. This is different from
the name of the VitessBackup object created to represent metadata about
the actual backup in storage.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessBackupStorage">VitessBackupStorage
</h3>
<p>
<p>VitessBackupStorage represents a storage location for Vitess backups.
It provides access to metadata about Vitess backups inside Kubernetes by
maintaining a set of VitessBackup objects that represent backups in the given
storage location. One VitessBackupStorage represents a storage location
defined at the VitessCluster level, so it provides access to metadata
about backups stored in that location for any keyspace and any shard in that
cluster.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code></br>
<em>
<a href="#planetscale.com/v2.VitessBackupStorageSpec">
VitessBackupStorageSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>location</code></br>
<em>
<a href="#planetscale.com/v2.VitessBackupLocation">
VitessBackupLocation
</a>
</em>
</td>
<td>
<p>Location specifies the Vitess parameters for connecting to the backup
storage location.</p>
</td>
</tr>
<tr>
<td>
<code>subcontroller</code></br>
<em>
<a href="#planetscale.com/v2.VitessBackupSubcontrollerSpec">
VitessBackupSubcontrollerSpec
</a>
</em>
</td>
<td>
<p>Subcontroller specifies any parameters needed for launching the VitessBackupStorage subcontroller pod.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#planetscale.com/v2.VitessBackupStorageStatus">
VitessBackupStorageStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessBackupStorageSpec">VitessBackupStorageSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessBackupStorage">VitessBackupStorage</a>)
</p>
<p>
<p>VitessBackupStorageSpec defines the desired state of VitessBackupStorage.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>location</code></br>
<em>
<a href="#planetscale.com/v2.VitessBackupLocation">
VitessBackupLocation
</a>
</em>
</td>
<td>
<p>Location specifies the Vitess parameters for connecting to the backup
storage location.</p>
</td>
</tr>
<tr>
<td>
<code>subcontroller</code></br>
<em>
<a href="#planetscale.com/v2.VitessBackupSubcontrollerSpec">
VitessBackupSubcontrollerSpec
</a>
</em>
</td>
<td>
<p>Subcontroller specifies any parameters needed for launching the VitessBackupStorage subcontroller pod.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessBackupStorageStatus">VitessBackupStorageStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessBackupStorage">VitessBackupStorage</a>)
</p>
<p>
<p>VitessBackupStorageStatus defines the observed state of VitessBackupStorage.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>observedGeneration</code></br>
<em>
int64
</em>
</td>
<td>
<p>The generation observed by the controller.</p>
</td>
</tr>
<tr>
<td>
<code>totalBackupCount</code></br>
<em>
int32
</em>
</td>
<td>
<p>TotalBackupCount is the total number of backups found in this storage
location, across all keyspaces and shards.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessBackupSubcontrollerSpec">VitessBackupSubcontrollerSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.ClusterBackupSpec">ClusterBackupSpec</a>, 
<a href="#planetscale.com/v2.VitessBackupStorageSpec">VitessBackupStorageSpec</a>)
</p>
<p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>serviceAccountName</code></br>
<em>
string
</em>
</td>
<td>
<p>ServiceAccountName specifies the ServiceAccount used to launch the VitessBackupStorage subcontroller pod in the
namespace of the VitessCluster. If empty (the default), the same account as the operator will be reused. If your
VitessCluster is in a different namespace than the operator, this account is unlikely to work.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessCell">VitessCell
</h3>
<p>
<p>VitessCell represents a group of Nodes in a given failure domain (Zone),
plus Vitess components like the lockserver and gateway that are local
to each cell. Together, these cell-local components make it possible for
Vitess instances (tablets) to run on those Nodes, and for clients to reach
Vitess instances in the cell.</p>
<p>Note that VitessCell does not &ldquo;own&rdquo; the VitessKeyspaces deployed in it,
just like a Node does not own the Pods deployed on it. In addition, each
VitessKeyspace can deploy Vitess instances in multiple VitessCells,
just like a Deployment can manage Pods that run on multiple Nodes.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code></br>
<em>
<a href="#planetscale.com/v2.VitessCellSpec">
VitessCellSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>VitessCellTemplate</code></br>
<em>
<a href="#planetscale.com/v2.VitessCellTemplate">
VitessCellTemplate
</a>
</em>
</td>
<td>
<p>
(Members of <code>VitessCellTemplate</code> are embedded into this type.)
</p>
<p>VitessCellTemplate contains the user-specified parts of VitessCellSpec.
These are the parts that are configurable inside VitessCluster.
The rest of the fields below are filled in by the parent controller.</p>
</td>
</tr>
<tr>
<td>
<code>globalLockserver</code></br>
<em>
<a href="#planetscale.com/v2.VitessLockserverParams">
VitessLockserverParams
</a>
</em>
</td>
<td>
<p>GlobalLockserver are the params to connect to the global lockserver.</p>
</td>
</tr>
<tr>
<td>
<code>allCells</code></br>
<em>
[]string
</em>
</td>
<td>
<p>AllCells is a list of all cells in the Vitess cluster.</p>
</td>
</tr>
<tr>
<td>
<code>images</code></br>
<em>
<a href="#planetscale.com/v2.VitessCellImages">
VitessCellImages
</a>
</em>
</td>
<td>
<p>Images are not customizable by users at the cell level because version
skew across the cluster is discouraged except during rolling updates,
in which case this field is automatically managed by the VitessCluster
controller that owns this VitessCell.</p>
</td>
</tr>
<tr>
<td>
<code>imagePullPolicies</code></br>
<em>
<a href="#planetscale.com/v2.VitessImagePullPolicies">
VitessImagePullPolicies
</a>
</em>
</td>
<td>
<p>ImagePullPolicies are inherited from the VitessCluster spec.</p>
</td>
</tr>
<tr>
<td>
<code>imagePullSecrets</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#localobjectreference-v1-core">
[]Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
<p>ImagePullSecrets are inherited from the VitessCluster spec.</p>
</td>
</tr>
<tr>
<td>
<code>extraVitessFlags</code></br>
<em>
map[string]string
</em>
</td>
<td>
<p>ExtraVitessFlags is inherited from the parent&rsquo;s VitessClusterSpec.</p>
</td>
</tr>
<tr>
<td>
<code>topologyReconciliation</code></br>
<em>
<a href="#planetscale.com/v2.TopoReconcileConfig">
TopoReconcileConfig
</a>
</em>
</td>
<td>
<p>TopologyReconciliation is inherited from the parent&rsquo;s VitessClusterSpec.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#planetscale.com/v2.VitessCellStatus">
VitessCellStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessCellGatewaySpec">VitessCellGatewaySpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessCellTemplate">VitessCellTemplate</a>)
</p>
<p>
<p>VitessCellGatewaySpec specifies the per-cell deployment parameters for vtgate.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>replicas</code></br>
<em>
int32
</em>
</td>
<td>
<p>Replicas is the number of vtgate instances to deploy in this cell.</p>
</td>
</tr>
<tr>
<td>
<code>autoscaler</code></br>
<em>
<a href="#planetscale.com/v2.AutoscalerSpec">
AutoscalerSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Autoscaler specifies the pod autoscaling configuration to use
for the vtgate workload.</p>
</td>
</tr>
<tr>
<td>
<code>resources</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#resourcerequirements-v1-core">
Kubernetes core/v1.ResourceRequirements
</a>
</em>
</td>
<td>
<p>Resources determines the compute resources reserved for each vtgate replica.</p>
</td>
</tr>
<tr>
<td>
<code>authentication</code></br>
<em>
<a href="#planetscale.com/v2.VitessGatewayAuthentication">
VitessGatewayAuthentication
</a>
</em>
</td>
<td>
<p>Authentication configures how Vitess Gateway authenticates MySQL client connections.</p>
</td>
</tr>
<tr>
<td>
<code>secureTransport</code></br>
<em>
<a href="#planetscale.com/v2.VitessGatewaySecureTransport">
VitessGatewaySecureTransport
</a>
</em>
</td>
<td>
<p>SecureTransport configures secure transport connections for vtgate.</p>
</td>
</tr>
<tr>
<td>
<code>extraFlags</code></br>
<em>
map[string]string
</em>
</td>
<td>
<p>ExtraFlags can optionally be used to override default flags set by the
operator, or pass additional flags to vtgate. All entries must be
key-value string pairs of the form &ldquo;flag&rdquo;: &ldquo;value&rdquo;. The flag name should
not have any prefix (just &ldquo;flag&rdquo;, not &ldquo;-flag&rdquo;). To set a boolean flag,
set the string value to either &ldquo;true&rdquo; or &ldquo;false&rdquo;.</p>
</td>
</tr>
<tr>
<td>
<code>extraEnv</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#envvar-v1-core">
[]Kubernetes core/v1.EnvVar
</a>
</em>
</td>
<td>
<p>ExtraEnv can optionally be used to override default environment variables
set by the operator, or pass additional environment variables.</p>
</td>
</tr>
<tr>
<td>
<code>extraVolumes</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#volume-v1-core">
[]Kubernetes core/v1.Volume
</a>
</em>
</td>
<td>
<p>ExtraVolumes can optionally be used to override default Pod volumes
defined by the operator, or provide additional volumes to the Pod.
Note that when adding a new volume, you should usually also add a
volumeMount to specify where in each container&rsquo;s filesystem the volume
should be mounted.</p>
</td>
</tr>
<tr>
<td>
<code>extraVolumeMounts</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#volumemount-v1-core">
[]Kubernetes core/v1.VolumeMount
</a>
</em>
</td>
<td>
<p>ExtraVolumeMounts can optionally be used to override default Pod
volumeMounts defined by the operator, or specify additional mounts.
Typically, these are used to mount volumes defined through extraVolumes.</p>
</td>
</tr>
<tr>
<td>
<code>initContainers</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#container-v1-core">
[]Kubernetes core/v1.Container
</a>
</em>
</td>
<td>
<p>InitContainers can optionally be used to supply extra init containers
that will be run to completion one after another before any app containers are started.</p>
</td>
</tr>
<tr>
<td>
<code>sidecarContainers</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#container-v1-core">
[]Kubernetes core/v1.Container
</a>
</em>
</td>
<td>
<p>SidecarContainers can optionally be used to supply extra containers
that run alongside the main containers.</p>
</td>
</tr>
<tr>
<td>
<code>affinity</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#affinity-v1-core">
Kubernetes core/v1.Affinity
</a>
</em>
</td>
<td>
<p>Affinity allows you to set rules that constrain the scheduling of
your vtgate pods. WARNING: These affinity rules will override all default affinities
that we set; in turn, we can&rsquo;t guarantee optimal scheduling of your pods if you
choose to set this field.</p>
</td>
</tr>
<tr>
<td>
<code>annotations</code></br>
<em>
map[string]string
</em>
</td>
<td>
<p>Annotations can optionally be used to attach custom annotations to Pods
created for this component. These will be attached to the underlying
Pods that the vtgate Deployment creates.</p>
</td>
</tr>
<tr>
<td>
<code>extraLabels</code></br>
<em>
map[string]string
</em>
</td>
<td>
<p>ExtraLabels can optionally be used to attach custom labels to Pods
created for this component. These will be attached to the underlying
Pods that the vtgate Deployment creates.</p>
</td>
</tr>
<tr>
<td>
<code>service</code></br>
<em>
<a href="#planetscale.com/v2.ServiceOverrides">
ServiceOverrides
</a>
</em>
</td>
<td>
<p>Service can optionally be used to customize the per-cell vtgate Service.</p>
</td>
</tr>
<tr>
<td>
<code>tolerations</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#toleration-v1-core">
[]Kubernetes core/v1.Toleration
</a>
</em>
</td>
<td>
<p>Tolerations allow you to schedule pods onto nodes with matching taints.</p>
</td>
</tr>
<tr>
<td>
<code>topologySpreadConstraints</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#topologyspreadconstraint-v1-core">
[]Kubernetes core/v1.TopologySpreadConstraint
</a>
</em>
</td>
<td>
<p>TopologySpreadConstraint can optionally be used to
specify how to spread vtgate pods among the given topology</p>
</td>
</tr>
<tr>
<td>
<code>lifecycle</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#lifecycle-v1-core">
Kubernetes core/v1.Lifecycle
</a>
</em>
</td>
<td>
<p>Lifecycle can optionally be used to add container lifecycle hooks
to the vtgate container.</p>
</td>
</tr>
<tr>
<td>
<code>terminationGracePeriodSeconds</code></br>
<em>
int64
</em>
</td>
<td>
<p>TerminationGracePeriodSeconds can optionally be used to customize
terminationGracePeriodSeconds of the vtgate pod.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessCellGatewayStatus">VitessCellGatewayStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessCellStatus">VitessCellStatus</a>)
</p>
<p>
<p>VitessCellGatewayStatus is a summary of the status of vtgate in this cell.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>available</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#conditionstatus-v1-core">
Kubernetes core/v1.ConditionStatus
</a>
</em>
</td>
<td>
<p>Available indicates whether the vtgate service is fully available.</p>
</td>
</tr>
<tr>
<td>
<code>serviceName</code></br>
<em>
string
</em>
</td>
<td>
<p>ServiceName is the name of the Service for this cell&rsquo;s vtgate.</p>
</td>
</tr>
<tr>
<td>
<code>labelSelector</code></br>
<em>
string
</em>
</td>
<td>
<p>LabelSelector is required by the Scale subresource, which is used by
HorizontalPodAutoscaler when reading pod metrics.</p>
</td>
</tr>
<tr>
<td>
<code>replicas</code></br>
<em>
int32
</em>
</td>
<td>
<p>Replicas is required by the Scale subresource, which is used by
HorizontalPodAutoscaler to determine the current number of replicas.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessCellImages">VitessCellImages
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessCellSpec">VitessCellSpec</a>)
</p>
<p>
<p>VitessCellImages specifies container images to use for this cell.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>vtgate</code></br>
<em>
string
</em>
</td>
<td>
<p>Vtgate is the container image (including version tag) to use for Vitess Gateway instances.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessCellKeyspaceStatus">VitessCellKeyspaceStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessCellStatus">VitessCellStatus</a>)
</p>
<p>
<p>VitessCellKeyspaceStatus summarizes the status of a keyspace deployed in this cell.</p>
</p>
<h3 id="planetscale.com/v2.VitessCellSpec">VitessCellSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessCell">VitessCell</a>)
</p>
<p>
<p>VitessCellSpec defines the desired state of a VitessCell.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>VitessCellTemplate</code></br>
<em>
<a href="#planetscale.com/v2.VitessCellTemplate">
VitessCellTemplate
</a>
</em>
</td>
<td>
<p>
(Members of <code>VitessCellTemplate</code> are embedded into this type.)
</p>
<p>VitessCellTemplate contains the user-specified parts of VitessCellSpec.
These are the parts that are configurable inside VitessCluster.
The rest of the fields below are filled in by the parent controller.</p>
</td>
</tr>
<tr>
<td>
<code>globalLockserver</code></br>
<em>
<a href="#planetscale.com/v2.VitessLockserverParams">
VitessLockserverParams
</a>
</em>
</td>
<td>
<p>GlobalLockserver are the params to connect to the global lockserver.</p>
</td>
</tr>
<tr>
<td>
<code>allCells</code></br>
<em>
[]string
</em>
</td>
<td>
<p>AllCells is a list of all cells in the Vitess cluster.</p>
</td>
</tr>
<tr>
<td>
<code>images</code></br>
<em>
<a href="#planetscale.com/v2.VitessCellImages">
VitessCellImages
</a>
</em>
</td>
<td>
<p>Images are not customizable by users at the cell level because version
skew across the cluster is discouraged except during rolling updates,
in which case this field is automatically managed by the VitessCluster
controller that owns this VitessCell.</p>
</td>
</tr>
<tr>
<td>
<code>imagePullPolicies</code></br>
<em>
<a href="#planetscale.com/v2.VitessImagePullPolicies">
VitessImagePullPolicies
</a>
</em>
</td>
<td>
<p>ImagePullPolicies are inherited from the VitessCluster spec.</p>
</td>
</tr>
<tr>
<td>
<code>imagePullSecrets</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#localobjectreference-v1-core">
[]Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
<p>ImagePullSecrets are inherited from the VitessCluster spec.</p>
</td>
</tr>
<tr>
<td>
<code>extraVitessFlags</code></br>
<em>
map[string]string
</em>
</td>
<td>
<p>ExtraVitessFlags is inherited from the parent&rsquo;s VitessClusterSpec.</p>
</td>
</tr>
<tr>
<td>
<code>topologyReconciliation</code></br>
<em>
<a href="#planetscale.com/v2.TopoReconcileConfig">
TopoReconcileConfig
</a>
</em>
</td>
<td>
<p>TopologyReconciliation is inherited from the parent&rsquo;s VitessClusterSpec.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessCellStatus">VitessCellStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessCell">VitessCell</a>)
</p>
<p>
<p>VitessCellStatus defines the observed state of VitessCell</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>observedGeneration</code></br>
<em>
int64
</em>
</td>
<td>
<p>The generation observed by the controller.</p>
</td>
</tr>
<tr>
<td>
<code>lockserver</code></br>
<em>
<a href="#planetscale.com/v2.LockserverStatus">
LockserverStatus
</a>
</em>
</td>
<td>
<p>Lockserver is a summary of the status of the cell-local lockserver.</p>
</td>
</tr>
<tr>
<td>
<code>gateway</code></br>
<em>
<a href="#planetscale.com/v2.VitessCellGatewayStatus">
VitessCellGatewayStatus
</a>
</em>
</td>
<td>
<p>Gateway is a summary of the status of vtgate in this cell.</p>
</td>
</tr>
<tr>
<td>
<code>keyspaces</code></br>
<em>
<a href="#planetscale.com/v2.VitessCellKeyspaceStatus">
map[string]planetscale.dev/vitess-operator/pkg/apis/planetscale/v2.VitessCellKeyspaceStatus
</a>
</em>
</td>
<td>
<p>Keyspaces is a summary of keyspaces deployed in this cell.
This summary could be empty either if there are no keyspaces,
or if the controller failed to read the current state.
Use the Idle condition to distinguish these scenarios
when the difference matters.</p>
</td>
</tr>
<tr>
<td>
<code>idle</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#conditionstatus-v1-core">
Kubernetes core/v1.ConditionStatus
</a>
</em>
</td>
<td>
<p>Idle is a condition indicating whether the cell can be turned down.
If Idle is True, there are no keyspaces deployed in the cell, so it
should be safe to turn down the cell.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessCellTemplate">VitessCellTemplate
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessCellSpec">VitessCellSpec</a>, 
<a href="#planetscale.com/v2.VitessClusterSpec">VitessClusterSpec</a>)
</p>
<p>
<p>VitessCellTemplate contains only the user-specified parts of a VitessCell object.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is the cell name as it should be provided to Vitess.
Note that this is different from the VitessCell object&rsquo;s
metadata.name, which is generated by the operator.</p>
</td>
</tr>
<tr>
<td>
<code>zone</code></br>
<em>
string
</em>
</td>
<td>
<p>Zone is the name of the Availability Zone that this Vitess Cell should run in.
This value should match the value of the &ldquo;failure-domain.beta.kubernetes.io/zone&rdquo;
label on the Kubernetes Nodes in that AZ.
If the Kubernetes Nodes don&rsquo;t have such a label, leave this empty.</p>
</td>
</tr>
<tr>
<td>
<code>lockserver</code></br>
<em>
<a href="#planetscale.com/v2.LockserverSpec">
LockserverSpec
</a>
</em>
</td>
<td>
<p>Lockserver specifies either a deployed or external lockserver
to be used as the Vitess cell-local topology store.
Default: Put this cell&rsquo;s topology data in the global lockserver instead of its own lockserver.</p>
</td>
</tr>
<tr>
<td>
<code>gateway</code></br>
<em>
<a href="#planetscale.com/v2.VitessCellGatewaySpec">
VitessCellGatewaySpec
</a>
</em>
</td>
<td>
<p>Gateway configures the Vitess Gateway deployment in this cell.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessClusterCellStatus">VitessClusterCellStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessClusterStatus">VitessClusterStatus</a>)
</p>
<p>
<p>VitessClusterCellStatus is the status of a cell within a VitessCluster.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>pendingChanges</code></br>
<em>
string
</em>
</td>
<td>
<p>PendingChanges describes changes to the cell that will be
applied the next time a rolling update allows.</p>
</td>
</tr>
<tr>
<td>
<code>gatewayAvailable</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#conditionstatus-v1-core">
Kubernetes core/v1.ConditionStatus
</a>
</em>
</td>
<td>
<p>GatewayAvailable indicates whether the vtgate service is fully available.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessClusterKeyspaceStatus">VitessClusterKeyspaceStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessClusterStatus">VitessClusterStatus</a>)
</p>
<p>
<p>VitessClusterKeyspaceStatus is the status of a keyspace within a VitessCluster.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>pendingChanges</code></br>
<em>
string
</em>
</td>
<td>
<p>PendingChanges describes changes to the keyspace that will be
applied the next time a rolling update allows.</p>
</td>
</tr>
<tr>
<td>
<code>desiredShards</code></br>
<em>
int32
</em>
</td>
<td>
<p>DesiredShards is the number of desired shards. This is computed from
information that&rsquo;s already available in the spec, but clients should
use this value instead of trying to compute shard partitionings on their
own.</p>
</td>
</tr>
<tr>
<td>
<code>shards</code></br>
<em>
int32
</em>
</td>
<td>
<p>Shards is the number of observed shards. This could be higher or lower
than desiredShards if the state has not yet converged.</p>
</td>
</tr>
<tr>
<td>
<code>readyShards</code></br>
<em>
int32
</em>
</td>
<td>
<p>ReadyShards is the number of desired shards that are Ready.</p>
</td>
</tr>
<tr>
<td>
<code>updatedShards</code></br>
<em>
int32
</em>
</td>
<td>
<p>UpdatedShards is the number of desired shards that are up-to-date
(have no pending changes).</p>
</td>
</tr>
<tr>
<td>
<code>desiredTablets</code></br>
<em>
int32
</em>
</td>
<td>
<p>DesiredTablets is the total number of desired tablets across all shards.
This is computed from information that&rsquo;s already available in the spec,
but clients should use this value instead of trying to compute shard
partitionings on their own.</p>
</td>
</tr>
<tr>
<td>
<code>tablets</code></br>
<em>
int32
</em>
</td>
<td>
<p>Tablets is the total number of observed tablets across all shards.
This could be higher or lower than desiredTablets if the state has not
yet converged.</p>
</td>
</tr>
<tr>
<td>
<code>readyTablets</code></br>
<em>
int32
</em>
</td>
<td>
<p>ReadyTablets is the number of desired tablets that are Ready.</p>
</td>
</tr>
<tr>
<td>
<code>updatedTablets</code></br>
<em>
int32
</em>
</td>
<td>
<p>UpdatedTablets is the number of desired tablets that are up-to-date
(have no pending changes).</p>
</td>
</tr>
<tr>
<td>
<code>cells</code></br>
<em>
[]string
</em>
</td>
<td>
<p>Cells is a list of cells in which any observed tablets for this keyspace
are deployed.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessClusterSpec">VitessClusterSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessCluster">VitessCluster</a>)
</p>
<p>
<p>VitessClusterSpec defines the desired state of VitessCluster.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>images</code></br>
<em>
<a href="#planetscale.com/v2.VitessImages">
VitessImages
</a>
</em>
</td>
<td>
<p>Images specifies the container images (including version tag) to use
in the cluster.
Default: Let the operator choose.</p>
</td>
</tr>
<tr>
<td>
<code>imagePullPolicies</code></br>
<em>
<a href="#planetscale.com/v2.VitessImagePullPolicies">
VitessImagePullPolicies
</a>
</em>
</td>
<td>
<p>ImagePullPolicies specifies the container image pull policies to use for
images defined in the &lsquo;images&rsquo; field.</p>
</td>
</tr>
<tr>
<td>
<code>imagePullSecrets</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#localobjectreference-v1-core">
[]Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
<p>ImagePullSecrets specifies the image pull secrets to add to all Pods that
use the images defined in the &lsquo;images&rsquo; field.</p>
</td>
</tr>
<tr>
<td>
<code>backup</code></br>
<em>
<a href="#planetscale.com/v2.ClusterBackupSpec">
ClusterBackupSpec
</a>
</em>
</td>
<td>
<p>Backup specifies how to take and store Vitess backups.
This is optional but strongly recommended. In addition to disaster
recovery, Vitess currently depends on backups to support provisioning
of a new tablet in a shard with existing data, as an implementation detail.</p>
</td>
</tr>
<tr>
<td>
<code>globalLockserver</code></br>
<em>
<a href="#planetscale.com/v2.LockserverSpec">
LockserverSpec
</a>
</em>
</td>
<td>
<p>GlobalLockserver specifies either a deployed or external lockserver
to be used as the Vitess global topology store.
Default: Deploy an etcd cluster as the global lockserver.</p>
</td>
</tr>
<tr>
<td>
<code>vitessDashboard</code></br>
<em>
<a href="#planetscale.com/v2.VitessDashboardSpec">
VitessDashboardSpec
</a>
</em>
</td>
<td>
<p>Dashboard deploys a set of Vitess Dashboard servers (vtctld) for the Vitess cluster.</p>
</td>
</tr>
<tr>
<td>
<code>vtadmin</code></br>
<em>
<a href="#planetscale.com/v2.VtAdminSpec">
VtAdminSpec
</a>
</em>
</td>
<td>
<p>VtAdmin deploys a set of Vitess Admin servers for the Vitess cluster.</p>
</td>
</tr>
<tr>
<td>
<code>cells</code></br>
<em>
<a href="#planetscale.com/v2.VitessCellTemplate">
[]VitessCellTemplate
</a>
</em>
</td>
<td>
<p>Cells is a list of templates for VitessCells to create for this cluster.</p>
<p>Each VitessCell represents a set of Nodes in a given failure domain,
to which VitessKeyspaces can be deployed. The VitessCell also deploys
cell-local services that any keyspaces deployed there will need.</p>
<p>This field is required, but it may be set to an empty list: [].
Before removing any cell from this list, you should first ensure
that no keyspaces are set to deploy to this cell.</p>
</td>
</tr>
<tr>
<td>
<code>keyspaces</code></br>
<em>
<a href="#planetscale.com/v2.VitessKeyspaceTemplate">
[]VitessKeyspaceTemplate
</a>
</em>
</td>
<td>
<p>Keyspaces defines the logical databases to deploy.</p>
<p>A VitessKeyspace can deploy to multiple VitessCells.</p>
<p>This field is required, but it may be set to an empty list: [].
Before removing any keyspace from this list, you should first ensure
that it is undeployed from all cells by clearing the keyspace&rsquo;s list
of target cells.</p>
</td>
</tr>
<tr>
<td>
<code>extraVitessFlags</code></br>
<em>
map[string]string
</em>
</td>
<td>
<p>ExtraVitessFlags can optionally be used to pass flags to all Vitess components.
WARNING: Any flags passed here must be flags that can be accepted by vtgate, vtctld, vtorc, and vttablet.
An example use-case would be topo flags.</p>
<p>All entries must be key-value string pairs of the form &ldquo;flag&rdquo;: &ldquo;value&rdquo;. The flag name should
not have any prefix (just &ldquo;flag&rdquo;, not &ldquo;-flag&rdquo;). To set a boolean flag,
set the string value to either &ldquo;true&rdquo; or &ldquo;false&rdquo;.</p>
</td>
</tr>
<tr>
<td>
<code>topologyReconciliation</code></br>
<em>
<a href="#planetscale.com/v2.TopoReconcileConfig">
TopoReconcileConfig
</a>
</em>
</td>
<td>
<p>TopologyReconciliation can be used to enable or disable registration or pruning of various vitess components to and from topo records.</p>
</td>
</tr>
<tr>
<td>
<code>updateStrategy</code></br>
<em>
<a href="#planetscale.com/v2.VitessClusterUpdateStrategy">
VitessClusterUpdateStrategy
</a>
</em>
</td>
<td>
<p>UpdateStrategy specifies how components in the Vitess cluster will be updated
when a revision is made to the VitessCluster spec.</p>
</td>
</tr>
<tr>
<td>
<code>gatewayService</code></br>
<em>
<a href="#planetscale.com/v2.ServiceOverrides">
ServiceOverrides
</a>
</em>
</td>
<td>
<p>GatewayService can optionally be used to customize the global vtgate Service.
Note that per-cell vtgate Services can be customized within each cell
definition.</p>
</td>
</tr>
<tr>
<td>
<code>tabletService</code></br>
<em>
<a href="#planetscale.com/v2.ServiceOverrides">
ServiceOverrides
</a>
</em>
</td>
<td>
<p>TabletService can optionally be used to customize the global, headless vttablet Service.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessClusterStatus">VitessClusterStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessCluster">VitessCluster</a>)
</p>
<p>
<p>VitessClusterStatus defines the observed state of VitessCluster</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>observedGeneration</code></br>
<em>
int64
</em>
</td>
<td>
<p>The generation observed by the controller.</p>
</td>
</tr>
<tr>
<td>
<code>globalLockserver</code></br>
<em>
<a href="#planetscale.com/v2.LockserverStatus">
LockserverStatus
</a>
</em>
</td>
<td>
<p>GlobalLockserver is the status of the global lockserver.</p>
</td>
</tr>
<tr>
<td>
<code>gatewayServiceName</code></br>
<em>
string
</em>
</td>
<td>
<p>GatewayServiceName is the name of the cluster-wide vtgate Service.</p>
</td>
</tr>
<tr>
<td>
<code>vitessDashboard</code></br>
<em>
<a href="#planetscale.com/v2.VitessDashboardStatus">
VitessDashboardStatus
</a>
</em>
</td>
<td>
<p>VitessDashboard is a summary of the status of the vtctld deployment.</p>
</td>
</tr>
<tr>
<td>
<code>vtadmin</code></br>
<em>
<a href="#planetscale.com/v2.VtadminStatus">
VtadminStatus
</a>
</em>
</td>
<td>
<p>Vtadmin is a summary of the status of the vtadmin deployment.</p>
</td>
</tr>
<tr>
<td>
<code>cells</code></br>
<em>
<a href="#planetscale.com/v2.VitessClusterCellStatus">
map[string]planetscale.dev/vitess-operator/pkg/apis/planetscale/v2.VitessClusterCellStatus
</a>
</em>
</td>
<td>
<p>Cells is a summary of the status of desired cells.</p>
</td>
</tr>
<tr>
<td>
<code>keyspaces</code></br>
<em>
<a href="#planetscale.com/v2.VitessClusterKeyspaceStatus">
map[string]planetscale.dev/vitess-operator/pkg/apis/planetscale/v2.VitessClusterKeyspaceStatus
</a>
</em>
</td>
<td>
<p>Keyspaces is a summary of the status of desired keyspaces.</p>
</td>
</tr>
<tr>
<td>
<code>orphanedCells</code></br>
<em>
<a href="#planetscale.com/v2.OrphanStatus">
map[string]planetscale.dev/vitess-operator/pkg/apis/planetscale/v2.OrphanStatus
</a>
</em>
</td>
<td>
<p>OrphanedCells is a list of unwanted cells that could not be turned down.</p>
</td>
</tr>
<tr>
<td>
<code>orphanedKeyspaces</code></br>
<em>
<a href="#planetscale.com/v2.OrphanStatus">
map[string]planetscale.dev/vitess-operator/pkg/apis/planetscale/v2.OrphanStatus
</a>
</em>
</td>
<td>
<p>OrphanedKeyspaces is a list of unwanted keyspaces that could not be turned down.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessClusterUpdateStrategy">VitessClusterUpdateStrategy
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessClusterSpec">VitessClusterSpec</a>, 
<a href="#planetscale.com/v2.VitessKeyspaceSpec">VitessKeyspaceSpec</a>, 
<a href="#planetscale.com/v2.VitessShardSpec">VitessShardSpec</a>)
</p>
<p>
<p>VitessClusterUpdateStrategy indicates the strategy that the operator
will use to perform updates. It includes any additional parameters
necessary to perform the update for the indicated strategy.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>type</code></br>
<em>
<a href="#planetscale.com/v2.VitessClusterUpdateStrategyType">
VitessClusterUpdateStrategyType
</a>
</em>
</td>
<td>
<p>Type selects the overall update strategy.</p>
<p>Supported options are:</p>
<ul>
<li>External: Schedule updates on objects that should be updated,
but wait for an external tool to release them by adding the
&lsquo;rollout.planetscale.com/released&rsquo; annotation.</li>
<li>Immediate: Release updates to all cells, keyspaces, and shards
as soon as the VitessCluster spec is changed. Perform rolling
restart of one tablet Pod per shard at a time, with automatic
planned reparents whenever possible to avoid master downtime.</li>
</ul>
<p>Default: External</p>
</td>
</tr>
<tr>
<td>
<code>external</code></br>
<em>
<a href="#planetscale.com/v2.ExternalVitessClusterUpdateStrategyOptions">
ExternalVitessClusterUpdateStrategyOptions
</a>
</em>
</td>
<td>
<p>External can optionally be used to enable the user to customize their external update strategy
to allow certain updates to pass through immediately without using an external tool.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessClusterUpdateStrategyType">VitessClusterUpdateStrategyType
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessClusterUpdateStrategy">VitessClusterUpdateStrategy</a>)
</p>
<p>
<p>VitessClusterUpdateStrategyType is a string enumeration type that enumerates
all possible update strategies for the VitessCluster.</p>
</p>
<h3 id="planetscale.com/v2.VitessDashboardSpec">VitessDashboardSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessClusterSpec">VitessClusterSpec</a>)
</p>
<p>
<p>VitessDashboardSpec specifies deployment parameters for vtctld.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>cells</code></br>
<em>
[]string
</em>
</td>
<td>
<p>Cells is a list of cell names (as defined in the Cells list)
in which to deploy vtctld.
Default: Deploy to all defined cells.</p>
</td>
</tr>
<tr>
<td>
<code>replicas</code></br>
<em>
int32
</em>
</td>
<td>
<p>Replicas is the number of vtctld instances to deploy in each cell.</p>
</td>
</tr>
<tr>
<td>
<code>resources</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#resourcerequirements-v1-core">
Kubernetes core/v1.ResourceRequirements
</a>
</em>
</td>
<td>
<p>Resources determines the compute resources reserved for each vtctld replica.</p>
</td>
</tr>
<tr>
<td>
<code>extraFlags</code></br>
<em>
map[string]string
</em>
</td>
<td>
<p>ExtraFlags can optionally be used to override default flags set by the
operator, or pass additional flags to vtctld. All entries must be
key-value string pairs of the form &ldquo;flag&rdquo;: &ldquo;value&rdquo;. The flag name should
not have any prefix (just &ldquo;flag&rdquo;, not &ldquo;-flag&rdquo;). To set a boolean flag,
set the string value to either &ldquo;true&rdquo; or &ldquo;false&rdquo;.</p>
</td>
</tr>
<tr>
<td>
<code>extraEnv</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#envvar-v1-core">
[]Kubernetes core/v1.EnvVar
</a>
</em>
</td>
<td>
<p>ExtraEnv can optionally be used to override default environment variables
set by the operator, or pass additional environment variables.</p>
</td>
</tr>
<tr>
<td>
<code>extraVolumes</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#volume-v1-core">
[]Kubernetes core/v1.Volume
</a>
</em>
</td>
<td>
<p>ExtraVolumes can optionally be used to override default Pod volumes
defined by the operator, or provide additional volumes to the Pod.
Note that when adding a new volume, you should usually also add a
volumeMount to specify where in each container&rsquo;s filesystem the volume
should be mounted.</p>
</td>
</tr>
<tr>
<td>
<code>extraVolumeMounts</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#volumemount-v1-core">
[]Kubernetes core/v1.VolumeMount
</a>
</em>
</td>
<td>
<p>ExtraVolumeMounts can optionally be used to override default Pod
volumeMounts defined by the operator, or specify additional mounts.
Typically, these are used to mount volumes defined through extraVolumes.</p>
</td>
</tr>
<tr>
<td>
<code>initContainers</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#container-v1-core">
[]Kubernetes core/v1.Container
</a>
</em>
</td>
<td>
<p>InitContainers can optionally be used to supply extra init containers
that will be run to completion one after another before any app containers are started.</p>
</td>
</tr>
<tr>
<td>
<code>sidecarContainers</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#container-v1-core">
[]Kubernetes core/v1.Container
</a>
</em>
</td>
<td>
<p>SidecarContainers can optionally be used to supply extra containers
that run alongside the main containers.</p>
</td>
</tr>
<tr>
<td>
<code>affinity</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#affinity-v1-core">
Kubernetes core/v1.Affinity
</a>
</em>
</td>
<td>
<p>Affinity allows you to set rules that constrain the scheduling of
your vtctld pods. WARNING: These affinity rules will override all default affinities
that we set; in turn, we can&rsquo;t guarantee optimal scheduling of your pods if you
choose to set this field.</p>
</td>
</tr>
<tr>
<td>
<code>annotations</code></br>
<em>
map[string]string
</em>
</td>
<td>
<p>Annotations can optionally be used to attach custom annotations to Pods
created for this component. These will be attached to the underlying
Pods that the vtctld Deployment creates.</p>
</td>
</tr>
<tr>
<td>
<code>extraLabels</code></br>
<em>
map[string]string
</em>
</td>
<td>
<p>ExtraLabels can optionally be used to attach custom labels to Pods
created for this component. These will be attached to the underlying
Pods that the vtctld Deployment creates.</p>
</td>
</tr>
<tr>
<td>
<code>service</code></br>
<em>
<a href="#planetscale.com/v2.ServiceOverrides">
ServiceOverrides
</a>
</em>
</td>
<td>
<p>Service can optionally be used to customize the vtctld Service.</p>
</td>
</tr>
<tr>
<td>
<code>tolerations</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#toleration-v1-core">
[]Kubernetes core/v1.Toleration
</a>
</em>
</td>
<td>
<p>Tolerations allow you to schedule pods onto nodes with matching taints.</p>
</td>
</tr>
<tr>
<td>
<code>topologySpreadConstraints</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#topologyspreadconstraint-v1-core">
[]Kubernetes core/v1.TopologySpreadConstraint
</a>
</em>
</td>
<td>
<p>TopologySpreadConstraint can optionally be used to
specify how to spread vtctld pods among the given topology</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessDashboardStatus">VitessDashboardStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessClusterStatus">VitessClusterStatus</a>)
</p>
<p>
<p>VitessDashboardStatus is a summary of the status of the vtctld deployment.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>available</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#conditionstatus-v1-core">
Kubernetes core/v1.ConditionStatus
</a>
</em>
</td>
<td>
<p>Available indicates whether the vtctld service has available endpoints.</p>
</td>
</tr>
<tr>
<td>
<code>serviceName</code></br>
<em>
string
</em>
</td>
<td>
<p>ServiceName is the name of the Service for this cluster&rsquo;s vtctld.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessGatewayAuthentication">VitessGatewayAuthentication
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessCellGatewaySpec">VitessCellGatewaySpec</a>)
</p>
<p>
<p>VitessGatewayAuthentication configures authentication for vtgate in this cell.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>static</code></br>
<em>
<a href="#planetscale.com/v2.VitessGatewayStaticAuthentication">
VitessGatewayStaticAuthentication
</a>
</em>
</td>
<td>
<p>Static configures vtgate to use a static file containing usernames and passwords.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessGatewaySecureTransport">VitessGatewaySecureTransport
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessCellGatewaySpec">VitessCellGatewaySpec</a>)
</p>
<p>
<p>VitessGatewaySecureTransport configures secure transport connections for vtgate.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>required</code></br>
<em>
bool
</em>
</td>
<td>
<p>Required configures vtgate to reject non-secure transport connections.
Applies only to MySQL protocol connections.
All GRPC transport is required to be encrypted when certs are set.</p>
</td>
</tr>
<tr>
<td>
<code>tls</code></br>
<em>
<a href="#planetscale.com/v2.VitessGatewayTLSSecureTransport">
VitessGatewayTLSSecureTransport
</a>
</em>
</td>
<td>
<p>TLS configures vtgate to use TLS encrypted transport.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessGatewayStaticAuthentication">VitessGatewayStaticAuthentication
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessGatewayAuthentication">VitessGatewayAuthentication</a>)
</p>
<p>
<p>VitessGatewayStaticAuthentication configures static file authentication for vtgate.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>secret</code></br>
<em>
<a href="#planetscale.com/v2.SecretSource">
SecretSource
</a>
</em>
</td>
<td>
<p>Secret configures vtgate to load the static auth file from a given key in a given Secret.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessGatewayTLSSecureTransport">VitessGatewayTLSSecureTransport
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessGatewaySecureTransport">VitessGatewaySecureTransport</a>)
</p>
<p>
<p>VitessGatewayAuthentication configures authentication for vtgate in this cell.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>clientCACertSecret</code></br>
<em>
<a href="#planetscale.com/v2.SecretSource">
SecretSource
</a>
</em>
</td>
<td>
<p>ClientCACertSecret configures vtgate to load the TLS certificate authority PEM file from a given key in a given Secret.
If specified, checks client certificates are signed by this CA certificate.
Optional.</p>
</td>
</tr>
<tr>
<td>
<code>certSecret</code></br>
<em>
<a href="#planetscale.com/v2.SecretSource">
SecretSource
</a>
</em>
</td>
<td>
<p>CertSecret configures vtgate to load the TLS cert PEM file from a given key in a given Secret.</p>
</td>
</tr>
<tr>
<td>
<code>keySecret</code></br>
<em>
<a href="#planetscale.com/v2.SecretSource">
SecretSource
</a>
</em>
</td>
<td>
<p>KeySecret configures vtgate to load the TLS key PEM file from a given key in a given Secret.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessImagePullPolicies">VitessImagePullPolicies
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessCellSpec">VitessCellSpec</a>, 
<a href="#planetscale.com/v2.VitessClusterSpec">VitessClusterSpec</a>, 
<a href="#planetscale.com/v2.VitessKeyspaceSpec">VitessKeyspaceSpec</a>, 
<a href="#planetscale.com/v2.VitessShardSpec">VitessShardSpec</a>)
</p>
<p>
<p>VitessImagePullPolicies specifies container image pull policies to use for Vitess components.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>vtctld</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#pullpolicy-v1-core">
Kubernetes core/v1.PullPolicy
</a>
</em>
</td>
<td>
<p>Vtctld is the container image pull policy to use for Vitess Dashboard instances.</p>
</td>
</tr>
<tr>
<td>
<code>vtadmin</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#pullpolicy-v1-core">
Kubernetes core/v1.PullPolicy
</a>
</em>
</td>
<td>
<p>Vtadmin is the container image pull policy to use for Vtadmin instances.</p>
</td>
</tr>
<tr>
<td>
<code>vtorc</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#pullpolicy-v1-core">
Kubernetes core/v1.PullPolicy
</a>
</em>
</td>
<td>
<p>Vtorc is the container image pull policy to use for Vitess Orchestrator instances.</p>
</td>
</tr>
<tr>
<td>
<code>vtgate</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#pullpolicy-v1-core">
Kubernetes core/v1.PullPolicy
</a>
</em>
</td>
<td>
<p>Vtgate is the container image pull policy to use for Vitess Gateway instances.</p>
</td>
</tr>
<tr>
<td>
<code>vttablet</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#pullpolicy-v1-core">
Kubernetes core/v1.PullPolicy
</a>
</em>
</td>
<td>
<p>Vttablet is the container image pull policy to use for Vitess Tablet instances.</p>
</td>
</tr>
<tr>
<td>
<code>vtbackup</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#pullpolicy-v1-core">
Kubernetes core/v1.PullPolicy
</a>
</em>
</td>
<td>
<p>Vtbackup is the container image pull policy to use for Vitess Backup jobs.</p>
</td>
</tr>
<tr>
<td>
<code>mysqld</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#pullpolicy-v1-core">
Kubernetes core/v1.PullPolicy
</a>
</em>
</td>
<td>
<p>Mysqld is the container image pull policy to use for mysqld.</p>
</td>
</tr>
<tr>
<td>
<code>mysqldExporter</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#pullpolicy-v1-core">
Kubernetes core/v1.PullPolicy
</a>
</em>
</td>
<td>
<p>MysqldExporter is the container image pull policy to use for mysqld-exporter.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessImages">VitessImages
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessClusterSpec">VitessClusterSpec</a>)
</p>
<p>
<p>VitessImages specifies container images to use for Vitess components.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>vtctld</code></br>
<em>
string
</em>
</td>
<td>
<p>Vtctld is the container image (including version tag) to use for Vitess Dashboard instances.</p>
</td>
</tr>
<tr>
<td>
<code>vtadmin</code></br>
<em>
string
</em>
</td>
<td>
<p>Vtadmin is the container image (including version tag) to use for Vitess Admin instances.</p>
</td>
</tr>
<tr>
<td>
<code>vtorc</code></br>
<em>
string
</em>
</td>
<td>
<p>Vtorc is the container image (including version tag) to use for Vitess Orchestrator instances.</p>
</td>
</tr>
<tr>
<td>
<code>vtgate</code></br>
<em>
string
</em>
</td>
<td>
<p>Vtgate is the container image (including version tag) to use for Vitess Gateway instances.</p>
</td>
</tr>
<tr>
<td>
<code>vttablet</code></br>
<em>
string
</em>
</td>
<td>
<p>Vttablet is the container image (including version tag) to use for Vitess Tablet instances.</p>
</td>
</tr>
<tr>
<td>
<code>vtbackup</code></br>
<em>
string
</em>
</td>
<td>
<p>Vtbackup is the container image (including version tag) to use for Vitess Backup jobs.</p>
</td>
</tr>
<tr>
<td>
<code>mysqld</code></br>
<em>
<a href="#planetscale.com/v2.MysqldImage">
MysqldImage
</a>
</em>
</td>
<td>
<p>Mysqld specifies the container image to use for mysqld, as well as
declaring which MySQL flavor setting in Vitess the image is
compatible with. Only one flavor image may be provided at a time.
mysqld running alongside each tablet.</p>
</td>
</tr>
<tr>
<td>
<code>mysqldExporter</code></br>
<em>
string
</em>
</td>
<td>
<p>MysqldExporter specifies the container image to use for mysqld-exporter.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessKeyRange">VitessKeyRange
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessKeyspaceKeyRangeShard">VitessKeyspaceKeyRangeShard</a>, 
<a href="#planetscale.com/v2.VitessShardSpec">VitessShardSpec</a>)
</p>
<p>
<p>VitessKeyRange specifies a range of keyspace IDs.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>start</code></br>
<em>
string
</em>
</td>
<td>
<p>Start is a lowercase hexadecimal string representation of an arbitrary-length sequence of bytes.
If Start is the empty string, the key range is unbounded at the bottom.
If Start is not empty, the bytes of a keyspace ID must compare greater
than or equal to Start in lexicographical order to be in the range.</p>
</td>
</tr>
<tr>
<td>
<code>end</code></br>
<em>
string
</em>
</td>
<td>
<p>End is a lowercase hexadecimal string representation of an arbitrary-length sequence of bytes.
If End is the empty string, the key range is unbounded at the top.
If End is not empty, the bytes of a keyspace ID must compare strictly less than End in
lexicographical order to be in the range.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessKeyspace">VitessKeyspace
</h3>
<p>
<p>VitessKeyspace represents the deployment of a logical database in Vitess.
Each keyspace consists of a number of shards, which then consist of tablets.
The tablets belonging to one VitessKeyspace can ultimately be deployed across
various VitessCells.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code></br>
<em>
<a href="#planetscale.com/v2.VitessKeyspaceSpec">
VitessKeyspaceSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>VitessKeyspaceTemplate</code></br>
<em>
<a href="#planetscale.com/v2.VitessKeyspaceTemplate">
VitessKeyspaceTemplate
</a>
</em>
</td>
<td>
<p>
(Members of <code>VitessKeyspaceTemplate</code> are embedded into this type.)
</p>
<p>VitessKeyspaceTemplate contains the user-specified parts of VitessKeyspaceSpec.
These are the parts that are configurable inside VitessCluster.
The rest of the fields below are filled in by the parent controller.</p>
</td>
</tr>
<tr>
<td>
<code>globalLockserver</code></br>
<em>
<a href="#planetscale.com/v2.VitessLockserverParams">
VitessLockserverParams
</a>
</em>
</td>
<td>
<p>GlobalLockserver are the params to connect to the global lockserver.</p>
</td>
</tr>
<tr>
<td>
<code>images</code></br>
<em>
<a href="#planetscale.com/v2.VitessKeyspaceImages">
VitessKeyspaceImages
</a>
</em>
</td>
<td>
<p>Images are inherited from the VitessCluster spec, unless the user has
specified keyspace-level overrides. Version skew across the cluster is
discouraged except during rolling updates, in which case this field is
automatically managed by the VitessCluster controller that owns this
VitessKeyspace, or else when a user has specified a keyspace-level
images on VitessKeyspaceTemplate.</p>
</td>
</tr>
<tr>
<td>
<code>imagePullPolicies</code></br>
<em>
<a href="#planetscale.com/v2.VitessImagePullPolicies">
VitessImagePullPolicies
</a>
</em>
</td>
<td>
<p>ImagePullPolicies are inherited from the VitessCluster spec.</p>
</td>
</tr>
<tr>
<td>
<code>imagePullSecrets</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#localobjectreference-v1-core">
[]Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
<p>ImagePullSecrets are inherited from the VitessCluster spec.</p>
</td>
</tr>
<tr>
<td>
<code>zoneMap</code></br>
<em>
map[string]string
</em>
</td>
<td>
<p>ZoneMap is a map from Vitess cell name to zone (failure domain) name
for all cells defined in the VitessCluster.</p>
</td>
</tr>
<tr>
<td>
<code>backupLocations</code></br>
<em>
<a href="#planetscale.com/v2.VitessBackupLocation">
[]VitessBackupLocation
</a>
</em>
</td>
<td>
<p>BackupLocations are the backup locations defined in the VitessCluster.</p>
</td>
</tr>
<tr>
<td>
<code>backupEngine</code></br>
<em>
<a href="#planetscale.com/v2.VitessBackupEngine">
VitessBackupEngine
</a>
</em>
</td>
<td>
<p>BackupEngine specifies the Vitess backup engine to use, either &ldquo;builtin&rdquo; or &ldquo;xtrabackup&rdquo;.</p>
</td>
</tr>
<tr>
<td>
<code>extraVitessFlags</code></br>
<em>
map[string]string
</em>
</td>
<td>
<p>ExtraVitessFlags is inherited from the parent&rsquo;s VitessClusterSpec.</p>
</td>
</tr>
<tr>
<td>
<code>topologyReconciliation</code></br>
<em>
<a href="#planetscale.com/v2.TopoReconcileConfig">
TopoReconcileConfig
</a>
</em>
</td>
<td>
<p>TopologyReconciliation is inherited from the parent&rsquo;s VitessClusterSpec.</p>
</td>
</tr>
<tr>
<td>
<code>updateStrategy</code></br>
<em>
<a href="#planetscale.com/v2.VitessClusterUpdateStrategy">
VitessClusterUpdateStrategy
</a>
</em>
</td>
<td>
<p>UpdateStrategy is inherited from the parent&rsquo;s VitessClusterSpec.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#planetscale.com/v2.VitessKeyspaceStatus">
VitessKeyspaceStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessKeyspaceCondition">VitessKeyspaceCondition
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessKeyspaceStatus">VitessKeyspaceStatus</a>)
</p>
<p>
<p>VitessKeyspaceCondition contains details for the current condition of this VitessKeyspace.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>type</code></br>
<em>
<a href="#planetscale.com/v2.VitessKeyspaceConditionType">
VitessKeyspaceConditionType
</a>
</em>
</td>
<td>
<p>Type is the type of the condition.</p>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#conditionstatus-v1-core">
Kubernetes core/v1.ConditionStatus
</a>
</em>
</td>
<td>
<p>Status is the status of the condition.
Can be True, False, Unknown.</p>
</td>
</tr>
<tr>
<td>
<code>lastTransitionTime</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<p>Last time the condition transitioned from one status to another.
Optional.</p>
</td>
</tr>
<tr>
<td>
<code>reason</code></br>
<em>
string
</em>
</td>
<td>
<p>Unique, one-word, PascalCase reason for the condition&rsquo;s last transition.
Optional.</p>
</td>
</tr>
<tr>
<td>
<code>message</code></br>
<em>
string
</em>
</td>
<td>
<p>Human-readable message indicating details about last transition.
Optional.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessKeyspaceConditionType">VitessKeyspaceConditionType
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessKeyspaceCondition">VitessKeyspaceCondition</a>)
</p>
<p>
<p>VitessKeyspaceConditionType is a valid value for the key of a VitessKeyspaceCondition map where the key is a
VitessKeyspaceConditionType and the value is a VitessKeyspaceCondition.</p>
</p>
<h3 id="planetscale.com/v2.VitessKeyspaceCustomPartitioning">VitessKeyspaceCustomPartitioning
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessKeyspacePartitioning">VitessKeyspacePartitioning</a>)
</p>
<p>
<p>VitessKeyspaceCustomPartitioning lets you explicitly specify the key range of every shard.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>shards</code></br>
<em>
<a href="#planetscale.com/v2.VitessKeyspaceKeyRangeShard">
[]VitessKeyspaceKeyRangeShard
</a>
</em>
</td>
<td>
<p>Shards is a list of explicit shard specifications.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessKeyspaceEqualPartitioning">VitessKeyspaceEqualPartitioning
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessKeyspacePartitioning">VitessKeyspacePartitioning</a>)
</p>
<p>
<p>VitessKeyspaceEqualPartitioning splits the keyspace into some number of equal parts.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>parts</code></br>
<em>
int32
</em>
</td>
<td>
<p>Parts is the number of equal parts to split the keyspace into.
If you need shards that are not equal-sized, use custom partitioning instead.</p>
<p>Note that if the number of parts is not a power of 2, the key ranges will
only be roughly equal in size.</p>
<p>WARNING: DO NOT change the number of parts in a partitioning after deploying.
That&rsquo;s effectively deleting the old partitioning and adding a new one,
which can lead to downtime or data loss. Instead, add an additional
partitioning with the desired number of parts, perform a resharding
migration, and then remove the old partitioning.</p>
</td>
</tr>
<tr>
<td>
<code>shardTemplate</code></br>
<em>
<a href="#planetscale.com/v2.VitessShardTemplate">
VitessShardTemplate
</a>
</em>
</td>
<td>
<p>ShardTemplate is the configuration used for each equal-sized shard.
If you need shards that don&rsquo;t all share the same configuration,
use custom partitioning instead.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessKeyspaceImages">VitessKeyspaceImages
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessKeyspaceSpec">VitessKeyspaceSpec</a>, 
<a href="#planetscale.com/v2.VitessShardSpec">VitessShardSpec</a>)
</p>
<p>
<p>VitessKeyspaceImages specifies container images to use for this keyspace.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>vttablet</code></br>
<em>
string
</em>
</td>
<td>
<p>Vttablet is the container image (including version tag) to use for Vitess Tablet instances.</p>
</td>
</tr>
<tr>
<td>
<code>vtorc</code></br>
<em>
string
</em>
</td>
<td>
<p>Vtorc is the container image (including version tag) to use for Vitess Orchestrator instances.</p>
</td>
</tr>
<tr>
<td>
<code>vtbackup</code></br>
<em>
string
</em>
</td>
<td>
<p>Vtbackup is the container image (including version tag) to use for Vitess Backup jobs.</p>
</td>
</tr>
<tr>
<td>
<code>mysqld</code></br>
<em>
<a href="#planetscale.com/v2.MysqldImage">
MysqldImage
</a>
</em>
</td>
<td>
<p>Mysqld specifies the container image to use for mysqld, as well as
declaring which MySQL flavor setting in Vitess the image is
compatible with. Only one flavor image may be provided at a time.
mysqld running alongside each tablet.</p>
</td>
</tr>
<tr>
<td>
<code>mysqldExporter</code></br>
<em>
string
</em>
</td>
<td>
<p>MysqldExporter specifies the container image for mysqld-exporter.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessKeyspaceKeyRangeShard">VitessKeyspaceKeyRangeShard
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessKeyspaceCustomPartitioning">VitessKeyspaceCustomPartitioning</a>)
</p>
<p>
<p>VitessKeyspaceKeyRangeShard defines a shard based on a key range.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>keyRange</code></br>
<em>
<a href="#planetscale.com/v2.VitessKeyRange">
VitessKeyRange
</a>
</em>
</td>
<td>
<p>KeyRange is the range of keys that this shard serves.</p>
<p>WARNING: DO NOT change the key range of a shard after deploying.
That&rsquo;s effectively deleting the old shard and adding a new one,
which can lead to downtime or data loss. Instead, add an additional
partitioning with the desired set of shards, perform a resharding
migration, and then remove the old partitioning.</p>
</td>
</tr>
<tr>
<td>
<code>VitessShardTemplate</code></br>
<em>
<a href="#planetscale.com/v2.VitessShardTemplate">
VitessShardTemplate
</a>
</em>
</td>
<td>
<p>
(Members of <code>VitessShardTemplate</code> are embedded into this type.)
</p>
<p>VitessShardTemplate is the configuration for the shard.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessKeyspacePartitioning">VitessKeyspacePartitioning
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessKeyspaceTemplate">VitessKeyspaceTemplate</a>)
</p>
<p>
<p>VitessKeyspacePartitioning defines a set of shards by dividing the keyspace into key ranges.
Each field is a different method of dividing the keyspace. Only one field should be set on
a given partitioning.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>equal</code></br>
<em>
<a href="#planetscale.com/v2.VitessKeyspaceEqualPartitioning">
VitessKeyspaceEqualPartitioning
</a>
</em>
</td>
<td>
<p>Equal partitioning splits the keyspace into some number of equal parts,
assuming that the keyspace IDs are uniformly distributed, for example
because they&rsquo;re generated by a hash vindex.</p>
</td>
</tr>
<tr>
<td>
<code>custom</code></br>
<em>
<a href="#planetscale.com/v2.VitessKeyspaceCustomPartitioning">
VitessKeyspaceCustomPartitioning
</a>
</em>
</td>
<td>
<p>Custom partitioning lets you explicitly specify the key range of every shard,
in case you don&rsquo;t want them to be divided equally.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessKeyspacePartitioningStatus">VitessKeyspacePartitioningStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessKeyspaceStatus">VitessKeyspaceStatus</a>)
</p>
<p>
<p>VitessKeyspacePartitioningStatus aggregates status for all shards in a given partitioning.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>shardNames</code></br>
<em>
[]string
</em>
</td>
<td>
<p>ShardNames is a sorted list of shards in this partitioning,
in the format Vitess uses for shard names.</p>
</td>
</tr>
<tr>
<td>
<code>servingWrites</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#conditionstatus-v1-core">
Kubernetes core/v1.ConditionStatus
</a>
</em>
</td>
<td>
<p>ServingWrites is a condition indicating whether all shards in this
partitioning are serving writes for their key ranges.
Note that False only means not all shards are serving writes; it&rsquo;s still
possible that some shards in this partitioning are serving writes.
Check the per-shard status for full details.</p>
</td>
</tr>
<tr>
<td>
<code>desiredTablets</code></br>
<em>
int32
</em>
</td>
<td>
<p>DesiredTablets is the number of desired tablets. This is computed from
information that&rsquo;s already available in the spec, but clients should
use this value instead of trying to compute shard partitionings on their
own.</p>
</td>
</tr>
<tr>
<td>
<code>tablets</code></br>
<em>
int32
</em>
</td>
<td>
<p>Tablets is the number of observed tablets. This could be higher or
lower than desiredTablets if the state has not yet converged.</p>
</td>
</tr>
<tr>
<td>
<code>readyTablets</code></br>
<em>
int32
</em>
</td>
<td>
<p>ReadyTablets is the number of desired tablets that are Ready.</p>
</td>
</tr>
<tr>
<td>
<code>updatedTablets</code></br>
<em>
int32
</em>
</td>
<td>
<p>UpdatedTablets is the number of desired tablets that are up-to-date
(have no pending changes).</p>
</td>
</tr>
<tr>
<td>
<code>desiredShards</code></br>
<em>
int32
</em>
</td>
<td>
<p>DesiredShards is the number of desired shards. This is computed from
information that&rsquo;s already available in the spec, but clients should
use this value instead of trying to compute shard partitionings on their
own.</p>
</td>
</tr>
<tr>
<td>
<code>readyShards</code></br>
<em>
int32
</em>
</td>
<td>
<p>ReadyShards is the number of desired shards that are Ready.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessKeyspaceShardStatus">VitessKeyspaceShardStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessKeyspaceStatus">VitessKeyspaceStatus</a>)
</p>
<p>
<p>VitessKeyspaceShardStatus is the status of a shard within a keyspace.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>hasMaster</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#conditionstatus-v1-core">
Kubernetes core/v1.ConditionStatus
</a>
</em>
</td>
<td>
<p>HasMaster is a condition indicating whether the Vitess topology
reflects a master for this shard.</p>
</td>
</tr>
<tr>
<td>
<code>servingWrites</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#conditionstatus-v1-core">
Kubernetes core/v1.ConditionStatus
</a>
</em>
</td>
<td>
<p>ServingWrites is a condition indicating whether this shard is the one
that serves writes for its key range, according to Vitess topology.
A shard might be deployed without serving writes if, for example, it is
the target of a resharding operation that is still in progress.</p>
</td>
</tr>
<tr>
<td>
<code>desiredTablets</code></br>
<em>
int32
</em>
</td>
<td>
<p>DesiredTablets is the number of desired tablets. This is computed from
information that&rsquo;s already available in the spec, but clients should
use this value instead of trying to compute shard partitionings on their
own.</p>
</td>
</tr>
<tr>
<td>
<code>tablets</code></br>
<em>
int32
</em>
</td>
<td>
<p>Tablets is the number of observed tablets. This could be higher or
lower than desiredTablets if the state has not yet converged.</p>
</td>
</tr>
<tr>
<td>
<code>readyTablets</code></br>
<em>
int32
</em>
</td>
<td>
<p>ReadyTablets is the number of desired tablets that are Ready.</p>
</td>
</tr>
<tr>
<td>
<code>updatedTablets</code></br>
<em>
int32
</em>
</td>
<td>
<p>UpdatedTablets is the number of desired tablets that are up-to-date
(have no pending changes).</p>
</td>
</tr>
<tr>
<td>
<code>pendingChanges</code></br>
<em>
string
</em>
</td>
<td>
<p>PendingChanges describes changes to the shard that will be applied
the next time a rolling update allows.</p>
</td>
</tr>
<tr>
<td>
<code>cells</code></br>
<em>
[]string
</em>
</td>
<td>
<p>Cells is a list of cells in which any tablets for this shard are deployed.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessKeyspaceSpec">VitessKeyspaceSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessKeyspace">VitessKeyspace</a>)
</p>
<p>
<p>VitessKeyspaceSpec defines the desired state of a VitessKeyspace.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>VitessKeyspaceTemplate</code></br>
<em>
<a href="#planetscale.com/v2.VitessKeyspaceTemplate">
VitessKeyspaceTemplate
</a>
</em>
</td>
<td>
<p>
(Members of <code>VitessKeyspaceTemplate</code> are embedded into this type.)
</p>
<p>VitessKeyspaceTemplate contains the user-specified parts of VitessKeyspaceSpec.
These are the parts that are configurable inside VitessCluster.
The rest of the fields below are filled in by the parent controller.</p>
</td>
</tr>
<tr>
<td>
<code>globalLockserver</code></br>
<em>
<a href="#planetscale.com/v2.VitessLockserverParams">
VitessLockserverParams
</a>
</em>
</td>
<td>
<p>GlobalLockserver are the params to connect to the global lockserver.</p>
</td>
</tr>
<tr>
<td>
<code>images</code></br>
<em>
<a href="#planetscale.com/v2.VitessKeyspaceImages">
VitessKeyspaceImages
</a>
</em>
</td>
<td>
<p>Images are inherited from the VitessCluster spec, unless the user has
specified keyspace-level overrides. Version skew across the cluster is
discouraged except during rolling updates, in which case this field is
automatically managed by the VitessCluster controller that owns this
VitessKeyspace, or else when a user has specified a keyspace-level
images on VitessKeyspaceTemplate.</p>
</td>
</tr>
<tr>
<td>
<code>imagePullPolicies</code></br>
<em>
<a href="#planetscale.com/v2.VitessImagePullPolicies">
VitessImagePullPolicies
</a>
</em>
</td>
<td>
<p>ImagePullPolicies are inherited from the VitessCluster spec.</p>
</td>
</tr>
<tr>
<td>
<code>imagePullSecrets</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#localobjectreference-v1-core">
[]Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
<p>ImagePullSecrets are inherited from the VitessCluster spec.</p>
</td>
</tr>
<tr>
<td>
<code>zoneMap</code></br>
<em>
map[string]string
</em>
</td>
<td>
<p>ZoneMap is a map from Vitess cell name to zone (failure domain) name
for all cells defined in the VitessCluster.</p>
</td>
</tr>
<tr>
<td>
<code>backupLocations</code></br>
<em>
<a href="#planetscale.com/v2.VitessBackupLocation">
[]VitessBackupLocation
</a>
</em>
</td>
<td>
<p>BackupLocations are the backup locations defined in the VitessCluster.</p>
</td>
</tr>
<tr>
<td>
<code>backupEngine</code></br>
<em>
<a href="#planetscale.com/v2.VitessBackupEngine">
VitessBackupEngine
</a>
</em>
</td>
<td>
<p>BackupEngine specifies the Vitess backup engine to use, either &ldquo;builtin&rdquo; or &ldquo;xtrabackup&rdquo;.</p>
</td>
</tr>
<tr>
<td>
<code>extraVitessFlags</code></br>
<em>
map[string]string
</em>
</td>
<td>
<p>ExtraVitessFlags is inherited from the parent&rsquo;s VitessClusterSpec.</p>
</td>
</tr>
<tr>
<td>
<code>topologyReconciliation</code></br>
<em>
<a href="#planetscale.com/v2.TopoReconcileConfig">
TopoReconcileConfig
</a>
</em>
</td>
<td>
<p>TopologyReconciliation is inherited from the parent&rsquo;s VitessClusterSpec.</p>
</td>
</tr>
<tr>
<td>
<code>updateStrategy</code></br>
<em>
<a href="#planetscale.com/v2.VitessClusterUpdateStrategy">
VitessClusterUpdateStrategy
</a>
</em>
</td>
<td>
<p>UpdateStrategy is inherited from the parent&rsquo;s VitessClusterSpec.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessKeyspaceStatus">VitessKeyspaceStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessKeyspace">VitessKeyspace</a>)
</p>
<p>
<p>VitessKeyspaceStatus defines the observed state of a VitessKeyspace.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>observedGeneration</code></br>
<em>
int64
</em>
</td>
<td>
<p>The generation observed by the controller.</p>
</td>
</tr>
<tr>
<td>
<code>shards</code></br>
<em>
<a href="#planetscale.com/v2.VitessKeyspaceShardStatus">
map[string]planetscale.dev/vitess-operator/pkg/apis/planetscale/v2.VitessKeyspaceShardStatus
</a>
</em>
</td>
<td>
<p>Shards is a summary of the status of all desired shards.</p>
</td>
</tr>
<tr>
<td>
<code>partitionings</code></br>
<em>
<a href="#planetscale.com/v2.VitessKeyspacePartitioningStatus">
[]VitessKeyspacePartitioningStatus
</a>
</em>
</td>
<td>
<p>Partitionings is an aggregation of status for all shards in each partitioning.</p>
</td>
</tr>
<tr>
<td>
<code>orphanedShards</code></br>
<em>
<a href="#planetscale.com/v2.OrphanStatus">
map[string]planetscale.dev/vitess-operator/pkg/apis/planetscale/v2.OrphanStatus
</a>
</em>
</td>
<td>
<p>OrphanedShards is a list of unwanted shards that could not be turned down.</p>
</td>
</tr>
<tr>
<td>
<code>idle</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#conditionstatus-v1-core">
Kubernetes core/v1.ConditionStatus
</a>
</em>
</td>
<td>
<p>Idle is a condition indicating whether the keyspace can be turned down.
If Idle is True, the keyspace is not deployed in any cells, so it should
be safe to turn down the keyspace.</p>
</td>
</tr>
<tr>
<td>
<code>resharding</code></br>
<em>
<a href="#planetscale.com/v2.ReshardingStatus">
ReshardingStatus
</a>
</em>
</td>
<td>
<p>ReshardingStatus provides information about an active resharding operation, if any.
This field is only present if the ReshardingActive condition is True. If that condition is Unknown,
it means the operator was unable to query resharding status from Vitess.</p>
</td>
</tr>
<tr>
<td>
<code>conditions</code></br>
<em>
<a href="#planetscale.com/v2.VitessKeyspaceCondition">
[]VitessKeyspaceCondition
</a>
</em>
</td>
<td>
<p>Conditions is a list of all VitessKeyspace specific conditions we want to set and monitor.
It&rsquo;s ok for multiple controllers to add conditions here, and those conditions will be preserved.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessKeyspaceTemplate">VitessKeyspaceTemplate
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessClusterSpec">VitessClusterSpec</a>, 
<a href="#planetscale.com/v2.VitessKeyspaceSpec">VitessKeyspaceSpec</a>)
</p>
<p>
<p>VitessKeyspaceTemplate contains only the user-specified parts of a VitessKeyspace object.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is the keyspace name as it should be provided to Vitess.
Note that this is different from the VitessKeyspace object&rsquo;s
metadata.name, which is generated by the operator.</p>
<p>WARNING: DO NOT change the name of a keyspace that was already deployed.
Keyspaces cannot be renamed, so this will be interpreted as an
instruction to delete the old keyspace and create a new one.</p>
</td>
</tr>
<tr>
<td>
<code>databaseName</code></br>
<em>
string
</em>
</td>
<td>
<p>DatabaseName is the name to use for the underlying, physical MySQL
database created to hold data for the keyspace.</p>
<p>This name is mostly hidden from Vitess clients, which should see and use
only the keyspace name as a logical database. However, you may want to
set this to control the name used by clients that bypass Vitess and
connect directly to the underlying MySQL, such as certain DBA tools.</p>
<p>The default, when the field is either left unset or set to empty string,
is to add a &ldquo;vt_&rdquo; prefix to the keyspace name since that has historically
been the default in Vitess itself. However, it&rsquo;s often preferable to set
this to be the same as the keyspace name to reduce confusion.</p>
<p>Default: Add a &ldquo;vt_&rdquo; prefix to the keyspace name.</p>
</td>
</tr>
<tr>
<td>
<code>durabilityPolicy</code></br>
<em>
string
</em>
</td>
<td>
<p>DurabilityPolicy is the name of the durability policy to use for the keyspace.
If unspecified, vtop will not set the durability policy.</p>
</td>
</tr>
<tr>
<td>
<code>vitessOrchestrator</code></br>
<em>
<a href="#planetscale.com/v2.VitessOrchestratorSpec">
VitessOrchestratorSpec
</a>
</em>
</td>
<td>
<p>VitessOrchestrator deploys a set of Vitess Orchestrator (vtorc) servers for the Keyspace.
It is highly recommended that you set disable_active_reparents=true
for the vttablets if enabling vtorc.</p>
</td>
</tr>
<tr>
<td>
<code>partitionings</code></br>
<em>
<a href="#planetscale.com/v2.VitessKeyspacePartitioning">
[]VitessKeyspacePartitioning
</a>
</em>
</td>
<td>
<p>Partitionings specify how to divide the keyspace up into shards by
defining the range of keyspace IDs that each shard contains.
For example, you might divide the keyspace into N equal-sized key ranges.</p>
<p>Note that this is distinct from defining how each row maps to a keyspace ID,
which is done in the VSchema. Partitioning is purely an operational concern
(scaling the infrastructure), while VSchema is an application-level concern
(modeling relationships between data). This separation of concerns allows
resharding to occur generically at the infrastructure level without any
knowledge of the data model.</p>
<p>Each partitioning must define a set of shards that fully covers the
space of all possible keyspace IDs; there can be no gaps between ranges.
There&rsquo;s usually only one partitioning present at a time, but during
resharding, it&rsquo;s necessary to launch the destination shards alongside
the source shards. When the resharding is complete, the old partitioning
can be removed, which will turn down (undeploy) any unneeded shards.</p>
<p>If only some shards are being split or joined during resharding,
the shards that aren&rsquo;t changing must be specified in both partitionings,
although the common shards will be shared (only deployed once).
If the per-shard configuration differs, the configuration in the latter
partitioning (in the order listed in this field) will be used.
For this reason, it&rsquo;s recommended to add new partitionings at the end,
and only remove partitionings from the beginning.</p>
<p>This field is required. An unsharded keyspace may be specified as a
partitioning into 1 part.</p>
</td>
</tr>
<tr>
<td>
<code>turndownPolicy</code></br>
<em>
<a href="#planetscale.com/v2.VitessKeyspaceTurndownPolicy">
VitessKeyspaceTurndownPolicy
</a>
</em>
</td>
<td>
<p>TurndownPolicy specifies what should happen if this keyspace is ever
removed from the VitessCluster spec. By default, removing a keyspace
entry from the VitessCluster spec will NOT actually turn down the
deployed resources, unless it can be verified that the keyspace was
previously set to have 0 total desired tablets across all shards.</p>
<p>With this default policy (RequireIdle), before removing the keyspace
entry from the spec, you must first edit the keyspace entry to remove
all tablet pools from all shards, and wait for that change to roll out.
If a keyspace entry is removed too soon, the keyspace resources will
remain deployed indefinitely, and the keyspace will be listed in the
orphanedKeyspaces field of VitessCluster status.</p>
<p>This is a safety mechanism to prevent accidental edits to the cluster
object from having immediate, destructive consequences. If the cluster
spec is only ever edited by automation whose edits you trust to be safe,
you can set the policy to Immediate to skip these checks.</p>
<p>Default: RequireIdle</p>
</td>
</tr>
<tr>
<td>
<code>annotations</code></br>
<em>
map[string]string
</em>
</td>
<td>
<p>Annotations can optionally be used to attach custom annotations to the VitessKeyspace object.</p>
</td>
</tr>
<tr>
<td>
<code>images</code></br>
<em>
<a href="#planetscale.com/v2.VitessKeyspaceTemplateImages">
VitessKeyspaceTemplateImages
</a>
</em>
</td>
<td>
<p>For special cases, users may specify per-VitessKeyspace images. An
example: migrating from MySQL 5.7 to MySQL 8.0 via a <code>MoveTables</code>
operation, after which the source keyspace is destroyed.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessKeyspaceTemplateImages">VitessKeyspaceTemplateImages
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessKeyspaceTemplate">VitessKeyspaceTemplate</a>)
</p>
<p>
<p>VitessKeyspaceTemplateImages specifies user-definable container images to
use for this keyspace. The images defined here by the user will override
those defined at the top-level in VitessCluster.spec.images.</p>
<p>While this field allows you to set a different Vitess version for some
components than the version defined at the top level, it is important to
note that Vitess only ensures compatibility between one version and the
next and previous one. For instance: N is only guaranteed  to be compatible
with N+1 and N-1. Do be careful when specifying multiple versions across
your cluster so that they respect this compatibility rule.</p>
<p>Note: this structure is a copy of VitessKeyspaceImages, once we have gotten
rid of MysqldImage and replaced it by MysqldImageNew (planned for v2.15), we
should be able to remove VitessKeyspaceTemplateImages entirely and just use
VitessKeyspaceImages instead as it contains exactly the same fields.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>vttablet</code></br>
<em>
string
</em>
</td>
<td>
<p>Vttablet is the container image (including version tag) to use for Vitess Tablet instances.</p>
</td>
</tr>
<tr>
<td>
<code>vtorc</code></br>
<em>
string
</em>
</td>
<td>
<p>Vtorc is the container image (including version tag) to use for Vitess Orchestrator instances.</p>
</td>
</tr>
<tr>
<td>
<code>vtbackup</code></br>
<em>
string
</em>
</td>
<td>
<p>Vtbackup is the container image (including version tag) to use for Vitess Backup jobs.</p>
</td>
</tr>
<tr>
<td>
<code>mysqld</code></br>
<em>
<a href="#planetscale.com/v2.MysqldImageNew">
MysqldImageNew
</a>
</em>
</td>
<td>
<p>Mysqld specifies the container image to use for mysqld, as well as
declaring which MySQL flavor setting in Vitess the image is
compatible with. Only one flavor image may be provided at a time.
mysqld running alongside each tablet.</p>
</td>
</tr>
<tr>
<td>
<code>mysqldExporter</code></br>
<em>
string
</em>
</td>
<td>
<p>MysqldExporter specifies the container image for mysqld-exporter.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessKeyspaceTurndownPolicy">VitessKeyspaceTurndownPolicy
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessKeyspaceTemplate">VitessKeyspaceTemplate</a>)
</p>
<p>
<p>VitessKeyspaceTurndownPolicy is the policy for turning down a keyspace.</p>
</p>
<h3 id="planetscale.com/v2.VitessLockserverParams">VitessLockserverParams
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.LockserverSpec">LockserverSpec</a>, 
<a href="#planetscale.com/v2.VitessCellSpec">VitessCellSpec</a>, 
<a href="#planetscale.com/v2.VitessKeyspaceSpec">VitessKeyspaceSpec</a>, 
<a href="#planetscale.com/v2.VitessShardSpec">VitessShardSpec</a>)
</p>
<p>
<p>VitessLockserverParams contains only the values that Vitess needs
to connect to a given lockserver.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>implementation</code></br>
<em>
string
</em>
</td>
<td>
<p>Implementation specifies which Vitess &ldquo;topo&rdquo; plugin to use.</p>
</td>
</tr>
<tr>
<td>
<code>address</code></br>
<em>
string
</em>
</td>
<td>
<p>Address is the host:port of the lockserver client endpoint.</p>
</td>
</tr>
<tr>
<td>
<code>rootPath</code></br>
<em>
string
</em>
</td>
<td>
<p>RootPath is a path prefix for all lockserver data belonging to a given Vitess cluster.
Multiple Vitess clusters can share a lockserver as long as they have unique root paths.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessOrchestratorSpec">VitessOrchestratorSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessKeyspaceTemplate">VitessKeyspaceTemplate</a>, 
<a href="#planetscale.com/v2.VitessShardSpec">VitessShardSpec</a>)
</p>
<p>
<p>VitessOrchestratorSpec specifies deployment parameters for vtorc.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>resources</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#resourcerequirements-v1-core">
Kubernetes core/v1.ResourceRequirements
</a>
</em>
</td>
<td>
<p>Resources determines the compute resources reserved for each vtorc replica.</p>
</td>
</tr>
<tr>
<td>
<code>extraFlags</code></br>
<em>
map[string]string
</em>
</td>
<td>
<p>ExtraFlags can optionally be used to override default flags set by the
operator, or pass additional flags to vtorc. All entries must be
key-value string pairs of the form &ldquo;flag&rdquo;: &ldquo;value&rdquo;. The flag name should
not have any prefix (just &ldquo;flag&rdquo;, not &ldquo;-flag&rdquo;). To set a boolean flag,
set the string value to either &ldquo;true&rdquo; or &ldquo;false&rdquo;.</p>
</td>
</tr>
<tr>
<td>
<code>extraEnv</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#envvar-v1-core">
[]Kubernetes core/v1.EnvVar
</a>
</em>
</td>
<td>
<p>ExtraEnv can optionally be used to override default environment variables
set by the operator, or pass additional environment variables.</p>
</td>
</tr>
<tr>
<td>
<code>extraVolumes</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#volume-v1-core">
[]Kubernetes core/v1.Volume
</a>
</em>
</td>
<td>
<p>ExtraVolumes can optionally be used to override default Pod volumes
defined by the operator, or provide additional volumes to the Pod.
Note that when adding a new volume, you should usually also add a
volumeMount to specify where in each container&rsquo;s filesystem the volume
should be mounted.</p>
</td>
</tr>
<tr>
<td>
<code>extraVolumeMounts</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#volumemount-v1-core">
[]Kubernetes core/v1.VolumeMount
</a>
</em>
</td>
<td>
<p>ExtraVolumeMounts can optionally be used to override default Pod
volumeMounts defined by the operator, or specify additional mounts.
Typically, these are used to mount volumes defined through extraVolumes.</p>
</td>
</tr>
<tr>
<td>
<code>initContainers</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#container-v1-core">
[]Kubernetes core/v1.Container
</a>
</em>
</td>
<td>
<p>InitContainers can optionally be used to supply extra init containers
that will be run to completion one after another before any app containers are started.</p>
</td>
</tr>
<tr>
<td>
<code>sidecarContainers</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#container-v1-core">
[]Kubernetes core/v1.Container
</a>
</em>
</td>
<td>
<p>SidecarContainers can optionally be used to supply extra containers
that run alongside the main containers.</p>
</td>
</tr>
<tr>
<td>
<code>affinity</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#affinity-v1-core">
Kubernetes core/v1.Affinity
</a>
</em>
</td>
<td>
<p>Affinity allows you to set rules that constrain the scheduling of
your vtorc pods. WARNING: These affinity rules will override all default affinities
that we set; in turn, we can&rsquo;t guarantee optimal scheduling of your pods if you
choose to set this field.</p>
</td>
</tr>
<tr>
<td>
<code>annotations</code></br>
<em>
map[string]string
</em>
</td>
<td>
<p>Annotations can optionally be used to attach custom annotations to Pods
created for this component. These will be attached to the underlying
Pods that the vtorc Deployment creates.</p>
</td>
</tr>
<tr>
<td>
<code>extraLabels</code></br>
<em>
map[string]string
</em>
</td>
<td>
<p>ExtraLabels can optionally be used to attach custom labels to Pods
created for this component. These will be attached to the underlying
Pods that the vtorc Deployment creates.</p>
</td>
</tr>
<tr>
<td>
<code>service</code></br>
<em>
<a href="#planetscale.com/v2.ServiceOverrides">
ServiceOverrides
</a>
</em>
</td>
<td>
<p>Service can optionally be used to customize the vtorc Service.</p>
</td>
</tr>
<tr>
<td>
<code>tolerations</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#toleration-v1-core">
[]Kubernetes core/v1.Toleration
</a>
</em>
</td>
<td>
<p>Tolerations allow you to schedule pods onto nodes with matching taints.</p>
</td>
</tr>
<tr>
<td>
<code>topologySpreadConstraints</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#topologyspreadconstraint-v1-core">
[]Kubernetes core/v1.TopologySpreadConstraint
</a>
</em>
</td>
<td>
<p>TopologySpreadConstraint can optionally be used to
specify how to spread vtorc pods among the given topology</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessOrchestratorStatus">VitessOrchestratorStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessShardStatus">VitessShardStatus</a>)
</p>
<p>
<p>VitessOrchestratorStatus is a summary of the status of the vtorc deployment.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>available</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#conditionstatus-v1-core">
Kubernetes core/v1.ConditionStatus
</a>
</em>
</td>
<td>
<p>Available indicates whether the vtctld service has available endpoints.</p>
</td>
</tr>
<tr>
<td>
<code>serviceName</code></br>
<em>
string
</em>
</td>
<td>
<p>ServiceName is the name of the Service for this cluster&rsquo;s vtorc.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessReplicationSpec">VitessReplicationSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessShardTemplate">VitessShardTemplate</a>)
</p>
<p>
<p>VitessReplicationSpec specifies how Vitess will set up MySQL replication.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>initializeMaster</code></br>
<em>
bool
</em>
</td>
<td>
<p>InitializeMaster specifies whether to choose an initial master for a
new or restored shard that has no master yet.</p>
<p>Default: true.</p>
</td>
</tr>
<tr>
<td>
<code>initializeBackup</code></br>
<em>
bool
</em>
</td>
<td>
<p>InitializeBackup specifies whether to take an initial placeholder backup
as part of preparing tablets to begin replication. This only takes effect
if a backup location is defined in the VitessCluster.</p>
<p>Default: true.</p>
</td>
</tr>
<tr>
<td>
<code>recoverRestartedMaster</code></br>
<em>
bool
</em>
</td>
<td>
<p>RecoverRestartedMaster specifies whether the operator attempts to repair
replication when the master MySQL restarts in-place (due to a crash) or
its Pod gets deleted and recreated, causing the Pod IP to change.</p>
<p>Default: true.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessShard">VitessShard
</h3>
<p>
<p>VitessShard represents a group of Vitess instances (tablets) that store a subset
of the data in a logical database (keyspace).</p>
<p>The tablets belonging to one VitessShard can ultimately be deployed across
various VitessCells. All the tablets in a given shard, across all cells,
use MySQL replication to stay eventually consistent with the MySQL master
for that shard.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code></br>
<em>
<a href="#planetscale.com/v2.VitessShardSpec">
VitessShardSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>VitessShardTemplate</code></br>
<em>
<a href="#planetscale.com/v2.VitessShardTemplate">
VitessShardTemplate
</a>
</em>
</td>
<td>
<p>
(Members of <code>VitessShardTemplate</code> are embedded into this type.)
</p>
<p>VitessShardTemplate contains the user-specified parts of VitessShardSpec.
These are the parts that are configurable inside VitessCluster.
The rest of the fields below are filled in by the parent controller.</p>
</td>
</tr>
<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is the shard name as it&rsquo;s known to Vitess.</p>
</td>
</tr>
<tr>
<td>
<code>databaseName</code></br>
<em>
string
</em>
</td>
<td>
<p>DatabaseName is the name to use for the underlying MySQL database.
It is inherited from the parent keyspace, so it can only be configured at
the keyspace level.</p>
</td>
</tr>
<tr>
<td>
<code>zoneMap</code></br>
<em>
map[string]string
</em>
</td>
<td>
<p>ZoneMap is a map from Vitess cell name to zone (failure domain) name
for all cells defined in the VitessCluster.</p>
</td>
</tr>
<tr>
<td>
<code>images</code></br>
<em>
<a href="#planetscale.com/v2.VitessKeyspaceImages">
VitessKeyspaceImages
</a>
</em>
</td>
<td>
<p>Images are not customizable by users at the shard level because version
skew across the shard is discouraged except during rolling updates,
in which case this field is automatically managed by the VitessKeyspace
controller that owns this VitessShard.</p>
</td>
</tr>
<tr>
<td>
<code>imagePullPolicies</code></br>
<em>
<a href="#planetscale.com/v2.VitessImagePullPolicies">
VitessImagePullPolicies
</a>
</em>
</td>
<td>
<p>ImagePullPolicies are inherited from the VitessCluster spec.</p>
</td>
</tr>
<tr>
<td>
<code>imagePullSecrets</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#localobjectreference-v1-core">
[]Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
<p>ImagePullSecrets are inherited from the VitessCluster spec.</p>
</td>
</tr>
<tr>
<td>
<code>keyRange</code></br>
<em>
<a href="#planetscale.com/v2.VitessKeyRange">
VitessKeyRange
</a>
</em>
</td>
<td>
<p>KeyRange is the range of keyspace IDs served by this shard.</p>
</td>
</tr>
<tr>
<td>
<code>globalLockserver</code></br>
<em>
<a href="#planetscale.com/v2.VitessLockserverParams">
VitessLockserverParams
</a>
</em>
</td>
<td>
<p>GlobalLockserver are the params to connect to the global lockserver.</p>
</td>
</tr>
<tr>
<td>
<code>vitessOrchestrator</code></br>
<em>
<a href="#planetscale.com/v2.VitessOrchestratorSpec">
VitessOrchestratorSpec
</a>
</em>
</td>
<td>
<p>VitessOrchestrator is inherited from the parent&rsquo;s VitessKeyspace.</p>
</td>
</tr>
<tr>
<td>
<code>backupLocations</code></br>
<em>
<a href="#planetscale.com/v2.VitessBackupLocation">
[]VitessBackupLocation
</a>
</em>
</td>
<td>
<p>BackupLocations are the backup locations defined in the VitessCluster.</p>
</td>
</tr>
<tr>
<td>
<code>backupEngine</code></br>
<em>
<a href="#planetscale.com/v2.VitessBackupEngine">
VitessBackupEngine
</a>
</em>
</td>
<td>
<p>BackupEngine specifies the Vitess backup engine to use, either &ldquo;builtin&rdquo; or &ldquo;xtrabackup&rdquo;.</p>
</td>
</tr>
<tr>
<td>
<code>extraVitessFlags</code></br>
<em>
map[string]string
</em>
</td>
<td>
<p>ExtraVitessFlags is inherited from the parent&rsquo;s VitessClusterSpec.</p>
</td>
</tr>
<tr>
<td>
<code>topologyReconciliation</code></br>
<em>
<a href="#planetscale.com/v2.TopoReconcileConfig">
TopoReconcileConfig
</a>
</em>
</td>
<td>
<p>TopologyReconciliation is inherited from the parent&rsquo;s VitessClusterSpec.</p>
</td>
</tr>
<tr>
<td>
<code>updateStrategy</code></br>
<em>
<a href="#planetscale.com/v2.VitessClusterUpdateStrategy">
VitessClusterUpdateStrategy
</a>
</em>
</td>
<td>
<p>UpdateStrategy is inherited from the parent&rsquo;s VitessClusterSpec.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#planetscale.com/v2.VitessShardStatus">
VitessShardStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessShardCondition">VitessShardCondition
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessShardStatus">VitessShardStatus</a>)
</p>
<p>
<p>VitessShardCondition contains details for the current condition of this VitessShard.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>status</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#conditionstatus-v1-core">
Kubernetes core/v1.ConditionStatus
</a>
</em>
</td>
<td>
<p>Status is the status of the condition.
Can be True, False, Unknown.</p>
</td>
</tr>
<tr>
<td>
<code>lastTransitionTime</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<p>Last time the condition transitioned from one status to another.
Optional.</p>
</td>
</tr>
<tr>
<td>
<code>reason</code></br>
<em>
string
</em>
</td>
<td>
<p>Unique, one-word, PascalCase reason for the condition&rsquo;s last transition.
Optional.</p>
</td>
</tr>
<tr>
<td>
<code>message</code></br>
<em>
string
</em>
</td>
<td>
<p>Human-readable message indicating details about last transition.
Optional.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessShardConditionType">VitessShardConditionType
(<code>string</code> alias)</p></h3>
<p>
<p>VitessShardConditionType is a valid value for the key of a VitessShardCondition map where the key is a
VitessShardConditionType and the value is a VitessShardCondition.</p>
</p>
<h3 id="planetscale.com/v2.VitessShardSpec">VitessShardSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessShard">VitessShard</a>)
</p>
<p>
<p>VitessShardSpec defines the desired state of a VitessShard.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>VitessShardTemplate</code></br>
<em>
<a href="#planetscale.com/v2.VitessShardTemplate">
VitessShardTemplate
</a>
</em>
</td>
<td>
<p>
(Members of <code>VitessShardTemplate</code> are embedded into this type.)
</p>
<p>VitessShardTemplate contains the user-specified parts of VitessShardSpec.
These are the parts that are configurable inside VitessCluster.
The rest of the fields below are filled in by the parent controller.</p>
</td>
</tr>
<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is the shard name as it&rsquo;s known to Vitess.</p>
</td>
</tr>
<tr>
<td>
<code>databaseName</code></br>
<em>
string
</em>
</td>
<td>
<p>DatabaseName is the name to use for the underlying MySQL database.
It is inherited from the parent keyspace, so it can only be configured at
the keyspace level.</p>
</td>
</tr>
<tr>
<td>
<code>zoneMap</code></br>
<em>
map[string]string
</em>
</td>
<td>
<p>ZoneMap is a map from Vitess cell name to zone (failure domain) name
for all cells defined in the VitessCluster.</p>
</td>
</tr>
<tr>
<td>
<code>images</code></br>
<em>
<a href="#planetscale.com/v2.VitessKeyspaceImages">
VitessKeyspaceImages
</a>
</em>
</td>
<td>
<p>Images are not customizable by users at the shard level because version
skew across the shard is discouraged except during rolling updates,
in which case this field is automatically managed by the VitessKeyspace
controller that owns this VitessShard.</p>
</td>
</tr>
<tr>
<td>
<code>imagePullPolicies</code></br>
<em>
<a href="#planetscale.com/v2.VitessImagePullPolicies">
VitessImagePullPolicies
</a>
</em>
</td>
<td>
<p>ImagePullPolicies are inherited from the VitessCluster spec.</p>
</td>
</tr>
<tr>
<td>
<code>imagePullSecrets</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#localobjectreference-v1-core">
[]Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
<p>ImagePullSecrets are inherited from the VitessCluster spec.</p>
</td>
</tr>
<tr>
<td>
<code>keyRange</code></br>
<em>
<a href="#planetscale.com/v2.VitessKeyRange">
VitessKeyRange
</a>
</em>
</td>
<td>
<p>KeyRange is the range of keyspace IDs served by this shard.</p>
</td>
</tr>
<tr>
<td>
<code>globalLockserver</code></br>
<em>
<a href="#planetscale.com/v2.VitessLockserverParams">
VitessLockserverParams
</a>
</em>
</td>
<td>
<p>GlobalLockserver are the params to connect to the global lockserver.</p>
</td>
</tr>
<tr>
<td>
<code>vitessOrchestrator</code></br>
<em>
<a href="#planetscale.com/v2.VitessOrchestratorSpec">
VitessOrchestratorSpec
</a>
</em>
</td>
<td>
<p>VitessOrchestrator is inherited from the parent&rsquo;s VitessKeyspace.</p>
</td>
</tr>
<tr>
<td>
<code>backupLocations</code></br>
<em>
<a href="#planetscale.com/v2.VitessBackupLocation">
[]VitessBackupLocation
</a>
</em>
</td>
<td>
<p>BackupLocations are the backup locations defined in the VitessCluster.</p>
</td>
</tr>
<tr>
<td>
<code>backupEngine</code></br>
<em>
<a href="#planetscale.com/v2.VitessBackupEngine">
VitessBackupEngine
</a>
</em>
</td>
<td>
<p>BackupEngine specifies the Vitess backup engine to use, either &ldquo;builtin&rdquo; or &ldquo;xtrabackup&rdquo;.</p>
</td>
</tr>
<tr>
<td>
<code>extraVitessFlags</code></br>
<em>
map[string]string
</em>
</td>
<td>
<p>ExtraVitessFlags is inherited from the parent&rsquo;s VitessClusterSpec.</p>
</td>
</tr>
<tr>
<td>
<code>topologyReconciliation</code></br>
<em>
<a href="#planetscale.com/v2.TopoReconcileConfig">
TopoReconcileConfig
</a>
</em>
</td>
<td>
<p>TopologyReconciliation is inherited from the parent&rsquo;s VitessClusterSpec.</p>
</td>
</tr>
<tr>
<td>
<code>updateStrategy</code></br>
<em>
<a href="#planetscale.com/v2.VitessClusterUpdateStrategy">
VitessClusterUpdateStrategy
</a>
</em>
</td>
<td>
<p>UpdateStrategy is inherited from the parent&rsquo;s VitessClusterSpec.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessShardStatus">VitessShardStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessShard">VitessShard</a>)
</p>
<p>
<p>VitessShardStatus defines the observed state of a VitessShard.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>observedGeneration</code></br>
<em>
int64
</em>
</td>
<td>
<p>The generation observed by the controller.</p>
</td>
</tr>
<tr>
<td>
<code>tablets</code></br>
<em>
<a href="#planetscale.com/v2.VitessTabletStatus">
map[string]planetscale.dev/vitess-operator/pkg/apis/planetscale/v2.VitessTabletStatus
</a>
</em>
</td>
<td>
<p>Tablets is a summary of the status of all desired tablets in the shard.</p>
</td>
</tr>
<tr>
<td>
<code>orphanedTablets</code></br>
<em>
<a href="#planetscale.com/v2.OrphanStatus">
map[string]planetscale.dev/vitess-operator/pkg/apis/planetscale/v2.OrphanStatus
</a>
</em>
</td>
<td>
<p>OrphanedTablets is a list of unwanted tablets that could not be turned down.</p>
</td>
</tr>
<tr>
<td>
<code>cells</code></br>
<em>
[]string
</em>
</td>
<td>
<p>Cells is a list of cells in which any tablets for this shard are deployed.</p>
</td>
</tr>
<tr>
<td>
<code>vitessOrchestrator</code></br>
<em>
<a href="#planetscale.com/v2.VitessOrchestratorStatus">
VitessOrchestratorStatus
</a>
</em>
</td>
<td>
<p>VitessOrchestrator is a summary of the status of the vtorc deployment.</p>
</td>
</tr>
<tr>
<td>
<code>hasMaster</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#conditionstatus-v1-core">
Kubernetes core/v1.ConditionStatus
</a>
</em>
</td>
<td>
<p>HasMaster is a condition indicating whether the Vitess topology
reflects a master for this shard.</p>
</td>
</tr>
<tr>
<td>
<code>hasInitialBackup</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#conditionstatus-v1-core">
Kubernetes core/v1.ConditionStatus
</a>
</em>
</td>
<td>
<p>HasInitialBackup is a condition indicating whether the initial backup
has been seeded for the shard.</p>
</td>
</tr>
<tr>
<td>
<code>servingWrites</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#conditionstatus-v1-core">
Kubernetes core/v1.ConditionStatus
</a>
</em>
</td>
<td>
<p>ServingWrites is a condition indicating whether this shard is the one
that serves writes for its key range, according to Vitess topology.
A shard might be deployed without serving writes if, for example, it is
the target of a resharding operation that is still in progress.</p>
</td>
</tr>
<tr>
<td>
<code>idle</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#conditionstatus-v1-core">
Kubernetes core/v1.ConditionStatus
</a>
</em>
</td>
<td>
<p>Idle is a condition indicating whether the shard can be turned down.
If Idle is True, the shard is not part of the active shard set
(partitioning) for any tablet type in any cell, so it should be safe
to turn down the shard.</p>
</td>
</tr>
<tr>
<td>
<code>conditions</code></br>
<em>
<a href="#planetscale.com/v2.VitessShardCondition">
map[planetscale.dev/vitess-operator/pkg/apis/planetscale/v2.VitessShardConditionType]planetscale.dev/vitess-operator/pkg/apis/planetscale/v2.VitessShardCondition
</a>
</em>
</td>
<td>
<p>Conditions is a map of all VitessShard specific conditions we want to set and monitor.
It&rsquo;s ok for multiple controllers to add conditions here, and those conditions will be preserved.</p>
</td>
</tr>
<tr>
<td>
<code>masterAlias</code></br>
<em>
string
</em>
</td>
<td>
<p>MasterAlias is the tablet alias of the master according to the global
shard record. This could be empty either because there is no master,
or because the shard record could not be read. Check the HasMaster
condition whenever the distinction is important.</p>
</td>
</tr>
<tr>
<td>
<code>backupLocations</code></br>
<em>
<a href="#planetscale.com/v2.*planetscale.dev/vitess-operator/pkg/apis/planetscale/v2.ShardBackupLocationStatus">
[]ShardBackupLocationStatus
</a>
</em>
</td>
<td>
<p>BackupLocations reports information about the backups for this shard in
each backup location.</p>
</td>
</tr>
<tr>
<td>
<code>lowestPodGeneration</code></br>
<em>
int64
</em>
</td>
<td>
<p>LowestPodGeneration is the oldest VitessShard object generation seen across
all child Pods. The tablet information in VitessShard status is guaranteed to be
at least as up-to-date as this VitessShard generation. Changes made in
subsequent generations that affect tablets may not be reflected in status yet.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessShardTabletPool">VitessShardTabletPool
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessShardTemplate">VitessShardTemplate</a>)
</p>
<p>
<p>VitessShardTabletPool defines a pool of tablets with a similar purpose.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>cell</code></br>
<em>
string
</em>
</td>
<td>
<p>Cell is the name of the Vitess cell in which to deploy this pool.</p>
</td>
</tr>
<tr>
<td>
<code>type</code></br>
<em>
<a href="#planetscale.com/v2.VitessTabletPoolType">
VitessTabletPoolType
</a>
</em>
</td>
<td>
<p>Type is the type of tablet contained in this tablet pool.</p>
<p>The allowed types are:</p>
<ul>
<li>replica - master-eligible tablets that serve transactional (OLTP) workloads</li>
<li>rdonly - master-ineligible tablets (can never be promoted to master) that serve batch/analytical (OLAP) workloads</li>
<li>externalmaster - tablets pointed at an external, read-write MySQL endpoint</li>
<li>externalreplica - tablets pointed at an external, read-only MySQL endpoint that serve transactional (OLTP) workloads</li>
<li>externalrdonly - tablets pointed at an external, read-only MySQL endpoint that serve batch/analytical (OLAP) workloads</li>
</ul>
</td>
</tr>
<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is the pool&rsquo;s unique name within the (cell,type) pair.
This field is optional, and defaults to an empty.
Assigning different names to this field enables the existence of multiple pools with a specific tablet type in a given cell,
which can be beneficial for unmanaged tablets.
Hence, you must specify ExternalDatastore when assigning a name to this field.</p>
</td>
</tr>
<tr>
<td>
<code>replicas</code></br>
<em>
int32
</em>
</td>
<td>
<p>Replicas is the number of tablets to deploy in this pool.
This field is required, although it may be set to 0,
which will scale the pool down to 0 tablets.</p>
</td>
</tr>
<tr>
<td>
<code>dataVolumeClaimTemplate</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#persistentvolumeclaimspec-v1-core">
Kubernetes core/v1.PersistentVolumeClaimSpec
</a>
</em>
</td>
<td>
<p>DataVolumeClaimTemplate configures the PersistentVolumeClaims that will be created
for each tablet to store its database files.
This field is required for local MySQL, but should be omitted in the case of externally
managed MySQL.</p>
<p>IMPORTANT: For a tablet pool in a Kubernetes cluster that spans multiple
zones, you should ensure that <code>volumeBindingMode: WaitForFirstConsumer</code>
is set on the StorageClass specified in the storageClassName field here.</p>
</td>
</tr>
<tr>
<td>
<code>backupLocationName</code></br>
<em>
string
</em>
</td>
<td>
<p>BackupLocationName is the name of the backup location to use for this
tablet pool. It must match the name of one of the backup locations
defined in the VitessCluster.
Default: Use the backup location whose name is empty.</p>
</td>
</tr>
<tr>
<td>
<code>vttablet</code></br>
<em>
<a href="#planetscale.com/v2.VttabletSpec">
VttabletSpec
</a>
</em>
</td>
<td>
<p>Vttablet configures the vttablet server within each tablet.</p>
</td>
</tr>
<tr>
<td>
<code>mysqld</code></br>
<em>
<a href="#planetscale.com/v2.MysqldSpec">
MysqldSpec
</a>
</em>
</td>
<td>
<p>Mysqld configures a local MySQL running inside each tablet Pod.
You must specify either Mysqld or ExternalDatastore, but not both.</p>
</td>
</tr>
<tr>
<td>
<code>mysqldExporter</code></br>
<em>
<a href="#planetscale.com/v2.MysqldExporterSpec">
MysqldExporterSpec
</a>
</em>
</td>
<td>
<p>MysqldExporter configures a MySQL exporter running inside each tablet Pod.</p>
</td>
</tr>
<tr>
<td>
<code>externalDatastore</code></br>
<em>
<a href="#planetscale.com/v2.ExternalDatastore">
ExternalDatastore
</a>
</em>
</td>
<td>
<p>ExternalDatastore provides information for an externally managed MySQL.
You must specify either Mysqld or ExternalDatastore, but not both.</p>
</td>
</tr>
<tr>
<td>
<code>affinity</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#affinity-v1-core">
Kubernetes core/v1.Affinity
</a>
</em>
</td>
<td>
<p>Affinity allows you to set rules that constrain the scheduling of
your vttablet pods. Affinity rules will affect all underlying
tablets in the specified tablet pool the same way. WARNING: These affinity rules
will override all default affinities that we set; in turn, we can&rsquo;t guarantee
optimal scheduling of your pods if you choose to set this field.</p>
</td>
</tr>
<tr>
<td>
<code>annotations</code></br>
<em>
map[string]string
</em>
</td>
<td>
<p>Annotations can optionally be used to attach custom annotations to Pods
created for this component.</p>
</td>
</tr>
<tr>
<td>
<code>extraLabels</code></br>
<em>
map[string]string
</em>
</td>
<td>
<p>ExtraLabels can optionally be used to attach custom labels to Pods
created for this component.</p>
</td>
</tr>
<tr>
<td>
<code>extraEnv</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#envvar-v1-core">
[]Kubernetes core/v1.EnvVar
</a>
</em>
</td>
<td>
<p>ExtraEnv can optionally be used to override default environment variables
set by the operator, or pass additional environment variables.
These values are applied to both the vttablet and mysqld containers.</p>
</td>
</tr>
<tr>
<td>
<code>extraVolumes</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#volume-v1-core">
[]Kubernetes core/v1.Volume
</a>
</em>
</td>
<td>
<p>ExtraVolumes can optionally be used to override default Pod volumes
defined by the operator, or provide additional volumes to the Pod.
Note that when adding a new volume, you should usually also add a
volumeMount to specify where in each container&rsquo;s filesystem the volume
should be mounted.
These volumes are available to be mounted by both vttablet and mysqld.</p>
</td>
</tr>
<tr>
<td>
<code>extraVolumeMounts</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#volumemount-v1-core">
[]Kubernetes core/v1.VolumeMount
</a>
</em>
</td>
<td>
<p>ExtraVolumeMounts can optionally be used to override default Pod
volumeMounts defined by the operator, or specify additional mounts.
Typically, these are used to mount volumes defined through extraVolumes.
These values are applied to both the vttablet and mysqld containers.</p>
</td>
</tr>
<tr>
<td>
<code>initContainers</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#container-v1-core">
[]Kubernetes core/v1.Container
</a>
</em>
</td>
<td>
<p>InitContainers can optionally be used to supply extra init containers
that will be run to completion one after another before any app containers are started.</p>
</td>
</tr>
<tr>
<td>
<code>sidecarContainers</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#container-v1-core">
[]Kubernetes core/v1.Container
</a>
</em>
</td>
<td>
<p>SidecarContainers can optionally be used to supply extra containers
that run alongside the main containers.</p>
</td>
</tr>
<tr>
<td>
<code>tolerations</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#toleration-v1-core">
[]Kubernetes core/v1.Toleration
</a>
</em>
</td>
<td>
<p>Tolerations allow you to schedule pods onto nodes with matching taints.</p>
</td>
</tr>
<tr>
<td>
<code>topologySpreadConstraints</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#topologyspreadconstraint-v1-core">
[]Kubernetes core/v1.TopologySpreadConstraint
</a>
</em>
</td>
<td>
<p>TopologySpreadConstraint can optionally be used to
specify how to spread vttablet pods among the given topology</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessShardTemplate">VitessShardTemplate
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessKeyspaceEqualPartitioning">VitessKeyspaceEqualPartitioning</a>, 
<a href="#planetscale.com/v2.VitessKeyspaceKeyRangeShard">VitessKeyspaceKeyRangeShard</a>, 
<a href="#planetscale.com/v2.VitessShardSpec">VitessShardSpec</a>)
</p>
<p>
<p>VitessShardTemplate contains only the user-specified parts of a VitessShard object.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>tabletPools</code></br>
<em>
<a href="#planetscale.com/v2.VitessShardTabletPool">
[]VitessShardTabletPool
</a>
</em>
</td>
<td>
<p>TabletPools specify groups of tablets in a given cell with a certain
tablet type and a shared configuration template.</p>
<p>There must be at most one pool in this list for each (cell,type,name) set.
Each shard must have at least one &ldquo;replica&rdquo; pool (in at least one cell)
in order to be able to serve.</p>
</td>
</tr>
<tr>
<td>
<code>databaseInitScriptSecret</code></br>
<em>
<a href="#planetscale.com/v2.SecretSource">
SecretSource
</a>
</em>
</td>
<td>
<p>DatabaseInitScriptSecret specifies the init_db.sql script file to use for this shard.
This SQL script file is executed immediately after bootstrapping an empty database
to set up initial tables and other MySQL-level entities needed by Vitess.</p>
</td>
</tr>
<tr>
<td>
<code>replication</code></br>
<em>
<a href="#planetscale.com/v2.VitessReplicationSpec">
VitessReplicationSpec
</a>
</em>
</td>
<td>
<p>Replication configures Vitess replication settings for the shard.</p>
</td>
</tr>
<tr>
<td>
<code>annotations</code></br>
<em>
map[string]string
</em>
</td>
<td>
<p>Annotations can optionally be used to attach custom annotations to the VitessShard object.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VitessTabletPoolType">VitessTabletPoolType
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessShardTabletPool">VitessShardTabletPool</a>)
</p>
<p>
<p>VitessTabletPoolType represents the tablet types for which it makes sense
to deploy a dedicated pool. Tablet types that indicate temporary or
transient states are not valid pool types.</p>
</p>
<h3 id="planetscale.com/v2.VitessTabletStatus">VitessTabletStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessShardStatus">VitessShardStatus</a>)
</p>
<p>
<p>VitessTabletStatus is the status of one tablet in a shard.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>poolType</code></br>
<em>
string
</em>
</td>
<td>
<p>PoolType is the target tablet type for the tablet pool.</p>
</td>
</tr>
<tr>
<td>
<code>index</code></br>
<em>
int32
</em>
</td>
<td>
<p>Index is the tablet&rsquo;s index within its tablet pool.</p>
</td>
</tr>
<tr>
<td>
<code>running</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#conditionstatus-v1-core">
Kubernetes core/v1.ConditionStatus
</a>
</em>
</td>
<td>
<p>Running indicates whether the vttablet Pod is running.</p>
</td>
</tr>
<tr>
<td>
<code>ready</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#conditionstatus-v1-core">
Kubernetes core/v1.ConditionStatus
</a>
</em>
</td>
<td>
<p>Ready indicates whether the vttablet Pod is passing health checks,
meaning it&rsquo;s ready to serve queries.</p>
</td>
</tr>
<tr>
<td>
<code>available</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#conditionstatus-v1-core">
Kubernetes core/v1.ConditionStatus
</a>
</em>
</td>
<td>
<p>Available indicates whether the vttablet Pod has been consistently Ready
for long enough to be considered stable.</p>
</td>
</tr>
<tr>
<td>
<code>dataVolumeBound</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#conditionstatus-v1-core">
Kubernetes core/v1.ConditionStatus
</a>
</em>
</td>
<td>
<p>DataVolumeBound indicates whether the main PersistentVolumeClaim has been
matched up with a PersistentVolume and bound to it.</p>
</td>
</tr>
<tr>
<td>
<code>type</code></br>
<em>
string
</em>
</td>
<td>
<p>Type is the observed tablet type as reflected in topology.</p>
</td>
</tr>
<tr>
<td>
<code>pendingChanges</code></br>
<em>
string
</em>
</td>
<td>
<p>PendingChanges describes changes to the tablet Pod that will be applied
the next time a rolling update allows.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VtAdminSpec">VtAdminSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessClusterSpec">VitessClusterSpec</a>)
</p>
<p>
<p>VtAdminSpec specifies deployment parameters for vtadmin.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>rbac</code></br>
<em>
<a href="#planetscale.com/v2.SecretSource">
SecretSource
</a>
</em>
</td>
<td>
<p>Rbac contains the rbac config file for vtadmin.
If it is omitted, then it is considered to disable rbac.</p>
</td>
</tr>
<tr>
<td>
<code>cells</code></br>
<em>
[]string
</em>
</td>
<td>
<p>Cells is a list of cell names (as defined in the Cells list)
in which to deploy vtadmin.
Default: Deploy to all defined cells.</p>
</td>
</tr>
<tr>
<td>
<code>apiAddresses</code></br>
<em>
[]string
</em>
</td>
<td>
<p>APIAddresses is a list of vtadmin api addresses
to be used by the vtadmin web for each cell
Either there should be only 1 element in the list
which is used by all the vtadmin-web deployments
or it should match the length of the Cells list</p>
</td>
</tr>
<tr>
<td>
<code>replicas</code></br>
<em>
int32
</em>
</td>
<td>
<p>Replicas is the number of vtadmin instances to deploy in each cell.</p>
</td>
</tr>
<tr>
<td>
<code>webResources</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#resourcerequirements-v1-core">
Kubernetes core/v1.ResourceRequirements
</a>
</em>
</td>
<td>
<p>WebResources determines the compute resources reserved for each vtadmin-web replica.</p>
</td>
</tr>
<tr>
<td>
<code>apiResources</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#resourcerequirements-v1-core">
Kubernetes core/v1.ResourceRequirements
</a>
</em>
</td>
<td>
<p>APIResources determines the compute resources reserved for each vtadmin-api replica.</p>
</td>
</tr>
<tr>
<td>
<code>readOnly</code></br>
<em>
bool
</em>
</td>
<td>
<p>ReadOnly specifies whether the web UI should be read-only
or should it allow users to take actions</p>
<p>Default: false.</p>
</td>
</tr>
<tr>
<td>
<code>extraFlags</code></br>
<em>
map[string]string
</em>
</td>
<td>
<p>ExtraFlags can optionally be used to override default flags set by the
operator, or pass additional flags to vtadmin-api. All entries must be
key-value string pairs of the form &ldquo;flag&rdquo;: &ldquo;value&rdquo;. The flag name should
not have any prefix (just &ldquo;flag&rdquo;, not &ldquo;-flag&rdquo;). To set a boolean flag,
set the string value to either &ldquo;true&rdquo; or &ldquo;false&rdquo;.</p>
</td>
</tr>
<tr>
<td>
<code>extraEnv</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#envvar-v1-core">
[]Kubernetes core/v1.EnvVar
</a>
</em>
</td>
<td>
<p>ExtraEnv can optionally be used to override default environment variables
set by the operator, or pass additional environment variables.</p>
</td>
</tr>
<tr>
<td>
<code>extraVolumes</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#volume-v1-core">
[]Kubernetes core/v1.Volume
</a>
</em>
</td>
<td>
<p>ExtraVolumes can optionally be used to override default Pod volumes
defined by the operator, or provide additional volumes to the Pod.
Note that when adding a new volume, you should usually also add a
volumeMount to specify where in each container&rsquo;s filesystem the volume
should be mounted.</p>
</td>
</tr>
<tr>
<td>
<code>extraVolumeMounts</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#volumemount-v1-core">
[]Kubernetes core/v1.VolumeMount
</a>
</em>
</td>
<td>
<p>ExtraVolumeMounts can optionally be used to override default Pod
volumeMounts defined by the operator, or specify additional mounts.
Typically, these are used to mount volumes defined through extraVolumes.</p>
</td>
</tr>
<tr>
<td>
<code>initContainers</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#container-v1-core">
[]Kubernetes core/v1.Container
</a>
</em>
</td>
<td>
<p>InitContainers can optionally be used to supply extra init containers
that will be run to completion one after another before any app containers are started.</p>
</td>
</tr>
<tr>
<td>
<code>sidecarContainers</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#container-v1-core">
[]Kubernetes core/v1.Container
</a>
</em>
</td>
<td>
<p>SidecarContainers can optionally be used to supply extra containers
that run alongside the main containers.</p>
</td>
</tr>
<tr>
<td>
<code>affinity</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#affinity-v1-core">
Kubernetes core/v1.Affinity
</a>
</em>
</td>
<td>
<p>Affinity allows you to set rules that constrain the scheduling of
your vtadmin pods. WARNING: These affinity rules will override all default affinities
that we set; in turn, we can&rsquo;t guarantee optimal scheduling of your pods if you
choose to set this field.</p>
</td>
</tr>
<tr>
<td>
<code>annotations</code></br>
<em>
map[string]string
</em>
</td>
<td>
<p>Annotations can optionally be used to attach custom annotations to Pods
created for this component. These will be attached to the underlying
Pods that the vtadmin Deployment creates.</p>
</td>
</tr>
<tr>
<td>
<code>extraLabels</code></br>
<em>
map[string]string
</em>
</td>
<td>
<p>ExtraLabels can optionally be used to attach custom labels to Pods
created for this component. These will be attached to the underlying
Pods that the vtadmin Deployment creates.</p>
</td>
</tr>
<tr>
<td>
<code>service</code></br>
<em>
<a href="#planetscale.com/v2.ServiceOverrides">
ServiceOverrides
</a>
</em>
</td>
<td>
<p>Service can optionally be used to customize the vtadmin Service.</p>
</td>
</tr>
<tr>
<td>
<code>tolerations</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#toleration-v1-core">
[]Kubernetes core/v1.Toleration
</a>
</em>
</td>
<td>
<p>Tolerations allow you to schedule pods onto nodes with matching taints.</p>
</td>
</tr>
<tr>
<td>
<code>topologySpreadConstraints</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#topologyspreadconstraint-v1-core">
[]Kubernetes core/v1.TopologySpreadConstraint
</a>
</em>
</td>
<td>
<p>TopologySpreadConstraint can optionally be used to
specify how to spread vtadmin pods among the given topology</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VtadminStatus">VtadminStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessClusterStatus">VitessClusterStatus</a>)
</p>
<p>
<p>VtadminStatus is a summary of the status of the vtadmin deployment.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>available</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#conditionstatus-v1-core">
Kubernetes core/v1.ConditionStatus
</a>
</em>
</td>
<td>
<p>Available indicates whether the vtadmin service has available endpoints.</p>
</td>
</tr>
<tr>
<td>
<code>serviceName</code></br>
<em>
string
</em>
</td>
<td>
<p>ServiceName is the name of the Service for this cluster&rsquo;s vtadmin.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.VttabletSpec">VttabletSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.VitessShardTabletPool">VitessShardTabletPool</a>)
</p>
<p>
<p>VttabletSpec configures the vttablet server within a tablet.</p>
</p>
<table class="table table-striped">
<thead class="thead-dark">
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>resources</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#resourcerequirements-v1-core">
Kubernetes core/v1.ResourceRequirements
</a>
</em>
</td>
<td>
<p>Resources specify the compute resources to allocate for just the vttablet
process (the Vitess query server that sits in front of MySQL).
This field is required.</p>
</td>
</tr>
<tr>
<td>
<code>extraFlags</code></br>
<em>
map[string]string
</em>
</td>
<td>
<p>ExtraFlags can optionally be used to override default flags set by the
operator, or pass additional flags to vttablet. All entries must be
key-value string pairs of the form &ldquo;flag&rdquo;: &ldquo;value&rdquo;. The flag name should
not have any prefix (just &ldquo;flag&rdquo;, not &ldquo;-flag&rdquo;). To set a boolean flag,
set the string value to either &ldquo;true&rdquo; or &ldquo;false&rdquo;.</p>
</td>
</tr>
<tr>
<td>
<code>lifecycle</code></br>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#lifecycle-v1-core">
Kubernetes core/v1.Lifecycle
</a>
</em>
</td>
<td>
<p>Lifecycle can optionally be used to add container lifecycle hooks
to vttablet container</p>
</td>
</tr>
<tr>
<td>
<code>terminationGracePeriodSeconds</code></br>
<em>
int64
</em>
</td>
<td>
<p>TerminationGracePeriodSeconds can optionally be used to customize
terminationGracePeriodSeconds of the vttablet pod.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="planetscale.com/v2.WorkflowState">WorkflowState
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#planetscale.com/v2.ReshardingStatus">ReshardingStatus</a>)
</p>
<p>
<p>WorkflowState represents the current state for the given Workflow.</p>
</p>
<hr/>
<p><em>
Generated with <code>gen-crd-api-reference-docs</code>.
</em></p>
</div>
<script src="https://code.jquery.com/jquery-3.3.1.slim.min.js" integrity="sha384-q8i/X+965DzO0rT7abK41JStQIAqVgRVzpbzo5smXKp4YfRvH+8abtTE1Pi6jizo" crossorigin="anonymous"></script>
<script src="https://cdnjs.cloudflare.com/ajax/libs/popper.js/1.14.7/umd/popper.min.js" integrity="sha384-UO2eT0CpHqdSJQ6hJty5KVphtPhzWj9WO1clHTMGa3JDZwrnQq4sF86dIHNDz0W1" crossorigin="anonymous"></script>
<script src="https://stackpath.bootstrapcdn.com/bootstrap/4.3.1/js/bootstrap.min.js" integrity="sha384-JjSmVgyd0p3pXB1rRibZUAYoIIy6OrQ6VrjIEaFf/nJGzIxFDsf4x0xIM+B07jRM" crossorigin="anonymous"></script>
</body>
</html>
