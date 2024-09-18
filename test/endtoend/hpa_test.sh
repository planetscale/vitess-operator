#!/bin/bash

source ./tools/test.env
source ./test/endtoend/utils.sh

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

echo "Apply latest operator-latest.yaml"
kubectl apply -f "operator-latest.yaml"
checkPodStatusWithTimeout "vitess-operator(.*)1/1(.*)Running(.*)"

echo "Apply cluster_autoscale.yaml"
kubectl apply -f cluster_autoscale.yaml

function verifyHpa() {
  regex=$1
  for i in {1..600} ; do
    out=$(kubectl get hpa --no-headers -o custom-columns=":metadata.name,:spec.maxReplicas,:spec.minReplicas,:spec.scaleTargetRef.name")
    echo "$out" | grep -E "$regex" > /dev/null 2>&1
    if [[ $? -eq 0 ]]; then
      echo "HorizontalPodAutoscaler $regex found"
      return 0
    fi
    sleep 1
  done
  echo -e "ERROR: HorizontalPodAutoscaler $regex not found"
  exit 1
}

verifyHpa "example-zone1-vtgate(.*)3\s+2\s+example-zone1-vtgate(.*)"

# Teardown
echo "Deleting Kind cluster. This also deletes the volume associated with it"
kind delete cluster --name kind-${BUILDKITE_BUILD_ID}
