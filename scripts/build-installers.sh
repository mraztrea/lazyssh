#!/usr/bin/env bash
# Build installers (.deb/.rpm/.pkg.tar.zst/.apk) and archives into dist/.
# Usage: scripts/build-installers.sh [version]   e.g. scripts/build-installers.sh v1.0.0
set -euo pipefail

cd "$(dirname "$0")/.."
GORELEASER="github.com/goreleaser/goreleaser/v2@v2.19.0"

if [[ $# -eq 0 ]]; then
  go run "$GORELEASER" release --snapshot --clean --skip=publish
else
  # GoReleaser takes the version from the git tag on HEAD.
  git rev-parse -q --verify "refs/tags/$1" >/dev/null || git tag "$1"
  go run "$GORELEASER" release --clean --skip=publish,validate
fi

echo
echo "Output in dist/:"
ls -1 dist | grep -E '\.(deb|rpm|apk|pkg\.tar\.zst|tar\.gz|zip)$'
