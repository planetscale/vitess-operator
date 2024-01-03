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

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	goruntime "runtime"
	"strings"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	"github.com/planetscale/operator-sdk-libs/pkg/k8sutil"
	"github.com/planetscale/operator-sdk-libs/pkg/leader"

	"planetscale.dev/vitess-operator/pkg/operator/controllermanager"
	"planetscale.dev/vitess-operator/pkg/operator/fork"
	"planetscale.dev/vitess-operator/version"
)

var (
	cacheInvalidateInterval = flag.Duration("cache_invalidate_interval", 10*time.Minute, "Interval at which to invalidate the local cache and relist objects from the API server")
)

// Change below variables to serve metrics on different host or port.
var (
	metricsHost       = "0.0.0.0"
	metricsPort int32 = 8383
)
var log = logf.Log.WithName("manager")

func printVersion() {
	log.Info(fmt.Sprintf("Operator Version: %s", version.Version))
	log.Info(fmt.Sprintf("Go Version: %s", goruntime.Version()))
	log.Info(fmt.Sprintf("Go OS/Arch: %s/%s", goruntime.GOOS, goruntime.GOARCH))
}

func main() {
	// forkPath is non-empty if this is a forked sub-process that's supposed to
	// do something other than run the main operator code.
	forkPath := fork.Path()

	controllermanager.InitFlags()

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

	options := manager.Options{
		Cache: cache.Options{
			SyncPeriod: cacheInvalidateInterval,
		},
		Metrics: server.Options{
			BindAddress: fmt.Sprintf("%s:%d", metricsHost, metricsPort),
		},
	}

	if namespace != "" {
		cacheConfigMap := map[string]cache.Config{}
		for _, ns := range strings.Split(namespace, ",") {
			cacheConfigMap[ns] = cache.Config{}
		}
		options.Cache.DefaultNamespaces = cacheConfigMap
	}
	mgr, err := controllermanager.New(forkPath, cfg, options)
	if err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	log.Info("Starting the manager.")

	// Start the manager
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		log.Error(err, "Manager exited non-zero")
		os.Exit(1)
	}
}
