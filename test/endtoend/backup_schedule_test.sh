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
    if [[ $(kubectl get VitessBackupSchedule | grep -E "${regex}" | wc -l) -eq 1 ]]; then
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
  # Sleep for 6 minutes, during this time we should have at the very minimum 7 backups.
  # At least: 6 backups from the every-minute schedule, and 1 backup from the every-five-minute schedule.
  sleep 360

  backupCount=$(kubectl get vtb --no-headers | wc -l)
  if [[ "${backupCount}" -lt 7 ]]; then
    echo "Did not find at least 7 backups"
    return 0
  fi

  echo -e "Check for Jobs' pods"
  checkPodStatusWithTimeout "example-vbsc-every-minute-(.*)0/1(.*)Completed(.*)" 3
  checkPodStatusWithTimeout "example-vbsc-every-five-minute-(.*)0/1(.*)Completed(.*)" 2
}

function setupKindConfig() {
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

# Test setup
STARTING_DIR="$PWD"
echo "Make temporary directory for the test"
mkdir -p -m 777 ./vtdataroot/backup
echo "Building the docker image"
docker build -f build/Dockerfile.release -t vitess-operator-pr:latest .
echo "Setting up the kind config"
setupKindConfig
echo "Creating Kind cluster"
kind create cluster --wait 30s --name kind-${BUILDKITE_BUILD_ID} --config ./vtdataroot/config.yaml --image ${KIND_VERSION}
echo "Loading docker image into Kind cluster"
kind load docker-image vitess-operator-pr:latest --name kind-${BUILDKITE_BUILD_ID}

cd "$PWD/test/endtoend/operator"
killall kubectl
setupKubectlAccessForCI

get_started "operator-latest.yaml" "101_initial_cluster_backup_schedule.yaml"
verifyVtGateVersion "20.0.1"
checkSemiSyncSetup
verifyListBackupsOutputWithSchedule

echo "Removing the temporary directory"
removeBackupFiles
rm -rf "$STARTING_DIR/vtdataroot"
echo "Deleting Kind cluster. This also deletes the volume associated with it"
kind delete cluster --name kind-${BUILDKITE_BUILD_ID}
