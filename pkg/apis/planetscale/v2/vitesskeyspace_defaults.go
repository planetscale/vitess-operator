/*
Copyright 2019 PlanetScale.

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

package v2

// DefaultVitessKeyspaceImages fills in unspecified keyspace-level images from cluster-level defaults.
// The clusterDefaults should have already had its unspecified fields filled in with operator defaults.
func DefaultVitessKeyspaceImages(dst *VitessKeyspaceImages, clusterDefaults *VitessImages) {
	if dst.Vttablet == "" {
		dst.Vttablet = clusterDefaults.Vttablet
	}
	if dst.Vtbackup == "" {
		dst.Vtbackup = clusterDefaults.Vtbackup
	}
	if dst.Mysqld == nil {
		dst.Mysqld = clusterDefaults.Mysqld
	}
	if dst.MysqldExporter == "" {
		dst.MysqldExporter = clusterDefaults.MysqldExporter
	}
}
