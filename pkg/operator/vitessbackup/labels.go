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
	// LocationLabel is the label key for the backup storage location name.
	LocationLabel = "backup.planetscale.com/location"
	// TypeLabel is the label key for the type of a backup.
	TypeLabel = "backup.planetscale.com/type"

	// TypeInit is a backup taken to initialize an empty shard.
	TypeInit = "init"
	// TypeFirstBackup is a backup taken when no other backup exist in an existing shard.
	TypeFirstBackup = "empty"
	// TypeUpdate is a backup taken to update the latest backup for a shard.
	TypeUpdate = "update"
)
