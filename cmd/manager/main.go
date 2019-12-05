/*
Copyright 2019 PlanetScale.

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

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	goruntime "runtime"
	"time"

	"k8s.io/klog"

	"github.com/spf13/pflag"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	appsv1defaults "k8s.io/kubernetes/pkg/apis/apps/v1"
	batchv1defaults "k8s.io/kubernetes/pkg/apis/batch/v1"
	corev1defaults "k8s.io/kubernetes/pkg/apis/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"

	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	"github.com/operator-framework/operator-sdk/pkg/leader"
	"github.com/operator-framework/operator-sdk/pkg/log/zap"
	sdkVersion "github.com/operator-framework/operator-sdk/version"

	planetscaleapis "planetscale.dev/vitess-operator/pkg/apis"
	"planetscale.dev/vitess-operator/pkg/controller"
	vbssubcontroller "planetscale.dev/vitess-operator/pkg/controller/vitessbackupstorage/subcontroller"
	"planetscale.dev/vitess-operator/pkg/operator/fork"
)

var (
	statusMetricsPollInterval = flag.Duration("status_metrics_poll_interval", 30*time.Second, "Interval at which to update status metrics")
	cacheInvalidateInterval   = flag.Duration("cache_invalidate_interval", 10*time.Minute, "Interval at which to invalidate the local cache and relist objects from the API server")
)

// Change below variables to serve metrics on different host or port.
var (
	metricsHost       = "0.0.0.0"
	metricsPort int32 = 8383
)
var log = logf.Log.WithName("cmd")

// schemeAddFuncs are all the things we register into our compiled-in API type system (Scheme).
var schemeAddFuncs = []func(*runtime.Scheme) error{
	// This is the types only (no defaulters) for all built-in APIs supported by client-go.
	kubernetesscheme.AddToScheme,

	/*
		These are the defaulters for some built-in APIs that we use.

		This code doesn't come with the Kubernetes client library,
		so we have to vendor it from the main server repository.

		Doing this lets us achieve `kubectl apply`-like semantics while
		still using statically-typed Go structs instead of JSON/YAML
		to define our objects.
	*/
	corev1defaults.AddToScheme,
	appsv1defaults.AddToScheme,
	batchv1defaults.AddToScheme,

	// This is our own CRDs.
	planetscaleapis.AddToScheme,
}

func printVersion() {
	log.Info(fmt.Sprintf("Go Version: %s", goruntime.Version()))
	log.Info(fmt.Sprintf("Go OS/Arch: %s/%s", goruntime.GOOS, goruntime.GOARCH))
	log.Info(fmt.Sprintf("Version of operator-sdk: %v", sdkVersion.Version))
}

func main() {
	// forkPath is non-empty if this is a forked sub-process that's supposed to
	// do something other than run the main operator code.
	forkPath := fork.Path()

	// Add the zap logger flag set to the CLI. The flag set must
	// be added before calling pflag.Parse().
	pflag.CommandLine.AddFlagSet(zap.FlagSet())

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
	logf.SetLogger(zap.Logger())

	printVersion()

	namespace, err := k8sutil.GetWatchNamespace()
	if err != nil {
		log.Error(err, "Failed to get watch namespace")
		os.Exit(1)
	}

	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	ctx := context.TODO()

	// Become the leader before proceeding if this is the root process.
	// Child processes use deterministic Pod names instead of leader election.
	if forkPath == "" {
		err = leader.Become(ctx, "vitess-operator-lock")
		if err != nil {
			log.Error(err, "")
			os.Exit(1)
		}
	}

	// Set up scheme for all resources we depend on.
	scheme := runtime.NewScheme()
	for _, add := range schemeAddFuncs {
		if err := add(scheme); err != nil {
			log.Error(err, "")
			os.Exit(1)
		}
	}

	// Create a new Cmd to provide shared dependencies and start components
	mgr, err := manager.New(cfg, manager.Options{
		Scheme:             scheme,
		Namespace:          namespace,
		SyncPeriod:         cacheInvalidateInterval,
		MetricsBindAddress: fmt.Sprintf("%s:%d", metricsHost, metricsPort),
	})
	if err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	log.Info("Registering Components.")

	// We use the fork path primarily to decide which controllers to run in this
	// manager process. Not all controllers run in the root process, for example.
	switch forkPath {
	case "":
		// Run all root controllers, defined as anything that registers itself
		// in the top-level 'pkg/controller' at package init time.
		if err := controller.AddToManager(mgr); err != nil {
			log.Error(err, "")
			os.Exit(1)
		}
	case vbssubcontroller.ForkPath:
		// Run only the vitessbackupstorage subcontroller.
		if err := vbssubcontroller.Add(mgr); err != nil {
			log.Error(err, "")
			os.Exit(1)
		}
	default:
		log.Error(fmt.Errorf("undefined fork path: %v", forkPath), "")
		os.Exit(1)
	}

	log.Info("Starting the Cmd.")

	// Start the Cmd
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		log.Error(err, "Manager exited non-zero")
		os.Exit(1)
	}
}
