#!/bin/bash

export ETCD_PORT=2379
export ETCD_URL="http://127.0.0.1:${ETCD_PORT}"

export KUBE_APISERVER_PORT=5000
export KUBE_APISERVER_URL="https://127.0.0.1:${KUBE_APISERVER_PORT}"
export KUBE_APISERVER_DATADIR=/vtop/kube_apiserver
