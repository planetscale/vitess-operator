/*
Copyright 2026 PlanetScale Inc.

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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	topodatapb "vitess.io/vitess/go/vt/proto/topodata"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
)

// testFlagsSpec returns a tablet spec that turns on as many operator-generated
// flags as possible: a local mysqld, S3 backups, and the xtrabackup engine.
func testFlagsSpec() *Spec {
	return &Spec{
		Alias:        topodatapb.TabletAlias{Cell: "zone1", Uid: 101},
		AliasStr:     "zone1-0000000101",
		Type:         planetscalev2.ReplicaPoolType,
		Labels:       map[string]string{planetscalev2.ClusterLabel: "example"},
		KeyspaceName: "commerce",
		Images: planetscalev2.VitessKeyspaceImages{
			Vttablet: "vitess/lite:v24.0.5",
			Vtbackup: "vitess/lite:v24.0.5",
			Mysqld:   &planetscalev2.MysqldImage{Mysql80Compatible: "mysql:8.0.40"},
		},
		Vttablet: &planetscalev2.VttabletSpec{},
		Mysqld:   &planetscalev2.MysqldSpec{},
		GlobalLockserver: planetscalev2.VitessLockserverParams{
			Implementation: "etcd2",
			Address:        "etcd:2379",
			RootPath:       "/vitess/example/global",
		},
		BackupLocation: &planetscalev2.VitessBackupLocation{
			S3: &planetscalev2.S3BackupLocation{
				Region:      "us-east-1",
				Bucket:      "backups",
				Endpoint:    "https://s3.example.com",
				MinPartSize: 5 * 1024 * 1024,
			},
		},
		BackupEngine: planetscalev2.VitessBackupEngineXtraBackup,
	}
}

func containerArgs(t *testing.T, containers []corev1.Container, name string) []string {
	t.Helper()
	for i := range containers {
		if containers[i].Name == name {
			return containers[i].Args
		}
	}
	require.Failf(t, "container not found", "no %q container", name)
	return nil
}

// assertDashedFlags checks that every flag in args uses the dashed spelling,
// since Vitess removes the underscore spellings, and that the deprecated glog
// flags are gone.
func assertDashedFlags(t *testing.T, args []string) {
	t.Helper()
	for _, arg := range args {
		name, _, _ := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		assert.NotContains(t, name, "_", "flag %q", arg)
		assert.NotEqual(t, "logtostderr", name, "flag %q", arg)
	}
}

func TestPodFlagsUseDashes(t *testing.T) {
	key := client.ObjectKey{Namespace: "default", Name: "example-vttablet-zone1-101"}

	t.Run("local mysqld", func(t *testing.T) {
		pod := NewPod(key, testFlagsSpec())
		vttabletArgs := containerArgs(t, pod.Spec.Containers, vttabletContainerName)
		assert.Contains(t, vttabletArgs, "--init-keyspace=commerce")
		assertDashedFlags(t, vttabletArgs)
		mysqlctldArgs := containerArgs(t, pod.Spec.Containers, MysqldContainerName)
		assert.Contains(t, mysqlctldArgs, "--tablet-uid=101")
		assertDashedFlags(t, mysqlctldArgs)
	})

	t.Run("external datastore", func(t *testing.T) {
		spec := testFlagsSpec()
		spec.Type = planetscalev2.ExternalReplicaPoolType
		spec.Mysqld = nil
		spec.ExternalDatastore = &planetscalev2.ExternalDatastore{
			User:               "vt",
			Host:               "mysql.example.com",
			Port:               3306,
			Database:           "commerce",
			CredentialsSecret:  planetscalev2.SecretSource{Name: "creds", Key: "password"},
			ServerCACertSecret: &planetscalev2.SecretSource{Name: "ca", Key: "ca.pem"},
		}
		vttabletArgs := containerArgs(t, NewPod(key, spec).Spec.Containers, vttabletContainerName)
		assert.Contains(t, vttabletArgs, "--db-host=mysql.example.com")
		assertDashedFlags(t, vttabletArgs)
	})
}

func TestBackupPodFlagsUseDashes(t *testing.T) {
	pod := NewBackupPod(client.ObjectKey{Namespace: "default", Name: "backup"}, &BackupSpec{TabletSpec: testFlagsSpec()}, "mysql:8.0.40")
	args := containerArgs(t, pod.Spec.Containers, vtbackupContainerName)
	assert.Contains(t, args, "--s3-backup-storage-bucket=backups")
	assertDashedFlags(t, args)
}

// TestExtraFlagsReplaceEitherSpelling checks that a user-provided flag replaces
// the operator's value whichever spelling it uses, and that the container never
// gets both spellings of one flag.
func TestExtraFlagsReplaceEitherSpelling(t *testing.T) {
	key := client.ObjectKey{Namespace: "default", Name: "example-vttablet-zone1-101"}

	t.Run("vttablet", func(t *testing.T) {
		spec := testFlagsSpec()
		spec.Vttablet.ExtraFlags = map[string]string{
			"db_charset":                             "latin1",
			"builtinbackup_incremental_restore_path": "/backups",
			// The operator derives this from the mysqld image, so it still wins.
			"mysql_server_version": "5.7.0-Vitess",
		}
		args := containerArgs(t, NewPod(key, spec).Spec.Containers, vttabletContainerName)
		assert.Contains(t, args, "--db_charset=latin1")
		assert.Contains(t, args, "--builtinbackup_incremental_restore_path=/backups")
		assert.Contains(t, args, "--mysql-server-version=8.0.40-Vitess")
		for _, arg := range args {
			assert.False(t, strings.HasPrefix(arg, "--db-charset="), "unexpected %q", arg)
			assert.False(t, strings.HasPrefix(arg, "--builtinbackup-incremental-restore-path="), "unexpected %q", arg)
			assert.False(t, strings.HasPrefix(arg, "--mysql_server_version="), "unexpected %q", arg)
		}
	})

	t.Run("vtbackup", func(t *testing.T) {
		spec := testFlagsSpec()
		spec.Vttablet.VtbackupExtraFlags = map[string]string{
			"--xtrabackup_stripes":                   "4",
			"builtinbackup_incremental_restore_path": "/backups",
			// vtbackupExtraFlags only adds flags: the base vtbackup flags
			// still win, as before, but never in two spellings.
			"db_charset": "latin1",
		}
		pod := NewBackupPod(client.ObjectKey{Namespace: "default", Name: "backup"}, &BackupSpec{TabletSpec: spec}, "mysql:8.0.40")
		args := containerArgs(t, pod.Spec.Containers, vtbackupContainerName)
		assert.Contains(t, args, "--xtrabackup_stripes=4")
		assert.Contains(t, args, "--builtinbackup_incremental_restore_path=/backups")
		assert.Contains(t, args, "--db-charset=utf8mb4")
		for _, arg := range args {
			assert.False(t, strings.HasPrefix(arg, "--xtrabackup-stripes="), "unexpected %q", arg)
			assert.False(t, strings.HasPrefix(arg, "--builtinbackup-incremental-restore-path="), "unexpected %q", arg)
			assert.False(t, strings.HasPrefix(arg, "--db_charset="), "unexpected %q", arg)
		}
	})
}

// oldOperatorArgs respells a command line the way older operator versions
// wrote it: underscore flag names, plus --logtostderr.
func oldOperatorArgs(args []string) []string {
	old := []string{"--logtostderr=true"}
	for _, arg := range args {
		name, value, _ := strings.Cut(strings.TrimPrefix(arg, "--"), "=")
		old = append(old, "--"+strings.ReplaceAll(name, "-", "_")+"="+value)
	}
	return old
}

func setContainerArgs(containers []corev1.Container, name string, args []string) {
	for i := range containers {
		if containers[i].Name == name {
			containers[i].Args = args
		}
	}
}

// TestUpdatePodKeepsRespelledArgs checks that a Pod whose command lines only
// differ in flag spelling, as written by an older operator, is left alone, and
// that a new image still brings the new command line.
func TestUpdatePodKeepsRespelledArgs(t *testing.T) {
	key := client.ObjectKey{Namespace: "default", Name: "example-vttablet-zone1-101"}
	spec := testFlagsSpec()
	live := NewPod(key, spec)
	for _, name := range []string{vttabletContainerName, MysqldContainerName} {
		setContainerArgs(live.Spec.Containers, name, oldOperatorArgs(containerArgs(t, live.Spec.Containers, name)))
	}

	updated := live.DeepCopy()
	UpdatePod(updated, spec)
	assert.Equal(t, live, updated)

	// A new vttablet image brings the new command lines, also for mysqlctld,
	// which runs a binary copied from that image.
	spec.Images.Vttablet = "vitess/lite:v25.0.0"
	updated = live.DeepCopy()
	UpdatePod(updated, spec)
	assertDashedFlags(t, containerArgs(t, updated.Spec.Containers, vttabletContainerName))
	assertDashedFlags(t, containerArgs(t, updated.Spec.Containers, MysqldContainerName))

	// A mutable tag gets the new command lines right away, since it can start
	// a newer binary without changing.
	spec.Images.Vttablet = "vitess/lite:latest"
	live = NewPod(key, spec)
	for _, name := range []string{vttabletContainerName, MysqldContainerName} {
		setContainerArgs(live.Spec.Containers, name, oldOperatorArgs(containerArgs(t, live.Spec.Containers, name)))
	}
	updated = live.DeepCopy()
	UpdatePod(updated, spec)
	assertDashedFlags(t, containerArgs(t, updated.Spec.Containers, vttabletContainerName))
	assertDashedFlags(t, containerArgs(t, updated.Spec.Containers, MysqldContainerName))
}
