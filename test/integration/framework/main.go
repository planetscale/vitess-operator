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
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"planetscale.dev/vitess-operator/pkg/operator/controllermanager"
)

const installKubectl = `
Cannot find kubectl, cannot run integration tests

Please download kubectl and ensure it is somewhere in the PATH.
See tools/get-kube-binaries.sh

`

// deployDir is the path from the integration test binary working dir to the
// directory containing manifests to install vitess-operator.
const deployDir = "../../../deploy"

// operatorPod is a minimal fake Pod spec to pretend that we're running the
// operator in a Pod, even though we're actually just running it in the test
// process. Note that nothing actually acts on Pod objects because this
// integration test environment intentionally does not have any kubelets.
const operatorPod = `
apiVersion: v1
kind: Pod
metadata:
  namespace: default
  name: vitess-operator
  labels:
    app: vitess-operator
spec:
  containers:
  - name: vitess-operator
    image: planetscale/vitess-operator
    command:
    - vitess-operator
`

const defaultNamespace = `
apiVersion: v1
kind: Namespace
metadata:
  name: default
`

const defaultServiceAccount = `
apiVersion: v1
kind: ServiceAccount
metadata:
  namespace: default
  name: default
secrets:
- name: default-token
---
apiVersion: v1
kind: Secret
metadata:
  namespace: default
  name: default-token
  annotations:
    kubernetes.io/service-account.name: default
type: kubernetes.io/service-account-token
`

// getKubectlPath returns a path to a kube-apiserver executable.
func getKubectlPath() (string, error) {
	return exec.LookPath("kubectl")
}

// TestMain starts etcd, kube-apiserver, and vitess-operator before running tests.
func TestMain(tests func() int) {
	if err := testMain(tests); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func testMain(tests func() int) error {
	controllermanager.InitFlags()

	if _, err := getKubectlPath(); err != nil {
		return errors.New(installKubectl)
	}

	// stopEtcd, err := startEtcd()
	// if err != nil {
	// 	return fmt.Errorf("cannot run integration tests: unable to start etcd: %v", err)
	// }
	// defer stopEtcd()

	stopApiserver, err := startApiserver()
	if err != nil {
		return fmt.Errorf("cannot run integration tests: unable to start kube-apiserver: %v", err)
	}
	defer stopApiserver()

	klog.Info("set kubectl context")

	execKubectl("config", "set-context", "--user=testrunner")
	
	klog.Info("Waiting for kube-apiserver to be ready...")
	start := time.Now()
	for {
		out, kubectlErr := execKubectl("version")
		if kubectlErr == nil {
			break
		}
		if time.Since(start) > 60*time.Second {
			return fmt.Errorf("timed out waiting for kube-apiserver to be ready: %v\n%s", kubectlErr, out)
		}
		time.Sleep(time.Second)
	}

	klog.Info("kube-apiserver is ready!")

	return nil

	if out, err := execKubectlStdin(strings.NewReader(defaultNamespace), "apply", "-f", "-"); err != nil {
		return fmt.Errorf("cannot create default Namespace: %v\n%s", err, out)
	}

	// Create a default ServiceAccount. We have to do this in order to make the
	// apiserver accept Pods. Normally this is done automatically by parts of
	// the k8s distrubtion that we don't run in this integration test
	// environment, so we have to do it manually.
	if out, err := execKubectlStdin(strings.NewReader(defaultServiceAccount), "create", "-f", "-"); err != nil {
		return fmt.Errorf("cannot create default ServiceAccount: %v\n%s", err, out)
	}

	// Install vitess-operator base files, but not the Deployment itself.
	files := []string{
		"service_account.yaml",
		"role.yaml",
		"role_binding.yaml",
		"priority.yaml",
		"crds/",
	}
	for _, file := range files {
		filePath := path.Join(deployDir, file)
		klog.Infof("Installing %v...", filePath)
		if out, err := execKubectl("apply", "-f", filePath); err != nil {
			return fmt.Errorf("cannot install %v: %v\n%s", filePath, err, out)
		}
	}

	klog.Info("Waiting for CRDs to be ready...")
	start = time.Now()
	for {
		out, kubectlErr := execKubectl("get", "vt,vtc,vtk,vts,vtbs,vtb,etcdls")
		if kubectlErr == nil {
			break
		}
		if time.Since(start) > 30*time.Second {
			return fmt.Errorf("timed out waiting for CRDs to be ready: %v\n%s", kubectlErr, out)
		}
		time.Sleep(time.Second)
	}

	// Create a fake Pod to represent the operator, which is actually just
	// running in this test process.
	if out, err := execKubectlStdin(strings.NewReader(operatorPod), "create", "-f", "-"); err != nil {
		return fmt.Errorf("cannot create vitess-operator Pod: %v\n%s", err, out)
	}

	// Set env vars that vitess-operator expects, to simulate the values
	// provided in deploy/operator.yaml.
	os.Setenv("WATCH_NAMESPACE", "default")
	os.Setenv("POD_NAME", "vitess-operator")
	os.Setenv("PS_OPERATOR_POD_NAMESPACE", "default")
	os.Setenv("PS_OPERATOR_POD_NAME", "vitess-operator")
	os.Setenv("OPERATOR_NAME", "vitess-operator")

	// Set operator flags to values that are good for testing.
	// For example, we set faster resync so we can test periodic topo pruning
	// without having to wait too long.
	flag.Set("vitesscluster_resync_period", "5s")
	flag.Set("vitesscell_resync_period", "5s")
	flag.Set("vitesskeyspace_resync_period", "5s")
	flag.Set("vitessshard_resync_period", "5s")

	// Start vitess-operator in this test process.
	mgr, err := controllermanager.New("", ApiserverConfig(), manager.Options{
		Namespace: "default",
	})
	if err != nil {
		return fmt.Errorf("cannot create controller-manager: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		if err := mgr.Start(ctx); err != nil {
			klog.Errorf("cannot start controller-manager: %v", err)
		}
	}()

	// Now actually run the tests.
	if exitCode := tests(); exitCode != 0 {
		// If one of the tests failed, dump all the events in the apiserver.
		out, err := execKubectl("get", "events")
		if err != nil {
			klog.Errorf("cannot dump apiserver events")
		} else {
			klog.Infof("apiserver events:\n%s", out)
		}

		return fmt.Errorf("one or more tests failed with exit code: %v", exitCode)
	}
	return nil
}

func execKubectl(args ...string) ([]byte, error) {
	return execKubectlStdin(nil, args...)
}

func execKubectlStdin(stdin io.Reader, args ...string) ([]byte, error) {
	execPath, err := exec.LookPath("kubectl")
	if err != nil {
		return nil, fmt.Errorf("cannot exec kubectl: %v", err)
	}
	// cmdline := append([]string{"--server", ApiserverURL()}, args...)
	cmdline := append([]string{"--insecure-skip-tls-verify=true", "--username=foo", "--password=bar", fmt.Sprintf("--server=%s",ApiserverURL())}, cmdline3...)
	cmd := exec.Command(execPath, cmdline...)

	cmd.Stdin = stdin
	return cmd.CombinedOutput()
}
