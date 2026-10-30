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

package vitessbackup

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
)

func TestRootKeyPrefix(t *testing.T) {
	if got, want := rootKeyPrefix("", "cluster"), "cluster"; got != want {
		t.Errorf("rootKeyPrefix() = %v; want %v", got, want)
	}
	if got, want := rootKeyPrefix("user/prefix/", "cluster"), "user/prefix/cluster"; got != want {
		t.Errorf("rootKeyPrefix() = %v; want %v", got, want)
	}
	if got, want := rootKeyPrefix("user/prefix", "cluster"), "user/prefix/cluster"; got != want {
		t.Errorf("rootKeyPrefix() = %v; want %v", got, want)
	}
}

// TestStorageFlagsUseDashes checks that backup storage flags use the dashed
// spelling, since Vitess removes the underscore spellings.
func TestStorageFlagsUseDashes(t *testing.T) {
	locations := map[string]*planetscalev2.VitessBackupLocation{
		"s3": {S3: &planetscalev2.S3BackupLocation{
			Region: "us-east-1", Bucket: "b", Endpoint: "https://s3.example.com", MinPartSize: 5 * 1024 * 1024,
		}},
		"gcs":    {GCS: &planetscalev2.GCSBackupLocation{Bucket: "b"}},
		"azblob": {Azblob: &planetscalev2.AzblobBackupLocation{Account: "a", Container: "c"}},
		"ceph":   {Ceph: &planetscalev2.CephBackupLocation{AuthSecret: planetscalev2.SecretSource{Name: "ceph", Key: "conf"}}},
		"file":   {Volume: &corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
	}
	for name, location := range locations {
		t.Run(name, func(t *testing.T) {
			flags := StorageFlags(location, "example")
			if _, ok := flags["backup-storage-implementation"]; !ok {
				t.Errorf("missing backup-storage-implementation in %v", flags)
			}
			for key := range flags {
				if strings.Contains(key, "_") {
					t.Errorf("flag %q uses underscores", key)
				}
			}
		})
	}
}
