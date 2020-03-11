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

package vitesscluster

import (
	"testing"

	"github.com/stretchr/testify/assert"
	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	topodatapb "vitess.io/vitess/go/vt/proto/topodata"
)

func TestBuildCellAliases(t *testing.T) {
	awsuseast1Alias := topodatapb.CellsAlias{
		Cells: []string{
			"awsuseast1a",
			"awsuseast1b",
			"awsuseast1c",
		},
	}
	gcpuscentral1Alias := topodatapb.CellsAlias{
		Cells: []string{
			"gcpuscentral1a",
			"gcpuscentral1c",
			"gcpuscentral1f",
		},
	}
	awsCellAliases := map[string]*topodatapb.CellsAlias{
		"planetscale_operator_default": &awsuseast1Alias,
	}
	gcpCellAliases := map[string]*topodatapb.CellsAlias{
		"planetscale_operator_default": &gcpuscentral1Alias,
	}

	awsInput := map[string]*planetscalev2.LockserverSpec{
		"awsuseast1a": nil,
		"awsuseast1b": nil,
		"awsuseast1c": nil,
	}

	gcpInput := map[string]*planetscalev2.LockserverSpec{
		"gcpuscentral1a": nil,
		"gcpuscentral1c": nil,
		"gcpuscentral1f": nil,
	}

	results := buildCellsAliases(awsInput)
	for alias, cells := range awsCellAliases {
		assert.Contains(t, results, alias)
		for _, cell := range cells.Cells {
			assert.Contains(t, results[alias].Cells, cell)
		}
	}
	results = buildCellsAliases(gcpInput)
	for alias, cells := range gcpCellAliases {
		assert.Contains(t, results, alias)
		for _, cell := range cells.Cells {
			assert.Contains(t, results[alias].Cells, cell)
		}
	}
}
