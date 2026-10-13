#!/bin/bash
source ./tools/test.env

set -euo pipefail

# This script downloads the kubectl, kind and vtctldclient binaries that are
# used as part of the end-to-end test environment, and places them in
# tools/_bin/. kubectl and kind downloads are verified against pinned SHA-256
# checksums. vtctldclient is copied out of the same digest-pinned vitess/lite
# image that the end-to-end manifests run, so the client matches the server.
#
# The test framework expects these binaries to be found in the PATH.

# Provides OS, ARCH, PLATFORM, verify_checksum and install_kubectl.
source "${BASH_SOURCE%/*}/install-kubectl.sh"

DIR="${BASH_SOURCE%/*}"
mkdir -p "${DIR}/_bin"
cd "${DIR}/_bin"

KIND_BINARY_VERSION="v0.30.0"

# The Vitess build the end-to-end tests run. Keep this in sync with the
# vitess/lite images in test/endtoend and with vitess.io/vitess in go.mod.
VITESS_LITE_IMAGE="vitess/lite:mysql80@sha256:3b9fe8685170c1e03c0ce8d65cd357e34a09545e976ea9562897b9dcbd065d8c"
VTCTLDCLIENT_BINARY="vtctldclient-${VITESS_LITE_IMAGE##*@sha256:}"

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
    if [[ -f "${VTCTLDCLIENT_BINARY}" ]]; then
        echo "Pinned vtctldclient already installed"
        ln -sf "${VTCTLDCLIENT_BINARY}" vtctldclient
    elif [[ "${PLATFORM}" != "linux-amd64" ]]; then
        # The vitess/lite image is only published for linux/amd64.
        # Fall back to a host-installed vtctldclient further down PATH.
        if command -v vtctldclient &> /dev/null; then
            echo "No vitess/lite vtctldclient for ${PLATFORM}. Using host vtctldclient."
        else
            echo "No vitess/lite vtctldclient for ${PLATFORM}. Install vtctldclient manually." >&2
            exit 1
        fi
    else
        echo "Copying vtctldclient from ${VITESS_LITE_IMAGE}..."
        local container
        container="$(docker create "${VITESS_LITE_IMAGE}")"
        docker cp "${container}:/vt/bin/vtctldclient" vtctldclient.tmp
        docker rm "${container}" > /dev/null
        chmod +x vtctldclient.tmp
        mv vtctldclient.tmp "${VTCTLDCLIENT_BINARY}"
        ln -sf "${VTCTLDCLIENT_BINARY}" vtctldclient
        echo "vtctldclient installed"
    fi
}

install_kubectl
install_kind
install_vtctldclient
