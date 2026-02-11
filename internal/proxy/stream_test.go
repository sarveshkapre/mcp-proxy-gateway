package proxy

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sarveshkapre/mcp-proxy-gateway/internal/record"
)

func TestSSEPassthroughStreamsAndSkipsRecord(t *testing.T) {
	t.Parallel()

	upstreamSawAuth := make(chan string, 1)
	upstreamSawAccept := make(chan string, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamSawAuth <- r.Header.Get("Authorization")
		upstreamSawAccept <- r.Header.Get("Accept")

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)

		fl, _ := w.(http.Flusher)
		_, _ = fmt.Fprint(w, "data: hello\n\n")
		if fl != nil {
			fl.Flush()
		}
		_, _ = fmt.Fprint(w, "data: done\n\n")
	}))
	t.Cleanup(upstream.Close)

	tmpDir := t.TempDir()
	recordPath := filepath.Join(tmpDir, "records.ndjson")
	rec := record.NewRecorder(recordPath, nil, 0, 0)

	upstreamURL := mustParseURL(t, upstream.URL)
	srv := NewServer(upstreamURL, nil, rec, nil, false, nil, nil, false, 1024, 5*time.Second, nil)
	gw := httptest.NewServer(srv)
	t.Cleanup(gw.Close)

	reqBody := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"tool":"web.search","arguments":{"query":"hello"}}}`)
	req, err := http.NewRequest(http.MethodPost, gw.URL+"/rpc", bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer test-token")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s", resp.StatusCode, string(b))
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("content-type=%q", ct)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !bytes.Contains(body, []byte("data: hello")) {
		t.Fatalf("expected SSE body to contain hello, got=%q", string(body))
	}

	if got := <-upstreamSawAccept; !strings.Contains(strings.ToLower(got), "text/event-stream") {
		t.Fatalf("upstream Accept=%q", got)
	}
	if got := <-upstreamSawAuth; got != "Bearer test-token" {
		t.Fatalf("upstream Authorization=%q", got)
	}

	if _, err := os.Stat(recordPath); err == nil {
		t.Fatalf("expected streamed request to skip recorder append, but record file exists at %s", recordPath)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat recordPath: %v", err)
	}
}

func TestUnexpectedSSEWithoutClientAcceptReturnsJSONRPCError(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, "data: hello\n\n")
	}))
	t.Cleanup(upstream.Close)

	upstreamURL := mustParseURL(t, upstream.URL)
	srv := NewServer(upstreamURL, nil, nil, nil, false, nil, nil, false, 1024, 5*time.Second, nil)
	gw := httptest.NewServer(srv)
	t.Cleanup(gw.Close)

	reqBody := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"tool":"web.search","arguments":{"query":"hello"}}}`)
	req, err := http.NewRequest(http.MethodPost, gw.URL+"/rpc", bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s", resp.StatusCode, string(b))
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content-type=%q", ct)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !bytes.Contains(body, []byte("requires Accept: text/event-stream")) {
		t.Fatalf("expected upstream-streaming error, got=%q", string(body))
	}
}
