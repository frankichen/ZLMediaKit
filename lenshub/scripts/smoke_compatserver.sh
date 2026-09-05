#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN="$ROOT/compatserver/lenshub-p2p-compat"
CFG="$ROOT/compatserver/config.example.json"
if [[ ! -x "$BIN" ]]; then
  "$ROOT/scripts/build_compatserver.sh"
fi
"$BIN" -config "$CFG" > /tmp/lenshub-p2p-compat-smoke.log 2>&1 &
pid=$!
cleanup() { kill "$pid" >/dev/null 2>&1 || true; wait "$pid" >/dev/null 2>&1 || true; }
trap cleanup EXIT
sleep 1
curl -fsS http://127.0.0.1:18180/healthz >/tmp/lenshub-p2p-compat-health.json
printf '{"cmd":"register","group_id":"gongshi-test-group-01","p2pid":"PPCS-GSTEST-20260905-0001"}\n' | nc -w 2 127.0.0.1 12306 >/tmp/lenshub-p2p-compat-register.json
printf '{"cmd":"lookup","group_id":"gongshi-test-group-01","p2pid":"PPCS-GSTEST-20260905-0001"}\n' | nc -w 2 127.0.0.1 12308 >/tmp/lenshub-p2p-compat-lookup.json
cat /tmp/lenshub-p2p-compat-health.json; echo
cat /tmp/lenshub-p2p-compat-register.json; echo
cat /tmp/lenshub-p2p-compat-lookup.json; echo
