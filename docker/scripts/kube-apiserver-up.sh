#!/bin/bash

source /vtop/docker/scripts/env.sh

echo "starting kube-apiserver on ${KUBE_APISERVER_URL}"

echo "{\"apiVersion\": \"abac.authorization.kubernetes.io/v1beta1\", \"kind\": \"Policy\", \"spec\": {\"user\":\"system:anonymous\", \"namespace\": \"*\", \"resource\": \"*\", \"apiGroup\": \"*\"}}" >> "${KUBE_APISERVER_DATADIR}/auth-policy.json"

kube-apiserver \
    --cert-dir "${KUBE_APISERVER_DATADIR}" \
    --secure-port "${KUBE_APISERVER_PORT}" \
    --etcd-servers "${ETCD_URL}" \
    --service-account-issuer https://kubernetes.default.svc.cluster.local \
    --service-account-key-file "${KUBE_APISERVER_DATADIR}/apiserver.crt" \
	--service-account-signing-key-file "${KUBE_APISERVER_DATADIR}/apiserver.key" \
	--authorization-policy-file "${KUBE_APISERVER_DATADIR}/auth-policy.json" \
	--authorization-mode ABAC \
    > /vtop/tmp/kube-apiserver.out 2>&1 &
PID=$!
echo $PID > /vtop/tmp/kube-apiserver.pid
sleep 5

echo "kube-apiserver is up"
