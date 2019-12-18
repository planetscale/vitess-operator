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

package update

import (
	corev1 "k8s.io/api/core/v1"
)

// Volumes updates entries in 'dst' based on the values in 'src'.
// It leaves extra entries (found in 'dst' but not in 'src') untouched,
// since those might be set by mutating admission webhooks or other controllers.
func Volumes(dst *[]corev1.Volume, src []corev1.Volume) {
srcLoop:
	for srcIndex := range src {
		srcObj := &src[srcIndex]
		// If this item is already there, update it.
		for dstIndex := range *dst {
			dstObj := &(*dst)[dstIndex]
			if dstObj.Name == srcObj.Name {
				*dstObj = *srcObj
				continue srcLoop
			}
		}
		// Otherwise, append it.
		*dst = append(*dst, *srcObj)
	}
}

// VolumeMounts updates entries in 'dst' based on the values in 'src'.
// It leaves extra entries (found in 'dst' but not in 'src') untouched,
// since those might be set by mutating admission webhooks or other controllers.
func VolumeMounts(dst *[]corev1.VolumeMount, src []corev1.VolumeMount) {
srcLoop:
	for srcIndex := range src {
		srcObj := &src[srcIndex]
		// If this item is already there, update it.
		for dstIndex := range *dst {
			dstObj := &(*dst)[dstIndex]
			if dstObj.MountPath == srcObj.MountPath {
				*dstObj = *srcObj
				continue srcLoop
			}
		}
		// Otherwise, append it.
		*dst = append(*dst, *srcObj)
	}
}

// Containers updates entries in 'dst' based on the values in 'src'.
// It leaves extra entries (found in 'dst' but not in 'src') untouched,
// since those might be set by mutating admission webhooks or other controllers.
func Containers(dst *[]corev1.Container, src []corev1.Container) {
srcLoop:
	for srcIndex := range src {
		srcObj := &src[srcIndex]
		// If this item is already there, update it.
		for dstIndex := range *dst {
			dstObj := &(*dst)[dstIndex]
			if dstObj.Name == srcObj.Name {
				Container(dstObj, srcObj)
				continue srcLoop
			}
		}
		// Otherwise, append it.
		*dst = append(*dst, *srcObj)
	}
}

// Container updates entries in 'dst' based on the values in 'src'.
// It leaves extra entries (found in 'dst' but not in 'src') untouched
// for certain fields of Container, since those might be set by mutating
// admission webhooks, other controllers, or the API server.
func Container(dst, src *corev1.Container) {
	// Save fields that need to be recursively merged.
	dstVolumeMounts := dst.VolumeMounts
	VolumeMounts(&dstVolumeMounts, src.VolumeMounts)

	dstResources := dst.Resources
	ResourceRequirements(&dstResources, &src.Resources)

	dstSecurityContext := dst.SecurityContext
	SecurityContext(&dstSecurityContext, src.SecurityContext)

	// Overwrite everything we didn't specifically save.
	*dst = *src

	// Restore saved fields.
	dst.VolumeMounts = dstVolumeMounts
	dst.Resources = dstResources
	dst.SecurityContext = dstSecurityContext
}

// SecurityContext updates entries in 'dst' based on the values in 'src'.
// It leaves extra entries (found in 'dst' but not in 'src') untouched
// for certain fields of SecurityContext, since those might be set by mutating
// admission webhooks, other controllers, or the API server.
func SecurityContext(dst **corev1.SecurityContext, src *corev1.SecurityContext) {
	if *dst == nil || src == nil {
		// Only one side is set, so we don't need to merge anything.
		*dst = src
		return
	}

	// Save some original values.
	dstProcMount := (*dst).ProcMount

	// Copy everything else.
	**dst = *src

	// Restore saved values if the src didn't set them.
	if (*dst).ProcMount == nil {
		(*dst).ProcMount = dstProcMount
	}
}

// ResourceRequirements updates entries in 'dst' based on the values in 'src'.
// It leaves extra entries (found in 'dst' but not in 'src') untouched,
// since those might be set by mutating admission webhooks or other controllers.
func ResourceRequirements(dst, src *corev1.ResourceRequirements) {
	ResourceList(&dst.Limits, &src.Limits)
	ResourceList(&dst.Requests, &src.Requests)
}

// ResourceList updates entries in 'dst' based on the values in 'src'.
// It leaves extra entries (found in 'dst' but not in 'src') untouched,
// since those might be set by mutating admission webhooks or other controllers.
func ResourceList(dst, src *corev1.ResourceList) {
	if *dst == nil {
		if len(*src) > 0 {
			*dst = *src
		}
		return
	}
	for srcKey, srcVal := range *src {
		(*dst)[srcKey] = srcVal
	}
}

// LocalObjectReferences updates entries in 'dst' based on the values in 'src'.
// It leaves extra entries (found in 'dst' but not in 'src') untouched,
// since those might be set by mutating admission webhooks or other controllers.
func LocalObjectReferences(dst *[]corev1.LocalObjectReference, src []corev1.LocalObjectReference) {
srcLoop:
	for srcIndex := range src {
		srcObj := &src[srcIndex]
		// If this item is already there, update it.
		for dstIndex := range *dst {
			dstObj := &(*dst)[dstIndex]
			if dstObj.Name == srcObj.Name {
				*dstObj = *srcObj
				continue srcLoop
			}
		}
		// Otherwise, append it.
		*dst = append(*dst, *srcObj)
	}
}

// Env updates entries in 'dst' based on the values in 'src'.
// It leaves extra entries (found in 'dst' but not in 'src') untouched,
// since those might be set by mutating admission webhooks or other controllers.
func Env(dst *[]corev1.EnvVar, src []corev1.EnvVar) {
srcLoop:
	for srcIndex := range src {
		srcObj := &src[srcIndex]
		// If this item is already there, update it.
		for dstIndex := range *dst {
			dstObj := &(*dst)[dstIndex]
			if dstObj.Name == srcObj.Name {
				*dstObj = *srcObj
				continue srcLoop
			}
		}
		// Otherwise, append it.
		*dst = append(*dst, *srcObj)
	}
}
