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
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
)

const (
	maxSafeVReplicationLag = 10
)

func (r *reconcileHandler) reconcileResharding(ctx context.Context) error {
	workflowList, err := r.wr.ListAllWorkflows(ctx, r.vtk.Spec.Name)
	if err != nil {
		return err
	}

	reshardingInProgress := corev1.ConditionUnknown
	workflows := make([]planetscalev2.WorkflowStatus, 0, len(workflowList))
	for _, workflowName := range workflowList {
		workflow, err := r.wr.ShowWorkflow(ctx, workflowName, r.vtk.Spec.Name)
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
					r.vtk.Status.SetConditionStatus(planetscalev2.VitessKeyspaceReshardingInSync, corev1.ConditionFalse, "VReplicationError", fmt.Sprintf("Encountered a VReplication Error: %v", vReplRow.Message))
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

		if workflowStatus.State == planetscalev2.WorkflowCopying {
			r.vtk.Status.SetConditionStatus(planetscalev2.VitessKeyspaceReshardingInSync, corev1.ConditionFalse, "WorkflowCopying", fmt.Sprintf("Workflow %v is currently in Copy phase.", workflowStatus.Workflow))
		}

		// If MaxVReplicationLag ever exceeds max safe value, we need update our condition object.
		if workflow.MaxVReplicationLag >= maxSafeVReplicationLag {
			r.vtk.Status.SetConditionStatus(planetscalev2.VitessKeyspaceReshardingInSync, corev1.ConditionFalse, "WorkflowLagging", fmt.Sprintf("Workflow %v is currently lagging by greater than 10 seconds.", workflowStatus.Workflow))
		}
	}
	if reshardingInProgress == corev1.ConditionTrue {
		r.vtk.Status.SetConditionStatus(planetscalev2.VitessKeyspaceReshardingActive, reshardingInProgress, "ReshardingActive", "At least one workflow has active resharding ongoing.")
	} else if reshardingInProgress == corev1.ConditionFalse {
		r.vtk.Status.SetConditionStatus(planetscalev2.VitessKeyspaceReshardingActive, reshardingInProgress, "ReshardingInactive", "Active workflows were found, but none that had source and target location involving the same keyspace.")
	} else {
		// ConditionUnknown so likely we have no active workflows.
		r.vtk.Status.SetConditionStatus(planetscalev2.VitessKeyspaceReshardingActive, reshardingInProgress, "NoActiveWorkflows", "No active workflows.")
	}

	sort.Strings(r.vtk.Status.ServingShards)

	return nil
}
