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
