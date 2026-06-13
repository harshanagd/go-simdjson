#!/bin/bash
set -euo pipefail

usage() {
  echo "Usage: $0 <version>"
  echo "  e.g. $0 4.6.1"
  echo ""
  echo "Downloads simdjson.h and simdjson.cpp from the given release,"
  echo "updates SIMDJSON_VERSION, and runs 'make release' to verify."
  exit 0
}

[[ "${1:-}" == "-h" || "${1:-}" == "--help" ]] && usage
[[ $# -ne 1 ]] && { echo "Error: version argument required. Use -h for help." >&2; exit 1; }

VERSION="$1"
BASE_URL="https://github.com/simdjson/simdjson/releases/download/v${VERSION}"

echo "==> Downloading simdjson.h and simdjson.cpp v${VERSION}..."
curl -sL --fail -o simdjson.h "${BASE_URL}/simdjson.h"
curl -sL --fail -o simdjson.cpp "${BASE_URL}/simdjson.cpp"

echo "==> Computing SHA256 hashes..."
SHA256_H=$(shasum -a 256 simdjson.h | awk '{print $1}')
SHA256_CPP=$(shasum -a 256 simdjson.cpp | awk '{print $1}')

echo "    simdjson.h:   ${SHA256_H}"
echo "    simdjson.cpp: ${SHA256_CPP}"

echo "==> Updating SIMDJSON_VERSION..."
cat > SIMDJSON_VERSION <<EOF
VERSION=${VERSION}
SOURCE=${BASE_URL}/
SHA256_H=${SHA256_H}
SHA256_CPP=${SHA256_CPP}
EOF

echo "==> Running make release..."
make release

echo "==> Done! simdjson updated to v${VERSION}"
