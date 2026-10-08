#!/usr/bin/env bash
# Builds the standalone semfrag binaries for every supported target. Go can
# cross-compile all of them from a single runner. With CGO disabled the binaries
# are static, so the "musl" assets are identical to the regular Linux builds;
# they are kept for backwards-compatible asset names.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

outdir="${OUTDIR:-build/binaries}"
version="${VERSION:-$(cat VERSION)}"

rm -rf "$outdir"
mkdir -p "$outdir"

ldflags="-s -w -X main.version=${version}"

emit() {
  local goos="$1" goarch="$2" name="$3"
  echo "Building ${goos}/${goarch} -> ${outdir}/${name}"
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
    go build -trimpath -ldflags "$ldflags" -o "${outdir}/${name}" ./cmd/semfrag
}

# goos/goarch/asset-arch triples. Asset names use x64 rather than amd64.
for spec in linux/amd64/x64 linux/arm64/arm64 darwin/amd64/x64 darwin/arm64/arm64 windows/amd64/x64 windows/arm64/arm64; do
  IFS=/ read -r goos goarch assetarch <<<"$spec"
  ext=""
  [ "$goos" = "windows" ] && ext=".exe"
  emit "$goos" "$goarch" "semfrag-${goos}-${assetarch}${ext}"
  if [ "$goos" = "linux" ]; then
    cp "${outdir}/semfrag-${goos}-${assetarch}${ext}" "${outdir}/semfrag-${goos}-${assetarch}-musl${ext}"
  fi
done

echo "Built standalone binaries for v${version} in ${outdir}"
