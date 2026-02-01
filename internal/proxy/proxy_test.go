package proxy

import (
  "bytes"
  "context"
  "encoding/json"
  "io"
  "net/http"
  "net/http/httptest"
  "net/url"
  "os"
  "testing"
  "time"

  "github.com/sarveshkapre/mcp-proxy-gateway/internal/jsonrpc"
  "github.com/sarveshkapre/mcp-proxy-gateway/internal/record"
  "github.com/sarveshkapre/mcp-proxy-gateway/internal/signature"
)

func TestHealthz(t *testing.T) {
  srv := NewServer(nil, nil, nil, nil, false, 1024, time.Second, nil)

  r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
  w := httptest.NewRecorder()
  srv.ServeHTTP(w, r)

  if w.Code != http.StatusOK {
    t.Fatalf("status=%d", w.Code)
  }
  if ct := w.Header().Get("Content-Type"); ct != "application/json" {
    t.Fatalf("content-type=%q", ct)
  }

  var body map[string]any
  if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
    t.Fatalf("parse json: %v", err)
  }
  if ok, _ := body["ok"].(bool); !ok {
    t.Fatalf("expected ok=true")
  }
  if upstreamConfigured, _ := body["upstream_configured"].(bool); upstreamConfigured {
    t.Fatalf("expected upstream_configured=false")
  }
}

func TestRequestTooLargeReturns413(t *testing.T) {
  srv := NewServer(nil, nil, nil, nil, false, 10, time.Second, nil)

  r := httptest.NewRequest(http.MethodPost, "/rpc", bytes.NewReader([]byte("01234567890")))
  r.Header.Set("Content-Type", "application/json")
  w := httptest.NewRecorder()
  srv.ServeHTTP(w, r)

  if w.Code != http.StatusRequestEntityTooLarge {
    t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
  }
  if !bytes.Contains(w.Body.Bytes(), []byte("request too large")) {
    t.Fatalf("expected error message, got=%s", w.Body.String())
  }
}

func TestUpstreamResponseTooLargeReturnsJSONRPCError(t *testing.T) {
  upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    _, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"`))
    _, _ = w.Write(bytes.Repeat([]byte("a"), 50))
    _, _ = w.Write([]byte(`"}`))
  }))
  t.Cleanup(upstream.Close)

  upstreamURL, err := url.Parse(upstream.URL)
  if err != nil {
    t.Fatalf("parse upstream url: %v", err)
  }

  srv := NewServer(upstreamURL, nil, nil, nil, false, 40, time.Second, nil)

  req := []byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}`)
  r := httptest.NewRequest(http.MethodPost, "/rpc", bytes.NewReader(req))
  r.Header.Set("Content-Type", "application/json")
  w := httptest.NewRecorder()
  srv.ServeHTTP(w, r)

  if w.Code != http.StatusOK {
    t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
  }
  if !bytes.Contains(w.Body.Bytes(), []byte("upstream response too large")) {
    t.Fatalf("expected upstream response too large error, got=%s", w.Body.String())
  }
}

func TestUpstreamRequestUsesClientContext(t *testing.T) {
  type ctxKey struct{}
  started := make(chan struct{}, 1)

  transport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
    if req.Context().Value(ctxKey{}) != "marker" {
      t.Fatalf("expected context marker to propagate to upstream request")
    }
    started <- struct{}{}
    <-req.Context().Done()
    return nil, req.Context().Err()
  })

  srv := NewServer(mustParseURL(t, "http://example.invalid"), nil, nil, nil, false, 1024, 10*time.Second, nil)
  srv.client.Transport = transport

  reqBody := []byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}`)
  r := httptest.NewRequest(http.MethodPost, "/rpc", bytes.NewReader(reqBody))
  r.Header.Set("Content-Type", "application/json")

  ctx := context.WithValue(r.Context(), ctxKey{}, "marker")
  ctx, cancel := context.WithCancel(ctx)
  r = r.WithContext(ctx)
  defer cancel()

  w := httptest.NewRecorder()
  done := make(chan struct{})
  go func() {
    srv.ServeHTTP(w, r)
    close(done)
  }()

  select {
  case <-started:
  case <-time.After(2 * time.Second):
    t.Fatalf("upstream round trip did not start")
  }

  cancel()

  select {
  case <-done:
  case <-time.After(500 * time.Millisecond):
    t.Fatalf("proxy did not return quickly after cancellation")
  }

  if w.Code != http.StatusOK {
    t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
  }
  if !bytes.Contains(w.Body.Bytes(), []byte("upstream error")) {
    t.Fatalf("expected upstream error, got=%s", w.Body.String())
  }
}

func TestBatchReplay(t *testing.T) {
  req1 := json.RawMessage(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"tool":"web.search","arguments":{"query":"a"}}}`)
  req2 := json.RawMessage(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"tool":"web.search","arguments":{"query":"b"}}}`)

  replay := mustReplayStore(t, map[string]json.RawMessage{
    mustSig(t, req1): json.RawMessage(`{"jsonrpc":"2.0","id":1,"result":{"ok":true,"n":1}}`),
    mustSig(t, req2): json.RawMessage(`{"jsonrpc":"2.0","id":2,"result":{"ok":true,"n":2}}`),
  })

  srv := NewServer(nil, nil, nil, replay, true, 1024, time.Second, nil)
  batch := json.RawMessage("[" + string(req1) + "," + string(req2) + "]")

  r := httptest.NewRequest(http.MethodPost, "/rpc", bytes.NewReader(batch))
  r.Header.Set("Content-Type", "application/json")
  w := httptest.NewRecorder()
  srv.ServeHTTP(w, r)

  if w.Code != http.StatusOK {
    t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
  }

  var out []map[string]any
  if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
    t.Fatalf("unmarshal response: %v", err)
  }
  if len(out) != 2 {
    t.Fatalf("expected 2 responses, got %d", len(out))
  }
}

func TestBatchForwardsSequentially(t *testing.T) {
  calls := 0
  upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    calls++
    body, _ := io.ReadAll(r.Body)
    req := jsonrpc.Request{}
    _ = json.Unmarshal(body, &req)

    w.Header().Set("Content-Type", "application/json")
    resp, _ := json.Marshal(map[string]any{
      "jsonrpc": "2.0",
      "id":      json.RawMessage(req.ID),
      "result":  map[string]any{"ok": true},
    })
    _, _ = w.Write(resp)
  }))
  t.Cleanup(upstream.Close)

  upstreamURL, err := url.Parse(upstream.URL)
  if err != nil {
    t.Fatalf("parse upstream url: %v", err)
  }
  srv := NewServer(upstreamURL, nil, nil, nil, false, 1024, time.Second, nil)

  batch := json.RawMessage(`[
    {"jsonrpc":"2.0","id":1,"method":"ping"},
    {"jsonrpc":"2.0","id":2,"method":"ping"}
  ]`)

  r := httptest.NewRequest(http.MethodPost, "/rpc", bytes.NewReader(batch))
  r.Header.Set("Content-Type", "application/json")
  w := httptest.NewRecorder()
  srv.ServeHTTP(w, r)

  if w.Code != http.StatusOK {
    t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
  }

  var out []map[string]any
  if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
    t.Fatalf("unmarshal response: %v", err)
  }
  if len(out) != 2 {
    t.Fatalf("expected 2 responses, got %d", len(out))
  }
  if calls != 2 {
    t.Fatalf("expected 2 upstream calls, got %d", calls)
  }
}

func TestBatchNotificationsReturn204(t *testing.T) {
  srv := NewServer(nil, nil, nil, nil, false, 1024, time.Second, nil)
  batch := json.RawMessage(`[
    {"jsonrpc":"2.0","method":"ping"},
    {"jsonrpc":"2.0","method":"ping"}
  ]`)

  r := httptest.NewRequest(http.MethodPost, "/rpc", bytes.NewReader(batch))
  r.Header.Set("Content-Type", "application/json")
  w := httptest.NewRecorder()
  srv.ServeHTTP(w, r)

  if w.Code != http.StatusNoContent {
    t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
  }
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func mustParseURL(t *testing.T, raw string) *url.URL {
  t.Helper()
  u, err := url.Parse(raw)
  if err != nil {
    t.Fatalf("parse url: %v", err)
  }
  return u
}

func mustSig(t *testing.T, raw json.RawMessage) string {
  t.Helper()
  req := jsonrpc.Request{}
  if err := json.Unmarshal(raw, &req); err != nil {
    t.Fatalf("unmarshal request: %v", err)
  }
  sig, err := signature.FromRequest(&req)
  if err != nil {
    t.Fatalf("compute signature: %v", err)
  }
  return sig
}

func mustReplayStore(t *testing.T, entries map[string]json.RawMessage) *record.ReplayStore {
  t.Helper()
  file, err := os.CreateTemp(t.TempDir(), "records-*.ndjson")
  if err != nil {
    t.Fatalf("temp file: %v", err)
  }
  _ = file.Close()

  rec := record.NewRecorder(file.Name(), nil)
  for sig, resp := range entries {
    if err := rec.Append(sig, json.RawMessage(`{"jsonrpc":"2.0"}`), resp); err != nil {
      t.Fatalf("append: %v", err)
    }
  }
  store, err := record.LoadReplay(file.Name(), record.ReplayMatchSignature)
  if err != nil {
    t.Fatalf("load replay: %v", err)
  }
  return store
}
