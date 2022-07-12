#!/bin/bash

# This file contains utility functions used in the end to end testing of the operator

# Use this to debug issues. It will print the commands as they run
# set -x
shopt -s expand_aliases
alias vtctlclient="vtctlclient --server=localhost:15999"
alias mysql="mysql -h 127.0.0.1 -P 15306 -u user"
BUILDKITE_BUILD_ID=${BUILDKITE_BUILD_ID:-"0"}

function checkSemiSyncSetup() {
  for vttablet in $(kubectl get pods --no-headers -o custom-columns=":metadata.name" | grep "vttablet") ; do
    echo "Checking semi-sync in $vttablet"
    kubectl exec "$vttablet" -c mysqld -- mysql -S "/vt/socket/mysql.sock" -u root -e "show variables like 'rpl_semi_sync_slave_enabled'" | grep "ON"
    if [ $? -ne 0 ]; then
      echo "Semi Sync not setup on $vttablet"
      exit 1
    fi
  done
}

# getAllReplicaTablets returns the list of all the replica tablets as a space separated list
function getAllReplicaTablets() {
  vtctlclient ListAllTablets | grep "replica" | awk '{print $1}' | tr '\n' ' '
}

function printMysqlErrorFiles() {
  for vttablet in $(kubectl get pods --no-headers -o custom-columns=":metadata.name" | grep "vttablet") ; do
    echo "Finding error.log file in $vttablet"
    kubectl logs "$vttablet" -c mysqld
    kubectl logs "$vttablet" -c vttablet
  done
}

function printBackupLogFiles() {
  for vtbackup in $(kubectl get pods --no-headers -o custom-columns=":metadata.name" | grep "vtbackup") ; do
    echo "Printing logs of $vtbackup"
    kubectl logs "$vtbackup"
    echo "Description of $vtbackup"
    kubectl describe pod "$vtbackup"
    echo "User in $vtbackup"
    kubectl exec "$vtbackup" -- whoami
  done
}

function removeBackupFiles() {
  for vttablet in $(kubectl get pods --no-headers -o custom-columns=":metadata.name" | grep "vttablet") ; do
    echo "Removing backup files using $vttablet"
    kubectl exec "$vttablet" -c vttablet -- rm -rf /vt/backups/example
    return 0
  done
}

# takeBackup:
# $1: keyspace-shard for which the backup needs to be taken
function takeBackup() {
  keyspaceShard=$1
  initialBackupCount=$(kubectl get vtb --no-headers | wc -l)
  finalBackupCount=$((initialBackupCount+1))

  # issue the backupShard command to vtctlclient
  vtctlclient BackupShard "$keyspaceShard"

  for i in {1..600} ; do
    out=$(kubectl get vtb --no-headers | wc -l)
    echo "$out" | grep "$finalBackupCount" > /dev/null 2>&1
    if [ $? -eq 0 ]; then
      echo "Backup created"
      return 0
    fi
    sleep 3
  done
}

function dockerContainersInspect() {
  for container in $(docker container ls --format '{{.Names}}') ; do
    echo "Container - $container"
    docker container inspect "$container"
  done
}

# checkPodStatusWithTimeout:
# $1: regex used to match pod names
# $2: number of pods to match (default: 1)
function checkPodStatusWithTimeout() {
  regex=$1
  nb=$2

  # Number of pods to match defaults to one
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

function verifyVtGateVersion() {
  version=$1
  data=$(mysql -e "select @@version")
  echo "$data" | grep "$version" > /dev/null 2>&1
  if [ $? -ne 0 ]; then
    echo -e "The data in $shard's tables is incorrect, got:\n$data"
    exit 1
  fi
}


# verifyDurabilityPolicy verifies the durability policy
# in the given keyspace
function verifyDurabilityPolicy() {
  keyspace=$1
  durabilityPolicy=$2
  data=$(vtctlclient GetKeyspace "$keyspace")
  echo "$data" | grep "\"durability_policy\": \"$durabilityPolicy\"" > /dev/null 2>&1
  if [ $? -ne 0 ]; then
    echo -e "The durability policy in $keyspace is incorrect, got:\n$data"
    exit 1
  fi
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

# get_started:
# $1: operator file to use
# $2: initial config file to use
function get_started() {
    echo "Apply latest $1"
    kubectl apply -f "$1"
    checkPodStatusWithTimeout "vitess-operator(.*)1/1(.*)Running(.*)"

    echo "Apply $2"
    kubectl apply -f "$2"
    checkPodStatusWithTimeout "example-zone1-vtctld(.*)1/1(.*)Running(.*)"
    checkPodStatusWithTimeout "example-zone1-vtgate(.*)1/1(.*)Running(.*)"
    checkPodStatusWithTimeout "example-etcd(.*)1/1(.*)Running(.*)" 3
    checkPodStatusWithTimeout "example-vttablet-zone1(.*)3/3(.*)Running(.*)" 3

    sleep 10
    echo "Creating vschema and commerce SQL schema"

    ./pf.sh > /dev/null 2>&1 &
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
