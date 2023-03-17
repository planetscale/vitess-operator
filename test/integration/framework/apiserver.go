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
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"

	"k8s.io/client-go/rest"
	"k8s.io/klog"
)

var apiserverURL = ""
var apiserverToken = "31ada4fd-adec-460c-809a-9e56ceb75269"
var apiserverDatadir = ""

const installApiserver = `
Cannot find kube-apiserver, cannot run integration tests

Please download kube-apiserver and ensure it is somewhere in the PATH.
See tools/get-kube-binaries.sh

`

// getApiserverPath returns a path to a kube-apiserver executable.
func getApiserverPath() (string, error) {
	return exec.LookPath("kube-apiserver")
}

// startApiserver executes a kube-apiserver instance.
// The returned function will signal the process and wait for it to exit.
func startApiserver() (func(), error) {
	apiserverPath, err := getApiserverPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, installApiserver)
		return nil, fmt.Errorf("could not find kube-apiserver in PATH: %v", err)
	}
	apiserverPort, err := getAvailablePort()
	if err != nil {
		return nil, fmt.Errorf("could not get a port: %v", err)
	}
	apiserverURL = fmt.Sprintf("https://127.0.0.1:%d", apiserverPort)
	klog.Infof("starting kube-apiserver on %s", apiserverURL)

	apiserverDataDir, err := ioutil.TempDir(os.TempDir(), "integration_test_apiserver_data")
	if err != nil {
		return nil, fmt.Errorf("unable to make temp kube-apiserver data dir: %v", err)
	}

	apiserverDatadir = apiserverDataDir

	klog.Infof("storing kube-apiserver data in: %v", apiserverDatadir)

	os.WriteFile(fmt.Sprintf("%s/token.csv", apiserverDatadir), []byte(fmt.Sprintf("%s,testrunner,1", apiserverToken)), 0644)

	abac1 := "{\"apiVersion\": \"abac.authorization.kubernetes.io/v1beta1\", \"kind\": \"Policy\", \"spec\": {\"user\": \"testrunner\", \"namespace\": \"*\", \"resource\": \"*\", \"apiGroup\": \"*\"}}"
	abac2 := "{\"apiVersion\": \"abac.authorization.kubernetes.io/v1beta1\", \"kind\": \"Policy\", \"spec\": {\"group\": \"system:authenticated\", \"readonly\": true, \"nonResourcePath\": \"*\"}}"

	os.WriteFile(fmt.Sprintf("%s/auth-policy.json", apiserverDatadir), []byte(fmt.Sprintf("%s\n%s", abac1, abac2)), 0644)


	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(
		ctx,
		apiserverPath,
		"--authorization-policy-file", fmt.Sprintf("%s/auth-policy.json", apiserverDatadir),
		"--authorization-mode", "ABAC",
		"--cert-dir", apiserverDatadir,
		"--etcd-servers", etcdURL,
		"--secure-port", strconv.Itoa(apiserverPort),
		"--service-account-issuer", "api",
		"--service-account-key-file", fmt.Sprintf("%s/apiserver.key", apiserverDatadir),
		"--service-account-signing-key-file", fmt.Sprintf("%s/apiserver.key", apiserverDatadir),
		"--token-auth-file", fmt.Sprintf("%s/token.csv", apiserverDatadir),
	)

	// Uncomment these to see kube-apiserver output in test logs.
	// For operator tests, we generally don't expect problems at this level.
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	stop := func() {
		cancel()
		err := cmd.Wait()
		klog.Infof("kube-apiserver exit status: %v", err)
		err = os.RemoveAll(apiserverDatadir)
		if err != nil {
			klog.Warningf("error during kube-apiserver cleanup: %v", err)
		}
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to run kube-apiserver: %v", err)
	}
	return stop, nil
}

// ApiserverConfig returns a rest.Config to connect to the test instance.
func ApiserverConfig() *rest.Config {
	return &rest.Config{
		Host: ApiserverURL(),
		BearerToken: apiserverToken,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
	}
}

// ApiserverCert returns the generated kube-apiserver certificate authority
func ApiserverCert() string {
	return fmt.Sprintf("%s/apiserver.crt", apiserverDatadir)
}

// ApiserverToken returns the token used for authentication
func ApiserverToken() string {
	return apiserverToken
}

// ApiserverURL returns the URL of the kube-apiserver instance started by TestMain.
func ApiserverURL() string {
	return apiserverURL
}
