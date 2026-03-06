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

/*
Package subcontroller is a part of the VitessBackupStorage controller that runs
as its own Pod, separate from the operator's main controller-manager.

You can think of this like a part of the VitessBackupStorage controller that's
forked off into a new Pod for each VitessBackupStorage object, so each Pod can
independently mount things like Secrets or NFS volumes that are specific to a
given VitessBackupStorage object.

See cmd/manager/main.go for details.
*/
package subcontroller

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/sirupsen/logrus"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/metrics"
	"planetscale.dev/vitess-operator/pkg/operator/reconciler"
	"planetscale.dev/vitess-operator/pkg/operator/results"
	"planetscale.dev/vitess-operator/pkg/operator/resync"
)

const (
	controllerName = "vitessbackupstorage-subcontroller"

	// ForkPath is the fork path for running just this subcontroller.
	// See cmd/manager/main.go for details.
	ForkPath = controllerName

	VBSNamespaceEnvVar = "PS_OPERATOR_VBS_NAMESPACE"
	VBSNameEnvVar      = "PS_OPERATOR_VBS_NAME"
)

var (
	resyncPeriod     = flag.Duration("vitessbackupstorage_subcontroller_resync_period", 60*time.Second, "reconcile each vitessbackupstorage with this period even if no Kubernetes events occur")
	reconcileTimeout = flag.Duration("vitessbackupstorage_subcontroller_reconcile_timeout", 10*time.Minute, "timeout for a single reconcile pass of the vitessbackupstorage subcontroller")
	requestTimeout   = flag.Duration("vitessbackupstorage_subcontroller_request_timeout", 10*time.Second, "timeout for a single request by the vitessbackupstorage subcontroller to read the status of a backup")
)

var log = logrus.WithField("subcontroller", "VitessBackupStorage")

// watchResources should contain all the resource types that this controller creates.
var watchResources = []client.Object{
	&planetscalev2.VitessBackup{},
}

// Add creates a new subcontroller and adds it to the Manager.
//
// Note that this Add function is intentionally NOT registered in the top-level
// pkg/controller package, because this controller does not run in the root
// process. See cmd/manager/main.go for details.
func Add(mgr manager.Manager) error {
	r, err := newReconciler(mgr)
	if err != nil {
		return err
	}
	return add(mgr, r)
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) (*ReconcileVitessBackupStorage, error) {
	// This subcontroller runs in a forked subprocess and only processes one object.
	var key client.ObjectKey
	key.Namespace = os.Getenv(VBSNamespaceEnvVar)
	if key.Namespace == "" {
		return nil, fmt.Errorf("vitessbackupstorage subcontroller requires %v env var to be set", VBSNamespaceEnvVar)
	}
	key.Name = os.Getenv(VBSNameEnvVar)
	if key.Name == "" {
		return nil, fmt.Errorf("vitessbackupstorage subcontroller requires %v env var to be set", VBSNameEnvVar)
	}

	c := mgr.GetClient()
	scheme := mgr.GetScheme()
	recorder := mgr.GetEventRecorderFor(controllerName)

	return &ReconcileVitessBackupStorage{
		client:     c,
		scheme:     scheme,
		resync:     resync.NewPeriodic(controllerName, *resyncPeriod),
		recorder:   recorder,
		reconciler: reconciler.New(c, scheme, recorder),
		objectKey:  key,
	}, nil
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r *ReconcileVitessBackupStorage) error {
	// Create a new controller
	c, err := controller.New(controllerName, mgr,
		controller.Options{
			Reconciler: r,
		})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource VitessBackupStorage
	if err := c.Watch(source.Kind(mgr.GetCache(), &planetscalev2.VitessBackupStorage{}, &handler.TypedEnqueueRequestForObject[*planetscalev2.VitessBackupStorage]{})); err != nil {
		return err
	}

	// Watch for changes to secondary resources and requeue the owner VitessBackupStorage.
	for _, resource := range watchResources {
		err := c.Watch(source.Kind(mgr.GetCache(), resource, handler.EnqueueRequestForOwner(
			mgr.GetScheme(),
			mgr.GetRESTMapper(),
			&planetscalev2.VitessBackupStorage{},
			handler.OnlyControllerOwner(),
		)))
		if err != nil {
			return err
		}
	}

	// Periodically resync even when no Kubernetes events have come in.
	if err := c.Watch(r.resync.WatchSource()); err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileVitessBackupStorage{}

// ReconcileVitessBackupStorage reconciles a VitessBackupStorage object
type ReconcileVitessBackupStorage struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client     client.Client
	scheme     *runtime.Scheme
	resync     *resync.Periodic
	recorder   record.EventRecorder
	reconciler *reconciler.Reconciler
	objectKey  client.ObjectKey
}

// Reconcile reads that state of the cluster for a VitessBackupStorage object and makes changes based on the state read
// and what is in the VitessBackupStorage.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileVitessBackupStorage) Reconcile(cctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	ctx, cancel := context.WithTimeout(cctx, *reconcileTimeout)
	defer cancel()
	resultBuilder := &results.Builder{}

	// Ignore everything except the one object we care about.
	if request.NamespacedName != r.objectKey {
		return resultBuilder.Result()
	}

	log := log.WithFields(logrus.Fields{
		"namespace":           request.Namespace,
		"vitessbackupstorage": request.Name,
	})
	log.Info("Reconciling VitessBackupStorage")

	// Fetch the VitessBackupStorage instance.
	vbs := &planetscalev2.VitessBackupStorage{}
	err := r.client.Get(ctx, request.NamespacedName, vbs)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return resultBuilder.Result()
		}
		// Error reading the object - requeue the request.
		return resultBuilder.Error(err)
	}

	// Reset status, since that's all out of date info that we will recompute now.
	oldStatus := vbs.Status
	vbs.Status = *planetscalev2.NewVitessBackupStorageStatus()

	resultBuilder.Merge(r.reconcileBackups(ctx, vbs))

	// Update status if needed.
	vbs.Status.ObservedGeneration = vbs.Generation
	if !apiequality.Semantic.DeepEqual(&vbs.Status, &oldStatus) {
		if err := r.client.Status().Update(ctx, vbs); err != nil {
			if !apierrors.IsConflict(err) {
				r.recorder.Eventf(vbs, corev1.EventTypeWarning, "StatusUpdateFailed", "failed to update status: %v", err)
			}
			resultBuilder.Error(err)
		}
	}

	// Request a periodic resync to relist the backups.
	r.resync.Enqueue(request.NamespacedName)

	result, err := resultBuilder.Result()
	reconcileCount.WithLabelValues(vbs.Name, metrics.Result(err)).Inc()
	return result, err
}
