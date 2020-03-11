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

package v2

// DefaultVitessShard fills in VitessShard defaults for unspecified fields.
// Note: This should only be used for nillable fields passed down from a parent because controllers run in parallel,
// and the defaulting code for a parent object may not have been run yet, meaning the values passed down from that parent
// might not be safe to deref.
func DefaultVitessShard(dst *VitessShard) {
	DefaultTopoReconcileConfig(&dst.Spec.TopologyReconciliation)
}
