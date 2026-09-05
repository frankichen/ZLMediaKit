#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN="$ROOT/compatserver/lenshub-p2p-compat"
CFG="$ROOT/compatserver/config.example.json"

cd "$ROOT/compatserver"
go test ./...
if [[ ! -x "$BIN" ]]; then
  "$ROOT/scripts/build_compatserver.sh"
fi

"$BIN" -config "$CFG" >/tmp/lenshub-p2p-compat-smoke.log 2>&1 &
pid=$!
cleanup() {
  kill "$pid" >/dev/null 2>&1 || true
  wait "$pid" >/dev/null 2>&1 || true
}
trap cleanup EXIT

for _ in {1..20}; do
  if curl -fsS http://127.0.0.1:18180/healthz >/tmp/lenshub-p2p-compat-health.json 2>/dev/null; then
    break
  fi
  sleep 0.1
done
curl -fsS http://127.0.0.1:18180/healthz >/tmp/lenshub-p2p-compat-health.json
go run ./cmd/ppppprobe -addr 127.0.0.1:12305 -did PPCS-020070-BNRLZ
curl -fsS http://127.0.0.1:18180/readyz >/tmp/lenshub-p2p-compat-ready.json
printf '{"cmd":"stats"}\n' | nc -w 2 127.0.0.1 12306 >/tmp/lenshub-p2p-compat-stats.json

cat /tmp/lenshub-p2p-compat-health.json; echo
cat /tmp/lenshub-p2p-compat-ready.json; echo
cat /tmp/lenshub-p2p-compat-stats.json; echo
