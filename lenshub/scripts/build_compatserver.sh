#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT/compatserver"
GOOS=${GOOS:-linux} GOARCH=${GOARCH:-amd64} go build -trimpath -o "$ROOT/compatserver/lenshub-p2p-compat" .
echo "built: $ROOT/compatserver/lenshub-p2p-compat"
