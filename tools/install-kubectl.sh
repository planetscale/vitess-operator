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

KUBECTL_VERSION="v1.34.1"
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
            sha256="7721f265e18709862655affba5343e85e1980639395d5754473dafaadcaa69e3"
            ;;
        linux-arm64)
            sha256="420e6110e3ba7ee5a3927b5af868d18df17aae36b720529ffa4e9e945aa95450"
            ;;
        darwin-amd64)
            sha256="bb211f2b31f2b3bc60562b44cc1e3b712a16a98e9072968ba255beb04cefcfdf"
            ;;
        darwin-arm64)
            sha256="d80e5fa36f2b14005e5bb35d3a72818acb1aea9a081af05340a000e5fbdb2f76"
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
