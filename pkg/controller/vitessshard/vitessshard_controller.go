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

package vitessshard

import (
	"context"
	"flag"
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
	"planetscale.dev/vitess-operator/pkg/operator/environment"
	"planetscale.dev/vitess-operator/pkg/operator/reconciler"
	"planetscale.dev/vitess-operator/pkg/operator/results"
	"planetscale.dev/vitess-operator/pkg/operator/resync"
	"planetscale.dev/vitess-operator/pkg/operator/vitessshard"
)

const (
	controllerName = "vitessshard-controller"
)

var (
	maxConcurrentReconciles = flag.Int("vitessshard_concurrent_reconciles", 10, "the maximum number of different vitessshards to reconcile concurrently")
	resyncPeriod            = flag.Duration("vitessshard_resync_period", 30*time.Second, "reconcile vitessshards with this period even if no Kubernetes events occur")
	onlineFileSystemExpansion = flag.Bool("enable_online_fs_expansion", true, "if true, pod referencing the resized volume do not need to be restarted, but provided that the volume plug-in supports, such as GCE-PD, AWS-EBS, Cinder, and Ceph RBD")
)

var log = logrus.WithField("controller", "VitessShard")

// watchResources should contain all the resource types that this controller creates.
var watchResources = []client.Object{
	&corev1.Pod{},
	&corev1.PersistentVolumeClaim{},
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
	recorder := mgr.GetEventRecorderFor(controllerName)

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
	if err := c.Watch(source.Kind(mgr.GetCache(), &planetscalev2.VitessShard{}, &handler.TypedEnqueueRequestForObject[*planetscalev2.VitessShard]{})); err != nil {
		return err
	}

	// Watch for changes to secondary resources and requeue the owner VitessShard.
	for _, resource := range watchResources {
		err := c.Watch(source.Kind(mgr.GetCache(), resource, handler.EnqueueRequestForOwner(
			mgr.GetScheme(),
			mgr.GetRESTMapper(),
			&planetscalev2.VitessShard{},
			handler.OnlyControllerOwner(),
		)))
		if err != nil {
			return err
		}
	}

	// Watch for changes in VitessBackups, which we don't own, and requeue associated VitessShards.
	err = c.Watch(source.Kind(mgr.GetCache(), &planetscalev2.VitessBackup{}, handler.TypedEnqueueRequestsFromMapFunc[*planetscalev2.VitessBackup](shardBackupMapper)))
	if err != nil {
		return err
	}

	// Periodically resync even when no Kubernetes events have come in.
	if err := c.Watch(r.resync.WatchSource()); err != nil {
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
func (r *ReconcileVitessShard) Reconcile(cctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	ctx, cancel := context.WithTimeout(cctx, environment.ReconcileTimeout())
	defer cancel()

	resultBuilder := &results.Builder{}

	log := log.WithFields(logrus.Fields{
		"namespace":   request.Namespace,
		"vitessshard": request.Name,
	})
	// Shards have a periodic reconciliation to check replication,
	// so we log at Debug because this is noisy.
	log.Debug("Reconciling VitessShard")

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
	planetscalev2.DefaultVitessShard(vts)

	// Reset status, since that's all out of date info that we will recompute now.
	oldStatus := vts.Status
	vts.Status = planetscalev2.NewVitessShardStatus()
	// Make sure we don't rinse old conditions - these states need to persist. They may have been added by some other controller.
	// Only persist if old condition is not nil, otherwise use the map allocated above when `NewVitessShardStatus()` was called.
	if oldStatus.Conditions != nil {
		vts.Status.Conditions = oldStatus.DeepCopyConditions()
	}

	// Create/update vtorc.
	vtorcResult, err := r.reconcileVtorc(ctx, vts)
	resultBuilder.Merge(vtorcResult, err)

	// Create/update desired tablets.
	tabletResult, err := r.reconcileTablets(ctx, vts)
	resultBuilder.Merge(tabletResult, err)

	// Mark tablet pods for disk size updates if needed.
	// NOTE: This must always be done after reconcileTablets, so Status.Tablets is populated
	diskUpdateResult, err := r.reconcileDisk(ctx, vts)
	resultBuilder.Merge(diskUpdateResult, err)

	// Perform rolling updates on tablets if needed.
	// NOTE: This must always be done after reconcileTablets, so Status.Tablets is populated.
	rolloutResult, err := r.reconcileRollout(ctx, vts)
	resultBuilder.Merge(rolloutResult, err)

	// Check latest Vitess topology state and update as needed.
	// NOTE: This must always be done after reconcileTablets, so Status.Tablets is populated.
	topoResult, err := r.reconcileTopology(ctx, vts)
	resultBuilder.Merge(topoResult, err)

	// Take initial or periodic backups, if appropriate.
	backupResult, err := r.reconcileBackupJob(ctx, vts)
	resultBuilder.Merge(backupResult, err)

	// Update status if needed.
	vts.Status.ObservedGeneration = vts.Generation
	if !apiequality.Semantic.DeepEqual(&vts.Status, &oldStatus) {
		if err := r.client.Status().Update(ctx, vts); err != nil {
			if !apierrors.IsConflict(err) {
				r.recorder.Eventf(vts, corev1.EventTypeWarning, "StatusUpdateFailed", "failed to update status: %v", err)
			}
			resultBuilder.Error(err)
		}
	}

	// Request a periodic resync for the shard so we can recheck topology and
	// backup freshness even if no Kubernetes events have occurred.
	r.resync.Enqueue(request.NamespacedName)

	result, err := resultBuilder.Result()
	reconcileCount.WithLabelValues(metricLabels(vts, err)...).Inc()
	return result, err
}

// Map maps a VitessBackup to a list of requests for VitessShards.
func shardBackupMapper(ctx context.Context, vtb *planetscalev2.VitessBackup) []reconcile.Request {
	// Request reconciliation for the VitessShard that matches this VitessBackup.
	return []reconcile.Request{
		{
			NamespacedName: client.ObjectKey{
				Namespace: vtb.Namespace,
				Name:      vitessshard.NameFromLabels(vtb.Labels),
			},
		},
	}
}
