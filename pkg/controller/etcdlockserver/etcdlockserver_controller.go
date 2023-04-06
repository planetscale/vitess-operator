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

package etcdlockserver

import (
	"context"
	"flag"

	"github.com/sirupsen/logrus"

	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
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
	"planetscale.dev/vitess-operator/pkg/operator/metrics"
	"planetscale.dev/vitess-operator/pkg/operator/reconciler"
	"planetscale.dev/vitess-operator/pkg/operator/results"
)

const (
	controllerName = "etcdlockserver-controller"
)

var (
	maxConcurrentReconciles = flag.Int("etcdlockserver_concurrent_reconciles", 10, "the maximum number of different etcdlockservers to reconcile concurrently")
)

var log = logrus.WithField("controller", "EtcdLockserver")

// watchResources should contain all the resource types that this controller creates.
var watchResources = []client.Object{
	&corev1.Pod{},
	&corev1.Service{},
	&corev1.PersistentVolumeClaim{},
	&policyv1.PodDisruptionBudget{},
}

// Add creates a new EtcdLockserver Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	c := mgr.GetClient()
	scheme := mgr.GetScheme()
	recorder := mgr.GetEventRecorderFor(controllerName)

	return &ReconcileEtcdLockserver{
		client:     c,
		scheme:     scheme,
		recorder:   recorder,
		reconciler: reconciler.New(c, scheme, recorder),
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New(controllerName, mgr, controller.Options{
		Reconciler:              r,
		MaxConcurrentReconciles: *maxConcurrentReconciles,
	})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource EtcdLockserver
	if err := c.Watch(&source.Kind{Type: &planetscalev2.EtcdLockserver{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return err
	}

	// Watch for changes to secondary resources and requeue the owner EtcdLockserver.
	for _, resource := range watchResources {
		err := c.Watch(&source.Kind{Type: resource}, &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &planetscalev2.EtcdLockserver{},
		})
		if err != nil {
			return err
		}
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileEtcdLockserver{}

// ReconcileEtcdLockserver reconciles a EtcdLockserver object
type ReconcileEtcdLockserver struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client     client.Client
	scheme     *runtime.Scheme
	recorder   record.EventRecorder
	reconciler *reconciler.Reconciler
}

// Reconcile reads that state of the cluster for a EtcdLockserver object and makes changes based on the state read
// and what is in the EtcdLockserver.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileEtcdLockserver) Reconcile(cctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	ctx, cancel := context.WithTimeout(cctx, environment.ReconcileTimeout())
	defer cancel()

	resultBuilder := &results.Builder{}
	log := log.WithFields(logrus.Fields{
		"namespace":      request.Namespace,
		"etcdlockserver": request.Name,
	})
	log.Info("Reconciling EtcdLockserver")

	// Fetch the EtcdLockserver instance.
	ls := &planetscalev2.EtcdLockserver{}
	err := r.client.Get(ctx, request.NamespacedName, ls)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return resultBuilder.Result()
		}
		// Error reading the object - requeue the request.
		return resultBuilder.Error(err)
	}

	// Materialize defaults.
	planetscalev2.DefaultEtcdLockserver(ls)

	// Reset status, since that's all out of date info that we will recompute now.
	oldStatus := ls.Status
	ls.Status = *planetscalev2.NewEtcdLockserverStatus()

	// Create/update Services.
	svcResult, err := r.reconcileServices(ctx, ls)
	resultBuilder.Merge(svcResult, err)

	// Create/update desired etcd members.
	memberResult, err := r.reconcileMembers(ctx, ls)
	resultBuilder.Merge(memberResult, err)

	// Create/update PDB.
	pdbResult, err := r.reconcilePodDisruptionBudget(ctx, ls)
	resultBuilder.Merge(pdbResult, err)

	// Update status if needed.
	ls.Status.ObservedGeneration = ls.Generation
	if !apiequality.Semantic.DeepEqual(&ls.Status, &oldStatus) {
		if err := r.client.Status().Update(ctx, ls); err != nil {
			if !apierrors.IsConflict(err) {
				r.recorder.Eventf(ls, corev1.EventTypeWarning, "StatusUpdateFailed", "failed to update status: %v", err)
			}
			resultBuilder.Error(err)
		}
	}

	// Update metrics.
	available := float64(0)
	if ls.Status.Available == corev1.ConditionTrue {
		available = 1
	}
	clusterAvailable.WithLabelValues(ls.Name).Set(available)

	result, err := resultBuilder.Result()
	reconcileCount.WithLabelValues(ls.Name, metrics.Result(err)).Inc()
	return result, err
}
