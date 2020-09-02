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
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/toposerver"

	"vitess.io/vitess/go/vt/logutil"
	"vitess.io/vitess/go/vt/vttablet/tmclient"
	"vitess.io/vitess/go/vt/wrangler"
)

const (
	maxSafeVReplicationLag = 10
)

func (r *reconcileHandler) reconcileResharding(ctx context.Context) error {
	wr, ts, err := newWrangler(ctx, r.vtk.Spec.GlobalLockserver)
	if err != nil {
		return err
	}
	defer ts.Close()

	workflowList, err := wr.ListAllWorkflows(ctx, r.vtk.Spec.Name)
	if err != nil {
		workflowList = make([]string, 0)
	}

	reshardingInProgress := corev1.ConditionUnknown
	workflows := make([]planetscalev2.WorkflowStatus, 0, len(workflowList))
	for _, workflowName := range workflowList {
		workflow, err := wr.ShowWorkflow(ctx, workflowName, r.vtk.Spec.Name)
		if err != nil {
			return err
		}
		if workflow.SourceLocation.Keyspace != workflow.TargetLocation.Keyspace {
			if reshardingInProgress == corev1.ConditionUnknown {
				reshardingInProgress = corev1.ConditionFalse
			}
			continue
		}
		reshardingInProgress = corev1.ConditionTrue

		workflowStatus := planetscalev2.WorkflowStatus{
			Workflow: workflow.Workflow,
			State:    planetscalev2.WorkflowUnknown,
		}

		// We aggregate status across all the shards for the workflow so we can definitely know if we are in two states:
		// Copying, or Error. We also do this so we can determine what all of the serving shards are.
		// At a high level we mostly need to know if we are still in the Copying phase (for any shard whatsoever), or if
		// we have an error in resharding somewhere that needs to be surfaced.
		for name, status := range workflow.ShardStatuses {
			if status.MasterIsServing {
				shard := strings.Split(name, "/")[0]
				r.vtk.Status.ServingShards = append(r.vtk.Status.ServingShards, shard)
			}
			for _, vReplRow := range status.MasterReplicationStatuses {
				if vReplRow.State == "Error" {
					workflowStatus.State = planetscalev2.WorkflowError
					workflowStatus.ErrorMessage = vReplRow.Message
					break
				}
				if vReplRow.State == "Copying" {
					workflowStatus.State = planetscalev2.WorkflowCopying
				}
				if (vReplRow.State == "Running" || vReplRow.State == "Lagging") && workflowStatus.State == planetscalev2.WorkflowUnknown {
					workflowStatus.State = planetscalev2.WorkflowRunning
				}
			}
		}
		workflows = append(workflows, workflowStatus)

		// If MaxVReplicationLag ever exceeds max safe value, we need to set UnsafeVReplicationLag to true.
		if workflow.MaxVReplicationLag >= maxSafeVReplicationLag {
			workflowStatus.UnsafeVReplicationLag = true
		}
	}
	r.vtk.Status.ReshardingInProgress = reshardingInProgress
	sort.Strings(r.vtk.Status.ServingShards)

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
