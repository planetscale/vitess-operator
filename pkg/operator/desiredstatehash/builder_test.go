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

package desiredstatehash

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestEmptyValues(t *testing.T) {
	b := NewBuilder()

	// Add a non-empty value. Make sure this changes the hash.
	start := b.String()
	b.AddStringMapKeys("initial value", map[string]string{
		"key": "value",
	})
	want := b.String()
	if want == start {
		t.Error("b.String() didn't change after adding initial value")
	}

	// Adding values that are empty should have no effect on the hash.
	// This tests that we can add new state items over time without needless
	// changes to the hashes of existing Pods.
	b.AddStringMapKeys("nil StringMapKeys", nil)
	b.AddStringMapKeys("empty StringMapKeys", map[string]string{})
	b.AddStringList("nil StringList", nil)
	b.AddStringList("empty StringList", []string{})
	b.AddContainersUpdates("nil ContainersUpdates", nil)
	b.AddContainersUpdates("empty ContainersUpdates", []corev1.Container{})
	b.AddTolerations("nil Tolerations", nil)
	b.AddTolerations("empty Tolerations", []corev1.Toleration{})
	b.AddVolumeNames("nil VolumeNames", nil)
	b.AddVolumeNames("empty VolumeNames", []corev1.Volume{})

	if got := b.String(); got != want {
		t.Errorf("b.String() = %q; want %q", got, want)
	}
}

func TestTolerations(t *testing.T) {
	a := NewBuilder()
	b := NewBuilder()

	tolerations := []corev1.Toleration{
		corev1.Toleration{
			Key:               "key1",
			Operator:          "",
			Value:             "",
			Effect:            "",
			TolerationSeconds: nil,
		},
		corev1.Toleration{
			Key:               "key2",
			Operator:          "",
			Value:             "",
			Effect:            "",
			TolerationSeconds: nil,
		},
		corev1.Toleration{
			Key:               "key3",
			Operator:          "",
			Value:             "",
			Effect:            "",
			TolerationSeconds: nil,
		},
	}

	a.AddTolerations("Tolerations1", tolerations)
	tolerationsDiffOrder := []corev1.Toleration{
		corev1.Toleration{
			Key:               "key3",
			Operator:          "",
			Value:             "",
			Effect:            "",
			TolerationSeconds: nil,
		},
		corev1.Toleration{
			Key:               "key2",
			Operator:          "",
			Value:             "",
			Effect:            "",
			TolerationSeconds: nil,
		},
		corev1.Toleration{
			Key:               "key1",
			Operator:          "",
			Value:             "",
			Effect:            "",
			TolerationSeconds: nil,
		},
	}
	b.AddTolerations("Tolerations1", tolerationsDiffOrder)

	if a.String() != b.String() {
		t.Errorf("tolerations are same, but generating different hashes.")
	}
}

func TestAddVolumeNames(t *testing.T) {
	a := NewBuilder()
	b := NewBuilder()

	volumes := []corev1.Volume{
		corev1.Volume{
			Name: "vol1",
		},
		corev1.Volume{
			Name: "vol2",
		},
		corev1.Volume{
			Name: "vol3",
		},
	}
	a.AddVolumeNames("VolumeNames", volumes)

	volumesReordered := []corev1.Volume{
		corev1.Volume{
			Name: "vol2",
		},
		corev1.Volume{
			Name: "vol3",
		},
		corev1.Volume{
			Name: "vol1",
		},
	}

	b.AddVolumeNames("VolumeNames", volumesReordered)

	if a.String() != b.String() {
		t.Errorf("volumes are same, but generating different hashes.")
	}
}
