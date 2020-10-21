/*
Copyright 2020 PlanetScale Inc.

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

package contenthash

import (
	"sort"

	corev1 "k8s.io/api/core/v1"
)

// ContainersUpdates returns a hex-encoded hash of a list of Containers.
//
// This is not guaranteed to detect changes in any part of the Container.
// We only try to detect certain changes that otherwise would not be applied by
// update.PodContainers(), such as when things are removed from lists in which
// extra items are usually tolerated because they might have been injected.
func ContainersUpdates(in []corev1.Container) string {
	m := map[string]string{}

	for i := range in {
		c := &in[i]
		m[c.Name] = ContainerUpdates(c)
	}

	return StringMap(m)
}

// ContainerUpdates returns a hex-encoded hash of a Container.
//
// This is not guaranteed to detect changes in any part of the Container.
// We only try to detect certain changes that otherwise would not be applied by
// update.PodContainers(), such as when things are removed from lists in which
// extra items are usually tolerated because they might have been injected.
func ContainerUpdates(in *corev1.Container) string {
	content := map[string]string{
		"VolumeMounts": volumeMountUpdates(in.VolumeMounts),
	}

	if len(in.Resources.Limits) > 0 {
		content["ResourceLimits"] = resourceListKeys(in.Resources.Limits)
	}
	if len(in.Resources.Requests) > 0 {
		content["ResourceRequests"] = resourceListKeys(in.Resources.Requests)
	}

	return StringMap(content)
}

func volumeMountUpdates(in []corev1.VolumeMount) string {
	// We only care if the unordered set of mount paths changes.
	mountPaths := make([]string, 0, len(in))

	for i := range in {
		mountPaths = append(mountPaths, in[i].MountPath)
	}

	sort.Strings(mountPaths)

	return StringList(mountPaths)
}

func resourceListKeys(in corev1.ResourceList) string {
	keys := make([]string, 0, len(in))
	for k := range in {
		keys = append(keys, string(k))
	}

	sort.Strings(keys)

	return StringList(keys)
}
