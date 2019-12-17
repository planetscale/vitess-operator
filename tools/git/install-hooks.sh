#!/bin/bash

# Install git hooks.

set -euo pipefail

# Go to repo root.
DIR="${BASH_SOURCE%/*}"
cd "${DIR}/../.."

mkdir -p .git/hooks
ln -sf "../../tools/git/commit-msg" .git/hooks/commit-msg
git config core.hooksPath .git/hooks
