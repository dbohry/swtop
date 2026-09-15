#!/usr/bin/env bash
# Downloads the latest swtop release binary for this OS/arch and installs
# it to ./swtop (or the path given as the first argument).
#
# If the repo is private, export GITHUB_TOKEN (a PAT with read access)
# first — GitHub requires authentication to download release assets from
# private repos.
set -euo pipefail

REPO="dbohry/swtop"
INSTALL_PATH="${1:-./swtop}"
API="https://api.github.com/repos/${REPO}"

auth_header=()
if [ -n "${GITHUB_TOKEN:-}" ]; then
  auth_header=(-H "Authorization: Bearer ${GITHUB_TOKEN}")
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
version=$("$SCRIPT_DIR/latest-version.sh")

os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m)
case "$arch" in
  x86_64|amd64) arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *) echo "Error: unsupported architecture: $arch" >&2; exit 1 ;;
esac
case "$os" in
  linux|darwin) ;;
  *) echo "Error: unsupported OS: $os (use the .zip release asset manually on Windows)" >&2; exit 1 ;;
esac

asset_name="swtop-${os}-${arch}.tar.gz"

release_json=$(curl -fsSL "${auth_header[@]+"${auth_header[@]}"}" "${API}/releases/tags/${version}")

if command -v jq >/dev/null 2>&1; then
  asset_url=$(jq -r --arg name "$asset_name" '.assets[] | select(.name == $name) | .url' <<<"$release_json")
else
  asset_url=$(grep -B3 "\"name\": \"${asset_name}\"" <<<"$release_json" | grep '"url"' | head -n1 | sed -E 's/.*"url": *"([^"]+)".*/\1/')
fi

if [ -z "${asset_url:-}" ]; then
  echo "Error: could not find release asset ${asset_name} for ${version}" >&2
  exit 1
fi

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "Downloading swtop ${version} for ${os}/${arch}..."
curl -fsSL "${auth_header[@]+"${auth_header[@]}"}" -H "Accept: application/octet-stream" "$asset_url" -o "$tmp/$asset_name"
tar -xzf "$tmp/$asset_name" -C "$tmp"

chmod +x "$tmp/swtop-${os}-${arch}"
mv "$tmp/swtop-${os}-${arch}" "$INSTALL_PATH"

echo "Installed swtop ${version} to ${INSTALL_PATH}"
