#!/bin/bash

# This file contains utility functions used in the end to end testing of the operator

# Use this to debug issues. It will print the commands as they run
# set -x
shopt -s expand_aliases
alias vtctldclient="vtctldclient --server=localhost:15999"
alias mysql="mysql -h 127.0.0.1 -P 15306 -u user"
BUILDKITE_BUILD_ID=${BUILDKITE_BUILD_ID:-"0"}

function checkSemiSyncSetup() {
  for vttablet in $(kubectl get pods -n example --no-headers -o custom-columns=":metadata.name" | grep "vttablet") ; do
    echo "Checking semi-sync in $vttablet"
    checkSemiSyncWithRetry "$vttablet"
  done
}

function checkSemiSyncWithRetry() {
  vttablet=$1
  for i in {1..600} ; do
    kubectl exec -n example "$vttablet" -c mysqld -- mysql -S "/vt/socket/mysql.sock" -u root -e "show variables like 'rpl_semi_sync_%_enabled'" | grep "ON"
    if [[ $? -eq 0 ]]; then
      return
    fi
    sleep 1
  done
  echo "Semi Sync not setup on $vttablet"
  exit 1
}

# getAllReplicaTablets returns the list of all the replica tablets as a space separated list
function getAllReplicaTablets() {
  vtctldclient GetTablets | grep "replica" | awk '{print $1}' | tr '\n' ' '
}

# getAllPrimaryTablets returns the list of all the primary tablets as a space separated list
function getAllPrimaryTablets() {
  vtctldclient GetTablets | grep "primary" | awk '{print $1}' | tr '\n' ' '
}

# runSQLWithRetry runs the given SQL until it succeeds
function runSQLWithRetry() {
  query=$1
  for i in {1..600} ; do
    mysql -e "$query"
    if [[ $? -eq 0 ]]; then
      return
    fi
    echo "failed to run query $query, retrying (attempt #$i) ..."
    sleep 1
  done
  echo "Timed out trying to run $query"
  exit 1
}

function printMysqlErrorFiles() {
  for vttablet in $(kubectl get pods -n example --no-headers -o custom-columns=":metadata.name" | grep "vttablet") ; do
    echo "Finding error.log file in $vttablet"
    kubectl logs -n example "$vttablet" -c mysqld
    kubectl logs -n example "$vttablet" -c vttablet
  done
}

function printBackupLogFiles() {
  for vtbackup in $(kubectl get pods -n example --no-headers -o custom-columns=":metadata.name" | grep "vtbackup") ; do
    echo "Printing logs of $vtbackup"
    kubectl logs -n example "$vtbackup"
    echo "Description of $vtbackup"
    kubectl describe pod -n example "$vtbackup"
    echo "User in $vtbackup"
    kubectl exec -n example "$vtbackup" -- whoami
  done
}

function removeBackupFiles() {
  for vttablet in $(kubectl get pods -n example --no-headers -o custom-columns=":metadata.name" | grep "vttablet") ; do
    echo "Removing backup files using $vttablet"
    kubectl exec -n example "$vttablet" -c vttablet -- rm -rf /vt/backups/example
    return 0
  done
}

# takeBackup:
# $1: keyspace-shard for which the backup needs to be taken
function takeBackup() {
  keyspaceShard=$1
  initialBackupCount=$(kubectl get vtb -n example --no-headers | wc -l)
  finalBackupCount=$((initialBackupCount+1))

  # Issue the BackupShard command to vtctldclient.
  vtctldclient BackupShard "$keyspaceShard"

  if [[ $? -ne 0 ]]; then
    echo "Backup failed"
    exit 1
  fi
  # Ensure that we can view the backup files from the host.
  docker exec -it $(docker container ls --format '{{.Names}}' | grep kind) chmod o+rwx -R /backup > /dev/null
  echo "Backup completed"
}

function verifyListBackupsOutput() {
  for i in {1..30} ; do
    # Ensure that we can view the backup files from the host.
    docker exec -it $(docker container ls --format '{{.Names}}' | grep kind) chmod o+rwx -R /backup > /dev/null
    backupCount=$(kubectl get vtb -n example --no-headers | wc -l)
    out=$(vtctldclient GetBackups "$keyspaceShard" | wc -l)
    echo "$out" | grep "$backupCount" > /dev/null 2>&1
    if [[ $? -eq 0 ]]; then
      echo "GetBackups output is correct"
      return 0
    fi
    sleep 3
  done
  echo -e "ERROR: GetBackups output not correct - $out. $backupCount backups expected."
  exit 1
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
  if [[ -z "$nb" ]]; then
    nb=1
  fi

  # We use this for loop instead of `kubectl wait` because we don't have access to the full pod name
  # and `kubectl wait` does not support regex to match resource name.
  for i in {1..1200} ; do
    out=$(kubectl get pods -A)
    echo "$out" | grep -E "$regex" | wc -l | grep "$nb" > /dev/null 2>&1
    if [[ $? -eq 0 ]]; then
      echo "$regex found"
      return
    fi
    sleep 1
  done
  echo -e "ERROR: checkPodStatusWithTimeout timeout to find pod matching:\ngot:\n$out\nfor regex: $regex"
  echo "$regex" | grep "vttablet" > /dev/null 2>&1
  if [[ $? -eq 0 ]]; then
    printMysqlErrorFiles
  fi
  exit 1
}

function checkPodExistWithTimeout() {
  regex=$1

  # We use this for loop instead of `kubectl wait` because we don't have access to the full pod name
  # and `kubectl wait` does not support regex to match resource name.
  for i in {1..1200} ; do
    out=$(kubectl get pods -A)
    echo "$out" | grep -E "$regex" > /dev/null 2>&1
    if [[ $? -eq 0 ]]; then
      echo "$regex found"
      return
    fi
    sleep 1
  done
  echo -e "ERROR: checkPodStatusWithTimeout timeout to find pod matching:\ngot:\n$out\nfor regex: $regex"
  echo "$regex" | grep "vttablet" > /dev/null 2>&1
  if [[ $? -eq 0 ]]; then
    printMysqlErrorFiles
  fi
  exit 1
}

# ensurePodResourcesSet:
# $1: regex used to match pod names
function ensurePodResourcesSet() {
  regex=$1

  baseCmd='kubectl get pods -A -o custom-columns="NAME:metadata.name,CONTAINERS:spec.containers[*].name,RESOURCE:spec.containers[*].resources'

  # We don't check for .limits.cpu because it is usually unset
  for resource in '.limits.memory"' '.requests.cpu"' '.requests.memory"' ; do
    cmd=${baseCmd}${resource}
    out=$(eval "$cmd")

    numContainers=$(echo "$out" | grep -E "$regex" | awk '{print $2}' | awk -F ',' '{print NF}')
    numContainersWithResources=$(echo "$out" | grep -E "$regex" | awk '{print $3}' | awk -F ',' '{print NF}')
    if [[ $numContainers != $numContainersWithResources ]]; then
      echo "one or more containers in pods with $regex do not have $resource set"
      exit 1
    fi
  done
}

function insertWithRetry() {
  for i in {1..600} ; do
    mysql --table < ../common/delete_commerce_data.sql && mysql --table < ../common/insert_commerce_data.sql
    if [[ $? -eq 0 ]]; then
      return
    fi
    echo "failed to insert commerce data, retrying (attempt #$i) ..."
    sleep 1
  done
}

function verifyVtGateVersion() {
  version=$1
  podName=$(kubectl get pods -n example --no-headers -o custom-columns=":metadata.name" | grep "vtgate")
  data=$(kubectl logs -n example "$podName" | head)
  echo "$data" | grep "$version" > /dev/null 2>&1
  if [[ $? -ne 0 ]]; then
    echo -e "The vtgate version is incorrect, expected: $version, got:\n$data"
    exit 1
  fi
}


# verifyDurabilityPolicy verifies the durability policy
# in the given keyspace
function verifyDurabilityPolicy() {
  keyspace=$1
  durabilityPolicy=$2
  data=$(vtctldclient LegacyVtctlCommand -- GetKeyspace "$keyspace")
  echo "$data" | grep "\"durability_policy\": \"$durabilityPolicy\"" > /dev/null 2>&1
  if [[ $? -ne 0 ]]; then
    echo -e "The durability policy in $keyspace is incorrect, got:\n$data"
    exit 1
  fi
}

# verifyCustomSidecarDBName verifies that the custom sidecar DB name
# is set in the keyspace's topo record and that it is being used on
# all tablet/mysqld instances in the keyspace.
# The first parameter must be the keyspace and the second the expected
# sidecar DB name. The third parameter is optional and if passed (any
# value) then it is assumed that there is no mysqld container in the
# vttablet pods (external database) and there are instead separate
# mysql pods.
function verifyCustomSidecarDBName() {
  local keyspace=$1 db_name=$2 external=$3
  if [[ -z "${keyspace}" || -z "${db_name}" ]]; then
    echo "The keyspace or sidecar DB name are empty; usage: verifyCustomSidecarDBName <keyspace> <db_name> [external]"
    exit 1
  fi

  # First confirm that the keyspace record has the value.
  local kscfg
  kscfg=$(vtctldclient GetKeyspace "${keyspace}")
  if [[ ! "${kscfg}" =~ [\"]?sidecar_db_name[\"]?[[:space:]]*:[[:space:]]*[\"]?${db_name}[\"]? ]]; then
    echo -e "Keyspace configuration for ${keyspace} does not have the expected sidecar DB name of ${db_name}; output:\n${kscfg}"
    exit 1
  fi
  echo "Keyspace configuration for ${keyspace} has the expected sidecar DB name of ${db_name}"

  # Then confirm that each tablet/mysqld instance in the keyspace used that value.
  local container="-c mysqld"
  local mysqlCMD="mysql -S /vt/socket/mysql.sock -N -u root -e \"show databases like '${db_name}'\""
  local selector="planetscale.com/cluster=example,planetscale.com/keyspace=${keyspace},planetscale.com/component=vttablet"
  if [[ -n "${external}" ]]; then
    # There is no mysqld container in the vttablet pod.
    # See setup and connection details in 101_initial_cluster_unmanaged_tablet.yaml.
    container=""
    mysqlCMD="mysql --protocol=tcp -P3306 -NB -u root -ppassword -e \"show databases like '${db_name}'\" 2>/dev/null"
    selector="app=mysql"
  fi 
  local pods pod
  pods=$(kubectl get pods -n example --no-headers --selector="${selector}" -o custom-columns=":metadata.name")
  for pod in $(echo "${pods}"); do
    local sdb
    sdb=$(eval "kubectl exec -n example ${pod} ${container} -- ${mysqlCMD}")
    if [[ "${sdb}" != "${db_name}" ]]; then
      echo "Custom sidecar DB name ${db_name} not being used in ${pod} pod"
      exit 1
    fi
    echo "Found custom sidecar DB name ${db_name} being used in ${pod} pod"
  done
}

function waitForKeyspaceToBeServing() {
  ks=$1
  shard=$2
  nb_of_replica=$3
  for i in {1..600} ; do
    out=$(mysql --table --execute="show vitess_tablets")
    numtablets=$(echo "$out" | grep -E "$ks(.*)$shard(.*)PRIMARY(.*)SERVING|$ks(.*)$shard(.*)REPLICA(.*)SERVING" | wc -l)
    if [[ $numtablets -ge $((nb_of_replica+1)) ]]; then
      echo "Shard $ks/$shard is serving"
      return
    fi
    echo "Shard $ks/$shard is not fully serving. Output: $out"
    echo "Retrying (attempt #$i) ..."
    sleep 1
  done
}

function applySchemaWithRetry() {
  schema=$1
  ks=$2
  drop_sql=$3
  for i in {1..600} ; do
    vtctldclient ApplySchema --sql-file="$schema" $ks
    if [[ $? -eq 0 ]]; then
      return
    fi
    if [[ -n "$drop_sql" ]]; then
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
  if [[ $? -ne 0 ]]; then
    echo -e "The data in $shard's tables is incorrect, got:\n$data"
    exit 1
  fi
}

function setupKubectlAccessForCI() {
  if [[ "$BUILDKITE_BUILD_ID" != "0" ]]; then
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

function setupKindConfig() {
  echo "Setting up the kind config"
  if [[ "$BUILDKITE_BUILD_ID" != "0" ]]; then
    # The script is being run from buildkite, so we can't mount the current
    # working directory to kind. The current directory in the docker is workdir
    # So if we try and mount that, we get an error. Instead we need to mount the
    # path where the code was checked out be buildkite
    dockerContainerName=$(docker container ls --filter "ancestor=docker" --format '{{.Names}}')
    CHECKOUT_PATH=$(docker container inspect -f '{{range .Mounts}}{{ if eq .Destination "/workdir" }}{{println .Source }}{{ end }}{{end}}' "$dockerContainerName")
    BACKUP_DIR="$CHECKOUT_PATH/vtdataroot/backup"
  else
    BACKUP_DIR="$PWD/vtdataroot/backup"
  fi
  cat ./test/endtoend/kindBackupConfig.yaml | sed "s,PATH,$BACKUP_DIR,1" > ./vtdataroot/config.yaml
}

function createKindCluster() {
  echo "Creating Kind cluster"
  kind create cluster --wait 30s --name kind-${BUILDKITE_BUILD_ID} --config ./vtdataroot/config.yaml --image ${KIND_VERSION}
  echo "Loading docker image into Kind cluster"
  kind load docker-image vitess-operator-pr:latest --name kind-${BUILDKITE_BUILD_ID}
}

function createExampleNamespace() {
  echo "Creating the example namespace"
  kubectl create namespace example
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
    checkPodStatusWithTimeout "example-commerce-x-x-zone1-vtorc(.*)1/1(.*)Running(.*)"

    sleep 10
    echo "Creating vschema and commerce SQL schema"

    ./pf.sh > /dev/null 2>&1 &
    sleep 5

    waitForKeyspaceToBeServing commerce - 2
    sleep 5

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

function checkVitessBackupScheduleStatusWithTimeout() {
  regex=$1

  for i in {1..1200} ; do
    if [[ $(kubectl get VitessBackupSchedule -n example | grep -E "${regex}" | wc -l) -eq 1 ]]; then
      echo "$regex found"
      return
    fi
    sleep 1
  done
  echo -e "ERROR: checkPodStatusWithTimeout timeout to find pod matching:\ngot:\n$out\nfor regex: $regex"
  exit 1
}
