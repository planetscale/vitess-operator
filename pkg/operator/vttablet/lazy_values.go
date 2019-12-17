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
	"planetscale.dev/vitess-operator/pkg/operator/lazy"
)

var (
	// tabletAnnotations are the annotations to place on the vttablet Pod.
	tabletAnnotations lazy.StringMap

	// tabletEnvVars are the env vars for both the vttablet and mysqld containers.
	// We share many environment variables across both containers for consistency.
	// Some Vitess code that looks at these runs in both containers.
	tabletEnvVars lazy.EnvVars
	// vttabletEnvVars are extra env vars for only the vttablet container.
	vttabletEnvVars lazy.EnvVars
	// extraMyCnf is a list of file paths to put into the EXTRA_MY_CNF env var.
	extraMyCnf lazy.Strings

	// tabletInitContainers are the initContainers for the vttablet Pod.
	tabletInitContainers lazy.Containers

	// tabletVolumes are the volumes for the vttablet Pod.
	tabletVolumes lazy.Volumes
	// tabletVolumeMounts are the common volume mounts for both the vttablet and mysqld containers.
	// We mount many of the same things in both containers for consistency.
	tabletVolumeMounts lazy.VolumeMounts
	// vttabletVolumeMounts are the extra volume mounts for only the vttablet container.
	vttabletVolumeMounts lazy.VolumeMounts
	// mysqldVolumeMounts are the extra volume mounts for only the mysqld container.
	mysqldVolumeMounts lazy.VolumeMounts

	// vttabletFlags are the flags for vttablet.
	vttabletFlags lazy.VitessFlags
	// mysqlctldFlags are the flags for mysqlctld.
	mysqlctldFlags lazy.VitessFlags
	// vtbackupFlags are the flags for vtbackup.
	vtbackupFlags lazy.VitessFlags
)
