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

package vitesstopo

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"vitess.io/vitess/go/vt/proto/topodata"
	"vitess.io/vitess/go/vt/topo"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/lockserver"
	"planetscale.dev/vitess-operator/pkg/operator/results"
)

const (
	// topoRequeueDelay is how long to wait before retrying when a topology
	// server call failed. We typically return success with a requeue delay
	// instead of returning an error, because it's unlikely that retrying
	// immediately will be worthwhile.
	topoRequeueDelay = 5 * time.Second
)

type RegisterCellsParams struct {
	// EventObj holds the object type that the recorder will use when writing events.
	EventObj         runtime.Object
	TopoServer       *topo.Server
	Recorder         record.EventRecorder
	GlobalLockserver *planetscalev2.LockserverSpec
	ClusterName      string
	GlobalTopoImpl   string
	// DesiredCells is a map of cell names to their lockserver specs.
	DesiredCells map[string]*planetscalev2.LockserverSpec
}

func RegisterCells(ctx context.Context, c RegisterCellsParams) (reconcile.Result, error) {
	resultBuilder := &results.Builder{}

	objMeta, err := meta.Accessor(c.EventObj)
	if err != nil {
		return resultBuilder.Error(err)
	}

	for name, lockserverSpec := range c.DesiredCells {
		params := lockserver.LocalConnectionParams(c.GlobalLockserver, lockserverSpec, objMeta.GetNamespace(), c.ClusterName, name)
		if params == nil {
			c.Recorder.Eventf(c.EventObj, corev1.EventTypeWarning, "TopoInvalid", "no local lockserver is defined for cell %v", name)
			continue
		}
		if params.Implementation != c.GlobalTopoImpl {
			c.Recorder.Eventf(c.EventObj, corev1.EventTypeWarning, "TopoInvalid", "local lockserver implementation for cell %v doesn't match global topo implementation", name)
			continue
		}
		updated := false
		err := c.TopoServer.UpdateCellInfoFields(ctx, name, func(cellInfo *topodata.CellInfo) error {
			// Skip the update if it already matches.
			if cellInfo.ServerAddress == params.Address && cellInfo.Root == params.RootPath {
				return topo.NewError(topo.NoUpdateNeeded, "")
			}
			cellInfo.ServerAddress = params.Address
			cellInfo.Root = params.RootPath
			updated = true
			return nil
		})
		if err != nil {
			// Record the error and continue trying other cells.
			c.Recorder.Eventf(c.EventObj, corev1.EventTypeWarning, "TopoUpdateFailed", "failed to update lockserver address for cell %v", name)
			resultBuilder.RequeueAfter(topoRequeueDelay)
		}
		if updated {
			c.Recorder.Eventf(c.EventObj, corev1.EventTypeNormal, "TopoUpdated", "updated lockserver addess for cell %v", name)
		}
	}

	return resultBuilder.Result()
}
