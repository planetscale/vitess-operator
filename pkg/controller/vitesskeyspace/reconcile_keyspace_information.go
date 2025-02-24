/*
Copyright 2022 PlanetScale Inc.

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

	corev1 "k8s.io/api/core/v1"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	topodatapb "vitess.io/vitess/go/vt/proto/topodata"
	vtctldatapb "vitess.io/vitess/go/vt/proto/vtctldata"
	"vitess.io/vitess/go/vt/topo"

	"planetscale.dev/vitess-operator/pkg/operator/results"
)

func (r *reconcileHandler) reconcileKeyspaceInformation(ctx context.Context) (reconcile.Result, error) {
	resultBuilder := &results.Builder{}

	// Initialize the topo server before using it.
	// This call is idempotent, so it is safe to call each time
	// before using the topo server.
	err := r.tsInit(ctx)
	if err != nil {
		return resultBuilder.RequeueAfter(topoRequeueDelay)
	}

	topoServer := r.ts.Server
	keyspaceName := r.vtk.Spec.Name
	durabilityPolicy := r.vtk.Spec.DurabilityPolicy
	sidecarDbName := r.vtk.Spec.SidecarDbName

	keyspaceInfo, err := topoServer.GetKeyspace(ctx, keyspaceName)
	if err != nil {
		// The keyspace information record does not exist in the topo server.
		// We should create the record
		if topo.IsErrType(err, topo.NoNode) {
			// Create a normal keyspace with the requested durability policy
			_, err := r.wr.VtctldServer().CreateKeyspace(ctx, &vtctldatapb.CreateKeyspaceRequest{
				Name:             keyspaceName,
				Type:             topodatapb.KeyspaceType_NORMAL,
				DurabilityPolicy: durabilityPolicy,
				SidecarDbName:    sidecarDbName,
			})
			if err != nil {
				resultBuilder.Error(err)
			}
			return resultBuilder.Result()
		}
		// GetKeyspace returned a error other than NoNode.
		// Maybe the topo server is temporarily unreachable
		// We should retry after some time.
		r.recorder.Eventf(r.vtk, corev1.EventTypeWarning, "GetKeyspace", "failed to get keyspace %v: %v", keyspaceName, err)
		return resultBuilder.RequeueAfter(topoRequeueDelay)
	}

	// DurabilityPolicy doesn't match the one requested by the user
	// We change the durability policy using the SetKeyspaceDurabilityPolicy rpc
	if durabilityPolicy != "" && keyspaceInfo.DurabilityPolicy != durabilityPolicy {
		_, err := r.wr.VtctldServer().SetKeyspaceDurabilityPolicy(ctx, &vtctldatapb.SetKeyspaceDurabilityPolicyRequest{
			Keyspace:         keyspaceName,
			DurabilityPolicy: durabilityPolicy,
		})
		if err != nil {
			resultBuilder.Error(err)
		}
	}
	return resultBuilder.Result()
}
