package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sarveshkapre/mcp-proxy-gateway/internal/jsonrpc"
	"github.com/sarveshkapre/mcp-proxy-gateway/internal/record"
	"github.com/sarveshkapre/mcp-proxy-gateway/internal/signature"
)

func BenchmarkServeHTTPBatchReplayHit(b *testing.B) {
	replay := benchmarkReplayStore(b, []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"tool":"web.search","arguments":{"query":"bench"}}}`), []byte(`{"jsonrpc":"2.0","id":1,"result":{"ok":true}}`))
	srv := NewServer(nil, nil, nil, replay, true, nil, nil, false, 1<<20, 2*time.Second, nil)
	batch := benchmarkBatchPayload(10)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		r := httptest.NewRequest(http.MethodPost, "/rpc", bytes.NewReader(batch))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			b.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
	}
}

func BenchmarkServeHTTPBatchUpstream(b *testing.B) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"ok":true}}`))
	}))
	b.Cleanup(upstream.Close)

	srv := NewServer(mustParseURLFromBenchmark(b, upstream.URL), nil, nil, nil, false, nil, nil, false, 1<<20, 2*time.Second, nil)
	batch := benchmarkBatchPayload(10)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		r := httptest.NewRequest(http.MethodPost, "/rpc", bytes.NewReader(batch))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			b.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
	}
}

func benchmarkBatchPayload(size int) []byte {
	var sb strings.Builder
	sb.WriteString("[")
	for i := 1; i <= size; i++ {
		if i > 1 {
			sb.WriteString(",")
		}
		sb.WriteString(fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"tools/call","params":{"tool":"web.search","arguments":{"query":"bench"}}}`, i))
	}
	sb.WriteString("]")
	return []byte(sb.String())
}

func benchmarkReplayStore(b *testing.B, rawReq []byte, rawResp []byte) *record.ReplayStore {
	b.Helper()

	req := jsonrpc.Request{}
	if err := json.Unmarshal(rawReq, &req); err != nil {
		b.Fatalf("unmarshal request: %v", err)
	}
	sig, err := signature.FromRequest(&req)
	if err != nil {
		b.Fatalf("signature: %v", err)
	}

	file, err := os.CreateTemp(b.TempDir(), "replay-*.ndjson")
	if err != nil {
		b.Fatalf("temp file: %v", err)
	}
	_ = file.Close()

	rec := record.NewRecorder(file.Name(), nil, 0, 0)
	if err := rec.Append(sig, rawReq, rawResp); err != nil {
		b.Fatalf("append: %v", err)
	}
	store, err := record.LoadReplay(file.Name(), record.ReplayMatchSignature)
	if err != nil {
		b.Fatalf("load replay: %v", err)
	}
	return store
}

func mustParseURLFromBenchmark(b *testing.B, raw string) *url.URL {
	b.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		b.Fatalf("parse url: %v", err)
	}
	return u
}
