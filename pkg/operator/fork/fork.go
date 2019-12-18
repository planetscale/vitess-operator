/*
Copyright 2019 PlanetScale Inc.

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
Package fork implements a pattern for forking sub-processes as new Pods.

This can be used when some code that's part of the operator (usually an entire
controller) needs to run under a different Pod context (e.g. different service
account, volume mounts, etc.). The forked Pod runs the same container image as
the parent Pod, so the code versions should stay synchronized as long as
immutable image tags are used. This avoids the operational and maintenance
burden of creating and publishing separate binaries and container images for
each sub-process.

The forked Pod is given an additional environment variable telling it which
forked code path it should run instead of the normal code path. The main()
function should be written to take this into account by calling fork.Path() to
determine which fork it should follow, if any.

The parent Pod must also give some environment variables to the operator's
Container to let this package find the Pod in which it's currently running:

  env:
  - name: PS_OPERATOR_POD_NAME
    valueFrom:
      fieldRef:
        fieldPath: metadata.name
  - name: PS_OPERATOR_POD_NAMESPACE
    valueFrom:
      fieldRef:
        fieldPath: metadata.namespace

The parent Pod uses its own Pod spec as a basis to build the child Pod spec.
*/
package fork

import (
	"context"
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"planetscale.dev/vitess-operator/pkg/operator/update"
)

const (
	envForkPath     = "PS_OPERATOR_FORK_PATH"
	envPodName      = "PS_OPERATOR_POD_NAME"
	envPodNamespace = "PS_OPERATOR_POD_NAMESPACE"
)

// Path returns the name of the forked code path that this process should take.
// It returns "" if no fork should be taken (i.e. this is the root process).
func Path() string {
	return os.Getenv(envForkPath)
}

// NewPodSpec returns the specification for a child Pod to be forked off from the
// Pod in which you're currently running.
func NewPodSpec(ctx context.Context, c client.Client, forkPath string) (*corev1.PodSpec, error) {
	// Get the Pod we're currently running in.
	parentPod, err := getParentPod(ctx, c)
	if err != nil {
		return nil, fmt.Errorf("can't get parent Pod: %v", err)
	}
	spec := parentPod.Spec

	// Clear the NodeName so we let the scheduler choose a node.
	spec.NodeName = ""

	// Set the fork path env var on all containers.
	childEnv := []corev1.EnvVar{
		{
			Name:  envForkPath,
			Value: forkPath,
		},
	}
	for i := range spec.Containers {
		container := &spec.Containers[i]
		update.Env(&container.Env, childEnv)
	}

	return &spec, nil
}

func getParentPod(ctx context.Context, c client.Client) (*corev1.Pod, error) {
	var key client.ObjectKey
	key.Namespace = os.Getenv(envPodNamespace)
	if key.Namespace == "" {
		return nil, fmt.Errorf("forking requires %v env var to be set on the Container in the parent Pod", envPodNamespace)
	}
	key.Name = os.Getenv(envPodName)
	if key.Name == "" {
		return nil, fmt.Errorf("forking requires %v env var to be set on the Container in the parent Pod", envPodName)
	}
	pod := &corev1.Pod{}
	err := c.Get(ctx, key, pod)
	return pod, err
}
