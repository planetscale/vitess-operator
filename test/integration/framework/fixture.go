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
	*testing.T
	ctx context.Context

	teardownFuncs []func(ctx context.Context) error

	client client.Client
}

// NewFixture creates a new test fixture.
// Each Test*() function should create its own fixture and immediately defer a
// call to that fixture's TearDown() method.
func NewFixture(ctx context.Context, t *testing.T) *Fixture {
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
		T:      t,
		ctx:    ctx,
		client: kubeClient,
	}
}

// Context returns the Context for the running test.
func (f *Fixture) Context() context.Context {
	return f.ctx
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
		f.Fatal(err)
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
func (f *Fixture) CreateVitessClusterYAML(namespace, name, vtYAML string) *planetscalev2.VitessCluster {
	vt := &planetscalev2.VitessCluster{}
	MustDecodeYAML(vtYAML, vt)
	return f.CreateVitessCluster(namespace, name, vt)
}

// CreateVitessCluster creates a VitessCluster that will be deleted after this
// test finishes.
func (f *Fixture) CreateVitessCluster(namespace, name string, vt *planetscalev2.VitessCluster) *planetscalev2.VitessCluster {
	vt.Namespace = namespace
	vt.Name = name

	if err := f.client.Create(f.ctx, vt); err != nil {
		f.Fatal(err)
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
func (f *Fixture) TearDown() {
	for i := len(f.teardownFuncs) - 1; i >= 0; i-- {
		teardown := f.teardownFuncs[i]
		if err := teardown(f.ctx); err != nil {
			f.Logf("Error during teardown: %v", err)
			// Mark the test as failed, but continue trying to tear down.
			f.Fail()
		}
	}
}

// WaitFor polls the check function until it returns true, with a default interval and timeout.
// This is meant for use in integration tests, so frequent polling is fine.
//
// The check function should return nil if it is satisfied, or a non-nil error
// indicating why it's not satisfied.
//
// If the timeout expires before the check function is satisfied, the test will
// be aborted.
func (f *Fixture) WaitFor(condition string, check func() error) {
	f.Logf("Waiting for %v...", condition)
	start := time.Now()
	for {
		err := check()
		if err == nil {
			f.Logf("Done waiting for %v.", condition)
			return
		}
		if time.Since(start) > defaultWaitTimeout {
			f.Fatalf("Timed out waiting for %v: %v", condition, err)
		}
		time.Sleep(defaultWaitInterval)
	}
}

// MustGet waits up to a default timeout for the named object to exist and then returns it.
// If the timeout expires before the object appears, the test is aborted.
func (f *Fixture) MustGet(namespace, name string, obj runtime.Object) {
	key := client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}
	condition := fmt.Sprintf("%T %v/%v to exist", obj, namespace, name)
	f.WaitFor(condition, func() error {
		return f.client.Get(f.ctx, key, obj)
	})
}

// ExpectPods waits up to a default timeout for the given selector to match the
// expected number of Pods. If the timeout expires, the test is aborted.
func (f *Fixture) ExpectPods(listOpts *client.ListOptions, expectedCount int) *corev1.PodList {
	var pods *corev1.PodList

	condition := fmt.Sprintf("%v Pods matching %v to exist", expectedCount, listOpts)
	f.WaitFor(condition, func() error {
		pods = &corev1.PodList{}
		if err := f.client.List(f.ctx, listOpts, pods); err != nil {
			return err
		}
		if got, want := len(pods.Items), expectedCount; got != want {
			return fmt.Errorf("found %v matching Pods; want %v", got, want)
		}
		return nil
	})

	return pods
}

func (f *Fixture) deferTeardown(teardown func(ctx context.Context) error) {
	f.teardownFuncs = append(f.teardownFuncs, teardown)
}

// MustDecodeYAML decodes YAML into the given object.
// It will panic if the decode fails.
func MustDecodeYAML(yamlStr string, into interface{}) {
	if err := yaml.NewYAMLOrJSONDecoder(bytes.NewReader([]byte(yamlStr)), 0).Decode(into); err != nil {
		panic(fmt.Errorf("can't decode YAML: %v", err))
	}
}
