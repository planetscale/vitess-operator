package vitesstopo

import (
	"context"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"vitess.io/vitess/go/vt/proto/topodata"
	"vitess.io/vitess/go/vt/topo"

	v2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
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

func RegisterCells(ctx context.Context, vt *v2.VitessCluster, ts *topo.Server, recorder *record.EventRecorder, globalTopoImpl string, desiredCells map[string]*v2.VitessCellTemplate) (reconcile.Result, error) {
	resultBuilder := &results.Builder{}

	for name, cell := range desiredCells {
		params := lockserver.LocalConnectionParams(&vt.Spec.GlobalLockserver, &cell.Lockserver, vt.Name, cell.Name)
		if params == nil {
			(*recorder).Eventf(vt, v1.EventTypeWarning, "TopoInvalid", "no local lockserver is defined for cell %v", name)
			continue
		}
		if params.Implementation != globalTopoImpl {
			(*recorder).Eventf(vt, v1.EventTypeWarning, "TopoInvalid", "local lockserver implementation for cell %v doesn't match global topo implementation", name)
			continue
		}
		updated := false
		err := ts.UpdateCellInfoFields(ctx, name, func(cellInfo *topodata.CellInfo) error {
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
			(*recorder).Eventf(vt, v1.EventTypeWarning, "TopoUpdateFailed", "failed to update lockserver address for cell %v", name)
			resultBuilder.RequeueAfter(topoRequeueDelay)
		}
		if updated {
			(*recorder).Eventf(vt, v1.EventTypeNormal, "TopoUpdated", "updated lockserver addess for cell %v", name)
		}
	}

	return resultBuilder.Result()
}
