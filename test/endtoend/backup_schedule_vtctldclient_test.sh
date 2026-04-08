#!/bin/bash

source ./tools/test.env
source ./test/endtoend/utils.sh

function latestCompleteScheduledVtctldclientBackupName() {
  kubectl get vtb -n example --sort-by='.status.startTime' --no-headers \
    -o custom-columns="STORAGE:.status.storageName,COMPLETE:.status.complete" \
    | awk '$2 == "true" && $1 !~ /vtbackup-/ { name = $1 } END { print name }'
}

function suspendVtbackupSchedule() {
  kubectl patch vitessclusters.planetscale.com example -n example --type='json' \
    -p='[{"op":"add","path":"/spec/backup/schedules/0/suspend","value":true}]'

  for i in {1..120}; do
    suspended=$(kubectl get vitessbackupschedules.planetscale.com example-vbsc-vtbackup-every-minute -n example -o jsonpath='{.spec.suspend}')
    if [[ "${suspended}" == "true" ]]; then
      echo "vtbackup schedule suspended"
      return 0
    fi
    sleep 1
  done

  echo "ERROR: vtbackup schedule was not suspended"
  exit 1
}

function waitForAdditionalScheduledVtctldclientBackup() {
  baselineBackupName="$1"

  for i in {1..600}; do
    backupName=$(latestCompleteScheduledVtctldclientBackupName)
    if [[ -n "${backupName}" && "${backupName}" != "${baselineBackupName}" ]]; then
      echo "${backupName}"
      return 0
    fi
    sleep 1
  done

  echo "ERROR: did not observe a new vtctldclient scheduled backup"
  exit 1
}

function verifyVtctldclientScheduleJobs() {
  echo -e "Check for VitessBackupSchedule status"
  checkVitessBackupScheduleStatusWithTimeout "example-vbsc-vtbackup-every-minute(.*)"
  checkVitessBackupScheduleStatusWithTimeout "example-vbsc-vtctldclient-every-minute(.*)"

  echo -e "Check for number of backups in the cluster"
  # Wait for at least 3 backups: 1 from the initial vtbackup pod,
  # 1 from the vtbackup schedule, and 1 from the vtctldclient schedule
  for i in {1..600} ; do
    backupCount=$(kubectl get vtb -n example --no-headers | wc -l)
    echo "Found ${backupCount} backups"
    if [[ "${backupCount}" -ge 3 ]]; then
      echo -e "Verify Vitess can list the scheduled backups"
      verifyListBackupsOutput

      echo -e "Check for Jobs' pods"
      checkPodExistWithTimeout "example-vbsc-vtbackup-every-minute-(.*)0/1(.*)Completed(.*)"
      checkPodExistWithTimeout "example-vbsc-vtctldclient-every-minute-(.*)0/1(.*)Completed(.*)"

      echo -e "Verify vtctldclient job has correct backup-method label"
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

      echo -e "Verify vtctldclient job has no PVC"
      pvcExists=$(kubectl get pvc "${vtctldclientJobName}" -n example --no-headers 2>/dev/null | wc -l)
      if [[ "${pvcExists}" -ne 0 ]]; then
        echo "ERROR: Found PVC for vtctldclient job, but none should exist"
        exit 1
      fi
      echo "OK: vtctldclient job has no PVC"

      echo -e "Verify vtbackup job does have a PVC"
      vtbackupJobName=$(kubectl get jobs -n example --no-headers -o custom-columns=":metadata.name" | grep "vbsc-vtbackup-every-minute" | head -1 | tr -d ' ')
      if [[ -n "${vtbackupJobName}" ]]; then
        vtbackupPvcExists=$(kubectl get pvc "${vtbackupJobName}" -n example --no-headers 2>/dev/null | wc -l)
        if [[ "${vtbackupPvcExists}" -eq 0 ]]; then
          echo "ERROR: Expected PVC for vtbackup job, but none found"
          exit 1
        fi
        echo "OK: vtbackup job has a PVC"
      fi

      echo -e "Verify vtctldclient pod ran vtctldclient binary"
      vtctldclientPod=$(kubectl get pods -n example --no-headers -o custom-columns=":metadata.name" | grep "vbsc-vtctldclient-every-minute" | head -1 | tr -d ' ')
      containerCommand=$(kubectl get pod "${vtctldclientPod}" -n example -o jsonpath='{.spec.containers[0].command[0]}')
      if [[ "${containerCommand}" != "/vt/bin/vtctldclient" ]]; then
        echo "ERROR: Expected command '/vt/bin/vtctldclient', got '${containerCommand}'"
        exit 1
      fi
      echo "OK: vtctldclient pod ran vtctldclient binary"

      echo -e "Suspend vtbackup schedule and wait for another vtctldclient scheduled backup"
      baselineBackupName=$(latestCompleteScheduledVtctldclientBackupName)
      if [[ -z "${baselineBackupName}" ]]; then
        echo "ERROR: Could not find an existing vtctldclient scheduled backup"
        exit 1
      fi
      suspendVtbackupSchedule
      scheduledBackupName=$(waitForAdditionalScheduledVtctldclientBackup "${baselineBackupName}")
      backupTimestamp=$(echo "${scheduledBackupName}" | awk -F'.' '{print $1"."$2}')
      echo "Restoring from vtctldclient scheduled backup ${scheduledBackupName}"

      tabletAlias=$(vtctldclient GetTablets --keyspace commerce --tablet-type replica --shard '-' | head -1 | awk '{print $1}')
      if [[ -z "${tabletAlias}" ]]; then
        echo "ERROR: Could not find a replica tablet to restore"
        exit 1
      fi
      restoreBackup "${tabletAlias}" "${backupTimestamp}"
      return
    fi
    sleep 1
  done
  echo "Did not find at least 3 backups"
  exit 1
}

# Test setup
setupKindCluster
cd test/endtoend/operator || exit 1

get_started "operator-latest.yaml" "103_initial_cluster_vtctldclient_backup_schedule.yaml"
verifyVtGateVersion "24.0.0"
verifyVtctldclientScheduleJobs

# Teardown
teardownKindCluster
