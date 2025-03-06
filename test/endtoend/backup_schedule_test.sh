#!/bin/bash

source ./tools/test.env
source ./test/endtoend/utils.sh

function takedownShard() {
  echo "Apply 102_keyspace_teardown.yaml"
  kubectl apply -f 102_keyspace_teardown.yaml

  # wait for all the vttablets to disappear
  checkPodStatusWithTimeout "example-vttablet-zone1" 0
}

function verifyListBackupsOutputWithSchedule() {
  echo -e "Check for VitessBackupSchedule status"
  checkVitessBackupScheduleStatusWithTimeout "example-vbsc-every-minute(.*)"
  checkVitessBackupScheduleStatusWithTimeout "example-vbsc-every-five-minute(.*)"

  echo -e "Check for number of backups in the cluster"
  # Sleep for 10 minutes, after 10 minutes we should at least have 3 backups: 1 from the initial vtbackup pod
  # 1 minimum from the every minute schedule, and 1 from the every-five minute schedule
  for i in {1..600} ; do
    # Ensure that we can view the backup files from the host.
    docker exec -it $(docker container ls --format '{{.Names}}' | grep kind) chmod o+rwx -R /backup > /dev/null
    backupCount=$(kubectl get vtb --no-headers | wc -l)
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

get_started "operator-latest.yaml" "101_initial_cluster_backup_schedule.yaml"
verifyVtGateVersion "22.0.0"
checkSemiSyncSetup
verifyListBackupsOutputWithSchedule

echo "Removing the temporary directory"
removeBackupFiles
rm -rf "$STARTING_DIR/vtdataroot"
echo "Deleting Kind cluster. This also deletes the volume associated with it"
kind delete cluster --name kind-${BUILDKITE_BUILD_ID}
