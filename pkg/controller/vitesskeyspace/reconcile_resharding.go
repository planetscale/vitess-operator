package vitesskeyspace

import (
	"context"
	"sort"
	"strings"

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

	workflowList, err := wr.ListAllWorkflows(ctx, vtk.Spec.Name)
	if err != nil {
		workflowList = make([]string, 0)
	}

	reshardingInProgress := corev1.ConditionUnknown
	workflows := make([]planetscalev2.WorkflowStatus, 0, len(workflowList))
	for _, workflowName := range workflowList {
		workflow, err := wr.ShowWorkflow(ctx, workflowName, vtk.Spec.Name)
		if err != nil {
			return err
		}
		workflowStatus := planetscalev2.WorkflowStatus{
			Workflow: workflow.Workflow,
		}

		for name, status := range workflow.ShardStatuses {
			if status.MasterIsServing {
				shard := strings.Split(name, "/")[0]
				vtk.Status.ServingShards = append(vtk.Status.ServingShards, shard)
			}
			for _, vReplRow := range status.MasterReplicationStatuses {
				if vReplRow.State == "Error" {
					workflowStatus.State = planetscalev2.ErrorState
					break
				}
				if vReplRow.State == "Copying" {
					workflowStatus.State = planetscalev2.CopyingState
				}
				if vReplRow.State == "Lagging" && workflowStatus.State != planetscalev2.CopyingState {
					workflowStatus.State = planetscalev2.LaggingState
				}
				if vReplRow.State == "Running" && workflowStatus.State == "" {
					workflowStatus.State = planetscalev2.RunningState
				}
			}
		}
		workflows = append(workflows, workflowStatus)

		if workflow.SourceLocation.Keyspace == workflow.TargetLocation.Keyspace {
			reshardingInProgress = corev1.ConditionTrue
		} else if reshardingInProgress != corev1.ConditionTrue {
			reshardingInProgress = corev1.ConditionFalse
		}
	}
	vtk.Status.ReshardingInProgress = reshardingInProgress
	sort.Strings(vtk.Status.ServingShards)

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
