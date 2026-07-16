#!/bin/bash
source ./tools/test.env

set -euo pipefail

# This script downloads the kubectl, kind and vtctldclient binaries that are
# used as part of the end-to-end test environment, and places them in
# tools/_bin/. Downloads are verified against pinned SHA-256 checksums.
#
# The test framework expects these binaries to be found in the PATH.

# Provides OS, ARCH, PLATFORM, verify_checksum and install_kubectl.
source "${BASH_SOURCE%/*}/install-kubectl.sh"

DIR="${BASH_SOURCE%/*}"
mkdir -p "${DIR}/_bin"
cd "${DIR}/_bin"

KIND_BINARY_VERSION="v0.30.0"
VITESS_VERSION="24.0.0-rc1"
VITESS_FILE="vitess-${VITESS_VERSION}-4d416bb.tar.gz"

# Vitess only publishes a linux-amd64 release tarball.
VITESS_SHA256="b03ae826746190b4d36f678be00121144354f23ad8528f99c2ad91d5f4560ee7"

function install_kind() {
    local sha256
    case "${PLATFORM}" in
        linux-amd64)
            sha256="517ab7fc89ddeed5fa65abf71530d90648d9638ef0c4cde22c2c11f8097b8889"
            ;;
        linux-arm64)
            sha256="7ea2de9d2d190022ed4a8a4e3ac0636c8a455e460b9a13ccf19f15d07f4f00eb"
            ;;
        darwin-amd64)
            sha256="4f0b6e3b88bdc66d922c08469f05ef507d4903dd236e6319199bb9c868eed274"
            ;;
        darwin-arm64)
            sha256="ceaf40df1d1551c481fb50e3deb5c3deecad5fd599df5469626b70ddf52a1518"
            ;;
        *)
            echo "No pinned kind for platform ${PLATFORM}. Install kind manually." >&2
            exit 1
            ;;
    esac

    if [[ ! -f "kind-${KIND_BINARY_VERSION}" ]]; then
        echo "Downloading kind ${KIND_BINARY_VERSION}..."
        curl --silent -L "https://kind.sigs.k8s.io/dl/${KIND_BINARY_VERSION}/kind-${PLATFORM}" > kind.tmp
        verify_checksum kind.tmp "${sha256}"
        chmod +x kind.tmp
        mv kind.tmp "kind-${KIND_BINARY_VERSION}"
    fi
    echo "Using kind ${KIND_BINARY_VERSION}."
    ln -sf "kind-${KIND_BINARY_VERSION}" kind
}

function install_vtctldclient() {
    if [[ -f "vtctldclient-${VITESS_VERSION}" ]]; then
        echo "Pinned vtctldclient already installed"
        ln -sf "vtctldclient-${VITESS_VERSION}" vtctldclient
    elif [[ "${PLATFORM}" != "linux-amd64" ]]; then
        # Vitess does not publish release binaries for this platform.
        # Fall back to a host-installed vtctldclient further down PATH.
        if command -v vtctldclient &> /dev/null; then
            echo "No Vitess release binaries for ${PLATFORM}. Using host vtctldclient."
        else
            echo "No Vitess release binaries for ${PLATFORM}. Install vtctldclient manually." >&2
            exit 1
        fi
    else
        echo "Downloading vtctldclient ${VITESS_VERSION}..."
        wget -q "https://github.com/vitessio/vitess/releases/download/v${VITESS_VERSION}/${VITESS_FILE}"
        verify_checksum "${VITESS_FILE}" "${VITESS_SHA256}"
        # Extract only the required binary so tools/_bin does not shadow
        # other host binaries through PATH.
        tar -xzf "${VITESS_FILE}" --strip-components=2 "${VITESS_FILE%.tar.gz}/bin/vtctldclient"
        mv vtctldclient "vtctldclient-${VITESS_VERSION}"
        ln -sf "vtctldclient-${VITESS_VERSION}" vtctldclient
        rm "${VITESS_FILE}"
        echo "vtctldclient installed"
    fi
}

install_kubectl
install_kind
install_vtctldclient
