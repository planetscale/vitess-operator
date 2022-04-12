#!/bin/bash

source ./tools/test.env

BUILDKITE_BUILD_ID=${BUILDKITE_BUILD_ID:-"0"}

alias vtctlclient="vtctlclient -server=localhost:15999"
alias mysql="mysql -h 127.0.0.1 -P 15306 -u user"
shopt -s expand_aliases


function printMysqlErrorFiles() {
  for vttablet in $(kubectl get pods --no-headers -o custom-columns=":metadata.name" | grep "vttablet") ; do
    echo "Finding error.log file in $vttablet"
    kubectl logs "$vttablet" -c mysqld
    kubectl logs "$vttablet" -c vttablet
  done
}

# checkPodStatusWithTimeout:
# $1: regex used to match pod names
# $2: number of pods to match (default: 1)
function checkPodStatusWithTimeout() {
  regex=$1
  nb=$2

  # Number of pods to match defaults to zero
  if [ -z "$nb" ]; then
    nb=1
  fi

  # We use this for loop instead of `kubectl wait` because we don't have access to the full pod name
  # and `kubectl wait` does not support regex to match resource name.
  for i in {1..1200} ; do
    out=$(kubectl get pods)
    echo "$out" | grep -E "$regex" | wc -l | grep "$nb" > /dev/null 2>&1
    if [ $? -eq 0 ]; then
      echo "$regex found"
      return
    fi
    sleep 1
  done
  echo -e "ERROR: checkPodStatusWithTimeout timeout to find pod matching:\ngot:\n$out\nfor regex: $regex"
  exit 1
}

function insertWithRetry() {
  for i in {1..600} ; do
    mysql --table < ../common/delete_commerce_data.sql && mysql --table < ../common/insert_commerce_data.sql
    if [ $? -eq 0 ]; then
      return
    fi
    echo "failed to insert commerce data, retrying (attempt #$i) ..."
    sleep 1
  done
}

function waitForKeyspaceToBeServing() {
  ks=$1
  shard=$2
  nb_of_replica=$3
  for i in {1..600} ; do
    out=$(mysql --table --execute="show vitess_tablets")
    echo "$out" | grep -E "$ks(.*)$shard(.*)PRIMARY(.*)SERVING|$ks(.*)$shard(.*)REPLICA(.*)SERVING" | wc -l | grep "$((nb_of_replica+1))"
    if [ $? -eq 0 ]; then
      echo "Shard $ks/$shard is serving"
      return
    fi
    echo "Shard $ks/$shard is not fully serving, retrying (attempt #$i) ..."
    sleep 10
  done
}

function applySchemaWithRetry() {
  schema=$1
  ks=$2
  drop_sql=$3
  for i in {1..600} ; do
    vtctlclient ApplySchema -sql="$(cat $schema)" $ks
    if [ $? -eq 0 ]; then
      return
    fi
    if [ -n "$drop_sql" ]; then
      mysql --table < $drop_sql
    fi
    echo "failed to apply schema $schema, retrying (attempt #$i) ..."
    sleep 1
  done
}

function assertSelect() {
  sql=$1
  shard=$2
  expected=$3
  data=$(mysql --table < $sql)
  echo "$data" | grep "$expected" > /dev/null 2>&1
  if [ $? -ne 0 ]; then
    echo -e "The data in $shard's tables is incorrect, got:\n$data"
    exit 1
  fi
}

function get_started() {
    echo "Apply operator.yaml Version v2.5.1"
    kubectl apply -f operator.yaml
    checkPodStatusWithTimeout "vitess-operator(.*)1/1(.*)Running(.*)"

    echo "Apply 101_initial_cluster.yaml"
    kubectl apply -f 101_initial_cluster.yaml
    checkPodStatusWithTimeout "example-zone1-vtctld(.*)1/1(.*)Running(.*)"
    checkPodStatusWithTimeout "example-zone1-vtgate(.*)1/1(.*)Running(.*)"
    checkPodStatusWithTimeout "example-etcd(.*)1/1(.*)Running(.*)" 3
    checkPodStatusWithTimeout "example-vttablet-zone1(.*)3/3(.*)Running(.*)" 2

    sleep 10
    echo "Creating vschema and commerce SQL schema"

    ./pf.sh > /dev/null 2>&1 &
    sleep 5

    waitForKeyspaceToBeServing commerce - 1
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

function verifyVtGateVersion() {
  version=$1
  data=$(mysql -e "select @@version")
  echo "$data" | grep "$version" > /dev/null 2>&1
  if [ $? -ne 0 ]; then
    echo -e "The data in $shard's tables is incorrect, got:\n$data"
    exit 1
  fi
}

function move_tables() {
  echo "Apply 201_customer_tablets.yaml"
  kubectl apply -f 201_customer_tablets.yaml > /dev/null
  checkPodStatusWithTimeout "example-vttablet-zone1(.*)3/3(.*)Running(.*)" 4

  killall kubectl
  ./pf.sh > /dev/null 2>&1 &

  waitForKeyspaceToBeServing customer - 1

  sleep 10

  vtctlclient MoveTables -source commerce -tables 'customer,corder' Create customer.commerce2customer
  if [ $? -ne 0 ]; then
    echo "MoveTables failed"
    printMysqlErrorFiles
    exit 1
  fi

  sleep 10

  vdiff_out=$(vtctlclient VDiff customer.commerce2customer)
  echo "$vdiff_out" | grep "ProcessedRows: 5" | wc -l | grep "2" > /dev/null
  if [ $? -ne 0 ]; then
    echo -e "VDiff output is invalid, got:\n$vdiff_out"
    printMysqlErrorFiles
    exit 1
  fi

  vtctlclient MoveTables -tablet_types=rdonly,replica SwitchTraffic customer.commerce2customer
  if [ $? -ne 0 ]; then
    echo "SwitchTraffic for rdonly and replica failed"
    printMysqlErrorFiles
    exit 1
  fi

  vtctlclient MoveTables -tablet_types=primary SwitchTraffic customer.commerce2customer
  if [ $? -ne 0 ]; then
    echo "SwitchTraffic for primary failed"
    printMysqlErrorFiles
    exit 1
  fi

  vtctlclient MoveTables Complete customer.commerce2customer
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
  vtctlclient ApplyVSchema -vschema="$(cat vschema_commerce_seq.json)" commerce
  if [ $? -ne 0 ]; then
    echo "ApplyVschema commerce_seq during resharding failed"
    printMysqlErrorFiles
    exit 1
  fi
  sleep 4
  vtctlclient ApplyVSchema -vschema="$(cat vschema_customer_sharded.json)" customer
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
  checkPodStatusWithTimeout "example-vttablet-zone1(.*)3/3(.*)Running(.*)" 8

  killall kubectl
  ./pf.sh > /dev/null 2>&1 &
  sleep 5

  waitForKeyspaceToBeServing customer -80 1
  waitForKeyspaceToBeServing customer 80- 1

  echo "Ready to reshard ..."
  sleep 15

  vtctlclient Reshard -source_shards '-' -target_shards '-80,80-' Create customer.cust2cust
  if [ $? -ne 0 ]; then
    echo "Reshard Create failed"
    printMysqlErrorFiles
    exit 1
  fi

  sleep 15

  vdiff_out=$(vtctlclient VDiff customer.cust2cust)
  echo "$vdiff_out" | grep "ProcessedRows: 5" | wc -l | grep "2" > /dev/null
  if [ $? -ne 0 ]; then
    echo -e "VDiff output is invalid, got:\n$vdiff_out"
    # Allow failure
  fi

  vtctlclient Reshard -tablet_types=rdonly,replica SwitchTraffic customer.cust2cust
  if [ $? -ne 0 ]; then
    echo "Reshard SwitchTraffic for replica,rdonly failed"
    printMysqlErrorFiles
    exit 1
  fi
  vtctlclient Reshard -tablet_types=primary SwitchTraffic customer.cust2cust
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
  checkPodStatusWithTimeout "example-vttablet-zone1(.*)3/3(.*)Running(.*)" 6
  waitForKeyspaceToBeServing customer -80 1
  waitForKeyspaceToBeServing customer 80- 1
}

function upgradeToLatest() {
  echo "Apply operator-latest.yaml "
  kubectl apply -f operator-latest.yaml

  echo "Upgrade all the other binaries"
  kubectl apply -f cluster_upgrade.yaml

  sleep 200
  checkPodStatusWithTimeout "example-zone1-vtctld(.*)1/1(.*)Running(.*)"
  checkPodStatusWithTimeout "example-zone1-vtgate(.*)1/1(.*)Running(.*)"
  checkPodStatusWithTimeout "example-etcd(.*)1/1(.*)Running(.*)" 3
  checkPodStatusWithTimeout "example-vttablet-zone1(.*)3/3(.*)Running(.*)" 2

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

function setupKubectlAccessForCI() {
  if [ "$BUILDKITE_BUILD_ID" != "0" ]; then
    # The script is being run from buildkite, so we need to do stuff
    # https://github.com/kubernetes-sigs/kind/issues/1846#issuecomment-691565834
    # Since kind is running in a sibling container, communicating with it through kubectl is not trivial.
    # To accomplish we need to add the current docker container in the same network as the kind container
    # and change the kubectl configuration to use the port listed in the internal endpoint instead of the one
    # that is exported to the localhost by kind.
    dockerContainerName=$(docker container ls --filter "ancestor=docker" --format '{{.Names}}')
    docker network connect kind $dockerContainerName
    kind get kubeconfig --internal --name kind-${BUILDKITE_BUILD_ID} > $HOME/.kube/config
  fi
}

# Test setup
echo "Building the docker image"
docker build -f build/Dockerfile.release -t vitess-operator-pr:latest .
echo "Creating Kind cluster"
kind create cluster --wait 30s --name kind-${BUILDKITE_BUILD_ID}
echo "Loading docker image into Kind cluster"
kind load docker-image vitess-operator-pr:latest --name kind-${BUILDKITE_BUILD_ID}

cd "$PWD/test/upgrade/operator"
killall kubectl
setupKubectlAccessForCI

get_started
verifyVtGateVersion "12.0.3"
upgradeToLatest
verifyVtGateVersion "13.0.0"
move_tables
resharding

# Teardown
echo "Deleting Kind cluster"
kind delete cluster --name kind-${BUILDKITE_BUILD_ID}
