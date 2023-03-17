#!/bin/bash

source /vtop/docker/scripts/env.sh

echo "starting kube-apiserver on ${KUBE_APISERVER_URL}"

echo "31ada4fd-adec-460c-809a-9e56ceb75269,testrunner,1" >> "${KUBE_APISERVER_DATADIR}/token.csv"
echo "{\"apiVersion\": \"abac.authorization.kubernetes.io/v1beta1\", \"kind\": \"Policy\", \"spec\": {\"user\":\"testrunner\", \"namespace\": \"*\", \"resource\": \"*\", \"apiGroup\": \"*\"}}" >> "${KUBE_APISERVER_DATADIR}/auth-policy.json"
echo "{\"apiVersion\": \"abac.authorization.kubernetes.io/v1beta1\", \"kind\": \"Policy\", \"spec\": {\"group\": \"system:authenticated\", \"readonly\": true, \"nonResourcePath\": \"*\"}}" >> "${KUBE_APISERVER_DATADIR}/auth-policy.json"

kube-apiserver \
    --cert-dir "${KUBE_APISERVER_DATADIR}" \
    --secure-port "${KUBE_APISERVER_PORT}" \
    --etcd-servers "${ETCD_URL}" \
    --service-account-issuer api \
    --service-account-key-file "${KUBE_APISERVER_DATADIR}/apiserver.key" \
	--service-account-signing-key-file "${KUBE_APISERVER_DATADIR}/apiserver.key" \
	--authorization-policy-file "${KUBE_APISERVER_DATADIR}/auth-policy.json" \
	--authorization-mode ABAC \
    --token-auth-file "${KUBE_APISERVER_DATADIR}/token.csv" \
    > /vtop/tmp/kube-apiserver.out 2>&1 &
PID=$!
echo $PID > /vtop/tmp/kube-apiserver.pid
sleep 5

echo "kube-apiserver is up"
