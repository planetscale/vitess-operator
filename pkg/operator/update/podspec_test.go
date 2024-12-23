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
	// Make sure we don't touch topology spread constraints that were already there.
	val := []corev1.TopologySpreadConstraint{
		{
			MaxSkew:           1,
			TopologyKey:       "existing-retain",
			WhenUnsatisfiable: corev1.DoNotSchedule,
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"example-existing": "test",
				},
			},
		},
		{
			MaxSkew:           1,
			TopologyKey:       "existing-do-not-override",
			WhenUnsatisfiable: corev1.DoNotSchedule,
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"example-override": "test",
				},
			},
		},
	}
	want := []corev1.TopologySpreadConstraint{
		{
			MaxSkew:           1,
			TopologyKey:       "existing-retain",
			WhenUnsatisfiable: corev1.DoNotSchedule,
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"example-existing": "test",
				},
			},
		},
		{
			MaxSkew:           1,
			TopologyKey:       "existing-do-not-override",
			WhenUnsatisfiable: corev1.DoNotSchedule,
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"example-override": "test",
				},
			},
		},
		{
			MaxSkew:           2,
			TopologyKey:       "new-1",
			WhenUnsatisfiable: corev1.ScheduleAnyway,
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"example-new": "new-1",
				},
			},
		},
		{
			MaxSkew:           2,
			TopologyKey:       "new-2",
			WhenUnsatisfiable: corev1.DoNotSchedule,
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"example-override": "test",
				},
			},
		},
	}

	TopologySpreadConstraints(&val, []corev1.TopologySpreadConstraint{
		{
			MaxSkew:           2,
			TopologyKey:       "new-1",
			WhenUnsatisfiable: corev1.ScheduleAnyway,
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"example-new": "new-1",
				},
			},
		},
		{
			MaxSkew:           2,
			TopologyKey:       "new-2",
			WhenUnsatisfiable: corev1.DoNotSchedule,
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"example-override": "test",
				},
			},
		},
	})

	if !equality.Semantic.DeepEqual(val, want) {
		t.Errorf("val = %#v; want %#v", val, want)
	}
}
