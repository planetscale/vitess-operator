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

package vitesscluster

import (
	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	topodatapb "vitess.io/vitess/go/vt/proto/topodata"
)

func buildCellsAliases(desiredCells map[string]*planetscalev2.VitessCellTemplate) map[string]*topodatapb.CellsAlias {
	cellsAlias := make(map[string]*topodatapb.CellsAlias)
	for name := range desiredCells {
		alias := "planetscale_operator_default"
		if _, ok := cellsAlias[alias]; ok {
			cellsAlias[alias].Cells = append(cellsAlias[alias].Cells, name)
		} else {
			cells := topodatapb.CellsAlias{
				Cells: []string{
					name,
				},
			}
			cellsAlias[alias] = &cells
		}
	}
	return cellsAlias
}
