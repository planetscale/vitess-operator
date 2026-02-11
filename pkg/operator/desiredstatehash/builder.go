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

// Package desiredstatehash can be used to build up an annotation value that
// changes when certain parts of the desired state of an object change.
//
// The operator typically places this annotation on Pods that it directly
// manages so it can ignore extra items added to the Pod spec by other actors
// like admission controllers, while still knowing when items originally added
// by the operator need to be removed.
package desiredstatehash

import (
	corev1 "k8s.io/api/core/v1"

	"planetscale.dev/vitess-operator/pkg/operator/contenthash"
)

const (
	// Annotation is the name of the desired state hash annotation.
	Annotation = "planetscale.com/desired-state-hash"
)

// Builder collects all the releveant bits of desired state to be hashed.
//
// Add each item of state one by one, giving each one a unique item name.
// Then call String() to get the final hash value for the annotation.
type Builder struct {
	state map[string]string
}

// NewBuilder creates a desired state hash builder.
func NewBuilder() *Builder {
	return &Builder{
		state: map[string]string{},
	}
}

// String returns a hash of all the state added so far.
func (b *Builder) String() string {
	return contenthash.StringMap(b.state)
}

// AddStringMapKeys adds an item of state based on the keys from a string map.
func (b *Builder) AddStringMapKeys(itemName string, value map[string]string) {
	// Skip if the value is empty, so that defining new items doesn't cause any
	// Pods to be updated until someone actually sets a value for the new field.
	if len(value) == 0 {
		return
	}

	b.state[itemName] = contenthash.StringMapKeys(value)
}

// AddStringList adds an item of state based on a list of strings.
func (b *Builder) AddStringList(itemName string, value []string) {
	// Skip if the value is empty, so that defining new items doesn't cause any
	// Pods to be updated until someone actually sets a value for the new field.
	if len(value) == 0 {
		return
	}

	b.state[itemName] = contenthash.StringList(value)
}

// AddContainersUpdates adds an item of state based on a list of containers.
func (b *Builder) AddContainersUpdates(itemName string, value []corev1.Container) {
	// Skip if the value is empty, so that defining new items doesn't cause any
	// Pods to be updated until someone actually sets a value for the new field.
	if len(value) == 0 {
		return
	}

	b.state[itemName] = contenthash.ContainersUpdates(value)
}

// AddTolerations adds an item of state based on a list of tolerations.
func (b *Builder) AddTolerations(itemName string, value []corev1.Toleration) {
	// Skip if the value is empty, so that defining new items doesn't cause any
	// Pods to be updated until someone actually sets a value for the new field.
	if len(value) == 0 {
		return
	}

	b.state[itemName] = contenthash.Tolerations(value)
}

// AddTopologySpreadConstraints adds an item of state based on a list of topologySpreadConstraints.
func (b *Builder) AddTopologySpreadConstraints(itemName string, value []corev1.TopologySpreadConstraint) {
	// Skip if the value is empty, so that defining new items doesn't cause any
	// Pods to be updated until someone actually sets a value for the new field.
	if len(value) == 0 {
		return
	}

	b.state[itemName] = contenthash.TopologySpreadConstraints(value)
}

// AddVolumeNames add an item of state based on a list of Volume names.
func (b *Builder) AddVolumeNames(itemName string, vols []corev1.Volume) {
	volNames := make([]string, 0, len(vols))
	for i := range vols {
		volNames = append(volNames, vols[i].Name)
	}
	b.AddStringList(itemName, volNames)
}

// AddString adds a simple string to the state to be hashed.
func (b *Builder) AddString(itemName string, value string) {
	// Skip if the value is empty, following the pattern of other methods.
	if value == "" {
		return
	}

	b.state[itemName] = value
}
