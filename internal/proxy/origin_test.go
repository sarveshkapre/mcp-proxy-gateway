package proxy

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestOriginAllowlistRejectsUnexpectedOrigin(t *testing.T) {
	t.Parallel()

	srv := NewServer(nil, nil, nil, nil, false, []string{"http://allowed.local"}, nil, 1024, time.Second, nil)

	reqBody := []byte(`{"jsonrpc":"2.0","id":1,"method":"ping","params":{}}`)
	r := httptest.NewRequest(http.MethodPost, "/rpc", bytes.NewReader(reqBody))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", "http://evil.local")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestOriginAllowlistAllowsConfiguredOrigin(t *testing.T) {
	t.Parallel()

	srv := NewServer(nil, nil, nil, nil, false, []string{"http://allowed.local"}, nil, 1024, time.Second, nil)

	reqBody := []byte(`{"jsonrpc":"2.0","id":1,"method":"ping","params":{}}`)
	r := httptest.NewRequest(http.MethodPost, "/rpc", bytes.NewReader(reqBody))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", "http://allowed.local")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, r)

	// No upstream configured should still produce a JSON-RPC error (origin check passed).
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if got := w.Body.String(); got == "" {
		t.Fatalf("expected json-rpc error body, got empty")
	}
}
