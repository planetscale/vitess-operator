package vitesskeyspace

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/toposerver"

	"vitess.io/vitess/go/vt/logutil"
	"vitess.io/vitess/go/vt/vttablet/tmclient"
	"vitess.io/vitess/go/vt/wrangler"
)

func (r *ReconcileVitessKeyspace) reconcileResharding(ctx context.Context, vtk *planetscalev2.VitessKeyspace) error {
	wr, ts, err := newWrangler(ctx, vtk.Spec.GlobalLockserver)
	if err != nil {
		return err
	}
	defer ts.Close()

	workflows, err := wr.ListAllWorkflows(ctx, vtk.Spec.Name)
	if err != nil {
		workflows = make([]string, 0)
	}

	vtk.Status.ActiveWorkflows = workflows
	// TODO: Address when ConditionUnknown is applicable.
	if len(workflows) != 0 {
		vtk.Status.ReshardingInProgress = corev1.ConditionTrue
	} else {
		vtk.Status.ReshardingInProgress = corev1.ConditionFalse
	}

	return nil
}

// newWrangler initializes a new Vitess Wrangler that gives us access to information
// about resharding workflows.
func newWrangler(ctx context.Context, lockserverSpec planetscalev2.VitessLockserverParams) (*wrangler.Wrangler, *toposerver.Conn, error) {
	// We need to initialize for the first time if we got here.
	ts, err := toposerver.Open(ctx, lockserverSpec)
	if err != nil {
		return nil, nil, err
	}
	tmc := tmclient.NewTabletManagerClient()

	// Wrangler wraps the necessary clients and implements
	// multi-step Vitess cluster management workflows.
	return wrangler.New(logutil.NewConsoleLogger(), ts.Server, tmc), ts, nil
}
