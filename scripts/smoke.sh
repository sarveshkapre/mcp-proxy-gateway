#!/usr/bin/env bash
set -euo pipefail

PORT=8099
UPSTREAM_PORT=8100
REPLAY_FILE="./records.example.ndjson"
POLICY_FILE="./policy.example.yaml"
SMOKE_BIN="/tmp/mcp-proxy-gateway-smoke-bin"

REQUEST='{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"tool":"web.search","arguments":{"query":"hello","max_results":3}}}'
NOTIFICATION='{"jsonrpc":"2.0","method":"tools/call","params":{"tool":"web.search","arguments":{"query":"hello","max_results":3}}}'

cleanup() {
  if [ -n "${SERVER_PID:-}" ]; then
    kill "$SERVER_PID" >/dev/null 2>&1 || true
    wait "$SERVER_PID" >/dev/null 2>&1 || true
  fi
  if [ -n "${UPSTREAM_PID:-}" ]; then
    kill "$UPSTREAM_PID" >/dev/null 2>&1 || true
    wait "$UPSTREAM_PID" >/dev/null 2>&1 || true
  fi
  rm -f "$SMOKE_BIN" >/dev/null 2>&1 || true
}
trap cleanup EXIT

stop_server() {
  if [ -n "${SERVER_PID:-}" ]; then
    kill "$SERVER_PID" >/dev/null 2>&1 || true
    wait "$SERVER_PID" >/dev/null 2>&1 || true
    SERVER_PID=""
  fi
}

# Build once so PID tracking works reliably (avoid `go run` spawning orphaned children).
go build -o "$SMOKE_BIN" ./cmd/mcp-proxy-gateway

# Start gateway in replay mode.
"$SMOKE_BIN" \
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

notif_body="/tmp/mcp-proxy-gateway-smoke-notification-body.txt"
notif_status=$(printf "%s" "$NOTIFICATION" | curl -sS -X POST "http://localhost:${PORT}/rpc" \
  -H 'Content-Type: application/json' \
  -d @- \
  -o "$notif_body" \
  -w '%{http_code}')
[ "$notif_status" = "204" ]
[ ! -s "$notif_body" ]

health=$(curl -sS "http://localhost:${PORT}/healthz")
echo "$health" | grep -q '"ok":true'

metrics=$(curl -sS "http://localhost:${PORT}/metricsz")
echo "$metrics" | grep -q '"requests_total"'
echo "$metrics" | grep -Eq '"requests_total":[[:space:]]*[1-9]'
echo "$metrics" | grep -q '"latency_buckets_ms"'

echo "smoke ok"

stop_server

# Start a minimal upstream stub (SSE + JSON).
python3 - <<PY >/tmp/mcp-proxy-gateway-upstream.log 2>&1 &
import json
import time
from http.server import BaseHTTPRequestHandler, HTTPServer

class Handler(BaseHTTPRequestHandler):
    def do_POST(self):
        if self.path != "/rpc":
            self.send_response(404)
            self.end_headers()
            return
        auth = self.headers.get("Authorization", "")
        if auth != "Bearer smoke":
            self.send_response(401)
            self.end_headers()
            return
        length = int(self.headers.get("Content-Length", "0"))
        body = self.rfile.read(length)
        try:
            req = json.loads(body.decode("utf-8"))
        except Exception:
            self.send_response(400)
            self.end_headers()
            return

        accept = (self.headers.get("Accept") or "").lower()
        if "text/event-stream" in accept:
            self.send_response(200)
            self.send_header("Content-Type", "text/event-stream")
            self.send_header("Cache-Control", "no-store")
            self.end_headers()
            self.wfile.write(b"data: hello\\n\\n")
            self.wfile.flush()
            time.sleep(0.05)
            self.wfile.write(b"data: done\\n\\n")
            self.wfile.flush()
            return

        resp = {"jsonrpc": "2.0", "id": req.get("id"), "result": {"ok": True}}
        data = json.dumps(resp).encode("utf-8")
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def log_message(self, format, *args):
        return

HTTPServer(("127.0.0.1", ${UPSTREAM_PORT}), Handler).serve_forever()
PY
UPSTREAM_PID=$!

# Wait for upstream to be ready (retry for ~5s).
up_ready=0
for _ in $(seq 1 25); do
  code=$(printf "%s" "$REQUEST" | curl -s -X POST "http://localhost:${UPSTREAM_PORT}/rpc" \
    -H 'Content-Type: application/json' \
    -H 'Authorization: Bearer smoke' \
    -d @- \
    -o /dev/null \
    -w '%{http_code}' 2>/dev/null || true)
  if [ "$code" = "200" ]; then
    up_ready=1
    break
  fi
  sleep 0.2
done
if [ "$up_ready" -ne 1 ]; then
  echo "upstream not ready" >&2
  exit 1
fi

# Start gateway in proxy mode (policy + upstream stub).
"$SMOKE_BIN" \
  --listen :${PORT} \
  --upstream "http://localhost:${UPSTREAM_PORT}/rpc" \
  --policy "${POLICY_FILE}" \
  --prometheus-metrics \
  >/tmp/mcp-proxy-gateway-smoke.log 2>&1 &
SERVER_PID=$!

ready=0
for _ in $(seq 1 25); do
  if curl -sS "http://localhost:${PORT}/healthz" >/dev/null 2>&1; then
    ready=1
    break
  fi
  sleep 0.2
done
if [ "$ready" -ne 1 ]; then
  echo "server not ready (proxy mode)" >&2
  exit 1
fi

# Verify Origin allowlist rejects unexpected browser-originated requests.
origin_status=$(printf "%s" "$REQUEST" | curl -sS -X POST "http://localhost:${PORT}/rpc" \
  -H 'Content-Type: application/json' \
  -H 'Origin: http://evil.local' \
  -d @- \
  -o /dev/null \
  -w '%{http_code}')
[ "$origin_status" = "403" ]

# Verify Prometheus metrics endpoint is available when enabled.
prom=$(curl -sS "http://localhost:${PORT}/metrics")
echo "$prom" | grep -q 'mcp_proxy_gateway_requests_total'

# Verify SSE passthrough (also confirms Authorization header forwarding).
sse_headers="/tmp/mcp-proxy-gateway-smoke-sse-headers.txt"
sse_body="/tmp/mcp-proxy-gateway-smoke-sse-body.txt"
sse_status=$(printf "%s" "$REQUEST" | curl -sS -N -X POST "http://localhost:${PORT}/rpc" \
  -H 'Content-Type: application/json' \
  -H 'Accept: text/event-stream' \
  -H 'Authorization: Bearer smoke' \
  -d @- \
  -D "$sse_headers" \
  -o "$sse_body" \
  -w '%{http_code}')
[ "$sse_status" = "200" ]
grep -qi '^content-type:[[:space:]]*text/event-stream' "$sse_headers"
grep -q 'data: hello' "$sse_body"
grep -q 'data: done' "$sse_body"

echo "smoke stream ok"
