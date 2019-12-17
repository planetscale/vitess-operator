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

const (
	gcsBackupStorageImplementationName = "gcs"
	gcsAuthSecretVolumeName            = "gcs-auth"
	gcsAuthSecretMountPath             = "/vt/gcs-auth"
	gcsAuthSecretFileName              = "key.json"
	gcsAuthSecretFilePath              = gcsAuthSecretMountPath + "/" + gcsAuthSecretFileName

	s3BackupStorageImplementationName = "s3"
	s3AuthSecretVolumeName            = "s3-auth"
	s3AuthSecretMountPath             = "/home/vitess/.aws"
	s3AuthSecretFileName              = "credentials"
	s3AuthSecretFilePath              = s3AuthSecretMountPath + "/" + s3AuthSecretFileName

	fileBackupStorageImplementationName = "file"
	fileBackupStorageVolumeName         = "vitess-backups"
	fileBackupStorageMountPath          = "/vt/backups"
)
