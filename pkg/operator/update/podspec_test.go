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
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestTolerations(t *testing.T) {
	// Make sure we don't touch tolerations that were already there.
	val := []corev1.Toleration{
		{Key: "alreadyExists"},
	}
	want := []corev1.Toleration{
		{Key: "alreadyExists"},
		{Key: "newKey"},
	}

	Tolerations(&val, []corev1.Toleration{
		{Key: "newKey"},
	})

	if !equality.Semantic.DeepEqual(val, want) {
		t.Errorf("val = %#v; want %#v", val, want)
	}
}

func TestTopologySpreadConstraints(t *testing.T) {
	// Constraints that share a label selector but spread over different
	// topology keys are distinct, so all of them must be kept.
	selector := &metav1.LabelSelector{MatchLabels: map[string]string{"app": "vttablet"}}
	want := []corev1.TopologySpreadConstraint{
		{MaxSkew: 1, TopologyKey: "topology.kubernetes.io/zone", WhenUnsatisfiable: corev1.DoNotSchedule, LabelSelector: selector},
		{MaxSkew: 1, TopologyKey: "kubernetes.io/hostname", WhenUnsatisfiable: corev1.ScheduleAnyway, LabelSelector: selector},
	}

	tests := map[string][]corev1.TopologySpreadConstraint{
		"empty":      nil,
		"up to date": {want[0], want[1]},
	}
	for name, val := range tests {
		t.Run(name, func(t *testing.T) {
			TopologySpreadConstraints(&val, want)
			assert.Equal(t, want, val)
		})
	}
}

func TestTopologySpreadConstraintsReplacesMatchingEntry(t *testing.T) {
	selector := &metav1.LabelSelector{MatchLabels: map[string]string{"app": "vttablet"}}
	src := []corev1.TopologySpreadConstraint{
		{MaxSkew: 2, TopologyKey: "topology.kubernetes.io/zone", WhenUnsatisfiable: corev1.DoNotSchedule, LabelSelector: selector},
	}

	t.Run("older maxSkew", func(t *testing.T) {
		dst := []corev1.TopologySpreadConstraint{
			{MaxSkew: 1, TopologyKey: "topology.kubernetes.io/zone", WhenUnsatisfiable: corev1.DoNotSchedule, LabelSelector: selector},
		}
		TopologySpreadConstraints(&dst, src)
		assert.Equal(t, src, dst)
	})

	t.Run("selector changed", func(t *testing.T) {
		dst := []corev1.TopologySpreadConstraint{
			{
				MaxSkew:           2,
				TopologyKey:       "topology.kubernetes.io/zone",
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: map[string]string{"app": "old"}},
			},
		}
		TopologySpreadConstraints(&dst, src)
		assert.Equal(t, src, dst)
	})
}

func TestTopologySpreadConstraintsKeepsExtraEntries(t *testing.T) {
	// The extra entry shares the selector and comes first, so matching by
	// selector alone would overwrite it.
	selector := &metav1.LabelSelector{MatchLabels: map[string]string{"app": "vttablet"}}
	extra := corev1.TopologySpreadConstraint{
		MaxSkew:           1,
		TopologyKey:       "kubernetes.io/hostname",
		WhenUnsatisfiable: corev1.ScheduleAnyway,
		LabelSelector:     selector,
	}
	src := []corev1.TopologySpreadConstraint{
		{MaxSkew: 2, TopologyKey: "topology.kubernetes.io/zone", WhenUnsatisfiable: corev1.DoNotSchedule, LabelSelector: selector},
	}
	dst := []corev1.TopologySpreadConstraint{
		extra,
		{MaxSkew: 1, TopologyKey: "topology.kubernetes.io/zone", WhenUnsatisfiable: corev1.DoNotSchedule, LabelSelector: selector},
	}

	TopologySpreadConstraints(&dst, src)

	assert.Equal(t, []corev1.TopologySpreadConstraint{extra, src[0]}, dst)
}
