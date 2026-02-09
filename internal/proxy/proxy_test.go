package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/sarveshkapre/mcp-proxy-gateway/internal/config"
	"github.com/sarveshkapre/mcp-proxy-gateway/internal/jsonrpc"
	"github.com/sarveshkapre/mcp-proxy-gateway/internal/record"
	"github.com/sarveshkapre/mcp-proxy-gateway/internal/signature"
	"github.com/sarveshkapre/mcp-proxy-gateway/internal/validate"
)

func TestHealthz(t *testing.T) {
	srv := NewServer(nil, nil, nil, nil, false, nil, nil, 1024, time.Second, nil)

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

func TestMetricsz(t *testing.T) {
	srv := NewServer(nil, nil, nil, nil, false, nil, nil, 1024, time.Second, nil)

	r := httptest.NewRequest(http.MethodGet, "/metricsz", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content-type=%q", ct)
	}

	body := readMetrics(t, srv)
	for _, key := range []string{
		"requests_total",
		"batch_items_total",
		"replay_hits_total",
		"replay_misses_total",
		"validation_rejects_total",
		"upstream_errors_total",
		"latency_buckets_ms",
	} {
		if _, ok := body[key]; !ok {
			t.Fatalf("missing key %q in metrics payload", key)
		}
	}
}

func TestMetricsReplayHitAndMissCounters(t *testing.T) {
	hitRequest := json.RawMessage(`{"jsonrpc":"2.0","id":1,"method":"ping","params":{"q":"hit"}}`)
	missRequest := json.RawMessage(`{"jsonrpc":"2.0","id":2,"method":"ping","params":{"q":"miss"}}`)
	replay := mustReplayStore(t, map[string]json.RawMessage{
		mustSig(t, hitRequest): json.RawMessage(`{"jsonrpc":"2.0","id":1,"result":{"ok":true}}`),
	})

	srv := NewServer(nil, nil, nil, replay, false, nil, nil, 1024, time.Second, nil)

	for _, req := range []json.RawMessage{hitRequest, missRequest} {
		r := httptest.NewRequest(http.MethodPost, "/rpc", bytes.NewReader(req))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, r)
	}

	metrics := readMetrics(t, srv)
	if got := metricValue(t, metrics, "requests_total"); got != 2 {
		t.Fatalf("requests_total=%d want=2", got)
	}
	if got := metricValue(t, metrics, "replay_hits_total"); got != 1 {
		t.Fatalf("replay_hits_total=%d want=1", got)
	}
	if got := metricValue(t, metrics, "replay_misses_total"); got != 1 {
		t.Fatalf("replay_misses_total=%d want=1", got)
	}
}

func TestReplayMatchByMethodAtProxyLayerRemapsID(t *testing.T) {
	recordedReq := json.RawMessage(`{"jsonrpc":"2.0","id":1,"method":"ping","params":{"q":"recorded"}}`)
	replay := mustReplayStoreMatch(t, record.ReplayMatchMethod, []replayPair{
		{
			req:  recordedReq,
			resp: json.RawMessage(`{"jsonrpc":"2.0","id":1,"result":{"ok":true}}`),
		},
	})

	srv := NewServer(nil, nil, nil, replay, true, nil, nil, 1024, time.Second, nil)

	liveReq := []byte(`{"jsonrpc":"2.0","id":42,"method":"ping","params":{"q":"live"}}`)
	r := httptest.NewRequest(http.MethodPost, "/rpc", bytes.NewReader(liveReq))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if id, ok := out["id"].(float64); !ok || id != 42 {
		t.Fatalf("expected id=42, got=%v", out["id"])
	}
}

func TestReplayMatchByToolAtProxyLayerRemapsID(t *testing.T) {
	recordedReq := json.RawMessage(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"tool":"web.search","arguments":{"query":"recorded"}}}`)
	replay := mustReplayStoreMatch(t, record.ReplayMatchTool, []replayPair{
		{
			req:  recordedReq,
			resp: json.RawMessage(`{"jsonrpc":"2.0","id":1,"result":{"ok":true}}`),
		},
	})

	srv := NewServer(nil, nil, nil, replay, true, nil, nil, 1024, time.Second, nil)

	liveReq := []byte(`{"jsonrpc":"2.0","id":99,"method":"tools/call","params":{"tool":"web.search","arguments":{"query":"live"}}}`)
	r := httptest.NewRequest(http.MethodPost, "/rpc", bytes.NewReader(liveReq))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if id, ok := out["id"].(float64); !ok || id != 99 {
		t.Fatalf("expected id=99, got=%v", out["id"])
	}
}

func TestReplayMatchByMethodNotificationReturns204(t *testing.T) {
	recordedReq := json.RawMessage(`{"jsonrpc":"2.0","id":1,"method":"ping","params":{"q":"recorded"}}`)
	replay := mustReplayStoreMatch(t, record.ReplayMatchMethod, []replayPair{
		{
			req:  recordedReq,
			resp: json.RawMessage(`{"jsonrpc":"2.0","id":1,"result":{"ok":true}}`),
		},
	})

	srv := NewServer(nil, nil, nil, replay, true, nil, nil, 1024, time.Second, nil)

	// Notification: omit id.
	liveReq := []byte(`{"jsonrpc":"2.0","method":"ping","params":{"q":"live"}}`)
	r := httptest.NewRequest(http.MethodPost, "/rpc", bytes.NewReader(liveReq))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, r)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if w.Body.Len() != 0 {
		t.Fatalf("expected empty body for notification, got=%q", w.Body.String())
	}
}

func TestMetricsValidationRejectCounter(t *testing.T) {
	validator, err := validate.New(&config.Policy{
		Mode:       "enforce",
		AllowTools: []string{"web.search"},
		Tools:      map[string]config.ToolEntry{},
	})
	if err != nil {
		t.Fatalf("validator init: %v", err)
	}

	srv := NewServer(nil, validator, nil, nil, false, nil, nil, 1024, time.Second, nil)
	req := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"tool":"fs.read","arguments":{"path":"/tmp/a"}}}`)
	r := httptest.NewRequest(http.MethodPost, "/rpc", bytes.NewReader(req))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}

	metrics := readMetrics(t, srv)
	if got := metricValue(t, metrics, "validation_rejects_total"); got != 1 {
		t.Fatalf("validation_rejects_total=%d want=1", got)
	}
	if got := metricValue(t, metrics, "requests_total"); got != 1 {
		t.Fatalf("requests_total=%d want=1", got)
	}
}

func TestMetricsUpstreamErrorCounter(t *testing.T) {
	srv := NewServer(mustParseURL(t, "http://example.invalid"), nil, nil, nil, false, nil, nil, 1024, time.Second, nil)
	srv.client.Transport = roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
		return nil, errors.New("dial failed")
	})

	req := []byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}`)
	r := httptest.NewRequest(http.MethodPost, "/rpc", bytes.NewReader(req))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}

	metrics := readMetrics(t, srv)
	if got := metricValue(t, metrics, "upstream_errors_total"); got != 1 {
		t.Fatalf("upstream_errors_total=%d want=1", got)
	}
}

func TestMetricsBatchItemsCounter(t *testing.T) {
	srv := NewServer(nil, nil, nil, nil, false, nil, nil, 1024, time.Second, nil)
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

	metrics := readMetrics(t, srv)
	if got := metricValue(t, metrics, "requests_total"); got != 1 {
		t.Fatalf("requests_total=%d want=1", got)
	}
	if got := metricValue(t, metrics, "batch_items_total"); got != 2 {
		t.Fatalf("batch_items_total=%d want=2", got)
	}
	latency := metricMap(t, metrics, "latency_buckets_ms")
	if got := metricValue(t, latency, "total"); got != 2 {
		t.Fatalf("latency_buckets_ms.total=%d want=2", got)
	}
}

func TestRequestTooLargeReturns413(t *testing.T) {
	srv := NewServer(nil, nil, nil, nil, false, nil, nil, 10, time.Second, nil)

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

	srv := NewServer(upstreamURL, nil, nil, nil, false, nil, nil, 40, time.Second, nil)

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

	srv := NewServer(mustParseURL(t, "http://example.invalid"), nil, nil, nil, false, nil, nil, 1024, 10*time.Second, nil)
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

	srv := NewServer(nil, nil, nil, replay, true, nil, nil, 1024, time.Second, nil)
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
	srv := NewServer(upstreamURL, nil, nil, nil, false, nil, nil, 1024, time.Second, nil)

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
	srv := NewServer(nil, nil, nil, nil, false, nil, nil, 1024, time.Second, nil)
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

func TestSingleNotificationReturns204AndForwards(t *testing.T) {
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":null,"result":{"ok":true}}`))
	}))
	t.Cleanup(upstream.Close)

	upstreamURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatalf("parse upstream url: %v", err)
	}

	srv := NewServer(upstreamURL, nil, nil, nil, false, nil, nil, 1024, time.Second, nil)
	req := []byte(`{"jsonrpc":"2.0","method":"ping"}`)

	r := httptest.NewRequest(http.MethodPost, "/rpc", bytes.NewReader(req))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, r)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if w.Body.Len() != 0 {
		t.Fatalf("expected empty body, got=%q", w.Body.String())
	}
	if calls != 1 {
		t.Fatalf("expected 1 upstream call, got %d", calls)
	}
}

func TestSingleReplayResponseIDIsRewritten(t *testing.T) {
	storedReq := json.RawMessage(`{"jsonrpc":"2.0","id":1,"method":"ping","params":{"q":"same"}}`)
	incomingReq := json.RawMessage(`{"jsonrpc":"2.0","id":99,"method":"ping","params":{"q":"same"}}`)

	replay := mustReplayStore(t, map[string]json.RawMessage{
		mustSig(t, storedReq): json.RawMessage(`{"jsonrpc":"2.0","id":1,"result":{"ok":true}}`),
	})

	srv := NewServer(nil, nil, nil, replay, true, nil, nil, 1024, time.Second, nil)
	r := httptest.NewRequest(http.MethodPost, "/rpc", bytes.NewReader(incomingReq))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if id, ok := out["id"].(float64); !ok || id != 99 {
		t.Fatalf("expected id=99, got=%v", out["id"])
	}
}

func TestSingleNotificationReplayHitReturns204(t *testing.T) {
	storedReq := json.RawMessage(`{"jsonrpc":"2.0","id":1,"method":"ping","params":{"q":"same"}}`)
	incomingReq := json.RawMessage(`{"jsonrpc":"2.0","method":"ping","params":{"q":"same"}}`)

	replay := mustReplayStore(t, map[string]json.RawMessage{
		mustSig(t, storedReq): json.RawMessage(`{"jsonrpc":"2.0","id":1,"result":{"ok":true}}`),
	})

	srv := NewServer(nil, nil, nil, replay, true, nil, nil, 1024, time.Second, nil)
	r := httptest.NewRequest(http.MethodPost, "/rpc", bytes.NewReader(incomingReq))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, r)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if w.Body.Len() != 0 {
		t.Fatalf("expected empty body, got=%q", w.Body.String())
	}
}

func TestBatchReplayResponseIDIsRewritten(t *testing.T) {
	storedReq := json.RawMessage(`{"jsonrpc":"2.0","id":1,"method":"ping","params":{"q":"same"}}`)
	incomingReq := json.RawMessage(`{"jsonrpc":"2.0","id":42,"method":"ping","params":{"q":"same"}}`)

	replay := mustReplayStore(t, map[string]json.RawMessage{
		mustSig(t, storedReq): json.RawMessage(`{"jsonrpc":"2.0","id":1,"result":{"ok":true}}`),
	})

	srv := NewServer(nil, nil, nil, replay, true, nil, nil, 1024, time.Second, nil)
	batch := json.RawMessage("[" + string(incomingReq) + "]")

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
	if len(out) != 1 {
		t.Fatalf("expected 1 response, got %d", len(out))
	}
	if id, ok := out[0]["id"].(float64); !ok || id != 42 {
		t.Fatalf("expected id=42, got=%v", out[0]["id"])
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

	rec := record.NewRecorder(file.Name(), nil, 0, 0)
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

type replayPair struct {
	req  json.RawMessage
	resp json.RawMessage
}

func mustReplayStoreMatch(t *testing.T, match record.ReplayMatch, pairs []replayPair) *record.ReplayStore {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "records-*.ndjson")
	if err != nil {
		t.Fatalf("temp file: %v", err)
	}
	_ = file.Close()

	rec := record.NewRecorder(file.Name(), nil, 0, 0)
	for _, pair := range pairs {
		sig := mustSig(t, pair.req)
		if err := rec.Append(sig, pair.req, pair.resp); err != nil {
			t.Fatalf("append: %v", err)
		}
	}
	store, err := record.LoadReplay(file.Name(), match)
	if err != nil {
		t.Fatalf("load replay: %v", err)
	}
	return store
}

func readMetrics(t *testing.T, srv *Server) map[string]any {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, "/metricsz", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("metrics status=%d body=%s", w.Code, w.Body.String())
	}
	out := map[string]any{}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal metrics: %v", err)
	}
	return out
}

func metricValue(t *testing.T, m map[string]any, key string) uint64 {
	t.Helper()
	raw, ok := m[key]
	if !ok {
		t.Fatalf("missing metric %q", key)
	}
	val, ok := raw.(float64)
	if !ok {
		t.Fatalf("metric %q has non-numeric value %T", key, raw)
	}
	return uint64(val)
}

func metricMap(t *testing.T, m map[string]any, key string) map[string]any {
	t.Helper()
	raw, ok := m[key]
	if !ok {
		t.Fatalf("missing metric map %q", key)
	}
	val, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("metric map %q has invalid value %T", key, raw)
	}
	return val
}
