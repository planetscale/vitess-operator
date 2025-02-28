#!/bin/bash

source ./tools/test.env
source ./test/endtoend/utils.sh

function takedownShard() {
  echo "Apply 102_keyspace_teardown.yaml"
  kubectl apply -f 102_keyspace_teardown.yaml

  # wait for all the vttablets to disappear
  checkPodStatusWithTimeout "example-vttablet-zone1" 0
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

function verifyListBackupsOutputWithSchedule() {
  echo -e "Check for VitessBackupSchedule status"
  checkVitessBackupScheduleStatusWithTimeout "example-vbsc-every-minute(.*)"
  checkVitessBackupScheduleStatusWithTimeout "example-vbsc-every-five-minute(.*)"

  echo -e "Check for number of backups in the cluster"
  # Sleep for over 6 minutes, during this time we should have at the very minimum
  # 7 backups. At least: 6 backups from the every-minute schedule, and 1 backup
  # from the every-five-minute schedule.
  for i in {1..6} ; do
    # Ensure that we can view the backup files from the host.
    docker exec -it $(docker container ls --format '{{.Names}}' | grep kind) chmod o+rwx -R /backup > /dev/null
    backupCount=$(kubectl get vtb -n example --no-headers | wc -l)
    echo "Found ${backupCount} backups"
    if [[ "${backupCount}" -ge 7 ]]; then 
      break
    fi
    sleep 100
  done
  if [[ "${backupCount}" -lt 7 ]]; then
    echo "Did not find at least 7 backups"
    exit 1
  fi

  echo -e "Check for Jobs' pods"
  checkPodStatusWithTimeout "example-vbsc-every-minute-(.*)0/1(.*)Completed(.*)" 3
  checkPodStatusWithTimeout "example-vbsc-every-five-minute-(.*)0/1(.*)Completed(.*)" 2
}

# Test setup
STARTING_DIR="$PWD"
echo "Make temporary directory for the test"
mkdir -p -m 777 ./vtdataroot/backup
echo "Building the docker image"
docker build -f build/Dockerfile.release -t vitess-operator-pr:latest .
echo "Setting up the kind config"
setupKindConfig
createKindCluster

cd "$PWD/test/endtoend/operator"
killall kubectl
setupKubectlAccessForCI

createExampleNamespace
get_started "operator-latest.yaml" "101_initial_cluster_backup_schedule.yaml"
verifyVtGateVersion "22.0.0"
checkSemiSyncSetup
verifyListBackupsOutputWithSchedule

echo "Removing the temporary directory"
removeBackupFiles
rm -rf "$STARTING_DIR/vtdataroot"
echo "Deleting Kind cluster. This also deletes the volume associated with it"
kind delete cluster --name kind-${BUILDKITE_BUILD_ID}
