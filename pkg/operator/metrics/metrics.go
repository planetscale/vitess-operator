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

package metrics

import (
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	// Namespace is the metric namespace for all operator metrics.
	Namespace = "vitess_operator"

	// ClusterLabel is the label whose value gives the name of a VitessCluster.
	ClusterLabel = "cluster"
	// CellLabel is the label whose value gives the name of a Vitess cell.
	CellLabel = "cell"
	// KeyspaceLabel is the label whose value gives the name of a Vitess keyspace.
	KeyspaceLabel = "keyspace"
	// ShardLabel is the label whose value gives the name of a Vitess shard.
	ShardLabel = "shard"
	// BackupStorageLabel is the label whose value gives the name of a VitessBackupStorage object.
	BackupStorageLabel = "backup_storage"

	// ResultLabel is a common metrics label for the success/failure of an operation.
	ResultLabel = "result"
	// ResultSuccess indicates the operation succeeded.
	ResultSuccess = "success"
	// ResultError indicates an error occurred.
	ResultError = "error"
)

// Registry is the Prometheus registry for all operator metrics.
var Registry = metrics.Registry

// Result returns the appropriate ResultLabel value for an error.
func Result(err error) string {
	if err != nil {
		return ResultError
	}
	return ResultSuccess
}
