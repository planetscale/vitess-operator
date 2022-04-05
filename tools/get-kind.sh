#!/bin/bash

set -euo pipefail

# This script downloads kind binary that are
# used as part of the upgrade test environment,
# and places them in tools/_bin/.
#
# The upgrade test framework expects kind binary to be found in the PATH.

DIR="${BASH_SOURCE%/*}"
mkdir -p "${DIR}/_bin"
cd "${DIR}/_bin"

# Download kubectl if needed.
command -v kind && echo "kind already installed" && exit
echo "Downloading kind..."
curl -L https://kind.sigs.k8s.io/dl/v0.12.0/kind-linux-amd64 > "kind"
chmod +x "kind"
echo "Installed kind"

