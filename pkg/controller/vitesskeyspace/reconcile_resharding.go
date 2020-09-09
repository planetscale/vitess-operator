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
	"fmt"
	"reflect"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"vitess.io/vitess/go/vt/wrangler"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/results"
)

const (
	maxSafeVReplicationLag = 10
)

func (r *reconcileHandler) reconcileResharding(ctx context.Context) (reconcile.Result, error) {
	resultBuilder := &results.Builder{}

	err := r.tsInit(ctx)
	if err != nil {
		return resultBuilder.RequeueAfter(topoRequeueDelay)
	}

	workflowList, err := r.wr.ListAllWorkflows(ctx, r.vtk.Spec.Name)
	if err != nil {
		// The only reason this would fail is if runVExec fails. This could be a topo communication failure or any number
		// of indeterminable failures. We probably want to requeu faster than the resync period to try again, but wait a bit in
		// case it was a topo related failure.
		r.recorder.Eventf(r.vtk, corev1.EventTypeWarning, "ListAllWorkflowsFailed", "failed to list all workflows: %v", err)
		return resultBuilder.RequeueAfter(topoRequeueDelay)
	}

	// Look for a resharding workflow. If we find a second one bail out.
	var reshardingWorkflow *wrangler.ReplicationStatusResult
	for _, workflowName := range workflowList {
		workflow, err := r.wr.ShowWorkflow(ctx, workflowName, r.vtk.Spec.Name)
		if err != nil {
			// The only reason this would fail is if runVExec fails. This could be a topo communication failure or any number
			// of indeterminable failures. We probably want to requeu faster than the resync period to try again, but wait a bit in
			// case it was a topo related failure.
			r.recorder.Eventf(r.vtk, corev1.EventTypeWarning, "ShowWorkflowFailed", "failed to show workflow %v: %v", workflowName, err)
			return resultBuilder.RequeueAfter(topoRequeueDelay)
		}
		if workflow.SourceLocation.Keyspace != workflow.TargetLocation.Keyspace ||
			reflect.DeepEqual(workflow.SourceLocation.Shards, workflow.TargetLocation.Shards) {
			// If keyspaces are not the same we are not resharding. Likewise if keyspaces are the same but shards are identical,
			// we are also not resharding. Skip this workflow as it's not a resharding related vreplication workflow.
			continue
		}

		if reshardingWorkflow != nil {
			r.setConditionStatus(planetscalev2.VitessKeyspaceReshardingActive, corev1.ConditionUnknown, "MoreThanOneActiveReshardingWorkflow", "There is currently more than one active resharding workflow, and we don't know how to handle this situation.")
			r.setConditionStatus(planetscalev2.VitessKeyspaceReshardingInSync, corev1.ConditionUnknown, "MoreThanOneActiveReshardingWorkflow", "More than one resharding workflow. Can't determine which one follow for determining whether we are in sync or not.")
			r.recorder.Eventf(r.vtk, corev1.EventTypeWarning, "MultipleActiveReshardingWorkflows", "Found multiple active resharding workflows.")
			// This will take a while for a human operator to manually fix, so let's just re-queue at our normal resync rate.
			return resultBuilder.Result()
		}

		reshardingWorkflow = workflow
	}

	if reshardingWorkflow == nil {
		r.setConditionStatus(planetscalev2.VitessKeyspaceReshardingActive, corev1.ConditionFalse, "NoActiveReshardingWorkflows", "No active resharding workflows found.")
		return resultBuilder.Result()
	}
	r.setConditionStatus(planetscalev2.VitessKeyspaceReshardingActive, corev1.ConditionTrue, "ReshardingActive", "One active resharding workflow was found.")

	workflowStatus := &planetscalev2.ReshardingStatus{
		Workflow:     reshardingWorkflow.Workflow,
		State:        planetscalev2.WorkflowUnknown,
		SourceShards: reshardingWorkflow.SourceLocation.Shards,
		TargetShards: reshardingWorkflow.TargetLocation.Shards,
	}

	// We aggregate status across all the shards for the workflow so we can definitely know if we are in two states:
	// Copying, or Error. We also do this so we can determine what all of the serving shards are.
	// At a high level we mostly need to know if we are still in the Copying phase (for any shard whatsoever), or if
	// we have an error in resharding somewhere that needs to be surfaced.
	var errorMsgs []string
	for name, status := range reshardingWorkflow.ShardStatuses {
		if status.MasterIsServing {
			shard := strings.Split(name, "/")[0]
			workflowStatus.ServingShards = append(workflowStatus.ServingShards, shard)
		}
		for _, vReplRow := range status.MasterReplicationStatuses {
			if vReplRow.State == "Error" {
				workflowStatus.State = planetscalev2.WorkflowError
				errorMsgs = append(errorMsgs, vReplRow.Message)
				break
			}
			if vReplRow.State == "Copying" && workflowStatus.State != planetscalev2.WorkflowError {
				workflowStatus.State = planetscalev2.WorkflowCopying
			}
			if (vReplRow.State == "Running" || vReplRow.State == "Lagging") && workflowStatus.State == planetscalev2.WorkflowUnknown {
				workflowStatus.State = planetscalev2.WorkflowRunning
			}
		}
	}
	if workflowStatus.State == planetscalev2.WorkflowError {
		sort.Strings(errorMsgs)
		r.setConditionStatus(planetscalev2.VitessKeyspaceReshardingInSync, corev1.ConditionFalse, "VReplicationError", fmt.Sprintf("Encountered a VReplication Error: %v", errorMsgs[0]))
	}
	if workflowStatus.State == planetscalev2.WorkflowCopying {
		r.setConditionStatus(planetscalev2.VitessKeyspaceReshardingInSync, corev1.ConditionFalse, "WorkflowCopying", fmt.Sprintf("Workflow %v is currently in Copy phase.", workflowStatus.Workflow))
	}

	// If MaxVReplicationLag ever exceeds max safe value, we need update our condition object.
	// Copy phase should take precedence though. We don't care about vrepl lag if we are still in copy phase. Regardless we don't allow switching traffic.
	if reshardingWorkflow.MaxVReplicationLag >= maxSafeVReplicationLag && workflowStatus.State == planetscalev2.WorkflowRunning {
		r.setConditionStatus(planetscalev2.VitessKeyspaceReshardingInSync, corev1.ConditionFalse, "WorkflowLagging", fmt.Sprintf("Workflow %v is currently lagging by greater than 10 seconds.", workflowStatus.Workflow))
	}

	sort.Strings(workflowStatus.ServingShards)
	r.vtk.Status.Resharding = workflowStatus

	return resultBuilder.Result()
}
