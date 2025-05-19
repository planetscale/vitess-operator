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
  checkPodStatusWithTimeout "example-zone1-vtctld(.*)1/1(.*)Running(.*)"
  checkPodStatusWithTimeout "example-zone1-vtgate(.*)1/1(.*)Running(.*)"
  checkPodStatusWithTimeout "example-etcd(.*)1/1(.*)Running(.*)" 3
  checkPodStatusWithTimeout "example-vttablet-zone1(.*)3/3(.*)Running(.*)" 3

  sleep 10

  killall kubectl
  ./pf.sh > /dev/null 2>&1 &

  sleep 10

  waitForKeyspaceToBeServing commerce - 2
  sleep 5

  echo "show databases;" | mysql | grep "commerce" > /dev/null 2>&1
  if [[ $? -ne 0 ]]; then
    echo "Could not find commerce database"
    printMysqlErrorFiles
    exit 1
  fi

  echo "show tables;" | mysql commerce | grep -E 'corder|customer|product' | wc -l | grep 3 > /dev/null 2>&1
  if [[ $? -ne 0 ]]; then
    echo "Could not find commerce's tables"
    printMysqlErrorFiles
    exit 1
  fi

  assertSelect ../common/select_commerce_data.sql "commerce" << EOF
Using commerce
Customer
+-------------+--------------------+
| customer_id | email              |
+-------------+--------------------+
|           1 | alice@domain.com   |
|           2 | bob@domain.com     |
|           3 | charlie@domain.com |
|           4 | dan@domain.com     |
|           5 | eve@domain.com     |
+-------------+--------------------+
Product
+----------+-------------+-------+
| sku      | description | price |
+----------+-------------+-------+
| SKU-1001 | Monitor     |   100 |
| SKU-1002 | Keyboard    |    30 |
+----------+-------------+-------+
COrder
+----------+-------------+----------+-------+
| order_id | customer_id | sku      | price |
+----------+-------------+----------+-------+
|        1 |           1 | SKU-1001 |   100 |
|        2 |           2 | SKU-1002 |    30 |
|        3 |           3 | SKU-1002 |    30 |
|        4 |           4 | SKU-1002 |    30 |
|        5 |           5 | SKU-1002 |    30 |
+----------+-------------+----------+-------+
EOF
}

# Test setup
setupKindCluster
cd test/endtoend/operator || exit 1

get_started "operator-latest.yaml" "101_initial_cluster_backup.yaml"
verifyVtGateVersion "23.0.0"
checkSemiSyncSetup
takeBackup "commerce/-"
verifyListBackupsOutput
takedownShard
resurrectShard
checkSemiSyncSetup

# Teardown
echo "Removing the temporary directory"
removeBackupFiles
rm -rf "$STARTING_DIR/vtdataroot"
echo "Deleting Kind cluster. This also deletes the volume associated with it"
kind delete cluster --name kind-${BUILDKITE_BUILD_ID}
