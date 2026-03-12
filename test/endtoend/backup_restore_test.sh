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
  checkPodStatusWithTimeout "example-etcd(.*)1/1(.*)Running(.*)" 3
  checkPodStatusWithTimeout "example-zone1-vtctld(.*)1/1(.*)Running(.*)"
  checkPodStatusWithTimeout "example-zone1-vtgate(.*)1/1(.*)Running(.*)"
  checkPodStatusWithTimeout "example-vttablet-zone1(.*)3/3(.*)Running(.*)" 3

  setupPortForwarding
  waitForKeyspaceToBeServing commerce - 2
  verifyDataCommerce
}

function deleteSeedBackupFromStorage() {
  cleanup_pod="backup-storage-cleaner"

  cat <<EOF | kubectl apply -n example -f -
apiVersion: v1
kind: Pod
metadata:
  name: ${cleanup_pod}
spec:
  restartPolicy: Never
  containers:
  - name: cleanup
    image: busybox:1.36
    command: ["sh", "-c", "sleep 600"]
    volumeMounts:
    - name: backups
      mountPath: /vt/backups
  volumes:
  - name: backups
    persistentVolumeClaim:
      claimName: vitess-backups
EOF

  kubectl wait --for=condition=Ready -n example pod/${cleanup_pod} --timeout=120s

  for backup_name in $(vtctldclient GetBackups "$keyspaceShard" | grep "vtbackup-"); do
    echo "Deleting seed backup ${backup_name} from shared storage"
    kubectl exec -n example "$cleanup_pod" -- rm -rf "/vt/backups/example/${keyspaceShard}/${backup_name}"
  done

  kubectl delete pod -n example "$cleanup_pod" --ignore-not-found=true
}

# Test setup
setupKindCluster
cd test/endtoend/operator || exit 1

get_started "operator-latest.yaml" "101_initial_cluster_backup.yaml"
verifyVtGateVersion "24.0.0"
checkSemiSyncSetup
checkMysqldExporterMetrics
takeBackup "commerce/-"
verifyListBackupsOutput
deleteSeedBackupFromStorage
restoreBackup "$(vtctldclient GetTablets --keyspace commerce --tablet-type replica --shard '-' | head -1 | awk '{print $1}')"
takedownShard
resurrectShard
checkSemiSyncSetup

# Teardown
teardownKindCluster
