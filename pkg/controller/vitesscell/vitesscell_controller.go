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

package vitesscell

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
	maxConcurrentReconciles = flag.Int("vitesscell_concurrent_reconciles", 10, "the maximum number of different vitesscells to reconcile concurrently")
)

var log = logrus.WithField("controller", "VitessCell")

// watchResources should contain all the resource types that this controller creates.
var watchResources = []runtime.Object{
	&corev1.Service{},
	&appsv1.Deployment{},

	&planetscalev2.EtcdLockserver{},
}

// Add creates a new VitessCell Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	c := mgr.GetClient()
	scheme := mgr.GetScheme()
	recorder := mgr.GetRecorder("vitesscell-controller")

	return &ReconcileVitessCell{
		client:     c,
		scheme:     scheme,
		recorder:   recorder,
		reconciler: reconciler.New(c, scheme, recorder),
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("vitesscell-controller", mgr, controller.Options{
		Reconciler:              r,
		MaxConcurrentReconciles: *maxConcurrentReconciles,
	})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource VitessCell
	err = c.Watch(&source.Kind{Type: &planetscalev2.VitessCell{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resources and requeue the owner VitessCell.
	for _, resource := range watchResources {
		err := c.Watch(&source.Kind{Type: resource}, &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &planetscalev2.VitessCell{},
		})
		if err != nil {
			return err
		}
	}

	// Watch for changes in VitessKeyspaces, which we don't own, and requeue associated VitessCells.
	err = c.Watch(&source.Kind{Type: &planetscalev2.VitessKeyspace{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: &keyspaceCellsMapper{},
	})
	if err != nil {
		return err
	}

	// Watch for changes in Secrets, which we don't own, and requeue associated VitessCells.
	err = c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: &secretCellsMapper{
			client: mgr.GetClient(),
		},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileVitessCell{}

// ReconcileVitessCell reconciles a VitessCell object
type ReconcileVitessCell struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client     client.Client
	scheme     *runtime.Scheme
	recorder   record.EventRecorder
	reconciler *reconciler.Reconciler
}

// Reconcile reads that state of the cluster for a VitessCell object and makes changes based on the state read
// and what is in the VitessCell.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileVitessCell) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	ctx := context.TODO()
	resultBuilder := &results.Builder{}

	log := log.WithFields(logrus.Fields{
		"namespace":  request.Namespace,
		"vitesscell": request.Name,
	})
	log.Info("Reconciling VitessCell")

	// Fetch the VitessCell instance
	vtc := &planetscalev2.VitessCell{}
	err := r.client.Get(ctx, request.NamespacedName, vtc)
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

	// Reset status so it's all based on the latest observed state.
	oldStatus := vtc.Status
	vtc.Status = planetscalev2.NewVitessCellStatus()

	// Materialize all hard-coded default values into the object.
	// TODO(enisoc): Use versioned defaults when operator-sdk supports mutating webhooks.
	planetscalev2.DefaultVitessCell(vtc)

	// Create/update cell-local etcd, if requested.
	if err := r.reconcileLocalEtcd(ctx, vtc); err != nil {
		// Record result but continue.
		resultBuilder.Error(err)
	}

	// Create/update vtgate deployments.
	vtgateResult, err := r.reconcileVtgate(ctx, vtc)
	resultBuilder.Merge(vtgateResult, err)

	// Check which VitessKeyspaces are deployed to this cell.
	keyspaceResult, err := r.reconcileKeyspaces(ctx, vtc)
	resultBuilder.Merge(keyspaceResult, err)

	// Update status if needed.
	vtc.Status.ObservedGeneration = vtc.Generation
	if !apiequality.Semantic.DeepEqual(&vtc.Status, &oldStatus) {
		if err := r.client.Status().Update(ctx, vtc); err != nil {
			if !apierrors.IsConflict(err) {
				r.recorder.Eventf(vtc, corev1.EventTypeWarning, "StatusUpdateFailed", "failed to update status: %v", err)
			}
			resultBuilder.Error(err)
		}
	}

	result, err := resultBuilder.Result()
	reconcileCount.WithLabelValues(vtc.Labels[planetscalev2.ClusterLabel], vtc.Spec.Name, metrics.Result(err)).Inc()
	return result, err
}
