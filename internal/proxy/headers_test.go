package proxy

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestUpstreamForwardHeadersAllowlist(t *testing.T) {
	t.Parallel()

	type seen struct {
		auth       string
		trace      string
		requestID  string
		cookie     string
		notForward string
	}
	gotCh := make(chan seen, 1)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCh <- seen{
			auth:       r.Header.Get("Authorization"),
			trace:      r.Header.Get("Traceparent"),
			requestID:  r.Header.Get("X-Request-Id"),
			cookie:     r.Header.Get("Cookie"),
			notForward: r.Header.Get("X-Not-Forwarded"),
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":{"ok":true}}`)
	}))
	t.Cleanup(upstream.Close)

	upstreamURL := mustParseURL(t, upstream.URL)
	upstreamURL.Path = "/rpc"

	srv := NewServer(upstreamURL, nil, nil, nil, false, nil, []string{"Traceparent", "X-Request-Id"}, 1024, 5*time.Second, nil)
	gw := httptest.NewServer(srv)
	t.Cleanup(gw.Close)

	reqBody := []byte(`{"jsonrpc":"2.0","id":1,"method":"ping","params":{}}`)
	req, err := http.NewRequest(http.MethodPost, gw.URL+"/rpc", bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	req.Header.Set("X-Request-Id", "rid-123")
	req.Header.Set("Cookie", "session=secret")
	req.Header.Set("X-Not-Forwarded", "nope")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)

	select {
	case got := <-gotCh:
		if got.auth != "Bearer test-token" {
			t.Fatalf("upstream Authorization=%q", got.auth)
		}
		if got.trace == "" {
			t.Fatalf("expected Traceparent to be forwarded")
		}
		if got.requestID != "rid-123" {
			t.Fatalf("upstream X-Request-Id=%q", got.requestID)
		}
		if got.cookie != "" {
			t.Fatalf("expected Cookie to not be forwarded, got=%q", got.cookie)
		}
		if got.notForward != "" {
			t.Fatalf("expected X-Not-Forwarded to not be forwarded, got=%q", got.notForward)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for upstream")
	}
}
