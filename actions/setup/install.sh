#!/usr/bin/env bash
# Downloads the standalone semfrag binary for this runner and adds it to PATH.
# Shared by the setup, prepare and check actions.
set -euo pipefail

version="${SEMFRAG_INSTALL_VERSION:-}"
repo="joe-p/semfrag"

# Default to the version in this action's own package.json, so pinning the
# action to a commit also pins the binary. At a release commit this is the
# release itself; at any other commit it is the previous release.
if [ -z "$version" ]; then
  package_json="$(dirname "${BASH_SOURCE[0]}")/../../package.json"
  version="$(sed -nE 's/^[[:space:]]*"version"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/p' "$package_json" | head -n 1)"
  if [ -z "$version" ]; then
    echo "::error::Could not read the semfrag version from ${package_json}; pass the version input."
    exit 1
  fi
fi

case "${RUNNER_OS:-$(uname -s)}" in
  Linux) os="linux" ;;
  macOS | Darwin) os="darwin" ;;
  Windows) os="windows" ;;
  *)
    echo "::error::Unsupported runner OS: ${RUNNER_OS:-$(uname -s)}"
    exit 1
    ;;
esac

case "${RUNNER_ARCH:-$(uname -m)}" in
  X64 | x86_64 | amd64) arch="x64" ;;
  ARM64 | arm64 | aarch64) arch="arm64" ;;
  *)
    echo "::error::Unsupported runner architecture: ${RUNNER_ARCH:-$(uname -m)}"
    exit 1
    ;;
esac

asset="semfrag-${os}-${arch}"
if [ "$os" = "linux" ] && { [ -f /etc/alpine-release ] || ldd --version 2>&1 | grep -qi musl; }; then
  asset="${asset}-musl"
fi
exe="semfrag"
if [ "$os" = "windows" ]; then
  asset="${asset}.exe"
  exe="semfrag.exe"
fi

if [ "$version" = "latest" ]; then
  url="https://github.com/${repo}/releases/latest/download/${asset}"
else
  url="https://github.com/${repo}/releases/download/v${version#v}/${asset}"
fi

bin_dir="${RUNNER_TEMP:-${TMPDIR:-/tmp}}/semfrag/bin"
mkdir -p "$bin_dir"
echo "Downloading ${url}"
curl -fsSL --retry 3 -o "${bin_dir}/${exe}" "$url"
chmod +x "${bin_dir}/${exe}"

installed="$("${bin_dir}/${exe}" --version)"
echo "Installed semfrag ${installed}"

if [ -n "${GITHUB_PATH:-}" ]; then
  if command -v cygpath >/dev/null 2>&1; then
    cygpath -w "$bin_dir" >> "$GITHUB_PATH"
  else
    echo "$bin_dir" >> "$GITHUB_PATH"
  fi
fi
if [ -n "${GITHUB_OUTPUT:-}" ]; then
  echo "version=${installed}" >> "$GITHUB_OUTPUT"
fi
