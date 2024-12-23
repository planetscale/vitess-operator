/*
Copyright 2024 PlanetScale Inc.

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
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

func TestTopologySpreadConstraintMinDomainsSecondsNil(t *testing.T) {
	// Make sure nil is distinguishable from 0.
	nilHash := TopologySpreadConstraints([]corev1.TopologySpreadConstraint{
		{MinDomains: nil},
	})
	zeroHash := TopologySpreadConstraints([]corev1.TopologySpreadConstraint{

		{MinDomains: ptr.To(int32(0))},
	})
	if nilHash == zeroHash {
		t.Errorf("nilHash = zeroHash = %v; expected different values", nilHash)
	}
}

func TestTopologySpreadConstraintNodeAffinityPolicysNil(t *testing.T) {
	// Make sure nil is distinguishable from 0.
	nilHash := TopologySpreadConstraints([]corev1.TopologySpreadConstraint{
		{NodeAffinityPolicy: nil},
	})
	honorHash := TopologySpreadConstraints([]corev1.TopologySpreadConstraint{
		{NodeAffinityPolicy: ptr.To(corev1.NodeInclusionPolicyHonor)},
	})
	if nilHash == honorHash {
		t.Errorf("nilHash = zeroHash = %v; expected different values", nilHash)
	}
}

func TestTopologySpreadConstraintNodeTaintsPolicysNil(t *testing.T) {
	// Make sure nil is distinguishable from 0.
	nilHash := TopologySpreadConstraints([]corev1.TopologySpreadConstraint{
		{NodeTaintsPolicy: nil},
	})
	honorHash := TopologySpreadConstraints([]corev1.TopologySpreadConstraint{
		{NodeTaintsPolicy: ptr.To(corev1.NodeInclusionPolicyHonor)},
	})
	if nilHash == honorHash {
		t.Errorf("nilHash = zeroHash = %v; expected different values", nilHash)
	}
}
