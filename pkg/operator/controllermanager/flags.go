package controllermanager

import (
	"flag"
	"fmt"
	"os"

	"github.com/spf13/pflag"
	"vitess.io/vitess/go/vt/servenv"

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

	vtbackupFlags := servenv.GetFlagSetFor("vtbackup")
	flagsRequiredByVTop := map[string]bool{
		"backup-storage-implementation":  false,
		"ceph-backup-storage-config":     false,
		"azblob-backup-account-name":     false,
		"azblob-backup-account-key-file": false,
		"azblob-backup-container-name":   false,
		"azblob-backup-storage-root":     false,
		"file-backup-storage-root":       false,
		"gcs-backup-storage-bucket":      false,
		"gcs-backup-storage-root":        false,
		"s3-backup-aws-region":           false,
		"s3-backup-storage-bucket":       false,
		"s3-backup-storage-root":         false,
		"s3-backup-force-path-style":     false,
		"s3-backup-aws-endpoint":         false,
	}

	vtbackupFlags.VisitAll(func(f *pflag.Flag) {
		_, isRequired := flagsRequiredByVTop[f.Name]
		if isRequired {
			flagsRequiredByVTop[f.Name] = true
			pflag.CommandLine.AddFlag(f)
		}
	})

	for flagName, wasAdded := range flagsRequiredByVTop {
		if !wasAdded {
			fmt.Fprintf(os.Stderr, "unable to add the flag - %s\n", flagName)
			os.Exit(1)
		}
	}

	// expose flags to configure TLS communication to topology service and vttablets
	// vttablet set contains all required flags
	vttabletFlags := servenv.GetFlagSetFor("vttablet")
	tlsFlags := map[string]bool{
		"tablet-manager-grpc-ca":          false,
		"tablet-manager-grpc-server-name": false,
		"topo-global-server-address":      false,
		"topo-etcd-tls-ca":                false,
		"topo-etcd-tls-cert":              false,
		"topo-etcd-tls-key":               false,
	}
	vttabletFlags.VisitAll(func(f *pflag.Flag) {
		_, isRequired := tlsFlags[f.Name]
		if isRequired {
			isAlreadyAdded := pflag.CommandLine.Lookup(f.Name) != nil
			if !isAlreadyAdded {
				pflag.CommandLine.AddFlag(f)
				tlsFlags[f.Name] = true
			}
		}
	})
	for flagName, wasAdded := range tlsFlags {
		if !wasAdded {
			fmt.Fprintf(os.Stderr, "unable to add the flag - %s\n", flagName)
		}
	}

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
