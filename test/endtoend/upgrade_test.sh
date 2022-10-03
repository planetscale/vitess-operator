#!/bin/bash

source ./tools/test.env
source ./test/endtoend/utils.sh

function move_tables() {
  echo "Apply 201_customer_tablets.yaml"
  kubectl apply -f 201_customer_tablets.yaml > /dev/null
  checkPodStatusWithTimeout "example-vttablet-zone1(.*)3/3(.*)Running(.*)" 6

  killall kubectl
  ./pf.sh > /dev/null 2>&1 &

  waitForKeyspaceToBeServing customer - 2

  sleep 10

  vtctldclient LegacyVtctlCommand -- MoveTables --source commerce --tables 'customer,corder' Create customer.commerce2customer
  if [ $? -ne 0 ]; then
    echo "MoveTables failed"
    printMysqlErrorFiles
    exit 1
  fi

  sleep 10

  vdiff_out=$(vtctldclient LegacyVtctlCommand -- VDiff customer.commerce2customer)
  echo "$vdiff_out" | grep "ProcessedRows: 5" | wc -l | grep "2" > /dev/null
  if [ $? -ne 0 ]; then
    echo -e "VDiff output is invalid, got:\n$vdiff_out"
    printMysqlErrorFiles
    exit 1
  fi

  vtctldclient LegacyVtctlCommand -- MoveTables --tablet_types='rdonly,replica' SwitchTraffic customer.commerce2customer
  if [ $? -ne 0 ]; then
    echo "SwitchTraffic for rdonly and replica failed"
    printMysqlErrorFiles
    exit 1
  fi

  vtctldclient LegacyVtctlCommand -- MoveTables --tablet_types='primary' SwitchTraffic customer.commerce2customer
  if [ $? -ne 0 ]; then
    echo "SwitchTraffic for primary failed"
    printMysqlErrorFiles
    exit 1
  fi

  vtctldclient LegacyVtctlCommand -- MoveTables Complete customer.commerce2customer
  if [ $? -ne 0 ]; then
    echo "MoveTables Complete failed"
    printMysqlErrorFiles
    exit 1
  fi

  sleep 10
}

function resharding() {
  echo "Create new schemas for new shards"
  applySchemaWithRetry create_commerce_seq.sql commerce
  sleep 4
  vtctldclient ApplyVSchema --vschema-file="vschema_commerce_seq.json" commerce
  if [ $? -ne 0 ]; then
    echo "ApplyVschema commerce_seq during resharding failed"
    printMysqlErrorFiles
    exit 1
  fi
  sleep 4
  vtctldclient ApplyVSchema --vschema-file="vschema_customer_sharded.json" customer
  if [ $? -ne 0 ]; then
    echo "ApplyVschema customer_sharded during resharding failed"
    printMysqlErrorFiles
    exit 1
  fi
  sleep 4
  applySchemaWithRetry create_customer_sharded.sql customer
  sleep 4

  echo "Apply 302_new_shards.yaml"
  kubectl apply -f 302_new_shards.yaml
  checkPodStatusWithTimeout "example-vttablet-zone1(.*)3/3(.*)Running(.*)" 12

  killall kubectl
  ./pf.sh > /dev/null 2>&1 &
  sleep 5

  waitForKeyspaceToBeServing customer -80 2
  waitForKeyspaceToBeServing customer 80- 2

  echo "Ready to reshard ..."
  sleep 15

  vtctldclient LegacyVtctlCommand -- Reshard --source_shards '-' --target_shards '-80,80-' Create customer.cust2cust
  if [ $? -ne 0 ]; then
    echo "Reshard Create failed"
    printMysqlErrorFiles
    exit 1
  fi

  sleep 15

  vdiff_out=$(vtctldclient LegacyVtctlCommand -- VDiff customer.cust2cust)
  echo "$vdiff_out" | grep "ProcessedRows: 5" | wc -l | grep "2" > /dev/null
  if [ $? -ne 0 ]; then
    echo -e "VDiff output is invalid, got:\n$vdiff_out"
    # Allow failure
  fi

  vtctldclient LegacyVtctlCommand -- Reshard --tablet_types='rdonly,replica' SwitchTraffic customer.cust2cust
  if [ $? -ne 0 ]; then
    echo "Reshard SwitchTraffic for replica,rdonly failed"
    printMysqlErrorFiles
    exit 1
  fi
  vtctldclient LegacyVtctlCommand -- Reshard --tablet_types='primary' SwitchTraffic customer.cust2cust
  if [ $? -ne 0 ]; then
    echo "Reshard SwitchTraffic for primary failed"
    printMysqlErrorFiles
    exit 1
  fi

  sleep 10

  assertSelect ../common/select_customer-80_data.sql "customer/-80" << EOF
Using customer/-80
Customer
+-------------+--------------------+
| customer_id | email              |
+-------------+--------------------+
|           1 | alice@domain.com   |
|           2 | bob@domain.com     |
|           3 | charlie@domain.com |
|           5 | eve@domain.com     |
+-------------+--------------------+
COrder
+----------+-------------+----------+-------+
| order_id | customer_id | sku      | price |
+----------+-------------+----------+-------+
|        1 |           1 | SKU-1001 |   100 |
|        2 |           2 | SKU-1002 |    30 |
|        3 |           3 | SKU-1002 |    30 |
|        5 |           5 | SKU-1002 |    30 |
+----------+-------------+----------+-------+
EOF

  assertSelect ../common/select_customer80-_data.sql "customer/80-" << EOF
Using customer/80-
Customer
+-------------+----------------+
| customer_id | email          |
+-------------+----------------+
|           4 | dan@domain.com |
+-------------+----------------+
COrder
+----------+-------------+----------+-------+
| order_id | customer_id | sku      | price |
+----------+-------------+----------+-------+
|        4 |           4 | SKU-1002 |    30 |
+----------+-------------+----------+-------+
EOF

  kubectl apply -f 306_down_shard_0.yaml
  checkPodStatusWithTimeout "example-vttablet-zone1(.*)3/3(.*)Running(.*)" 9
  waitForKeyspaceToBeServing customer -80 2
  waitForKeyspaceToBeServing customer 80- 2
}

function upgradeToLatest() {
  echo "Apply operator-latest.yaml "
  kubectl apply -f operator-latest.yaml

  sleep 2

  echo "Upgrade all the other binaries"
  kubectl apply -f cluster_upgrade.yaml

  sleep 200
  checkPodStatusWithTimeout "example-zone1-vtctld(.*)1/1(.*)Running(.*)"
  checkPodStatusWithTimeout "example-zone1-vtgate(.*)1/1(.*)Running(.*)"
  checkPodStatusWithTimeout "example-etcd(.*)1/1(.*)Running(.*)" 3
  checkPodStatusWithTimeout "example-vttablet-zone1(.*)3/3(.*)Running(.*)" 3

  killall kubectl
  ./pf.sh > /dev/null 2>&1 &

  sleep 10

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
kind create cluster --wait 30s --name kind-${BUILDKITE_BUILD_ID}
echo "Loading docker image into Kind cluster"
kind load docker-image vitess-operator-pr:latest --name kind-${BUILDKITE_BUILD_ID}

cd "$PWD/test/endtoend/operator"
killall kubectl
setupKubectlAccessForCI

get_started "operator.yaml" "101_initial_cluster.yaml"
verifyVtGateVersion "14.0.0"
checkSemiSyncSetup
# Initially no durability policy is specified
verifyDurabilityPolicy "commerce" ""
upgradeToLatest
verifyVtGateVersion "15.0.0"
checkSemiSyncSetup
# After upgrading, we set the durability policy to semi_sync
verifyDurabilityPolicy "commerce" "semi_sync"
move_tables
resharding

# Teardown
echo "Deleting Kind cluster. This also deletes the volume associated with it"
kind delete cluster --name kind-${BUILDKITE_BUILD_ID}
