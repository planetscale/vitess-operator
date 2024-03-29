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

package lockserver

import (
	"fmt"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
)

const (
	// EtcdClientPort is the port for clients to connect to etcd.
	EtcdClientPort = 2379

	// VitessEtcdImplementationName is the topo plugin name to give Vitess to tell it to
	// connect to an etcd cluster as the lockserver (topology store).
	VitessEtcdImplementationName = "etcd2"
)

// GlobalConnectionParams returns the Vitess connection parameters for a
// VitessCluster's global lockserver.
func GlobalConnectionParams(lockSpec *planetscalev2.LockserverSpec, namespace, clusterName string) *planetscalev2.VitessLockserverParams {
	switch {
	case lockSpec.External != nil:
		return lockSpec.External
	case lockSpec.Etcd != nil:
		address := lockSpec.CellInfoAddress
		if len(address) == 0 {
			address = fmt.Sprintf("%s-client.%s.svc:%d", GlobalEtcdName(clusterName), namespace, EtcdClientPort)
		}
		return &planetscalev2.VitessLockserverParams{
			Implementation: VitessEtcdImplementationName,
			Address:        address,
			RootPath:       fmt.Sprintf("/vitess/%s/global", clusterName),
		}
	default:
		return nil
	}
}

// LocalConnectionParams returns the Vitess connection parameters for a
// VitessCluster cell's local lockserver.
func LocalConnectionParams(globalLockserverSpec, cellLockserverSpec *planetscalev2.LockserverSpec, namespace, clusterName, cellName string) *planetscalev2.VitessLockserverParams {
	// The addition of "/local/" is important in case the cell name happens to be "global".
	rootPath := fmt.Sprintf("/vitess/%s/local/%s", clusterName, cellName)

	switch {
	case cellLockserverSpec.External != nil:
		return cellLockserverSpec.External
	case cellLockserverSpec.Etcd != nil:
		address := cellLockserverSpec.CellInfoAddress
		if len(address) == 0 {
			// Point to the client Service created by the local EtcdCluster.
			address = fmt.Sprintf("%s-client.%s.svc:%d", LocalEtcdName(clusterName, cellName), namespace, EtcdClientPort)
		}
		return &planetscalev2.VitessLockserverParams{
			Implementation: VitessEtcdImplementationName,
			Address:        address,
			RootPath:       rootPath,
		}
	default:
		// No local lockserver was specified.
		// Share the global lockserver with a cell-specific RootPath.
		globalParams := GlobalConnectionParams(globalLockserverSpec, namespace, clusterName)
		if globalParams == nil {
			return nil
		}
		return &planetscalev2.VitessLockserverParams{
			Implementation: globalParams.Implementation,
			Address:        globalParams.Address,
			RootPath:       rootPath,
		}
	}
}
