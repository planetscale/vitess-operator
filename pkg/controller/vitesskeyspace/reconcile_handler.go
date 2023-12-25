/*
Copyright 2020 PlanetScale Inc.

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

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"vitess.io/vitess/go/mysql/collations"
	"vitess.io/vitess/go/vt/logutil"
	"vitess.io/vitess/go/vt/servenv"
	"vitess.io/vitess/go/vt/sqlparser"
	"vitess.io/vitess/go/vt/vttablet/tmclient"
	"vitess.io/vitess/go/vt/wrangler"

	v2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/reconciler"
	"planetscale.dev/vitess-operator/pkg/operator/toposerver"
)

// reconcileHandler provides context for this specific reconcile loop,
// and handles reconcile related subroutines.
type reconcileHandler struct {
	client              client.Client
	recorder            record.EventRecorder
	reconciler          *reconciler.Reconciler
	vtk                 *v2.VitessKeyspace
	oldStatus           *v2.VitessKeyspaceStatus
	untouchedConditions map[v2.VitessKeyspaceConditionType]bool
	// This field holds a toposerver connection. Please don't try to access until you have
	// run tsInit().
	ts *toposerver.Conn

	// This field holds a Wrangler. Please don't try to access until you have run tsInit()
	wr *wrangler.Wrangler
	// This field holds a tablet manager client internally for closing upon collection of reconcileHandler.
	// Please don't try to access until you have run tsInit().
	tmc tmclient.TabletManagerClient
}

// tsInit will initialize a toposerver connection, as well as a
// tablet manager client and wrangler for subroutine use.
func (r *reconcileHandler) tsInit(ctx context.Context) error {
	if r.ts != nil {
		return nil
	}

	// We need to initialize for the first time if we got here.
	ts, err := toposerver.Open(ctx, r.vtk.Spec.GlobalLockserver)
	if err != nil {
		r.recorder.Eventf(r.vtk, v1.EventTypeWarning, "TopoConnectFailed", "failed to connect to global lockserver: %v", err)
		// Give the lockserver some time to come up.
		log.Info("Could not connect to topo at vitesskeyspace controller.")
		return err
	}
	r.ts = ts

	if r.tmc == nil {
		r.tmc = tmclient.NewTabletManagerClient()
	}

	collationEnv := collations.NewEnvironment(servenv.MySQLServerVersion())
	parser, err := sqlparser.New(sqlparser.Options{
		MySQLServerVersion: servenv.MySQLServerVersion(),
		TruncateUILen:      servenv.TruncateUILen,
		TruncateErrLen:     servenv.TruncateErrLen,
	})
	if err != nil {
		return err
	}
	// Wrangler wraps the necessary clients and implements
	// multi-step Vitess cluster management workflows.
	wr := wrangler.New(logutil.NewConsoleLogger(), r.ts.Server, r.tmc, collationEnv, parser)
	r.wr = wr

	return nil
}

// close should be called in a defer upon construction of a reconcileHandler to
// defer the closing of underlying topo if we successfully created one.
func (r *reconcileHandler) close() {
	if r.ts != nil {
		r.ts.Close()
	}

	if r.tmc != nil {
		r.tmc.Close()
	}
}

func (r *reconcileHandler) updateStatus(ctx context.Context) error {
	// Before updating status, set conditions we haven't touched to unknown.
	for condition := range r.untouchedConditions {
		r.setConditionStatus(condition, v1.ConditionUnknown, "ReconcileFailed", "Failed to determine status of the condition.")
	}

	r.vtk.Status.ObservedGeneration = r.vtk.Generation
	if !equality.Semantic.DeepEqual(&r.vtk.Status, r.oldStatus) {
		if err := r.client.Status().Update(ctx, r.vtk); err != nil {
			if !errors.IsConflict(err) {
				r.recorder.Eventf(r.vtk, v1.EventTypeWarning, "StatusUpdateFailed", "failed to update status: %v", err)
			}
			return err
		}
	}

	return nil
}

func (r *reconcileHandler) setConditionStatus(condType v2.VitessKeyspaceConditionType, newStatus v1.ConditionStatus, reason, message string) {
	// Let's remove this condition from the untouched conditions set so our records are up to date.
	delete(r.untouchedConditions, condType)

	r.vtk.Status.SetConditionStatus(condType, newStatus, reason, message)
}
