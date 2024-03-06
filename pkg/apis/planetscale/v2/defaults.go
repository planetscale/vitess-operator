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

package v2

/*
All hard-coded default values for configurable aspects of objects in our API should go here.
However, hard-coded values that are not (yet) configurable in the API can live with the code.

These values are materialized into the object before processing begins.
This prevents defaults from being scattered throughout the processing code,
and in the future will allow us to associate defaults with specific API versions.

***COMPATIBILITY RULES***

* DO NOT change existing default values.
* DO NOT make these API-level default values configurable (e.g. by command-line flag).
* DO NOT add defaults that are only relevant to a specific user of the CRD.

New default values may only be added as new fields are added to the API Version (planetscale.com/v2),
and defaults for existing fields may only be changed along with the introduction of a new API Version
(e.g. `apiVersion: planetscale.com/v3`, but see below for a caveat). The defaults for that new API Version
will live in a different file in a different package (pkg/apis/planetscale/v3).

This rule, among other compatibility guarantees expected of all Kubernetes-style APIS, provides a contract
that makes it safe for users to upgrade without worrying about their configuration being changed silently
and in unpredictable ways.

CAVEAT: We CANNOT yet support adding a new API Version (e.g. planetscale.com/v3).

Currently, we only fill in these defaults in-memory, each time the controller processes an object.
Before we can add planetscale.com/v3, we'll need to do the following:

* Write a mutating admission webhook to materialize the v2 defaults into the actual data stored
  in the Kubernetes API server. This prevents the object's configuration from changing when the
  controller is updated to use v3.
* Write a CRD conversion webhook to translate objects back and forth between v2 and v3.
*/

const (
	// Mi is the scale factor for Mebi (2**20)
	Mi = 1 << 20
	// Gi is the scale factor for Gibi (2**30)
	Gi = 1 << 30

	defaultEtcdStorageRequestBytes = 1 * Gi
	defaultEtcdCPUMillis           = 100
	defaultEtcdMemoryBytes         = 256 * Mi
	defaultEtcdCreatePDB           = true
	defaultEtcdCreateClientService = true
	defaultEtcdCreatePeerService   = true

	defaultVtctldReplicas    = 1
	defaultVtctldCPUMillis   = 100
	defaultVtctldMemoryBytes = 128 * Mi

	defaultVtadminReplicas    = 1
	defaultVtadminCPUMillis   = 100
	defaultVtadminMemoryBytes = 128 * Mi

	defaultVtorcReplicas    = 0
	defaultVtorcCPUMillis   = 100
	defaultVtorcMemoryBytes = 128 * Mi

	defaultVtgateReplicas    = 2
	defaultVtgateCPUMillis   = 500
	defaultVtgateMemoryBytes = 1 * Gi

	defaultBackupIntervalHours     = 24
	defaultBackupMinRetentionHours = 72
	defaultBackupMinRetentionCount = 1
	defaultBackupEngine            = VitessBackupEngineBuiltIn

	// DefaultWebPort is the port for debug status pages and dashboard UIs.
	DefaultWebPort = 15000
	// DefaultAPIPort is the port for API endpoint.
	DefaultAPIPort = 15001
	// DefaultGrpcPort is the port for RPCs.
	DefaultGrpcPort = 15999
	// DefaultMysqlPort is the port for MySQL client connections.
	DefaultMysqlPort = 3306

	// DefaultWebPortName is the name for the web port.
	DefaultWebPortName = "web"
	// DefaultAPIPortName is the name for the api port.
	DefaultAPIPortName = "api"
	// DefaultGrpcPortName is the name for the RPC port.
	DefaultGrpcPortName = "grpc"
	// DefaultMysqlPortName is the name for the MySQL port.
	DefaultMysqlPortName = "mysql"

	defaultVitessLiteImage = "vitess/lite:v19.0.0""

	DefaultInitCPURequestMillis   = 100
	DefaultInitMemoryRequestBytes = 32 * (1 << 20) // 32 MiB
)

// DefaultImages are a set of images to use when the CRD doesn't specify.
var DefaultImages = &VitessImages{
	Vtctld:   defaultVitessLiteImage,
	Vtgate:   defaultVitessLiteImage,
	Vttablet: defaultVitessLiteImage,

	// Note: The vtbackup image is only used for the vtbackup binary itself,
	// which is copied over during Pod initialization. The vtbackup container
	// actually runs with the mysqld image specified below. This mirrors the way
	// that the mysqld container in a vttablet Pod runs the mysqld image with a
	// mysqlctld binary injected (copied from the vttablet image) at Pod
	// initialization, since vtbackup is effectively a modified mysqlctl(d).
	Vtbackup: defaultVitessLiteImage,

	Vtorc: defaultVitessLiteImage,

	// Note: We used to use a mysql-only image, but it's better to use the
	// same image as the vttablet container since vttablet now uses the
	// local mysqld binary in its container to do version detection, so the
	// binary needs to match what we use in the mysqld container. Using the
	// Vitess image rather than a mysql-only image also means the vtbackup Pod
	// has easy access to other programs we include in the Vitess image, like
	// Percona XtraBackup.
	Mysqld: &MysqldImage{
		Mysql56Compatible: defaultVitessLiteImage,
	},

	MysqldExporter: "prom/mysqld-exporter:v0.11.0",
}

var (
	// DefaultVitessPriorityClass is the name of the PriorityClass to use by
	// default for Pods that run Vitess components. This value can be configured
	// at operator startup time with the --default_vitess_priority_class flag.
	DefaultVitessPriorityClass = "vitess"

	// DefaultVitessServiceAccount is the name of the ServiceAccount to use by
	// default for Pods that run Vitess components. This value can be configured
	// at operator startup time with the --default_vitess_service_account flag.
	DefaultVitessServiceAccount = ""

	// DefaultVitessRunAsUser is the UID to use by default for Pods that run
	// Vitess components. This value can be configured at operator startup time
	// with the --default_vitess_run_as_user flag. A value less than 0 means
	// don't set RunAsUser at all.
	DefaultVitessRunAsUser int64 = 999

	// DefaultVitessFSGroup is the GID to use by default for Pods that run
	// Vitess components. This value can be configured at operator startup time
	// with the --default_vitess_fs_group flag. A value less than 0 means don't
	// set FSGroup at all.
	DefaultVitessFSGroup int64 = 999

	// DefaultEtcdServiceAccount is the name of the ServiceAccount to use by
	// default for etcd Pods. This value can be configured at operator startup
	// time with the --default_etcd_service_account flag.
	DefaultEtcdServiceAccount = ""

	// DefaultEtcdRunAsUser is the UID to use by default for etcd Pods.
	// This value can be configured at operator startup time with the
	// --default_etcd_run_as_user flag. A value less than 0 means don't set
	// RunAsUser at all.
	DefaultEtcdRunAsUser int64 = -1

	// DefaultEtcdFSGroup is the GID to use by default for etcd Pods.
	// This value can be configured at operator startup time with the
	// --default_etcd_fs_group flag. A value less than 0 means don't set FSGroup
	// at all.
	DefaultEtcdFSGroup int64 = -1

	// DefaultEtcdImage is the image to use for etcd when the CRD doesn't specify.
	// This value can be configured at operator startup time with the
	// --default_etcd_image flag.
	DefaultEtcdImage = "quay.io/coreos/etcd:v3.5.9"
)
