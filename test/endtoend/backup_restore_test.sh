#!/bin/bash

source ./tools/test.env
source ./test/endtoend/utils.sh

function takedownShard() {
  echo "Apply 102_keyspace_teardown.yaml"
  kubectl apply -f 102_keyspace_teardown.yaml

  # wait for all the vttablets to disappear
  checkPodStatusWithTimeout "example-vttablet-zone1" 0
}

function resurrectShard() {
  echo "Apply 101_initial_cluster_backup.yaml again"
  kubectl apply -f 101_initial_cluster_backup.yaml
  checkPodStatusWithTimeout "example-etcd(.*)1/1(.*)Running(.*)" 3
  checkPodStatusWithTimeout "example-zone1-vtctld(.*)1/1(.*)Running(.*)"
  checkPodStatusWithTimeout "example-zone1-vtgate(.*)1/1(.*)Running(.*)"
  checkPodStatusWithTimeout "example-vttablet-zone1(.*)3/3(.*)Running(.*)" 3

  setupPortForwarding
  waitForKeyspaceToBeServing commerce - 2
  verifyDataCommerce
}

# Test setup
setupKindCluster
cd test/endtoend/operator || exit 1

get_started "operator-latest.yaml" "101_initial_cluster_backup.yaml"
verifyVtGateVersion "24.0.0"
checkSemiSyncSetup
takeBackup "commerce/-"
verifyListBackupsOutput
takedownShard
resurrectShard
checkSemiSyncSetup

# Teardown
teardownKindCluster
