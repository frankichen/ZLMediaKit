#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN="$ROOT/compatserver/lenshub-p2p-compat"
CFG="$(mktemp /tmp/lenshub-p2p-compat-smoke-config.XXXXXX.json)"

cat >"$CFG" <<'JSON'
{
  "provider_type": "p2px_ppcs",
  "p2p_server_group_id": "gongshi-test-group-01",
  "public_ip": "127.0.0.1",
  "wakeup_udp_port": 12305,
  "pppp_udp_bind_ip": "127.0.0.1",
  "plain_tcp_port": 12306,
  "dslk_tcp_port": 12308,
  "health_http_addr": "127.0.0.1:18180",
  "allowed_did_prefixes": ["PPCS"],
  "presence_ttl_seconds": 90,
  "pppp_psk_env": "",
  "diagnostic_tcp_addr": "127.0.0.1:18181",
  "unsafe_allow_unverified_did_login_for_test": true
}
JSON

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
  rm -f "$CFG"
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
printf '{"cmd":"stats"}\n' | nc -w 2 127.0.0.1 18181 >/tmp/lenshub-p2p-compat-stats.json

cat /tmp/lenshub-p2p-compat-health.json; echo
cat /tmp/lenshub-p2p-compat-ready.json; echo
cat /tmp/lenshub-p2p-compat-stats.json; echo
