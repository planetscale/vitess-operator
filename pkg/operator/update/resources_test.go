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

// TestKeyspaceDiskSizeNoChanges tests that applying the same keyspace template to a keyspace
// template using KeyspaceDiskSize results in no updates.
func TestKeyspaceDiskSizeNoChanges(t *testing.T) {
	keyspaceNoChanges := vitessKeyspaceFromYAML(vitessKeyspaceBase)
	expectedKeyspaceNoChanges := vitessKeyspaceFromYAML(vitessKeyspaceBase)

	KeyspaceDiskSize(keyspaceNoChanges, expectedKeyspaceNoChanges)
	if !reflect.DeepEqual(*keyspaceNoChanges, *expectedKeyspaceNoChanges) {
		t.Errorf("want: no disk size updates, got: disk size updates")
	}
}

// TestKeyspaceDiskSizeAllPools tests that applying updates to all tablet pools in a keyspace
// template using KeyspaceDiskSize results in all the required updates.
func TestKeyspaceDiskSizeAllPools(t *testing.T) {
	// Applying changes to all tablet pools should work as expected.
	keyspaceUpdateAllPools := vitessKeyspaceFromYAML(vitessKeyspaceBase)
	expectedKeyspaceUpdateAllPools := vitessKeyspaceFromYAML(vitessKeyspaceUpdateAllPools)

	KeyspaceDiskSize(keyspaceUpdateAllPools, expectedKeyspaceUpdateAllPools)
	if !reflect.DeepEqual(*keyspaceUpdateAllPools, *expectedKeyspaceUpdateAllPools) {
		t.Errorf("want: all disk size updates, got: none or some disk size updates")
	}
}

// TestKeyspaceDiskSizeSomePools tests that applying updates to some (but not all) tablet pools in a keyspace
// template using KeyspaceDiskSize results in only the requested tablet pools being updated.
func TestKeyspaceDiskSizeSomePools(t *testing.T) {
	// Applying changes to some tablet pools should work as expected.
	keyspaceUpdateSomePools := vitessKeyspaceFromYAML(vitessKeyspaceBase)
	expectedKeyspaceUpdateSomePools := vitessKeyspaceFromYAML(vitessKeyspaceUpdateSomePools)

	KeyspaceDiskSize(keyspaceUpdateSomePools, expectedKeyspaceUpdateSomePools)
	if !reflect.DeepEqual(*keyspaceUpdateSomePools, *expectedKeyspaceUpdateSomePools) {
		t.Errorf("want: some disk size updates, got: none or all disk size updates")
	}
}

// TestKeyspaceDiskSizeMatch tests that using a non-isometric keyspace to apply updates to a keyspace
// template using KeyspaceDiskSize results in no updates.
func TestKeyspaceDiskSizeNoMatch(t *testing.T) {
	// Applying changes to keyspaces that aren't defined defined the same shouldn't work.
	keyspaceBase := vitessKeyspaceFromYAML(vitessKeyspaceBase)
	keyspaceNoMatch := vitessKeyspaceFromYAML(vitessKeyspaceNotMatchingBase)
	expectedKeyspaceNoMatch := vitessKeyspaceFromYAML(vitessKeyspaceBase)

	KeyspaceDiskSize(keyspaceBase, keyspaceNoMatch)
	if !reflect.DeepEqual(*keyspaceBase, *expectedKeyspaceNoMatch) {
		t.Errorf("want: no disk size updates, got: disk size updates")
	}
}

func vitessKeyspaceFromYAML(vtkYAML string) *planetscalev2.VitessKeyspaceTemplate{
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