#!/bin/bash

source /vtop/tools/env.sh


function fail() {
  echo "ERROR: $1"
  exit 1
}

curl "${ETCD_URL}" > /dev/null 2>&1 && fail "etcd is already running. Exiting."

echo "starting etcd on ${ETCD_URL}"

etcd \
    --data-dir /vtop/etcd/ \
    --listen-client-urls "${ETCD_URL}" \
    --advertise-client-urls "${ETCD_URL}" \
    --listen-peer-urls http://127.0.0.1:0 \
    > /vtop/tmp/etcd.out 2>&1 &
PID=$!
echo $PID > /vtop/tmp/etcd.pid
sleep 5

echo "etcd is up"
