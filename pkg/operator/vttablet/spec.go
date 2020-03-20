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

package vttablet

import (
	"fmt"

	topodatapb "vitess.io/vitess/go/vt/proto/topodata"

	corev1 "k8s.io/api/core/v1"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
)

// Spec specifies all the internal parameters needed to deploy a vttablet instance.
type Spec struct {
	Alias                    topodatapb.TabletAlias
	AliasStr                 string
	Type                     planetscalev2.VitessTabletPoolType
	Zone                     string
	Labels                   map[string]string
	Images                   planetscalev2.VitessKeyspaceImages
	ImagePullPolicies        planetscalev2.VitessImagePullPolicies
	Index                    int32
	KeyRange                 planetscalev2.VitessKeyRange
	KeyspaceName             string
	Vttablet                 *planetscalev2.VttabletSpec
	Mysqld                   *planetscalev2.MysqldSpec
	ExternalDatastore        *planetscalev2.ExternalDatastore
	DataVolumePVCSpec        *corev1.PersistentVolumeClaimSpec
	DataVolumePVCName        string
	GlobalLockserver         planetscalev2.VitessLockserverParams
	DatabaseInitScriptSecret planetscalev2.SecretSource
	EnableSemiSync           bool
	Annotations              map[string]string
	ExtraLabels              map[string]string
	BackupLocation           *planetscalev2.VitessBackupLocation
	BackupEngine             planetscalev2.VitessBackupEngine
	Affinity                 *corev1.Affinity
	ExtraEnv                 []corev1.EnvVar
	ExtraVolumes             []corev1.Volume
	ExtraVolumeMounts        []corev1.VolumeMount
	InitContainers           []corev1.Container
}

// localDatabaseName returns the MySQL database name for a tablet Spec in the case of locally managed MySQL.
func (spec *Spec) localDatabaseName() string {
	return "vt_" + spec.KeyspaceName
}

// shardLabels returns only the labels needed to select Pods in the same shard.
func (spec *Spec) shardLabels() map[string]string {
	return map[string]string{
		planetscalev2.ComponentLabel: spec.Labels[planetscalev2.ComponentLabel],
		planetscalev2.ClusterLabel:   spec.Labels[planetscalev2.ClusterLabel],
		planetscalev2.KeyspaceLabel:  spec.Labels[planetscalev2.KeyspaceLabel],
		planetscalev2.ShardLabel:     spec.Labels[planetscalev2.ShardLabel],
	}
}

// poolLabels returns the labels to select Pods in the same tablet pool.
func (spec *Spec) poolLabels() map[string]string {
	labels := spec.shardLabels()
	labels[planetscalev2.CellLabel] = spec.Labels[planetscalev2.CellLabel]
	labels[planetscalev2.TabletTypeLabel] = spec.Labels[planetscalev2.TabletTypeLabel]
	return labels
}

// tabletDir returns the path to the tablet dir.
// This path must match what Vitess generates:
// https://github.com/vitessio/vitess/blob/3d3e63d0/go/vt/mysqlctl/mycnf_gen.go#L93
func (spec *Spec) tabletDir() string {
	return fmt.Sprintf("%s/vt_%010d", vtDataRootPath, spec.Alias.Uid)
}

// myCnfFilePath returns the path to the my.cnf file for vttablet.
func (spec *Spec) myCnfFilePath() string {
	return spec.tabletDir() + "/my.cnf"
}

// mysqldUnixSocketPath returns the path to the mysqld socket file.
func (spec *Spec) mysqldUnixSocketPath() string {
	return spec.tabletDir() + "/mysql.sock"
}

// dbConfigCharset returns the charset for vttablet's mysql connection pools.
func (spec *Spec) dbConfigCharset() string {
	// For flavors that we know are 8.0-compatible, use the new default charset
	// that Vitess switched to for 8.0+.
	if spec.Images.Mysqld != nil && spec.Images.Mysqld.Mysql80Compatible != "" {
		return defaultMySQL80Charset
	}

	// For all others, use the old default that Vitess had since 5.6 until 8.0.
	return defaultMySQL56Charset
}
