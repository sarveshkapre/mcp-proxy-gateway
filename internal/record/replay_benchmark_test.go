package record

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/sarveshkapre/mcp-proxy-gateway/internal/jsonrpc"
)

func BenchmarkReplayLookupSignature(b *testing.B) {
	store := &ReplayStore{
		match:       ReplayMatchSignature,
		bySignature: map[string]json.RawMessage{},
		byMethod:    map[string]json.RawMessage{},
		byTool:      map[string]json.RawMessage{},
	}
	for i := 0; i < 50_000; i++ {
		store.bySignature[fmt.Sprintf("sig-%d", i)] = json.RawMessage(`{"jsonrpc":"2.0","id":1,"result":{"ok":true}}`)
	}
	req := &jsonrpc.Request{Method: "ping"}
	sig := "sig-4242"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, ok := store.Lookup(req, sig); !ok {
			b.Fatalf("unexpected miss")
		}
	}
}

func BenchmarkReplayLookupMethod(b *testing.B) {
	store := &ReplayStore{
		match:       ReplayMatchMethod,
		bySignature: map[string]json.RawMessage{},
		byMethod:    map[string]json.RawMessage{},
		byTool:      map[string]json.RawMessage{},
	}
	for i := 0; i < 10_000; i++ {
		store.byMethod[fmt.Sprintf("m-%d", i)] = json.RawMessage(`{"jsonrpc":"2.0","id":1,"result":{"ok":true}}`)
	}
	req := &jsonrpc.Request{Method: "m-4242"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, ok := store.Lookup(req, ""); !ok {
			b.Fatalf("unexpected miss")
		}
	}
}

func BenchmarkReplayLookupTool(b *testing.B) {
	store := &ReplayStore{
		match:       ReplayMatchTool,
		bySignature: map[string]json.RawMessage{},
		byMethod:    map[string]json.RawMessage{},
		byTool:      map[string]json.RawMessage{},
	}
	for i := 0; i < 10_000; i++ {
		store.byTool[fmt.Sprintf("tool-%d", i)] = json.RawMessage(`{"jsonrpc":"2.0","id":1,"result":{"ok":true}}`)
	}
	req := &jsonrpc.Request{
		Method: "tools/call",
		Params: json.RawMessage(`{"tool":"tool-4242","arguments":{"query":"hi"}}`),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, ok := store.Lookup(req, ""); !ok {
			b.Fatalf("unexpected miss")
		}
	}
}
