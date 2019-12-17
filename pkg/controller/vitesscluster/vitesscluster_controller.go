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

package vitesscluster

import (
	"context"
	"flag"

	"github.com/sirupsen/logrus"

	appsv1 "k8s.io/api/apps/v1"
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
)

var (
	maxConcurrentReconciles = flag.Int("vitesscluster_concurrent_reconciles", 10, "the maximum number of different VitessClusters to reconcile concurrently")
)

var log = logrus.WithField("controller", "VitessCluster")

// watchResources should contain all the resource types that this controller creates.
var watchResources = []runtime.Object{
	&corev1.Service{},
	&appsv1.Deployment{},

	&planetscalev2.VitessCell{},
	&planetscalev2.VitessKeyspace{},
	&planetscalev2.VitessBackupStorage{},
	&planetscalev2.EtcdLockserver{},
}

// Add creates a new VitessCluster Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	c := mgr.GetClient()
	scheme := mgr.GetScheme()
	recorder := mgr.GetRecorder("vitesscluster-controller")

	return &ReconcileVitessCluster{
		client:     c,
		scheme:     scheme,
		recorder:   recorder,
		reconciler: reconciler.New(c, scheme, recorder),
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("vitesscluster-controller", mgr, controller.Options{
		Reconciler:              r,
		MaxConcurrentReconciles: *maxConcurrentReconciles,
	})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource VitessCluster
	if err := c.Watch(&source.Kind{Type: &planetscalev2.VitessCluster{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return err
	}

	// Watch for changes to secondary resources and requeue the owner VitessCluster.
	for _, resource := range watchResources {
		err := c.Watch(&source.Kind{Type: resource}, &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &planetscalev2.VitessCluster{},
		})
		if err != nil {
			return err
		}
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileVitessCluster{}

// ReconcileVitessCluster reconciles a VitessCluster object
type ReconcileVitessCluster struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client     client.Client
	scheme     *runtime.Scheme
	recorder   record.EventRecorder
	reconciler *reconciler.Reconciler
}

// Reconcile reads that state of the cluster for a VitessCluster object and makes changes based on the state read
// and what is in the VitessCluster.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileVitessCluster) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	ctx := context.TODO()
	resultBuilder := &results.Builder{}
	log := log.WithFields(logrus.Fields{
		"namespace":     request.Namespace,
		"VitessCluster": request.Name,
	})
	log.Info("Reconciling VitessCluster")

	// Fetch the VitessCluster instance.
	vt := &planetscalev2.VitessCluster{}
	err := r.client.Get(ctx, request.NamespacedName, vt)
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
	oldStatus := vt.Status
	vt.Status = planetscalev2.NewVitessClusterStatus()

	// Materialize all hard-coded default values into the object.
	// TODO(enisoc): Use versioned defaults when operator-sdk supports mutating webhooks.
	planetscalev2.DefaultVitessCluster(vt)

	// Create/update global etcd, if requested.
	if err := r.reconcileGlobalEtcd(ctx, vt); err != nil {
		// Record result but continue to reconcile cells.
		resultBuilder.Error(err)
	}

	// Create/update VitessBackupStorage objects.
	if err := r.reconcileBackupStorage(ctx, vt); err != nil {
		resultBuilder.Error(err)
	}

	// Create/update desired VitessCells.
	if err := r.reconcileCells(ctx, vt); err != nil {
		resultBuilder.Error(err)
	}

	// Create/update desired VitessKeyspaces.
	if err := r.reconcileKeyspaces(ctx, vt); err != nil {
		resultBuilder.Error(err)
	}

	// Create/update vtgate service.
	vtgateResult, err := r.reconcileVtgate(ctx, vt)
	resultBuilder.Merge(vtgateResult, err)

	// Create/update vttablet service.
	vttabletResult, err := r.reconcileVttablet(ctx, vt)
	resultBuilder.Merge(vttabletResult, err)

	// Create/update vtctld deployments.
	vtctldResult, err := r.reconcileVtctld(ctx, vt)
	resultBuilder.Merge(vtctldResult, err)

	// Create/update Vitess topology records for cells as needed.
	topoResult, err := r.reconcileTopology(ctx, vt)
	resultBuilder.Merge(topoResult, err)

	// Update status if needed.
	vt.Status.ObservedGeneration = vt.Generation
	if !apiequality.Semantic.DeepEqual(&vt.Status, &oldStatus) {
		if err := r.client.Status().Update(ctx, vt); err != nil {
			if !apierrors.IsConflict(err) {
				r.recorder.Eventf(vt, corev1.EventTypeWarning, "StatusUpdateFailed", "failed to update status: %v", err)
			}
			resultBuilder.Error(err)
		}
	}

	result, err := resultBuilder.Result()
	reconcileCount.WithLabelValues(vt.Name, metrics.Result(err)).Inc()
	return result, err
}
