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

package update

import (
	corev1 "k8s.io/api/core/v1"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
)

// ServiceOverrides applies the specified overrides (if any) to the given Service.
func ServiceOverrides(svc *corev1.Service, so *planetscalev2.ServiceOverrides) {
	if so == nil {
		return
	}
	if len(so.Annotations) > 0 {
		Annotations(&svc.Annotations, so.Annotations)
	}
	if so.ClusterIP != "" {
		svc.Spec.ClusterIP = so.ClusterIP
	}
}

// InPlaceServiceOverrides applies only the overrides that are safe to update in-place.
func InPlaceServiceOverrides(svc *corev1.Service, so *planetscalev2.ServiceOverrides) {
	if so == nil {
		return
	}
	if len(so.Annotations) > 0 {
		Annotations(&svc.Annotations, so.Annotations)
	}
}
