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

package k8s

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// IntStrPtr returns a pointer to an intstr.IntOrString
func IntStrPtr(i intstr.IntOrString) *intstr.IntOrString {
	return &i
}

// ProtocolPtr returns a pointer to a corev1.Protocol
func ProtocolPtr(p corev1.Protocol) *corev1.Protocol {
	return &p
}
