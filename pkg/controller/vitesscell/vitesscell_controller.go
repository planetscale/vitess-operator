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

package vitesscell

import (
	"context"
	"flag"
	"time"

	"github.com/sirupsen/logrus"
	apilabels "k8s.io/apimachinery/pkg/labels"

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

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/environment"
	"planetscale.dev/vitess-operator/pkg/operator/metrics"
	"planetscale.dev/vitess-operator/pkg/operator/reconciler"
	"planetscale.dev/vitess-operator/pkg/operator/results"
	"planetscale.dev/vitess-operator/pkg/operator/resync"
)

const (
	controllerName = "vitesscell-controller"
)

var (
	maxConcurrentReconciles = flag.Int("vitesscell_concurrent_reconciles", 10, "the maximum number of different vitesscells to reconcile concurrently")
	resyncPeriod            = flag.Duration("vitesscell_resync_period", 30*time.Minute, "reconcile vitesscells with this period even if no Kubernetes events occur")
)

var log = logrus.WithField("controller", "VitessCell")

// watchResources should contain all the resource types that this controller creates.
var watchResources = []client.Object{
	&corev1.Service{},
	&appsv1.Deployment{},
	&autoscalingv2.HorizontalPodAutoscaler{},

	&planetscalev2.EtcdLockserver{},
}

// Add creates a new VitessCell Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) *ReconcileVitessCell {
	c := mgr.GetClient()
	scheme := mgr.GetScheme()
	recorder := mgr.GetEventRecorderFor(controllerName)

	return &ReconcileVitessCell{
		client:     c,
		scheme:     scheme,
		resync:     resync.NewPeriodic(controllerName, *resyncPeriod),
		recorder:   recorder,
		reconciler: reconciler.New(c, scheme, recorder),
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r *ReconcileVitessCell) error {
	// Create a new controller
	c, err := controller.New(controllerName, mgr, controller.Options{
		Reconciler:              r,
		MaxConcurrentReconciles: *maxConcurrentReconciles,
	})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource VitessCell
	err = c.Watch(source.Kind(mgr.GetCache(), &planetscalev2.VitessCell{}, &handler.TypedEnqueueRequestForObject[*planetscalev2.VitessCell]{}))
	if err != nil {
		return err
	}

	// Watch for changes to secondary resources and requeue the owner VitessCell.
	for _, resource := range watchResources {
		err := c.Watch(source.Kind(mgr.GetCache(), resource, handler.EnqueueRequestForOwner(
			mgr.GetScheme(),
			mgr.GetRESTMapper(),
			&planetscalev2.VitessCell{},
			handler.OnlyControllerOwner(),
		)))
		if err != nil {
			return err
		}
	}

	// Watch for changes in VitessKeyspaces, which we don't own, and requeue associated VitessCells.
	err = c.Watch(source.Kind[*planetscalev2.VitessKeyspace](mgr.GetCache(), &planetscalev2.VitessKeyspace{}, handler.TypedEnqueueRequestsFromMapFunc[*planetscalev2.VitessKeyspace](keyspaceCellsMapper)))
	if err != nil {
		return err
	}

	// Watch for changes in Secrets, which we don't own, and requeue associated VitessCells.
	scm := &secretCellsMapper{
		client: mgr.GetClient(),
	}
	err = c.Watch(source.Kind(mgr.GetCache(), &corev1.Secret{}, handler.TypedEnqueueRequestsFromMapFunc[*corev1.Secret](scm.Map)))
	if err != nil {
		return err
	}

	// Periodically resync even when no Kubernetes events have come in.
	if err := c.Watch(r.resync.WatchSource()); err != nil {
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
	resync     *resync.Periodic
	recorder   record.EventRecorder
	reconciler *reconciler.Reconciler
}

// Reconcile reads that state of the cluster for a VitessCell object and makes changes based on the state read
// and what is in the VitessCell.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileVitessCell) Reconcile(cctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	ctx, cancel := context.WithTimeout(cctx, environment.ReconcileTimeout())
	defer cancel()

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

	// List all VitessShard in the same cluster, we do this to determine what is the image used for the mysqld container
	// Note that this is cheap because it comes from the local cache.
	labels := map[string]string{
		planetscalev2.ClusterLabel: vtc.Labels[planetscalev2.ClusterLabel],
	}
	opts := &client.ListOptions{
		Namespace:     vtc.Namespace,
		LabelSelector: apilabels.SelectorFromSet(labels),
	}
	vts := &planetscalev2.VitessShardList{}
	if err := r.client.List(ctx, vts, opts); err != nil {
		r.recorder.Eventf(vtc, corev1.EventTypeWarning, "ListFailed", "failed to list VitessShard objects: %v", err)
		return resultBuilder.Error(err)
	}

	var mysqldImage string
	if len(vts.Items) > 0 {
		mysqldImage = vts.Items[0].Spec.Images.Mysqld.Image()
	}

	// Create/update vtgate deployments.
	vtgateResult, err := r.reconcileVtgate(ctx, vtc, mysqldImage)
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

	// Request a periodic resync for the cluster so we can recheck topology even
	// if no Kubernetes events have occurred.
	r.resync.Enqueue(request.NamespacedName)

	result, err := resultBuilder.Result()
	reconcileCount.WithLabelValues(vtc.Labels[planetscalev2.ClusterLabel], vtc.Spec.Name, metrics.Result(err)).Inc()
	return result, err
}
