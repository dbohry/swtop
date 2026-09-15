#!/usr/bin/env bash
# Prints the latest swtop release tag (e.g. v0.1.0) from GitHub Releases.
set -euo pipefail

REPO="dbohry/swtop"
API_URL="https://api.github.com/repos/${REPO}/releases/latest"

response=$(curl -fsSL -H "Accept: application/vnd.github+json" "$API_URL")

if command -v jq >/dev/null 2>&1; then
  version=$(jq -r '.tag_name' <<<"$response")
else
  version=$(grep -m1 '"tag_name"' <<<"$response" | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')
fi

if [ -z "${version:-}" ] || [ "$version" = "null" ]; then
  echo "Error: could not determine the latest release version" >&2
  exit 1
fi

echo "$version"
