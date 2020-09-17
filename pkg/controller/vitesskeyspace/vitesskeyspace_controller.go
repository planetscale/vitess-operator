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

package vitesskeyspace

import (
	"context"
	"flag"
	"time"

	"github.com/sirupsen/logrus"

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
	"planetscale.dev/vitess-operator/pkg/operator/k8s"
	"planetscale.dev/vitess-operator/pkg/operator/metrics"
	"planetscale.dev/vitess-operator/pkg/operator/reconciler"
	"planetscale.dev/vitess-operator/pkg/operator/results"
	"planetscale.dev/vitess-operator/pkg/operator/resync"
)

const (
	controllerName = "vitesskeyspace-controller"
)

var (
	maxConcurrentReconciles = flag.Int("vitesskeyspace_concurrent_reconciles", 10, "the maximum number of different vitesskeyspaces to reconcile concurrently")
	resyncPeriod            = flag.Duration("vitesskeyspace_resync_period", 15*time.Second, "reconcile vitesskeyspaces with this period even if no Kubernetes events occur")

	// keyspaceConditions lists all the conditions that the keyspace controller is responsible for updating.
	keyspaceConditions = map[planetscalev2.VitessKeyspaceConditionType]bool{
		planetscalev2.VitessKeyspaceReshardingActive: true,
		planetscalev2.VitessKeyspaceReshardingInSync: true,
	}
)

var log = logrus.WithField("controller", "VitessKeyspace")

// watchResources should contain all the resource types that this controller creates.
var watchResources = []runtime.Object{
	&planetscalev2.VitessShard{},
}

// Add creates a new VitessKeyspace Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) *ReconcileVitessKeyspace {
	c := mgr.GetClient()
	scheme := mgr.GetScheme()
	recorder := mgr.GetEventRecorderFor(controllerName)

	return &ReconcileVitessKeyspace{
		client:     c,
		scheme:     scheme,
		resync:     resync.NewPeriodic(controllerName, *resyncPeriod),
		recorder:   recorder,
		reconciler: reconciler.New(c, scheme, recorder),
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r *ReconcileVitessKeyspace) error {
	// Create a new controller
	c, err := controller.New(controllerName, mgr, controller.Options{
		Reconciler:              r,
		MaxConcurrentReconciles: *maxConcurrentReconciles,
	})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource VitessKeyspace
	err = c.Watch(&source.Kind{Type: &planetscalev2.VitessKeyspace{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resources and requeue the owner VitessKeyspace.
	for _, resource := range watchResources {
		err := c.Watch(&source.Kind{Type: resource}, &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &planetscalev2.VitessKeyspace{},
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

var _ reconcile.Reconciler = &ReconcileVitessKeyspace{}

// ReconcileVitessKeyspace reconciles a VitessKeyspace object
type ReconcileVitessKeyspace struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client     client.Client
	scheme     *runtime.Scheme
	resync     *resync.Periodic
	recorder   record.EventRecorder
	reconciler *reconciler.Reconciler
}

// Reconcile reads that state of the cluster for a VitessKeyspace object and makes changes based on the state read
// and what is in the VitessKeyspace.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileVitessKeyspace) Reconcile(request reconcile.Request) (finalResult reconcile.Result, finalErr error) {
	ctx, cancel := context.WithTimeout(context.TODO(), environment.ReconcileTimeout())
	defer cancel()

	resultBuilder := &results.Builder{}

	log := log.WithFields(logrus.Fields{
		"namespace":      request.Namespace,
		"vitesskeyspace": request.Name,
	})
	log.Debug("Reconciling VitessKeyspace")

	handler, err := r.NewReconcileHandler(ctx, request)
	if err != nil {
		return resultBuilder.Error(err)
	}
	if handler == nil {
		return resultBuilder.Result()
	}
	defer handler.close()

	defer func() {
		err := handler.updateStatus(ctx)
		if err != nil {
			finalResult, finalErr = resultBuilder.Error(err)
		}
	}()

	// Create/update desired VitessShards.
	if err := handler.reconcileShards(ctx); err != nil {
		resultBuilder.Error(err)
	}

	// Check latest Vitess topology state and update as needed.
	// NOTE: This must always be done after reconcileShards, so Status.Shards is populated.
	topoResult, err := handler.reconcileTopology(ctx)
	resultBuilder.Merge(topoResult, err)

	// Check resharding status and report back.
	reshardingResult, err := handler.reconcileResharding(ctx)
	resultBuilder.Merge(reshardingResult, err)

	// Request a periodic resync for the keyspace so we can recheck topology
	// even if no Kubernetes events have occurred.
	r.resync.Enqueue(request.NamespacedName)

	result, err := resultBuilder.Result()
	reconcileCount.WithLabelValues(handler.vtk.Labels[planetscalev2.ClusterLabel], handler.vtk.Spec.Name, metrics.Result(err)).Inc()
	return result, err
}

func (r *ReconcileVitessKeyspace) NewReconcileHandler(ctx context.Context, request reconcile.Request) (*reconcileHandler, error) {
	// Fetch the VitessKeyspace instance
	vtk := &planetscalev2.VitessKeyspace{}
	err := r.client.Get(ctx, request.NamespacedName, vtk)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return nil error so caller hits nil reconcileHandler and decides to bail.
			return nil, nil
		}
		// Error reading the object - return error so caller re-queues.
		return nil, err
	}
	planetscalev2.DefaultVitessKeyspace(vtk)

	oldStatus := vtk.Status.DeepCopy()
	vtk.Status = planetscalev2.NewVitessKeyspaceStatus()
	vtk.Status.Conditions = oldStatus.DeepCopyConditions()

	untouchedConditions := make(map[planetscalev2.VitessKeyspaceConditionType]bool, len(keyspaceConditions))
	for condition := range keyspaceConditions {
		untouchedConditions[condition] = true
	}

	// Idle means the keyspace is not deployed in any cells (there are no tablet pools defined).
	vtk.Status.Idle = k8s.ConditionStatus(len(vtk.Spec.CellNames()) == 0)

	return &reconcileHandler{
		client:              r.client,
		recorder:            r.recorder,
		reconciler:          r.reconciler,
		vtk:                 vtk,
		oldStatus:           oldStatus,
		untouchedConditions: untouchedConditions,
	}, nil
}
