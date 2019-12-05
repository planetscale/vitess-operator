/*
Copyright 2019 PlanetScale.

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

/*
Package conditions has functions for working with status conditions in various Kubernetes APIs.

Status conditions are typically different Go types for every Kubernetes API,
so there's no way to do this generically without reflection.
Instead of reflection, we just hard-code a few helpers for the types we need.

There are sometimes upstream helpers for working with these types, but they're
usually not in a place that's meant to be shared with other projects.
*/
package conditions

import (
	appsv1 "k8s.io/api/apps/v1"
)

// Deployment returns the given status condition from a Deployment.
// It returns nil if the condition is not populated in the object.
func Deployment(conditions []appsv1.DeploymentCondition, conditionType appsv1.DeploymentConditionType) *appsv1.DeploymentCondition {
	for i := range conditions {
		cond := &conditions[i]
		if cond.Type == conditionType {
			return cond
		}
	}
	return nil
}
