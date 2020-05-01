package update

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/util/yaml"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
)

const (
	vitessKeyspaceBase = `
name: keyspace1
partitionings:
  - equal:
        parts: 2
        shardTemplate:
          databaseInitScriptSecret:
            key: init_db.sql
            name: init-script-secret
          tabletPools:
          - cell: cell1
            type: replica
            replicas: 3
            mysqld: {}
            dataVolumeClaimTemplate:
              accessModes: [ReadWriteOnce]
              resources:
                requests:
                  storage: 1Gi
          - cell: cell2
            type: rdonly
            replicas: 3
            mysqld: {}
            dataVolumeClaimTemplate:
              accessModes: [ReadWriteOnce]
              resources:
                requests:
                  storage: 1Gi
  - equal:
        parts: 1
        shardTemplate:
          databaseInitScriptSecret:
            key: init_db.sql
            name: init-script-secret
          tabletPools:
          - cell: cell3
            type: replica
            replicas: 3
            mysqld: {}
            dataVolumeClaimTemplate:
              accessModes: [ReadWriteOnce]
              resources:
                requests:
                  storage: 1Gi
`
	vitessKeyspaceUpdateAllPools = `
name: keyspace1
partitionings:
  - equal:
        parts: 2
        shardTemplate:
          databaseInitScriptSecret:
            key: init_db.sql
            name: init-script-secret
          tabletPools:
          - cell: cell1
            type: replica
            replicas: 3
            mysqld: {}
            dataVolumeClaimTemplate:
              accessModes: [ReadWriteOnce]
              resources:
                requests:
                  storage: 2Gi
          - cell: cell2
            type: rdonly
            replicas: 3
            mysqld: {}
            dataVolumeClaimTemplate:
              accessModes: [ReadWriteOnce]
              resources:
                requests:
                  storage: 300Gi
  - equal:
        parts: 1
        shardTemplate:
          databaseInitScriptSecret:
            key: init_db.sql
            name: init-script-secret
          tabletPools:
          - cell: cell3
            type: replica
            replicas: 3
            mysqld: {}
            dataVolumeClaimTemplate:
              accessModes: [ReadWriteOnce]
              resources:
                requests:
                  storage: 2Gi
`
	vitessKeyspaceUpdateSomePools = `
name: keyspace1
partitionings:
  - equal:
        parts: 2
        shardTemplate:
          databaseInitScriptSecret:
            key: init_db.sql
            name: init-script-secret
          tabletPools:
          - cell: cell1
            type: replica
            replicas: 3
            mysqld: {}
            dataVolumeClaimTemplate:
              accessModes: [ReadWriteOnce]
              resources:
                requests:
                  storage: 1Gi
          - cell: cell2
            type: rdonly
            replicas: 3
            mysqld: {}
            dataVolumeClaimTemplate:
              accessModes: [ReadWriteOnce]
              resources:
                requests:
                  storage: 2Gi
  - equal:
        parts: 1
        shardTemplate:
          databaseInitScriptSecret:
            key: init_db.sql
            name: init-script-secret
          tabletPools:
          - cell: cell3
            type: replica
            replicas: 3
            mysqld: {}
            dataVolumeClaimTemplate:
              accessModes: [ReadWriteOnce]
              resources:
                requests:
                  storage: 2Gi
`
	vitessKeyspaceNotMatchingBase = `
name: keyspace1
partitionings:
  - equal:
        parts: 1
        shardTemplate:
          databaseInitScriptSecret:
            key: init_db.sql
            name: init-script-secret
          tabletPools:
          - cell: cell1
            type: replica
            replicas: 3
            mysqld: {}
            dataVolumeClaimTemplate:
              accessModes: [ReadWriteOnce]
              resources:
                requests:
                  storage: 2Gi
  - equal:
        parts: 1
        shardTemplate:
          databaseInitScriptSecret:
            key: init_db.sql
            name: init-script-secret
          tabletPools:
          - cell: cell3
            type: replica
            replicas: 3
            mysqld: {}
            dataVolumeClaimTemplate:
              accessModes: [ReadWriteOnce]
              resources:
                requests:
                  storage: 2Gi
`)

func TestKeyspaceDiskSize(t *testing.T) {
	// Applying no changes to a keyspace template should function idempotently.
	keyspaceNoChanges := createVitessKeyspaceYAML(vitessKeyspaceBase)
	expectedKeyspaceNoChanges := createVitessKeyspaceYAML(vitessKeyspaceBase)

	KeyspaceDiskSize(keyspaceNoChanges, expectedKeyspaceNoChanges)
	if !reflect.DeepEqual(*keyspaceNoChanges, *expectedKeyspaceNoChanges) {
		t.Errorf("want: no disk size updates, got: disk size updates")
	}

	// Applying changes to all tablet pools should work as expected.
	keyspaceUpdateAllPools := createVitessKeyspaceYAML(vitessKeyspaceBase)
	expectedKeyspaceUpdateAllPools := createVitessKeyspaceYAML(vitessKeyspaceUpdateAllPools)

	KeyspaceDiskSize(keyspaceUpdateAllPools, expectedKeyspaceUpdateAllPools)
	if !reflect.DeepEqual(*keyspaceUpdateAllPools, *expectedKeyspaceUpdateAllPools) {
		t.Errorf("want: all disk size updates, got: none or some disk size updates")
	}

	// Applying changes to some tablet pools should work as expected.
	keyspaceUpdateSomePools := createVitessKeyspaceYAML(vitessKeyspaceBase)
	expectedKeyspaceUpdateSomePools := createVitessKeyspaceYAML(vitessKeyspaceUpdateSomePools)

	KeyspaceDiskSize(keyspaceUpdateSomePools, expectedKeyspaceUpdateSomePools)
	if !reflect.DeepEqual(*keyspaceUpdateSomePools, *expectedKeyspaceUpdateSomePools) {
		t.Errorf("want: some disk size updates, got: none or all disk size updates")
	}

	// Applying changes to keyspaces that aren't defined defined the same shouldn't work.
	keyspaceBase := createVitessKeyspaceYAML(vitessKeyspaceBase)
	keyspaceNoMatch := createVitessKeyspaceYAML(vitessKeyspaceNotMatchingBase)
	expectedKeyspaceNoMatch := createVitessKeyspaceYAML(vitessKeyspaceBase)

	KeyspaceDiskSize(keyspaceBase, keyspaceNoMatch)
	if !reflect.DeepEqual(*keyspaceBase, *expectedKeyspaceNoMatch) {
		t.Errorf("want: no disk size updates, got: disk size updates")
	}
}

func createVitessKeyspaceYAML(vtkYAML string) *planetscalev2.VitessKeyspaceTemplate{
	vtk := &planetscalev2.VitessKeyspaceTemplate{}
	mustDecodeYAML(vtkYAML, vtk)
	return vtk
}

// We recreate the function here from our framework test to avoid an import cycle.
func mustDecodeYAML(yamlStr string, into interface{}) {
	if err := yaml.NewYAMLOrJSONDecoder(bytes.NewReader([]byte(yamlStr)), 0).Decode(into); err != nil {
		panic(fmt.Errorf("can't decode YAML: %v", err))
	}
}