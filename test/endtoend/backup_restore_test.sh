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

# checkLastCompleteBackupMetric:
# $1: keyspace
# $2: Vitess shard name (Spec.Name, e.g. "-")
# $3: unix timestamp; the series value must be greater than this
function checkLastCompleteBackupMetric() {
  local keyspace="$1"
  local shard="$2"
  local backup_start="$3"
  local pod metrics value

  for i in {1..200} ; do
    pod="$(kubectl get pods -n example --no-headers -l "planetscale.com/component=vbs-subcontroller" -o custom-columns=":metadata.name" | awk 'NF' | head -1)"
    if [[ -z "${pod}" ]]; then
      echo "vbs-subcontroller pod not found, retrying (attempt #${i}) ..."
      sleep 3
      continue
    fi

    if ! metrics="$(kubectl get --raw "/api/v1/namespaces/example/pods/${pod}:8383/proxy/metrics")"; then
      echo "failed to fetch metrics from ${pod}:8383, retrying (attempt #${i}) ..."
      sleep 3
      continue
    fi

    value="$(printf '%s\n' "${metrics}" | awk -v ks="${keyspace}" -v sh="${shard}" '
      $0 !~ /^vitess_operator_backup_last_complete_timestamp_seconds\{/ { next }
      index($0, "keyspace=\"" ks "\"") && index($0, "shard=\"" sh "\"") { print $NF; exit }
    ')"
    if [[ -n "${value}" ]] && awk -v v="${value}" -v start="${backup_start}" 'BEGIN { exit !(v+0 > start+0) }'; then
      echo "Last complete backup metric for ${keyspace}/${shard} is ${value} (after ${backup_start})"
      return 0
    fi

    echo "waiting for vitess_operator_backup_last_complete_timestamp_seconds{keyspace=${keyspace},shard=${shard}} > ${backup_start} (got: ${value:-none}), retrying (attempt #${i}) ..."
    sleep 3
  done

  echo "ERROR: timed out waiting for last-complete backup metric for ${keyspace}/${shard} > ${backup_start}"
  kubectl get pods -n example -l "planetscale.com/component=vbs-subcontroller" --output=yaml
  if [[ -n "${pod}" ]]; then
    kubectl get --raw "/api/v1/namespaces/example/pods/${pod}:8383/proxy/metrics" || true
  fi
  exit 1
}

# Test setup
setupKindCluster
cd test/endtoend/operator || exit 1

vtbackup_init_capture="$(mktemp)"
captureVtbackupInitPod "${vtbackup_init_capture}" &
vtbackup_init_capture_pid=$!
get_started "operator-latest.yaml" "101_initial_cluster_backup.yaml"
verifyVtGateVersion "25.0.0"
checkSemiSyncSetup
checkMysqldExporterMetrics
checkVtbackupMysqldConfigOverrides "${vtbackup_init_capture_pid}" "${vtbackup_init_capture}"
checkNoFlagDeprecationWarnings
backup_start="$(date -u +%s)"
takeBackup "commerce/-"
checkLastCompleteBackupMetric "commerce" "-" "${backup_start}"
verifyListBackupsOutput
deleteSeedBackupFromStorage
restoreBackup "$(vtctldclient GetTablets --keyspace commerce --tablet-type replica --shard '-' | head -1 | awk '{print $1}')"
takedownShard
resurrectShard
checkSemiSyncSetup

# Teardown
teardownKindCluster
