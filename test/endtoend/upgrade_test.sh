#!/bin/bash

source ./tools/test.env
source ./test/endtoend/utils.sh

function move_tables() {
  echo "Apply 201_customer_tablets.yaml"
  kubectl apply -f 201_customer_tablets.yaml > /dev/null
  checkPodStatusWithTimeout "example-customer-x-x-zone1-vtorc(.*)1/1(.*)Running(.*)"
  checkPodStatusWithTimeout "example-vttablet-zone1(.*)3/3(.*)Running(.*)" 6

  setupPortForwarding
  waitForKeyspaceToBeServing customer - 2

  echo "Execute MoveTables"
  vtctldclient LegacyVtctlCommand -- MoveTables --source commerce --tables 'customer,corder' Create customer.commerce2customer
  if [ $? -ne 0 ]; then
    echo "MoveTables failed"
    printMysqlErrorFiles
    exit 1
  fi

  sleep 10

  echo "Execute VDiff"
  vdiff_out=$(vtctldclient LegacyVtctlCommand -- VDiff customer.commerce2customer)
  echo "$vdiff_out" | grep "ProcessedRows: 5" | wc -l | grep "2" > /dev/null
  if [ $? -ne 0 ]; then
    echo -e "VDiff output is invalid, got:\n$vdiff_out"
    # Allow failure
  fi

  echo "SwitchTraffic for rdonly"
  vtctldclient LegacyVtctlCommand -- MoveTables --tablet_types='rdonly,replica' SwitchTraffic customer.commerce2customer
  if [ $? -ne 0 ]; then
    echo "SwitchTraffic for rdonly and replica failed"
    printMysqlErrorFiles
    exit 1
  fi

  echo "SwitchTraffic for primary"
  vtctldclient LegacyVtctlCommand -- MoveTables --tablet_types='primary' SwitchTraffic customer.commerce2customer
  if [ $? -ne 0 ]; then
    echo "SwitchTraffic for primary failed"
    printMysqlErrorFiles
    exit 1
  fi

  echo "Complete MoveTables"
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
  checkPodStatusWithTimeout "example-customer-8000-x-zone1-vtorc(.*)1/1(.*)Running(.*)"
  checkPodStatusWithTimeout "example-customer-x-8000-zone1-vtorc(.*)1/1(.*)Running(.*)"
  checkPodStatusWithTimeout "example-vttablet-zone1(.*)3/3(.*)Running(.*)" 12

  setupPortForwarding
  waitForKeyspaceToBeServing customer -8000 2
  waitForKeyspaceToBeServing customer 8000- 2

  echo "Ready to reshard ..."
  vtctldclient LegacyVtctlCommand -- Reshard --source_shards '-' --target_shards '-8000,8000-' Create customer.cust2cust
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

  assertSelect ../common/select_customer-80_data.sql "customer/-8000" << EOF
Using customer/-8000
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

  assertSelect ../common/select_customer80-_data.sql "customer/8000-" << EOF
Using customer/8000-
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

  # Complete the reshard process
  vtctldclient LegacyVtctlCommand -- Reshard Complete customer.cust2cust
  if [ $? -ne 0 ]; then
    echo "Reshard Complete failed"
    printMysqlErrorFiles
    exit 1
  fi

  kubectl apply -f 306_down_shard_0.yaml
  checkPodStatusWithTimeout "example-vttablet-zone1(.*)3/3(.*)Running(.*)" 9
  waitForKeyspaceToBeServing customer -8000 2
  waitForKeyspaceToBeServing customer 8000- 2
}

function scheduledBackups() {
  echo "Apply 401_scheduled_backups.yaml"
  kubectl apply -f 401_scheduled_backups.yaml > /dev/null

  checkVitessBackupScheduleStatusWithTimeout "example-vbsc-commerce(.*)"
  checkVitessBackupScheduleStatusWithTimeout "example-vbsc-customer(.*)"

  initialCommerceBackups=$(kubectl get vtb -n example --no-headers | grep "commerce-x-x" | wc -l)
  initialCustomerFirstShardBackups=$(kubectl get vtb -n example --no-headers | grep "customer-x-8000" | wc -l)
  initialCustomerSecondShardBackups=$(kubectl get vtb -n example --no-headers | grep "customer-8000-x" | wc -l)

  for i in {1..60} ; do
    commerceBackups=$(kubectl get vtb -n example --no-headers | grep "commerce-x-x" | wc -l)
    customerFirstShardBackups=$(kubectl get vtb -n example --no-headers | grep "customer-x-8000" | wc -l)
    customerSecondShardBackups=$(kubectl get vtb -n example --no-headers | grep "customer-8000-x" | wc -l)

    if [[ "${customerFirstShardBackups}" -ge $(( initialCustomerFirstShardBackups + 2 )) && "${customerSecondShardBackups}" -ge $(( initialCustomerSecondShardBackups + 2 )) && "${commerceBackups}" -ge $(( initialCommerceBackups + 2 )) ]]; then
      echo "Found all backups"
      return
    else
      echo "Got: ${customerFirstShardBackups} customer-x-8000 backups but want: $(( initialCustomerFirstShardBackups + 2 ))"
      echo "Got: ${customerSecondShardBackups} customer-8000-x backups but want: $(( initialCustomerSecondShardBackups + 2 ))"
      echo "Got: ${commerceBackups} commerce-x-x backups but want: $(( initialCommerceBackups + 2 ))"
      echo ""
    fi
    sleep 10
  done

  echo "Did not find the backups on time"
  exit 1
}

function verifyVtgateDeploymentStrategy() {
  echo "Verifying the deployment strategy of vtgate"
  vtgate=$(kubectl get deployments  -n example --no-headers -o custom-columns=":metadata.name" | grep "vtgate")
  if [[ $? -eq 1 ]]; then
    echo "Could not find the vtgate deployment"
    exit 1
  fi

  rollingUpdateStr=$(kubectl describe deployment -n example ${vtgate} | grep "RollingUpdateStrategy:")
  if [[ $? -eq 1 ]]; then
    echo "Could not find vtgate's rolling update strategy"
    exit 1
  fi

  if [[ "${rollingUpdateStr}" != "RollingUpdateStrategy:  0 max unavailable, 1 max surge" ]]; then
    echo "Could not find the correct rolling update strategy, got: ${rollingUpdateStr}"
    exit 1
  fi
  echo "Found the correct deployment strategy"
}

function upgradeToLatest() {
  echo "Upgrade Vitess Operator"
  kubectl apply -f operator-latest.yaml
  checkPodSpecBySelectorWithTimeout default "app=vitess-operator" 1 "image: vitess-operator-pr:latest"
  checkPodStatusWithTimeout "vitess-operator(.*)1/1(.*)Running(.*)"

  echo "Upgrade Vitess binaries"
  kubectl apply -f cluster_upgrade.yaml
  checkPodStatusWithTimeout "example-etcd(.*)1/1(.*)Running(.*)" 3
  checkPodStatusWithTimeout "example-zone1-vtctld(.*)1/1(.*)Running(.*)"
  checkPodStatusWithTimeout "example-zone1-vtgate(.*)1/1(.*)Running(.*)"
  checkPodStatusWithTimeout "example-commerce-x-x-zone1-vtorc(.*)1/1(.*)Running(.*)"
  checkPodStatusWithTimeout "example-vttablet-zone1(.*)3/3(.*)Running(.*)" 3

  # Wait for the cluster spec changes to take effect
  checkPodSpecBySelectorWithTimeout example "planetscale.com/component=vtctld" 1 "image: vitess/lite:mysql80"
  checkPodSpecBySelectorWithTimeout example "planetscale.com/component=vtgate" 1 "image: vitess/lite:mysql80"
  checkPodSpecBySelectorWithTimeout example "planetscale.com/component=vtorc" 1 "image: vitess/lite:mysql80"
  checkPodSpecBySelectorWithTimeout example "planetscale.com/component=vttablet" 12 "image: vitess/lite:mysql80"

  verifyVtgateDeploymentStrategy

  setupPortForwarding
  waitForKeyspaceToBeServing commerce - 2
  verifyDataCommerce
}

function verifyResourceSpec() {
  echo "Verifying resource spec"

  echo "mysqld_exporter flags:"
  checkPodSpecBySelectorWithTimeout example "planetscale.com/component=vttablet" 3 "--no-collect.info_schema.innodb_cmpmem$"
  checkPodSpecBySelectorWithTimeout example "planetscale.com/component=vttablet" 3 "--collect.info_schema.tables$"
  checkPodSpecBySelectorWithTimeout example "planetscale.com/component=vttablet" 3 "--collect.info_schema.tables.databases=\*$"
}

# Test setup
setupKindCluster
cd test/endtoend/operator || exit 1

get_started "operator-latest.yaml" "101_initial_cluster.yaml"
verifyVtGateVersion "22.0.1"
checkSemiSyncSetup
# Initially too durability policy should be specified
verifyDurabilityPolicy "commerce" "semi_sync"
upgradeToLatest
verifyVtGateVersion "23.0.0"
verifyResourceSpec
checkSemiSyncSetup
# After upgrading, we verify that the durability policy is still semi_sync
verifyDurabilityPolicy "commerce" "semi_sync"
move_tables
resharding

scheduledBackups

# Teardown
teardownKindCluster
