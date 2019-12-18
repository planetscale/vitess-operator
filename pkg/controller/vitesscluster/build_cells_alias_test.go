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

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"github.com/stretchr/testify/assert"
	topodatapb "vitess.io/vitess/go/vt/proto/topodata"
)

func makeCellTemplate(name string, zone string) *planetscalev2.VitessCellTemplate {
	cellTemplate := planetscalev2.VitessCellTemplate{
		Name: name,
		Zone: zone,
	}
	return &cellTemplate
}

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

	awsInput := map[string]*planetscalev2.VitessCellTemplate{
		"awsuseast1a": makeCellTemplate("awsuseast1a", "us-east-1a"),
		"awsuseast1b": makeCellTemplate("awsuseast1b", "us-east-1b"),
		"awsuseast1c": makeCellTemplate("awsuseast1c", "us-east-1c"),
	}

	gcpInput := map[string]*planetscalev2.VitessCellTemplate{
		"gcpuscentral1a": makeCellTemplate("awsuseast1a", "us-central1-a"),
		"gcpuscentral1c": makeCellTemplate("awsuseast1c", "us-central1-c"),
		"gcpuscentral1f": makeCellTemplate("awsuseast1f", "us-central1-f"),
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
