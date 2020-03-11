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

// SecretSource specifies where to find the data for a particular secret value.
type SecretSource struct {
	// Name is the name of a Kubernetes Secret object to use as the data source.
	// The Secret must be in the same namespace as the VitessCluster.
	//
	// The 'key' field defines the item to pick from the Secret object's 'data'
	// map.
	//
	// If a Secret name is not specified, the data source must be defined
	// with the 'volumeName' field instead.
	Name string `json:"name,omitempty"`

	// VolumeName directly specifies the name of a Volume in each Pod that
	// should be mounted. You must ensure a Volume by that name exists in all
	// relevant Pods, such as by using the appropriate ExtraVolumes fields.
	// If specified, this takes precedence over the 'name' field.
	//
	// The 'key' field defines the name of the file to load within this Volume.
	VolumeName string `json:"volumeName,omitempty"`

	// Key is the name of the item within the data source to use as the value.
	//
	// For a Kubernetes Secret object (specified with the 'name' field),
	// this is the key within the 'data' map.
	//
	// When 'volumeName' is used, this specifies the name of the file to load
	// within that Volume.
	Key string `json:"key"`
}

// IsSet returns true if at least one source is set.
func (s *SecretSource) IsSet() bool {
	return s.Key != "" && (s.Name != "" || s.VolumeName != "")
}
