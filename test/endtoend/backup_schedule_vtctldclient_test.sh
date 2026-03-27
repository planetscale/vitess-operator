#!/bin/bash

source ./tools/test.env
source ./test/endtoend/utils.sh

# verifyVtctldclientScheduleJobs checks that both vtbackup and vtctldclient
# backup schedules create jobs and that the vtctldclient jobs do not create PVCs.
function verifyVtctldclientScheduleJobs() {
  echo "=== Waiting for VitessBackupSchedule resources ==="
  checkVitessBackupScheduleStatusWithTimeout "example-vbsc-vtbackup-every-minute(.*)"
  checkVitessBackupScheduleStatusWithTimeout "example-vbsc-vtctldclient-every-minute(.*)"

  echo "=== Waiting for backup jobs to be created ==="
  # Wait for at least 2 backups: one from the initial vtbackup pod and
  # at least one from either schedule.
  for i in {1..600} ; do
    backupCount=$(kubectl get vtb -n example --no-headers 2>/dev/null | wc -l | tr -d ' ')
    echo "Found ${backupCount} backups"
    if [[ "${backupCount}" -ge 2 ]]; then
      echo "Found at least 2 backups"
      break
    fi
    sleep 1
  done
  if [[ "${backupCount}" -lt 2 ]]; then
    echo "ERROR: Did not find at least 2 backups within timeout"
    kubectl get vtb -n example 2>/dev/null
    kubectl get pods -A 2>/dev/null | grep vbsc
    exit 1
  fi

  echo "=== Checking vtbackup schedule pods ==="
  checkPodExistWithTimeout "example-vbsc-vtbackup-every-minute-(.*)0/1(.*)Completed(.*)"
  echo "OK: vtbackup schedule job completed"

  echo "=== Checking vtctldclient schedule pods ==="
  checkPodExistWithTimeout "example-vbsc-vtctldclient-every-minute-(.*)0/1(.*)Completed(.*)"
  echo "OK: vtctldclient schedule job completed"

  echo "=== Verifying vtctldclient job has correct backup-method label ==="
  vtctldclientJobName=$(kubectl get jobs -n example --no-headers -o custom-columns=":metadata.name" | grep "vbsc-vtctldclient-every-minute" | head -1 | tr -d ' ')
  if [[ -z "${vtctldclientJobName}" ]]; then
    echo "ERROR: Could not find vtctldclient job"
    exit 1
  fi
  backupMethodLabel=$(kubectl get job "${vtctldclientJobName}" -n example -o jsonpath='{.metadata.labels.planetscale\.com/backup-method}')
  if [[ "${backupMethodLabel}" != "vtctldclient" ]]; then
    echo "ERROR: Expected backup-method label 'vtctldclient', got '${backupMethodLabel}'"
    exit 1
  fi
  echo "OK: vtctldclient job has correct backup-method label"

  echo "=== Verifying vtctldclient job has no PVC ==="
  pvcExists=$(kubectl get pvc "${vtctldclientJobName}" -n example --no-headers 2>/dev/null | wc -l | tr -d ' ')
  if [[ "${pvcExists}" -ne 0 ]]; then
    echo "ERROR: Found PVC for vtctldclient job, but none should exist"
    exit 1
  fi
  echo "OK: vtctldclient job has no PVC"

  echo "=== Verifying vtbackup job does have a PVC ==="
  vtbackupJobName=$(kubectl get jobs -n example --no-headers -o custom-columns=":metadata.name" | grep "vbsc-vtbackup-every-minute" | head -1 | tr -d ' ')
  if [[ -z "${vtbackupJobName}" ]]; then
    echo "ERROR: Could not find vtbackup job"
    exit 1
  fi
  vtbackupPvcExists=$(kubectl get pvc "${vtbackupJobName}" -n example --no-headers 2>/dev/null | wc -l | tr -d ' ')
  if [[ "${vtbackupPvcExists}" -eq 0 ]]; then
    echo "ERROR: Expected PVC for vtbackup job, but none found"
    exit 1
  fi
  echo "OK: vtbackup job has a PVC"

  echo "=== Verifying vtctldclient pod ran vtctldclient binary ==="
  vtctldclientPod=$(kubectl get pods -n example --no-headers -o custom-columns=":metadata.name" | grep "vbsc-vtctldclient-every-minute" | head -1 | tr -d ' ')
  if [[ -z "${vtctldclientPod}" ]]; then
    echo "ERROR: Could not find vtctldclient pod"
    exit 1
  fi
  containerImage=$(kubectl get pod "${vtctldclientPod}" -n example -o jsonpath='{.spec.containers[0].image}')
  containerCommand=$(kubectl get pod "${vtctldclientPod}" -n example -o jsonpath='{.spec.containers[0].command[0]}')
  echo "vtctldclient pod image: ${containerImage}"
  echo "vtctldclient pod command: ${containerCommand}"
  if [[ "${containerCommand}" != "/vt/bin/vtctldclient" ]]; then
    echo "ERROR: Expected command '/vt/bin/vtctldclient', got '${containerCommand}'"
    exit 1
  fi
  echo "OK: vtctldclient pod ran vtctldclient binary"
}

# Test setup
setupKindCluster
cd test/endtoend/operator || exit 1

echo "=== Starting vtctldclient Backup Method E2E Test ==="

get_started "operator-latest.yaml" "103_initial_cluster_vtctldclient_backup_schedule.yaml"
verifyVtGateVersion "24.0.0"
verifyVtctldclientScheduleJobs

echo "=== vtctldclient Backup Method E2E Test PASSED ==="

# Teardown
teardownKindCluster
