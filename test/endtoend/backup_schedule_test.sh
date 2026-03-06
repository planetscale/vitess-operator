#!/bin/bash

source ./tools/test.env
source ./test/endtoend/utils.sh

function verifyListBackupsOutputWithSchedule() {
  echo -e "Check for VitessBackupSchedule status"
  checkVitessBackupScheduleStatusWithTimeout "example-vbsc-every-minute(.*)"
  checkVitessBackupScheduleStatusWithTimeout "example-vbsc-every-five-minute(.*)"

  echo -e "Check for number of backups in the cluster"
  # Sleep for 10 minutes, after 10 minutes we should at least have 3 backups: 1 from the initial vtbackup pod
  # 1 minimum from the every minute schedule, and 1 from the every-five minute schedule
  for i in {1..600} ; do
    backupCount=$(kubectl get vtb -n example --no-headers | wc -l)
    echo "Found ${backupCount} backups"
    if [[ "${backupCount}" -ge 3 ]]; then
      echo -e "Check for Jobs' pods"
      # Here check explicitly that the every five minute schedule ran at least once during the 10 minutes sleep
      checkPodExistWithTimeout "example-vbsc-every-minute-(.*)0/1(.*)Completed(.*)"
      checkPodExistWithTimeout "example-vbsc-every-five-minute-(.*)0/1(.*)Completed(.*)"
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

get_started "operator-latest.yaml" "101_initial_cluster_backup_schedule.yaml"
verifyVtGateVersion "24.0.0"
checkSemiSyncSetup
checkMysqldExporterMetrics
verifyListBackupsOutputWithSchedule

# Teardown
teardownKindCluster
