#!/bin/bash

source ./tools/test.env
source ./test/endtoend/utils.sh

# get_started_vtadmin:
function get_started_vtadmin() {
    echo "Apply latest operator-latest.yaml"
    kubectl apply -f "operator-latest.yaml"
    checkPodStatusWithTimeout "vitess-operator(.*)1/1(.*)Running(.*)"

    echo "Apply 101_initial_cluster_vtadmin.yaml"
    kubectl apply -f "101_initial_cluster_vtadmin.yaml"
    checkPodStatusWithTimeout "example-zone1-vtctld(.*)1/1(.*)Running(.*)"
    checkPodStatusWithTimeout "example-zone1-vtgate(.*)1/1(.*)Running(.*)"
    checkPodStatusWithTimeout "example-etcd(.*)1/1(.*)Running(.*)" 3
    checkPodStatusWithTimeout "example-vttablet-zone1(.*)3/3(.*)Running(.*)" 3
    checkPodStatusWithTimeout "example-zone1-vtadmin(.*)1/1(.*)Running(.*)"

    sleep 10
    echo "Creating vschema and commerce SQL schema"

    ./pf_vtadmin.sh > /dev/null 2>&1 &
    sleep 5

    waitForKeyspaceToBeServing commerce - 2
    sleep 5

    applySchemaWithRetry create_commerce_schema.sql commerce drop_all_commerce_tables.sql
    vtctlclient ApplyVSchema -vschema="$(cat vschema_commerce_initial.json)" commerce
    if [ $? -ne 0 ]; then
      echo "ApplySchema failed for initial commerce"
      printMysqlErrorFiles
      exit 1
    fi
    sleep 5

    echo "show databases;" | mysql | grep "commerce" > /dev/null 2>&1
    if [ $? -ne 0 ]; then
      echo "Could not find commerce database"
      printMysqlErrorFiles
      exit 1
    fi

    echo "show tables;" | mysql commerce | grep -E 'corder|customer|product' | wc -l | grep 3 > /dev/null 2>&1
    if [ $? -ne 0 ]; then
      echo "Could not find commerce's tables"
      printMysqlErrorFiles
      exit 1
    fi

    insertWithRetry

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

# verifyVtadminSetup verifies that we can query the vtadmin api end point
function verifyVtadminSetup() {
  # Verify the debug/env page can be curled and it contains the kubernetes environment variables like HOSTNAME
  curlRequestWithRetry "localhost:14001/debug/env" "HOSTNAME=example-zone1-vtadmin"
  # Verify the api/keyspaces page can be curled and it contains the name of the keyspace created
  curlRequestWithRetry "localhost:14001/api/keyspaces" "commerce"
  # Verify the other APIs work as well
  curlRequestWithRetry "localhost:14001/api/tablets" '"tablets":\[{"cluster":{"id":"zone1","name":"zone1"},"tablet":{"alias":{"cell":"zone1"'
  curlRequestWithRetry "localhost:14001/api/schemas" '"keyspace":"commerce","table_definitions":\[{"name":"corder","schema":"CREATE TABLE `corder` (\\n  `order_id` bigint(20) NOT NULL AUTO_INCREMENT'
}

function curlRequestWithRetry() {
  url=$1
  dataToAssert=$2
  for i in {1..600} ; do
    res=$(curl "$1")
    if [ $? -eq 0 ]; then
      echo "$res" | grep "$dataToAssert" > /dev/null 2>&1
      if [ $? -ne 0 ]; then
        echo -e "The data in $url is incorrect, got:\n$res"
        exit 1
      fi
      return
    fi
    echo "failed to query url $url, retrying (attempt #$i) ..."
    sleep 1
  done
}

# Test setup
echo "Building the docker image"
docker build -f build/Dockerfile.release -t vitess-operator-pr:latest .
echo "Creating Kind cluster"
kind create cluster --wait 30s --name kind-${BUILDKITE_BUILD_ID}
echo "Loading docker image into Kind cluster"
kind load docker-image vitess-operator-pr:latest --name kind-${BUILDKITE_BUILD_ID}

cd "$PWD/test/endtoend/operator"
killall kubectl
setupKubectlAccessForCI

get_started_vtadmin
verifyVtGateVersion "14.0.0"
checkSemiSyncSetup

# Check Vtadmin is setup
# In get_started_vtadmin we verify that the pod for vtadmin exists and is healthy
# We now try and query the vtadmin api
verifyVtadminSetup

# Teardown
#echo "Deleting Kind cluster. This also deletes the volume associated with it"
kind delete cluster --name kind-${BUILDKITE_BUILD_ID}
