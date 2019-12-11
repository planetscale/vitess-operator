package controllermanager

import (
	"fmt"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"

	"k8s.io/apimachinery/pkg/runtime"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	appsv1defaults "k8s.io/kubernetes/pkg/apis/apps/v1"
	batchv1defaults "k8s.io/kubernetes/pkg/apis/batch/v1"
	corev1defaults "k8s.io/kubernetes/pkg/apis/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	planetscaleapis "planetscale.dev/vitess-operator/pkg/apis"
	"planetscale.dev/vitess-operator/pkg/controller"
	vbssubcontroller "planetscale.dev/vitess-operator/pkg/controller/vitessbackupstorage/subcontroller"
)

var log = logf.Log.WithName("controller-manager")

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

func New(forkPath string, cfg *rest.Config, opts manager.Options) (manager.Manager, error) {
	// Set up scheme for all resources we depend on.
	opts.Scheme = runtime.NewScheme()
	for _, add := range schemeAddFuncs {
		if err := add(opts.Scheme); err != nil {
			return nil, err
		}
	}

	// Create a new manager to provide shared dependencies and start components
	mgr, err := manager.New(cfg, opts)
	if err != nil {
		return nil, err
	}

	log.Info("Registering Components.")

	// We use the fork path primarily to decide which controllers to run in this
	// manager process. Not all controllers run in the root process, for example.
	switch forkPath {
	case "":
		// Run all root controllers, defined as anything that registers itself
		// in the top-level 'pkg/controller' at package init time.
		if err := controller.AddToManager(mgr); err != nil {
			return nil, err
		}
	case vbssubcontroller.ForkPath:
		// Run only the vitessbackupstorage subcontroller.
		if err := vbssubcontroller.Add(mgr); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("undefined fork path: %v", forkPath)
	}

	return mgr, nil
}
