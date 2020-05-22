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

// Cell looks up an item in the Cells list by name.
// It returns a pointer to the item, or nil if the specified cell doesn't exist.
func (s *VitessClusterSpec) Cell(cellName string) *VitessCellTemplate {
	for i := range s.Cells {
		if s.Cells[i].Name == cellName {
			return &s.Cells[i]
		}
	}
	return nil
}

// ZoneMap returns a map from cell names to zone names.
func (s *VitessClusterSpec) ZoneMap() map[string]string {
	zones := make(map[string]string, len(s.Cells))
	for i := range s.Cells {
		cell := &s.Cells[i]
		zones[cell.Name] = cell.Zone
	}
	return zones
}

// Image returns the first mysqld flavor image that's set.
func (image *MysqldImage) Image() string {
	switch {
	case image.Mysql56Compatible != "":
		return image.Mysql56Compatible
	case image.Mysql80Compatible != "":
		return image.Mysql80Compatible
	case image.MariadbCompatible != "":
		return image.MariadbCompatible
	case image.Mariadb103Compatible != "":
		return image.Mariadb103Compatible
	default:
		return ""
	}
}

// Flavor returns Vitess flavor setting value
// for the first flavor that has an image set.
func (image *MysqldImage) Flavor() string {
	switch {
	case image.Mysql56Compatible != "":
		return "MySQL56"
	case image.Mysql80Compatible != "":
		return "MySQL80"
	case image.MariadbCompatible != "":
		return "MariaDB"
	case image.Mariadb103Compatible != "":
		return "MariaDB103"
	default:
		return ""
	}
}

func (externalOptions *ExternalVitessClusterUpdateStrategyOptions) Storage() bool {
	for _, resource := range externalOptions.AllowResourceChanges {
		if resource == "storage" {
			return true
			}
	 }

	return false
}