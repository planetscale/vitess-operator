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

package vitessshardreplication

import (
	"context"
	"flag"
	"time"

	"github.com/sirupsen/logrus"

	"vitess.io/vitess/go/vt/logutil"
	"vitess.io/vitess/go/vt/vttablet/tmclient"
	"vitess.io/vitess/go/vt/wrangler"

	// register grpc tabletmanager client
	_ "vitess.io/vitess/go/vt/vttablet/grpctmclient"

	corev1 "k8s.io/api/core/v1"
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
	"planetscale.dev/vitess-operator/pkg/operator/reconciler"
	"planetscale.dev/vitess-operator/pkg/operator/results"
	"planetscale.dev/vitess-operator/pkg/operator/resync"
	"planetscale.dev/vitess-operator/pkg/operator/toposerver"
)

const (
	controllerName = "vitessshardreplication-controller"

	// replicationRequeueDelay is how long to wait before retrying a replication
	// configuration operation that failed. We typically return success with a
	// requeue delay instead of returning an error, because it's unlikely that
	// retrying immediately will be worthwhile.
	replicationRequeueDelay = 5 * time.Second
)

var (
	maxConcurrentReconciles = flag.Int("vitessshardreplication_concurrent_reconciles", 10, "the maximum number of different vitessshards to reconcile replication concurrently")
	resyncPeriod            = flag.Duration("vitessshardreplication_resync_period", 30*time.Second, "reconcile replication on vitessshards with this period even if no Kubernetes events occur")
)

var log = logrus.WithField("controller", "VitessShardReplication")

// watchResources should contain all the resource types that this controller creates.
var watchResources = []runtime.Object{
	&corev1.Pod{},
}

// Add creates a new VitessShard Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) *ReconcileVitessShard {
	c := mgr.GetClient()
	scheme := mgr.GetScheme()
	recorder := mgr.GetRecorder(controllerName)

	return &ReconcileVitessShard{
		client:     c,
		scheme:     scheme,
		resync:     resync.NewPeriodic(controllerName, *resyncPeriod),
		recorder:   recorder,
		reconciler: reconciler.New(c, scheme, recorder),
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r *ReconcileVitessShard) error {
	// Create a new controller
	c, err := controller.New(controllerName, mgr, controller.Options{
		Reconciler:              r,
		MaxConcurrentReconciles: *maxConcurrentReconciles,
	})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource VitessShard
	if err := c.Watch(&source.Kind{Type: &planetscalev2.VitessShard{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return err
	}

	// Watch for changes to secondary resources and requeue the owner VitessShard.
	for _, resource := range watchResources {
		err := c.Watch(&source.Kind{Type: resource}, &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &planetscalev2.VitessShard{},
		})
		if err != nil {
			return err
		}
	}

	// Periodically resync even when no Kubernetes events have come in.
	if err := c.Watch(r.resync.WatchSource(), &handler.EnqueueRequestForObject{}); err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileVitessShard{}

// ReconcileVitessShard reconciles a VitessShard object
type ReconcileVitessShard struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client     client.Client
	scheme     *runtime.Scheme
	resync     *resync.Periodic
	recorder   record.EventRecorder
	reconciler *reconciler.Reconciler
}

// Reconcile reads that state of the cluster for a VitessShard object and makes changes based on the state read
// and what is in the VitessShard.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileVitessShard) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	ctx := context.TODO()
	resultBuilder := &results.Builder{}

	log := log.WithFields(logrus.Fields{
		"namespace":   request.Namespace,
		"vitessshard": request.Name,
	})
	// Shards have a periodic reconciliation to check replication,
	// so we log at Debug because this is noisy.
	log.Debug("Reconciling VitessShard replication")

	// Fetch the VitessShard instance
	vts := &planetscalev2.VitessShard{}
	err := r.client.Get(ctx, request.NamespacedName, vts)
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

	// Wait for the main VitessShard controller to update status for the latest
	// desired spec before reconciling replication.
	if vts.Status.ObservedGeneration == 0 || vts.Status.ObservedGeneration != vts.Generation {
		return resultBuilder.Result()
	}

	// Get a connection to Vitess topology for this cluster.
	ts, err := toposerver.Open(ctx, vts.Spec.GlobalLockserver)
	if err != nil {
		r.recorder.Eventf(vts, corev1.EventTypeWarning, "TopoConnectFailed", "failed to connect to global lockserver: %v", err)
		// Give the lockserver some time to come up.
		return resultBuilder.RequeueAfter(replicationRequeueDelay)
	}
	defer ts.Close()

	// TabletManagerClient lets us talk directly to the management API of vttablets.
	tmc := tmclient.NewTabletManagerClient()
	defer tmc.Close()

	// Wrangler wraps the necessary clients and implements
	// multi-step Vitess cluster management workflows.
	wr := wrangler.New(logutil.NewConsoleLogger(), ts.Server, tmc)

	// Check if we need to initialize the shard.
	// If it's already initialized, this will be a no-op.
	// If we are using external MySQL we will bail out early.
	ismResult, err := r.initShardMaster(ctx, vts, wr)
	resultBuilder.Merge(ismResult, err)

	// Check if we need to externally reparent
	// in the case of external MySQL.
	// If we are not using external MySQL we will bail out early.
	terResult, err := r.tabletExternallyReparent(ctx, vts, wr)
	resultBuilder.Merge(terResult, err)

	// Check if we need to start replication on a shard that's been restored
	// from backup. If it's already initialized, this will be a no-op.
	irsResult, err := r.initRestoredShard(ctx, vts, wr)
	resultBuilder.Merge(irsResult, err)

	// Try to fix replication if it's broken.
	repairResult, err := r.repairReplication(ctx, vts, wr)
	resultBuilder.Merge(repairResult, err)

	// Check if we've been asked to do a planned reparent.
	drainResult, err := r.reconcileDrain(ctx, vts, wr)
	resultBuilder.Merge(drainResult, err)

	// Request a periodic resync for the shard so we can recheck replication
	// even if no Kubernetes events have occurred.
	r.resync.Enqueue(request.NamespacedName)

	result, err := resultBuilder.Result()
	reconcileCount.WithLabelValues(metricLabels(vts, err)...).Inc()
	return result, err
}
