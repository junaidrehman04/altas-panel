#!/usr/bin/env bash
# Build the GitHub Release artifacts: linux binaries, tarballs, SHA256SUMS, install.sh.
# Usage: scripts/build-release.sh v0.3.0
set -euo pipefail

version=${1:-}
if [ -z "$version" ]; then
  echo "usage: scripts/build-release.sh vX.Y.Z" >&2
  exit 1
fi
case "$version" in
  v[0-9]*.[0-9]*.[0-9]*) ;;
  *)
    echo "version must look like v0.3.0" >&2
    exit 1
    ;;
esac

root=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
cd "$root"

npm ci --prefix apps/web
npm run build --prefix apps/web
rm -rf apps/api/internal/webui/dist
mkdir -p apps/api/internal/webui/dist
cp -a apps/web/dist/. apps/api/internal/webui/dist/

out="${root}/dist/release"
rm -rf "$out"
mkdir -p "$out"

for arch in amd64 arm64; do
  CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build -trimpath \
    -ldflags "-s -w -X github.com/junaidrehman04/altas/apps/api/internal/version.Version=${version}" \
    -o "${out}/altas-panel-linux-${arch}" ./apps/api/cmd/server
  stage=$(mktemp -d)
  cp "${out}/altas-panel-linux-${arch}" "${stage}/altas-panel"
  tar -C "$stage" -czf "${out}/altas-panel-linux-${arch}.tar.gz" altas-panel
  rm -rf "$stage"
done

cp "${root}/scripts/install.sh" "${out}/install.sh"
(
  cd "$out"
  sha256sum \
    altas-panel-linux-amd64 \
    altas-panel-linux-arm64 \
    altas-panel-linux-amd64.tar.gz \
    altas-panel-linux-arm64.tar.gz \
    install.sh \
    > SHA256SUMS
)
echo "release artifacts: ${out}"
