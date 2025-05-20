#!/bin/bash

source ./tools/test.env
source ./test/endtoend/utils.sh

# get_started_unmanaged:
function get_started_unmanaged() {
    echo "Apply latest operator-latest.yaml"
    kubectl apply -f "operator-latest.yaml"
    checkPodStatusWithTimeout "vitess-operator(.*)1/1(.*)Running(.*)"

    echo "Apply 101_initial_cluster_unmanaged_tablet.yaml"
    kubectl apply -f "101_initial_cluster_unmanaged_tablet.yaml"
    checkPodStatusWithTimeout "example-zone1-vtctld(.*)1/1(.*)Running(.*)"
    checkPodStatusWithTimeout "example-zone1-vtgate(.*)1/1(.*)Running(.*)"
    checkPodStatusWithTimeout "example-etcd(.*)1/1(.*)Running(.*)" 3
    checkPodStatusWithTimeout "example-vttablet-zone1(.*)1/1(.*)Running(.*)"

    setupPortForwarding
    waitForKeyspaceToBeServing commerce - 0

    # Confirm that the custom sidecar DB name is in place for our
    # external/unmanaged keyspace.
    verifyCustomSidecarDBName "commerce" "_vt_ext" "external"

    verifyDataCommerce create
}

# Test setup
setupKindCluster
cd test/endtoend/operator || exit 1

# Check Unmanaged tablet is running properly
get_started_unmanaged

# Teardown
teardownKindCluster
