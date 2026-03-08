#!/bin/bash

source ./tools/test.env
source ./test/endtoend/utils.sh

# get_started_two_keyspaces sets up the cluster with two keyspaces (commerce and customer).
# It waits for all components to be ready before returning.
function get_started_two_keyspaces() {
    echo "Apply latest $1"
    kubectl apply -f "$1"
    checkPodStatusWithTimeout "vitess-operator(.*)1/1(.*)Running(.*)"

    echo "Apply $2"
    kubectl apply -f "$2"
    checkPodStatusWithTimeout "example-etcd(.*)1/1(.*)Running(.*)" 3
    checkPodStatusWithTimeout "example-zone1-vtctld(.*)1/1(.*)Running(.*)"
    checkPodStatusWithTimeout "example-zone1-vtgate(.*)1/1(.*)Running(.*)"
    checkPodStatusWithTimeout "example-commerce-x-x-zone1-vtorc(.*)1/1(.*)Running(.*)"
    checkPodStatusWithTimeout "example-customer-x-x-zone1-vtorc(.*)1/1(.*)Running(.*)"
    # 3 tablets for commerce + 3 tablets for customer = 6 total
    checkPodStatusWithTimeout "example-vttablet-zone1(.*)3/3(.*)Running(.*)" 6

    setupPortForwarding
    waitForKeyspaceToBeServing commerce - 2
    waitForKeyspaceToBeServing customer - 2
    verifyDataCommerce create
}

# getVBSCName returns the full VBSC resource name matching a prefix pattern.
# The VBSC names have a hash suffix added by the operator, e.g. "example-vbsc-cluster-every-minute-ea63062d".
function getVBSCName() {
  local pattern=$1
  kubectl get VitessBackupSchedule -n example --no-headers -o custom-columns=":metadata.name" | grep "${pattern}" | head -1 | tr -d ' '
}

# verifyGeneratedSchedulesExist checks that a VBSC matching the given name prefix
# has non-empty generatedSchedules in its status.
function verifyGeneratedSchedulesExist() {
  local namePattern=$1
  echo "Checking that VBSC matching '${namePattern}' has generatedSchedules in status"
  for i in {1..600} ; do
    local vbscName
    vbscName=$(getVBSCName "${namePattern}")
    if [[ -n "${vbscName}" ]]; then
      status=$(kubectl get VitessBackupSchedule "${vbscName}" -n example -o jsonpath='{.status.generatedSchedules}' 2>/dev/null)
      if [[ -n "${status}" && "${status}" != "{}" && "${status}" != "map[]" ]]; then
        echo "generatedSchedules found for ${vbscName}: ${status}"
        return
      fi
    fi
    sleep 1
  done
  echo "ERROR: generatedSchedules not found for VBSC matching '${namePattern}' within timeout"
  exit 1
}

# verifyAutoExclusion checks that the cluster-scope schedule does NOT create jobs for the
# excluded keyspace (customer), while it DOES create jobs for the non-excluded keyspace (commerce).
# It also checks that the keyspace-scope schedule creates jobs for the customer keyspace.
function verifyAutoExclusion() {
  echo "Verifying auto-exclusion behavior"

  local clusterVBSC customerVBSC

  # Discover actual VBSC names
  for i in {1..300} ; do
    clusterVBSC=$(getVBSCName "cluster-every-minute")
    customerVBSC=$(getVBSCName "customer-ks-every-minute")
    if [[ -n "${clusterVBSC}" && -n "${customerVBSC}" ]]; then
      break
    fi
    sleep 1
  done
  if [[ -z "${clusterVBSC}" || -z "${customerVBSC}" ]]; then
    echo "ERROR: Could not find both VBSCs"
    exit 1
  fi
  echo "Found cluster VBSC: ${clusterVBSC}"
  echo "Found customer VBSC: ${customerVBSC}"

  # Wait for generatedSchedules to appear on cluster VBSC
  echo "Checking cluster-scope schedule excludes customer keyspace"
  for i in {1..300} ; do
    clusterSchedules=$(kubectl get VitessBackupSchedule "${clusterVBSC}" -n example -o jsonpath='{.status.generatedSchedules}' 2>/dev/null)
    if [[ -n "${clusterSchedules}" && "${clusterSchedules}" != "{}" ]]; then
      echo "Cluster-scope generatedSchedules: ${clusterSchedules}"
      # Check that commerce is present (the expanded strategy name contains "commerce")
      if echo "${clusterSchedules}" | grep -q "commerce"; then
        echo "OK: cluster-scope schedule includes commerce"
      else
        echo "ERROR: cluster-scope schedule does not include commerce"
        exit 1
      fi
      # Check that customer is NOT present (auto-excluded by keyspace-scope override)
      if echo "${clusterSchedules}" | grep -q "customer"; then
        echo "ERROR: cluster-scope schedule includes customer, but it should be auto-excluded"
        exit 1
      fi
      echo "OK: cluster-scope schedule correctly excludes customer (auto-exclusion working)"
      break
    fi
    sleep 1
  done

  # Verify keyspace-scope has customer entries in generatedSchedules
  echo "Checking keyspace-scope schedule includes customer keyspace"
  customerSchedules=$(kubectl get VitessBackupSchedule "${customerVBSC}" -n example -o jsonpath='{.status.generatedSchedules}' 2>/dev/null)
  echo "Keyspace-scope generatedSchedules: ${customerSchedules}"
  if echo "${customerSchedules}" | grep -q "customer"; then
    echo "OK: keyspace-scope schedule includes customer"
  else
    echo "ERROR: keyspace-scope schedule does not include customer"
    exit 1
  fi
}

# verifyBackupsCreatedWithKeyspaceScope checks that backup jobs are created by the
# keyspace-scope and cluster-scope schedules.
function verifyBackupsCreatedWithKeyspaceScope() {
  echo "Waiting for backup jobs to be created by scope-based schedules"
  # Wait for at least 2 backups (one from each scope: cluster-scope for commerce, keyspace-scope for customer)
  for i in {1..600} ; do
    backupCount=$(kubectl get vtb -n example --no-headers 2>/dev/null | wc -l | tr -d ' ')
    echo "Found ${backupCount} backups"
    if [[ "${backupCount}" -ge 2 ]]; then
      echo "Found at least 2 backups"

      echo "Checking for completed backup job pods"
      # Check for cluster-scope job pods (commerce keyspace)
      checkPodExistWithTimeout "example-vbsc-cluster-every-minute-(.*)0/1(.*)Completed(.*)"
      echo "OK: cluster-scope backup job completed for commerce"

      # Check for keyspace-scope job pods (customer keyspace)
      checkPodExistWithTimeout "example-vbsc-customer-ks-every-minute-(.*)0/1(.*)Completed(.*)"
      echo "OK: keyspace-scope backup job completed for customer"

      return
    fi
    sleep 1
  done
  echo "ERROR: Did not find at least 2 backups within timeout"
  kubectl get vtb -n example 2>/dev/null
  kubectl get pods -A 2>/dev/null | grep vbsc
  exit 1
}

# Test setup
setupKindCluster
cd test/endtoend/operator || exit 1

echo "=== Starting Keyspace-wide Backup Schedule E2E Test ==="

# Start cluster with two keyspaces and scope-based schedules
get_started_two_keyspaces "operator-latest.yaml" "102_initial_cluster_keyspace_backup_schedule.yaml"

echo "=== Verifying VitessBackupSchedule resources ==="
checkVitessBackupScheduleStatusWithTimeout "example-vbsc-cluster-every-minute(.*)"
checkVitessBackupScheduleStatusWithTimeout "example-vbsc-customer-ks-every-minute(.*)"

echo "=== Verifying frequency-based schedule generation ==="
verifyGeneratedSchedulesExist "cluster-every-minute"
verifyGeneratedSchedulesExist "customer-ks-every-minute"

echo "=== Verifying auto-exclusion ==="
verifyAutoExclusion

echo "=== Verifying backups are created ==="
verifyBackupsCreatedWithKeyspaceScope

echo "=== Keyspace-wide Backup Schedule E2E Test PASSED ==="

# Teardown
teardownKindCluster
