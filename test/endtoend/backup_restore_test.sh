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

  waitForKeyspaceToBeServing commerce - 2
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

function setupKindConfig() {
  if [ "$BUILDKITE_BUILD_ID" != "0" ]; then
    # The script is being run from buildkite, so we can't mount the current
    # working directory to kind. The current directory in the docker is workdir
    # So if we try and mount that, we get an error. Instead we need to mount the
    # path where the code was checked out be buildkite
    echo "Running in buildkite -- setting up environment"
    dockerContainerName=$(docker container ls --filter "ancestor=docker" --format '{{.Names}}')
    CHECKOUT_PATH=$(docker container inspect -f '{{range .Mounts}}{{ if eq .Destination "/workdir" }}{{println .Source }}{{ end }}{{end}}' "$dockerContainerName")
    echo "Using checkout path: $CHECKOUT_PATH"
    BACKUP_DIR="$CHECKOUT_PATH/vtdataroot/backup"
    echo "Using backup directory: $BACKUP_DIR"
  else
    BACKUP_DIR="$PWD/vtdataroot/backup"
  fi
  cat ./test/endtoend/kindBackupConfig.yaml | sed "s,PATH,$BACKUP_DIR,1" > ./vtdataroot/config.yaml
  echo "Using Kind config: $(cat ./vtdataroot/config.yaml)"
}

# Test setup
KUBE_VERSION="${KUBE_VERSION:-v1.24.15}"
STARTING_DIR="$PWD"
echo "Make temporary directory for the test"
mkdir -p -m 750 ./vtdataroot/backup
echo "Building the docker image"
docker build -f build/Dockerfile.release -t vitess-operator-pr:latest .
echo "Setting up the kind config"
setupKindConfig
export CLUSTER_NAME="kind-${BUILDKITE_BUILD_ID}"
echo "Creating Kind cluster with name: ${CLUSTER_NAME} and config: $(cat ./vtdataroot/config.yaml)"
kind create cluster --image "kindest/node:${KUBE_VERSION}" --wait 30s --name "${CLUSTER_NAME}" --config ./vtdataroot/config.yaml || die "Failed to create Kind cluster" 
setupKubectlAccessForCI
echo "Loading docker image into Kind cluster"
kind load docker-image vitess-operator-pr:latest --name "${CLUSTER_NAME}" || die "Failed to load docker image into Kind cluster"

cd "$PWD/test/endtoend/operator"
killall kubectl

get_started "operator-latest.yaml" "101_initial_cluster_backup.yaml"
verifyVtGateVersion "17.0.1"
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
kind delete cluster --name "${CLUSTER_NAME}"
