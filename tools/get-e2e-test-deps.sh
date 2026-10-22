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

KIND_BINARY_VERSION="v0.27.0"
VITESS_VERSION="23.0.0"
VITESS_FILE="vitess-${VITESS_VERSION}-bee41a6.tar.gz"

# Vitess only publishes a linux-amd64 release tarball.
VITESS_SHA256="1c2b85aa0d44205670855f2eb1c867aa7b30581e17303887937f7389567dcd0f"

function install_kind() {
    local sha256
    case "${PLATFORM}" in
        linux-amd64)
            sha256="a6875aaea358acf0ac07786b1a6755d08fd640f4c79b7a2e46681cc13f49a04b"
            ;;
        linux-arm64)
            sha256="5e4507a41c69679562610b1be82ba4f80693a7826f4e9c6e39236169a3e4f9d0"
            ;;
        darwin-amd64)
            sha256="3435134325b6b9406ccfec417b13bb46a808fc74e9a2ebb0ca31b379c8293863"
            ;;
        darwin-arm64)
            sha256="5240ca1acb587e1d0386532dd8c3373d81f5173b5af322919fc56f0cdd646596"
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
        curl --fail --location --silent --show-error \
          --output "${VITESS_FILE}.tmp" \
          "https://github.com/vitessio/vitess/releases/download/v${VITESS_VERSION}/${VITESS_FILE}"
        verify_checksum "${VITESS_FILE}.tmp" "${VITESS_SHA256}"
        mv "${VITESS_FILE}.tmp" "${VITESS_FILE}"
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
