# shellcheck shell=bash
# Shared helper for installing a pinned, checksum-verified kubectl into
# tools/_bin. Sourced by get-e2e-test-deps.sh and get-kube-binaries.sh so the
# version pin, platform detection and checksum verification live in one place.
#
# Defines OS, ARCH, PLATFORM and verify_checksum for the sourcing script.
# install_kubectl must be called from inside tools/_bin.

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "${ARCH}" in
    x86_64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
esac
PLATFORM="${OS}-${ARCH}"

KUBECTL_VERSION="v1.32.2"
KUBERNETES_RELEASE_URL="${KUBERNETES_RELEASE_URL:-https://dl.k8s.io}"

function verify_checksum() {
    local file="$1"
    local expected="$2"
    local actual
    if command -v sha256sum &> /dev/null; then
        actual="$(sha256sum "${file}" | cut -d' ' -f1)"
    else
        actual="$(shasum -a 256 "${file}" | cut -d' ' -f1)"
    fi
    if [[ "${actual}" != "${expected}" ]]; then
        echo "Checksum mismatch for ${file}: expected ${expected}, got ${actual}" >&2
        rm -f "${file}"
        exit 1
    fi
}

function install_kubectl() {
    local sha256
    case "${PLATFORM}" in
        linux-amd64)
            sha256="4f6a959dcc5b702135f8354cc7109b542a2933c46b808b248a214c1f69f817ea"
            ;;
        linux-arm64)
            sha256="7381bea99c83c264100f324c2ca6e7e13738a73b8928477ac805991440a065cd"
            ;;
        darwin-amd64)
            sha256="371b8fbd481e1e9052ace16d9c243e92618a2ea9a18c1aaf235d35fef20c0c32"
            ;;
        darwin-arm64)
            sha256="31b6318deaa72014b72121e1c7a2e12496d077cee49bbeda94250aec4c978ffb"
            ;;
        *)
            echo "No pinned kubectl for platform ${PLATFORM}; install kubectl manually." >&2
            exit 1
            ;;
    esac

    # Download to a temporary name and only move into place after
    # verification, so an interrupted download is never trusted on rerun.
    if [ ! -f "kubectl-${KUBECTL_VERSION}" ]; then
        echo "Downloading kubectl ${KUBECTL_VERSION}..."
        curl --silent -L "${KUBERNETES_RELEASE_URL}/${KUBECTL_VERSION}/bin/${OS}/${ARCH}/kubectl" > kubectl.tmp
        verify_checksum kubectl.tmp "${sha256}"
        chmod +x kubectl.tmp
        mv kubectl.tmp "kubectl-${KUBECTL_VERSION}"
    fi
    echo "Using kubectl ${KUBECTL_VERSION}."
    ln -sf "kubectl-${KUBECTL_VERSION}" kubectl
}
