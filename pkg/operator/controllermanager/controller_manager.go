package controllermanager

import (
	"fmt"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"

	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"planetscale.dev/vitess-operator/pkg/controller"
	vbssubcontroller "planetscale.dev/vitess-operator/pkg/controller/vitessbackupstorage/subcontroller"
)

var log = logf.Log.WithName("controller-manager")

func New(forkPath string, cfg *rest.Config, opts manager.Options) (manager.Manager, error) {
	// Set up scheme for all resources we depend on.
	var err error
	opts.Scheme, err = NewScheme()
	if err != nil {
		return nil, err
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
