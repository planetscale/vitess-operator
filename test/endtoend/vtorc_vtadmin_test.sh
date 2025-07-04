#!/bin/bash

source ./tools/test.env
source ./test/endtoend/utils.sh

# get_started_vtorc_vtadmin:
function get_started_vtorc_vtadmin() {
    echo "Apply latest operator-latest.yaml"
    kubectl apply -f "operator-latest.yaml"
    checkPodStatusWithTimeout "vitess-operator(.*)1/1(.*)Running(.*)"

    echo "Apply 101_initial_cluster_vtorc_vtadmin.yaml"
    kubectl apply -f "101_initial_cluster_vtorc_vtadmin.yaml"
    checkPodStatusWithTimeout "example-etcd(.*)1/1(.*)Running(.*)" 3
    checkPodStatusWithTimeout "example-zone1-vtctld(.*)1/1(.*)Running(.*)"
    checkPodStatusWithTimeout "example-zone1-vtgate(.*)1/1(.*)Running(.*)"
    checkPodStatusWithTimeout "example-commerce-x-x-zone1-vtorc(.*)1/1(.*)Running(.*)"
    checkPodStatusWithTimeout "example-vttablet-zone1(.*)3/3(.*)Running(.*)" 3
    checkPodStatusWithTimeout "example-zone1-vtadmin(.*)2/2(.*)Running(.*)"

    ensurePodResourcesSet "example-zone1-vtadmin"

    setupPortForwarding with_vtadmin
    waitForKeyspaceToBeServing commerce - 2
    verifyDataCommerce create
}

# verifyVtadminSetup verifies that we can query the vtadmin api end point
function verifyVtadminSetup() {
  # Verify the debug/env page can be curled and it contains the kubernetes environment variables like HOSTNAME
  curlGetRequestWithRetry "localhost:14001/debug/env" "HOSTNAME=example-zone1-vtadmin"
  # Verify the api/keyspaces page can be curled and it contains the name of the keyspace created
  curlGetRequestWithRetry "localhost:14001/api/keyspaces" "commerce"
  # Verify the other APIs work as well
  curlGetRequestWithRetry "localhost:14001/api/tablets" '"tablets":\[{"cluster":{"id":"example","name":"example"},"tablet":{"alias":{"cell":"zone1"'
  curlGetRequestWithRetry "localhost:14001/api/schemas" '"keyspace":"commerce","table_definitions":\[{"name":"corder","schema":"CREATE TABLE `corder` (\\n  `order_id`'
  # Verify that we are able to create a keyspace
  curlPostRequest "localhost:14001/api/keyspace/example" '{"name":"testKeyspace"}'
  # List the keyspaces and check that we have them both
  curlGetRequestWithRetry "localhost:14001/api/keyspaces" "commerce.*testKeyspace"
  # Try and delete the keyspace but this should fail because of the rbac rules
  curlDeleteRequest "localhost:14001/api/keyspace/example/testKeyspace" "unauthorized.*cannot.*delete.*keyspace"
  # We should still have both the keyspaces
  curlGetRequestWithRetry "localhost:14001/api/keyspaces" "commerce.*testKeyspace"
  # Delete the keyspace by using the vtctldclient
  vtctldclient DeleteKeyspace testKeyspace
  # Verify we still have the commerce keyspace and no other keyspace
  curlGetRequestWithRetry "localhost:14001/api/keyspaces" "commerce.*}}}}]"
  # Get the list of uuids of the tablets
  uuids=$(curl "localhost:14001/api/tablets" | grep "uid\":[0-9]*" -o | grep "[0-9]*" -o)
  echo "All uuids of tablets - $uuids"
  primaryUUID=$(echo "$uuids" | awk '{ if(NR==1){print $1;}}')
  echo "Primary UUID - $primaryUUID"
  # Verify it is indeed the primary
  request="localhost:14001/api/tablet/zone1-$primaryUUID"
  curlGetRequestWithRetry "$request" "type\":1"
  # Get the replica UUID
  replicaUUID=$(echo "$uuids" | awk '{ if(NR==2){print $1;}}')
  echo "Replica UUID - $replicaUUID"
  request="localhost:14001/api/tablet/zone1-$replicaUUID"
  curlGetRequestWithRetry "$request" "type\":2"
  # Run a PRS
  curlPostRequest "localhost:14001/api/shard/example/commerce/-/planned_failover" "{\"new_primary\":{\"cell\":\"zone1\",\"uid\":$replicaUUID}}"
  # Verify that the replica is now the primary
  curlGetRequestWithRetry "$request" "type\":1"

  # Also verify that the web page works
  chromiumHeadlessRequest "http://localhost:14000/schemas" "corder"
  chromiumHeadlessRequest "http://localhost:14000/schemas" "customer"
  chromiumHeadlessRequest "http://localhost:14000/keyspace/example/commerce/shards" "commerce/-"
}

# verifyVTOrcSetup verifies that VTOrc is running and repairing things that we mess up
function verifyVTOrcSetup() {
  # Stop replication using the vtctld and wait for VTOrc to repair
  allReplicaTablets=$(getAllReplicaTablets)
  for replica in $(echo "$allReplicaTablets") ; do
    vtctldclient StopReplication "$replica"
  done
  # Now that we have stopped replication on both the tablets, we know that this will
  # only succeed if VTOrc is able to fix it since we are running vttablet with disable active reparent
  # and semi-sync durability policy
  mysql -e "insert into customer(email) values('newemail@domain.com');"

  # Set primary tablets to read-only using the vtctld and wait for VTOrc to repair
  allPrimaryTablets=$(getAllPrimaryTablets)
  for primary in $(echo "$allPrimaryTablets") ; do
    vtctldclient SetWritable "$primary" false
  done

  # This query will only succeed after VTOrc has repaired the primary's to be read-write again
  runSQLWithRetry "insert into customer(email) values('newemail2@domain.com');"
}

function chromiumHeadlessRequest() {
  url=$1
  dataToAssert=$2
  for i in {1..600} ; do
    chromiumBinary=$(getChromiumBinaryName)
    res=$($chromiumBinary --headless --no-sandbox --disable-gpu --enable-logging --dump-dom  --virtual-time-budget=900000000 "$url")
    if [[ $? -eq 0 ]]; then
      echo "$res" | grep "$dataToAssert" > /dev/null 2>&1
      if [[ $? -ne 0 ]]; then
        echo -e "The data in $url is incorrect, got:\n$res, retrying"
        sleep 1
        continue
      fi
      return
    fi
    echo "failed to query url $url, retrying (attempt #$i) ..."
    sleep 1
  done
}

function getChromiumBinaryName() {
  which chromium-browser > /dev/null
  if [[ $? -eq 0 ]]; then
      echo "chromium-browser"
      return
  fi
  which chromium > /dev/null
  if [[ $? -eq 0 ]]; then
      echo "chromium"
      return
  fi
}

function curlGetRequestWithRetry() {
  url=$1
  dataToAssert=$2
  for i in {1..600} ; do
    res=$(curl "$url")
    if [[ $? -eq 0 ]]; then
      echo "$res" | grep "$dataToAssert" > /dev/null 2>&1
      if [[ $? -ne 0 ]]; then
        echo -e "The data in $url is incorrect, got:\n$res"
        exit 1
      fi
      return
    fi
    echo "failed to query url $url, retrying (attempt #$i) ..."
    sleep 1
  done
}

function curlDeleteRequest() {
  url=$1
  dataToAssert=$2
  res=$(curl -X DELETE "$url")
  if [[ $? -ne 0 ]]; then
    echo -e "The DELETE request to $url failed\n"
    exit 1
  fi
  echo "$res" | grep "$dataToAssert" > /dev/null 2>&1
  if [[ $? -ne 0 ]]; then
    echo -e "The data in delete request to $url is incorrect, got:\n$res"
    exit 1
  fi
}

function curlPostRequest() {
  url=$1
  data=$2
  curl -X POST -d "$data" "$url"
  if [[ $? -ne 0 ]]; then
    echo -e "The POST request to $url with data $data failed\n"
    exit 1
  fi
}

# Test setup
setupKindCluster
cd test/endtoend/operator || exit 1

get_started_vtorc_vtadmin
verifyVtGateVersion "23.0.0"
checkSemiSyncSetup

# Check Vtadmin is setup
# In get_started_vtorc_vtadmin we verify that the pod for vtadmin exists and is healthy
# We now try and query the vtadmin api
verifyVtadminSetup
# Next we check that VTOrc is running properly and is able to fix issues as they come up
verifyVTOrcSetup

# Teardown
teardownKindCluster
