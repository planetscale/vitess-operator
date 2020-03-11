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

package lazy

import (
	corev1 "k8s.io/api/core/v1"
)

type EnvVars struct {
	providers []func(Spec) []corev1.EnvVar
}

func (e *EnvVars) Add(p func(Spec) []corev1.EnvVar) {
	e.providers = append(e.providers, p)
}

func (e *EnvVars) Get(spec Spec) []corev1.EnvVar {
	res := []corev1.EnvVar{}
	for _, p := range e.providers {
		res = append(res, p(spec)...)
	}
	return res
}
