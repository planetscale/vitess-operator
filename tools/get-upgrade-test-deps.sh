#!/bin/bash
source ./tools/test.env

set -euo pipefail

# This script downloads kind binary that are
# used as part of the upgrade test environment,
# and places them in tools/_bin/.
#
# The upgrade test framework expects kind binary to be found in the PATH.

DIR="${BASH_SOURCE%/*}"
mkdir -p "${DIR}/_bin"
cd "${DIR}/_bin"

KUBE_VERSION="${KUBE_VERSION:-v1.21.1}"
KUBERNETES_RELEASE_URL="${KUBERNETES_RELEASE_URL:-https://dl.k8s.io}"

# Download kubectl if needed.
if [ ! -f "kubectl-${KUBE_VERSION}" ]; then
    echo "Downloading kubectl ${KUBE_VERSION}..."
    curl -L "${KUBERNETES_RELEASE_URL}/${KUBE_VERSION}/bin/linux/amd64/kubectl" > "kubectl-${KUBE_VERSION}"
    chmod +x "kubectl-${KUBE_VERSION}"
fi
echo "Using kubectl ${KUBE_VERSION}."
ln -sf "kubectl-${KUBE_VERSION}" kubectl

# Download kind if needed.
if ! command -v kind &> /dev/null
then
    echo "Downloading kind..."
    curl -L https://kind.sigs.k8s.io/dl/v0.12.0/kind-linux-amd64 > "kind"
    chmod +x "kind"
    echo "Installed kind"
else
    echo "Kind already installed"
fi

# Download vtctlclient if needed
if ! command -v vtctlclient &> /dev/null
then
  echo "Downloading vtctlclient..."
  version=13.0.0
  file=vitess-${version}-bc4a960.tar.gz
  wget https://github.com/vitessio/vitess/releases/download/v${version}/${file}
  tar -xzf ${file}
  cd ${file/.tar.gz/}
  cp -r ./bin/* ../
  cd ..
  rm ${file}
  rm -r ${file/.tar.gz/}
  echo "vtctlclient installed"
else
  echo "vtctlclient already installed"
fi
