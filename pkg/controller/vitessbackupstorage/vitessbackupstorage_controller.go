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

package vitessbackupstorage

import (
	"context"
	"flag"

	"github.com/sirupsen/logrus"

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
	"planetscale.dev/vitess-operator/pkg/operator/environment"
	"planetscale.dev/vitess-operator/pkg/operator/metrics"
	"planetscale.dev/vitess-operator/pkg/operator/reconciler"
	"planetscale.dev/vitess-operator/pkg/operator/results"
)

const (
	controllerName = "vitessbackupstorage-controller"
)

var (
	maxConcurrentReconciles = flag.Int("vitessbackupstorage_concurrent_reconciles", 10, "the maximum number of different vitessbackupstorages to reconcile concurrently")
)

var log = logrus.WithField("controller", "VitessBackupStorage")

// watchResources should contain all the resource types that this controller creates.
var watchResources = []client.Object{
	&corev1.Pod{},
}

// Add creates a new Controller and adds it to the Manager.
func Add(mgr manager.Manager) error {
	r, err := newReconciler(mgr)
	if err != nil {
		return err
	}
	return add(mgr, r)
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) (*ReconcileVitessBackupStorage, error) {
	c := mgr.GetClient()
	scheme := mgr.GetScheme()
	recorder := mgr.GetEventRecorderFor(controllerName)

	return &ReconcileVitessBackupStorage{
		client:     c,
		scheme:     scheme,
		recorder:   recorder,
		reconciler: reconciler.New(c, scheme, recorder),
	}, nil
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r *ReconcileVitessBackupStorage) error {
	// Create a new controller
	c, err := controller.New(controllerName, mgr,
		controller.Options{
			Reconciler:              r,
			MaxConcurrentReconciles: *maxConcurrentReconciles,
		})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource VitessBackupStorage
	if err := c.Watch(source.Kind(mgr.GetCache(), &planetscalev2.VitessBackupStorage{}), &handler.EnqueueRequestForObject{}); err != nil {
		return err
	}

	// Watch for changes to secondary resources and requeue the owner VitessBackupStorage.
	for _, resource := range watchResources {
		err := c.Watch(source.Kind(mgr.GetCache(), resource), handler.EnqueueRequestForOwner(
			mgr.GetScheme(),
			mgr.GetRESTMapper(),
			&planetscalev2.VitessBackupStorage{},
			handler.OnlyControllerOwner(),
		))
		if err != nil {
			return err
		}
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
	recorder   record.EventRecorder
	reconciler *reconciler.Reconciler
}

// Reconcile reads that state of the cluster for a VitessBackupStorage object and makes changes based on the state read
// and what is in the VitessBackupStorage.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileVitessBackupStorage) Reconcile(cctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	ctx, cancel := context.WithTimeout(cctx, environment.ReconcileTimeout())
	defer cancel()

	resultBuilder := &results.Builder{}

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

	resultBuilder.Merge(r.reconcileSubcontroller(ctx, vbs))

	result, err := resultBuilder.Result()
	reconcileCount.WithLabelValues(vbs.Name, metrics.Result(err)).Inc()
	return result, err
}
