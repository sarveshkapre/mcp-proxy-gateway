#!/bin/sh
set -euo pipefail

PORT=8099
REPLAY_FILE="./records.example.ndjson"

REQUEST='{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"tool":"web.search","arguments":{"query":"hello","max_results":3}}}'

cleanup() {
  if [ -n "${SERVER_PID:-}" ]; then
    kill "$SERVER_PID" >/dev/null 2>&1 || true
    wait "$SERVER_PID" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

# Start gateway in replay mode.
go run ./cmd/mcp-proxy-gateway \
  --listen :${PORT} \
  --replay "${REPLAY_FILE}" \
  --replay-strict \
  >/tmp/mcp-proxy-gateway-smoke.log 2>&1 &
SERVER_PID=$!

# Wait for server to be ready (retry for ~5s).
ready=0
for _ in $(seq 1 25); do
  if curl -sS "http://localhost:${PORT}/healthz" >/dev/null 2>&1; then
    ready=1
    break
  fi
  sleep 0.2
done
if [ "$ready" -ne 1 ]; then
  echo "server not ready" >&2
  exit 1
fi

response=$(printf "%s" "$REQUEST" | curl -sS -X POST "http://localhost:${PORT}/rpc" \
  -H 'Content-Type: application/json' \
  -d @-)

echo "$response" | grep -q '"jsonrpc"' 
echo "$response" | grep -q '"Example"'

health=$(curl -sS "http://localhost:${PORT}/healthz")
echo "$health" | grep -q '"ok":true'

echo "smoke ok"
