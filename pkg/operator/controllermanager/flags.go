package controllermanager

import (
	"flag"

	"github.com/spf13/pflag"

	"k8s.io/klog"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"planetscale.dev/vitess-operator/pkg/operator/environment"
)

func InitFlags() {
	// Add the zap logger flag set to the CLI. The flag set must
	// be added before calling pflag.Parse().
	var zapFlagSet flag.FlagSet
	zapOpts := zap.Options{}
	zapOpts.BindFlags(&zapFlagSet)
	pflag.CommandLine.AddGoFlagSet(&zapFlagSet)

	// Add the operator flag set to the CLI.
	pflag.CommandLine.AddFlagSet(environment.FlagSet())

	// Add flags registered by imported packages (e.g. glog and
	// controller-runtime)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)

	pflag.Parse()

	// Initialize flags for klog, which is necessary to configure logging from
	// the low-level k8s client libraries. We don't use glog ourselves, but we
	// have dependencies that use it, so we have to follow the instructions for
	// making klog coexist with glog:
	// https://github.com/kubernetes/klog/blob/master/examples/coexist_glog/coexist_glog.go
	klogFlags := flag.NewFlagSet("klog", flag.ExitOnError)
	klog.InitFlags(klogFlags)
	// Sync the glog and klog flags.
	pflag.CommandLine.VisitAll(func(f1 *pflag.Flag) {
		f2 := klogFlags.Lookup(f1.Name)
		if f2 != nil {
			value := f1.Value.String()
			f2.Value.Set(value)
		}
	})

	// Use a zap logr.Logger implementation. If none of the zap
	// flags are configured (or if the zap flag set is not being
	// used), this defaults to a production zap logger.
	//
	// The logger instantiated here can be changed to any logger
	// implementing the logr.Logger interface. This logger will
	// be propagated through the whole operator, generating
	// uniform and structured logs.
	logf.SetLogger(zap.New(zap.UseFlagOptions(&zapOpts)))
}
