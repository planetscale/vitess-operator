#!/bin/bash

set -euo pipefail

# This script downloads etcd and Kubernetes binaries that are
# used as part of the integration test environment,
# and places them in tools/_bin/.
#
# The integration test framework expects these binaries to be found in the PATH.

# This is the kube-apiserver version to test against.
KUBE_VERSION="${KUBE_VERSION:-v1.24.11}"
KUBERNETES_RELEASE_URL="${KUBERNETES_RELEASE_URL:-https://dl.k8s.io}"

# This should be the etcd version downloaded by kubernetes/hack/lib/etcd.sh
# as of the above Kubernetes version.
ETCD_VERSION="${ETCD_VERSION:-v3.3.15}"
ETCD_RELEASE_URL="${ETCD_RELEASE_URL:-https://github.com/coreos/etcd/releases/download}"

DIR="${BASH_SOURCE%/*}"
mkdir -p "${DIR}/_bin"
cd "${DIR}/_bin"

# Download kube-apiserver if needed.
if [ ! -f "kube-apiserver-${KUBE_VERSION}" ]; then
    echo "Downloading kube-apiserver ${KUBE_VERSION}..."
    curl -L "${KUBERNETES_RELEASE_URL}/${KUBE_VERSION}/bin/linux/amd64/kube-apiserver" > "kube-apiserver-${KUBE_VERSION}"
    chmod +x "kube-apiserver-${KUBE_VERSION}"
fi
echo "Using kube-apiserver ${KUBE_VERSION}."
ln -sf "kube-apiserver-${KUBE_VERSION}" kube-apiserver

# Download kubectl if needed.
if [ ! -f "kubectl-${KUBE_VERSION}" ]; then
    echo "Downloading kubectl ${KUBE_VERSION}..."
    curl -L "${KUBERNETES_RELEASE_URL}/${KUBE_VERSION}/bin/linux/amd64/kubectl" > "kubectl-${KUBE_VERSION}"
    chmod +x "kubectl-${KUBE_VERSION}"
fi
echo "Using kubectl ${KUBE_VERSION}."
ln -sf "kubectl-${KUBE_VERSION}" kubectl

# Download etcd if needed.
if [ ! -f "etcd-${ETCD_VERSION}" ]; then
    echo "Downloading etcd ${ETCD_VERSION}..."
    basename="etcd-${ETCD_VERSION}-linux-amd64"
    tarfile="${basename}.tar.gz"
    url="${ETCD_RELEASE_URL}/${ETCD_VERSION}/${tarfile}"
    curl -L "${url}" | tar -zx
    mv "${basename}/etcd" "etcd-${ETCD_VERSION}"
    rm -rf "${basename}"
fi
echo "Using etcd ${ETCD_VERSION}."
ln -sf "etcd-${ETCD_VERSION}" etcd
