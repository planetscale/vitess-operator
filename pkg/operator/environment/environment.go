/*
Copyright 2020 PlanetScale Inc.

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

/*
Package environment defines global environment variables to call throughout the operator codebase. These variables
have sane defaults and aren't required to be set as flags unless explicitly stated.
*/

package environment

import (
	"time"

	"github.com/spf13/pflag"
	"vitess.io/vitess/go/vt/vtenv"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
)

var (
	reconcileTimeout   time.Duration
	MySQLServerVersion = "8.0.40-Vitess"
	// truncateUILen truncate queries in debug UIs to the given length. 0 means unlimited.
	truncateUILen = 512
	// truncateErrLen truncate queries in error logs to the given length. 0 means unlimited.
	truncateErrLen = 0
)

// FlagSet returns the FlagSet for the operator.
func FlagSet() *pflag.FlagSet {
	operatorFlagSet := pflag.NewFlagSet("operator", pflag.ExitOnError)

	operatorFlagSet.DurationVar(&reconcileTimeout, "reconcile_timeout", 10*time.Minute, "Maximum time that any controller will spend trying to reconcile a single object before giving up.")

	operatorFlagSet.StringVar(&planetscalev2.DefaultVitessPriorityClass, "default_vitess_priority_class", planetscalev2.DefaultVitessPriorityClass, "Default PriorityClass to use for Pods that run Vitess components. An empty value means don't use any PriorityClass.")
	operatorFlagSet.StringVar(&planetscalev2.DefaultVitessServiceAccount, "default_vitess_service_account", planetscalev2.DefaultVitessServiceAccount, "Default ServiceAccount to use for Pods that run Vitess components. An empty value means let Kubernetes fill in a default.")
	operatorFlagSet.Int64Var(&planetscalev2.DefaultVitessRunAsUser, "default_vitess_run_as_user", planetscalev2.DefaultVitessRunAsUser, "Default UID to use for Pods that run Vitess components. A value less than 0 means don't set runAsUser at all.")
	operatorFlagSet.Int64Var(&planetscalev2.DefaultVitessFSGroup, "default_vitess_fs_group", planetscalev2.DefaultVitessFSGroup, "Default GID to use for Pods that run Vitess components. A value less than 0 means don't set fsGroup at all.")

	operatorFlagSet.StringVar(&planetscalev2.DefaultEtcdServiceAccount, "default_etcd_service_account", planetscalev2.DefaultEtcdServiceAccount, "Default ServiceAccount to use for etcd Pods. An empty value means let Kubernetes fill in a default.")
	operatorFlagSet.Int64Var(&planetscalev2.DefaultEtcdRunAsUser, "default_etcd_run_as_user", planetscalev2.DefaultEtcdRunAsUser, "Default UID to use for etcd Pods. A value less than 0 means don't set runAsUser at all.")
	operatorFlagSet.Int64Var(&planetscalev2.DefaultEtcdFSGroup, "default_etcd_fs_group", planetscalev2.DefaultEtcdFSGroup, "Default GID to use for etcd Pods. A value less than 0 means don't set fsGroup at all.")

	operatorFlagSet.StringVar(&planetscalev2.DefaultEtcdImage, "default_etcd_image", planetscalev2.DefaultEtcdImage, "Default etcd image to use when not specified in the CRD.")
	operatorFlagSet.StringVar(&planetscalev2.DefaultImages.MysqldExporter, "default_mysqld_exporter_image", planetscalev2.DefaultImages.MysqldExporter, "Default mysqld-exporter image to use when not specified in the CRD.")

	return operatorFlagSet
}

// ReconcileTimeout returns the global maximum reconcile timeout for all controllers.
func ReconcileTimeout() time.Duration {
	return reconcileTimeout
}

// VtEnvironment gets the vitess environment to be used in the operator.
func VtEnvironment() (*vtenv.Environment, error) {
	return vtenv.New(vtenv.Options{
		MySQLServerVersion: MySQLServerVersion,
		TruncateUILen:      truncateUILen,
		TruncateErrLen:     truncateErrLen,
	})
}
