/*
Copyright 2019 Google Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// This file is forked from github.com/GoogleCloudPlatform/metacontroller.

package framework

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
	"planetscale.dev/vitess-operator/pkg/operator/controllermanager"
)

const (
	defaultWaitTimeout  = 30 * time.Second
	defaultWaitInterval = 250 * time.Millisecond
)

// Fixture is a collection of scaffolding for each integration test method.
type Fixture struct {
	t *testing.T

	teardownFuncs []func(ctx context.Context) error

	client client.Client
}

func NewFixture(t *testing.T) *Fixture {
	config := ApiserverConfig()

	scheme, err := controllermanager.NewScheme()
	if err != nil {
		t.Fatalf("can't create Scheme: %v", err)
	}

	mapper, err := apiutil.NewDiscoveryRESTMapper(config)
	if err != nil {
		t.Fatalf("can't create Mapper: %v", err)
	}

	kubeClient, err := client.New(config, client.Options{
		Scheme: scheme,
		Mapper: mapper,
	})
	if err != nil {
		t.Fatalf("can't create Client: %v", err)
	}

	return &Fixture{
		t:      t,
		client: kubeClient,
	}
}

// Client returns the Kubernetes client.
func (f *Fixture) Client() client.Client {
	return f.client
}

// CreateNamespace creates a namespace that will be deleted after this test
// finishes.
func (f *Fixture) CreateNamespace(ctx context.Context, namespace string) *corev1.Namespace {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}
	if err := f.client.Create(ctx, ns); err != nil {
		f.t.Fatal(err)
	}
	f.deferTeardown(func(ctx context.Context) error {
		// Make a fresh object with just the name, so the delete is unconditional.
		return f.client.Delete(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}, client.PropagationPolicy(metav1.DeletePropagationForeground))
	})
	return ns
}

// CreateVitessClusterYAML creates a VitessCluster (from YAML input) that will
// be deleted after this test finishes.
func (f *Fixture) CreateVitessClusterYAML(ctx context.Context, namespace, name, vtYAML string) *planetscalev2.VitessCluster {
	vt := &planetscalev2.VitessCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	mustDecodeYAML(vtYAML, vt)

	if err := f.client.Create(ctx, vt); err != nil {
		f.t.Fatal(err)
	}
	f.deferTeardown(func(ctx context.Context) error {
		// Make a fresh object with just the name, so the delete is unconditional.
		return f.client.Delete(ctx, &planetscalev2.VitessCluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      name,
			},
		}, client.PropagationPolicy(metav1.DeletePropagationForeground))
	})
	return vt
}

// TearDown cleans up resources created through this instance of the test fixture.
func (f *Fixture) TearDown(ctx context.Context) {
	for i := len(f.teardownFuncs) - 1; i >= 0; i-- {
		teardown := f.teardownFuncs[i]
		if err := teardown(ctx); err != nil {
			f.t.Logf("Error during teardown: %v", err)
			// Mark the test as failed, but continue trying to tear down.
			f.t.Fail()
		}
	}
}

// Wait polls the condition until it's true, with a default interval and timeout.
// This is meant for use in integration tests, so frequent polling is fine.
//
// The condition function returns a bool indicating whether it is satisfied,
// as well as an error which should be non-nil if and only if the function was
// unable to determine whether or not the condition is satisfied (for example
// if the check involves calling a remote server and the request failed).
//
// If the condition function returns a non-nil error, Wait will log the error
// and continue retrying until the timeout.
func (f *Fixture) Wait(condition func() (bool, error)) error {
	start := time.Now()
	for {
		ok, err := condition()
		if err == nil && ok {
			return nil
		}
		if err != nil {
			// Log error, but keep trying until timeout.
			f.t.Logf("error while waiting for condition: %v", err)
		}
		if time.Since(start) > defaultWaitTimeout {
			return fmt.Errorf("timed out waiting for condition (%v)", err)
		}
		time.Sleep(defaultWaitInterval)
	}
}

// MustGet waits up to a default timeout for the named object to exist and then returns it.
// If the timeout expires before the object appears, the test is aborted.
func (f *Fixture) MustGet(namespace, name string, obj runtime.Object) {
	ctx := context.TODO()
	key := client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}
	f.t.Logf("Waiting for %T %v/%v to be created...", obj, namespace, name)
	err := f.Wait(func() (bool, error) {
		if err := f.client.Get(ctx, key, obj); err != nil {
			return false, err
		}
		return true, nil
	})
	if err != nil {
		f.t.Fatalf("Error waiting for %T %v/%v to be created: %v", obj, namespace, name, err)
	}
}

// ExpectPods waits up to a default timeout for the given selector to match the
// expected number of Pods. If the timeout expires, the test is aborted.
func (f *Fixture) ExpectPods(listOpts *client.ListOptions, expectedCount int) *corev1.PodList {
	ctx := context.TODO()

	var pods *corev1.PodList

	f.t.Logf("Waiting for %v Pods matching %v to be created...", expectedCount, listOpts)
	err := f.Wait(func() (bool, error) {
		pods = &corev1.PodList{}
		if err := f.client.List(ctx, listOpts, pods); err != nil {
			return false, err
		}
		if len(pods.Items) != expectedCount {
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		f.t.Fatalf("Error waiting for %v Pods matching %v to be created: %v", expectedCount, listOpts, err)
	}

	return pods
}

func (f *Fixture) deferTeardown(teardown func(ctx context.Context) error) {
	f.teardownFuncs = append(f.teardownFuncs, teardown)
}

func mustDecodeYAML(yamlStr string, into interface{}) {
	if err := yaml.NewYAMLOrJSONDecoder(bytes.NewReader([]byte(yamlStr)), 0).Decode(into); err != nil {
		panic(fmt.Errorf("can't decode YAML: %v", err))
	}
}
