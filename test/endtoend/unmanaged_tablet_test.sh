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

    sleep 10
    echo "Creating vschema and commerce SQL schema"

    ./pf.sh > /dev/null 2>&1 &
    sleep 5

    waitForKeyspaceToBeServing commerce - 0
    sleep 5

    # Confirm that the custom sidecar DB name is in place for our
    # external/unmanaged keyspace.
    verifyCustomSidecarDBName "commerce" "_vt_ext" "true"

    applySchemaWithRetry create_commerce_schema.sql commerce drop_all_commerce_tables.sql
    vtctldclient ApplyVSchema --vschema-file="vschema_commerce_initial.json" commerce
    if [[ $? -ne 0 ]]; then
      echo "ApplySchema failed for initial commerce"
      printMysqlErrorFiles
      exit 1
    fi
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

# Check Unmanaged tablet is running properly
get_started_unmanaged

# Teardown
echo "Deleting Kind cluster. This also deletes the volume associated with it"
kind delete cluster --name kind-${BUILDKITE_BUILD_ID}
