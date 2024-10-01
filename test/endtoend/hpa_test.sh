#!/bin/bash

source ./tools/test.env
source ./test/endtoend/utils.sh

function verifyHpaCount() {
  expectedCount=$1
  count=$(kubectl get hpa --no-headers | wc -l)
  if [[ "$count" -eq "$expectedCount" ]]; then
    echo "HorizontalPodAutoscaler count is $count"
    return 0
  fi
  echo -e "ERROR: expected $expectedCount HorizontalPodAutoscalers, got: $count"
  exit 1
}

function verifyHpaWithTimeout() {
  regex=$1
  for i in {1..600} ; do
    out=$(kubectl get hpa --no-headers)
    echo "$out" | grep -E "$regex" > /dev/null 2>&1
    if [[ $? -eq 0 ]]; then
      echo "HorizontalPodAutoscaler $regex found"
      return 0
    fi
    sleep 1
  done
  echo -e "HorizontalPodAutoscaler $regex not found"
  exit 1
}

# Test setup
echo "Building the docker image"
docker build -f build/Dockerfile.release -t vitess-operator-pr:latest .
echo "Creating Kind cluster"
kind create cluster --wait 30s --name kind-${BUILDKITE_BUILD_ID} --image ${KIND_VERSION}
echo "Loading docker image into Kind cluster"
kind load docker-image vitess-operator-pr:latest --name kind-${BUILDKITE_BUILD_ID}

cd "$PWD/test/endtoend/operator"
killall kubectl
setupKubectlAccessForCI

get_started "operator-latest.yaml" "101_initial_cluster_autoscale.yaml"
verifyVtGateVersion "21.0.0"
checkSemiSyncSetup

verifyHpaCount 0

echo "Apply cluster_autoscale.yaml"
kubectl apply -f cluster_autoscale.yaml

verifyHpaWithTimeout "example-zone1-(\w*)\s+VitessCell/example-zone1-(\1*)\s+[0-9]+%/80%\s+2\s+3\s+2"
verifyHpaCount 1

# Teardown
echo "Deleting Kind cluster. This also deletes the volume associated with it"
kind delete cluster --name kind-${BUILDKITE_BUILD_ID}
