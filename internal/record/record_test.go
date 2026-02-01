package record

import (
  "encoding/json"
  "os"
  "testing"
)

func TestReplayStore(t *testing.T) {
  file, err := os.CreateTemp(t.TempDir(), "records-*.ndjson")
  if err != nil {
    t.Fatalf("temp file: %v", err)
  }
  defer file.Close()

  entry := Entry{
    Time:      "2024-01-01T00:00:00Z",
    Signature: "abc123",
    Request:   json.RawMessage(`{"jsonrpc":"2.0"}`),
    Response:  json.RawMessage(`{"jsonrpc":"2.0","result":{"ok":true}}`),
  }
  data, err := json.Marshal(entry)
  if err != nil {
    t.Fatalf("marshal: %v", err)
  }
  if _, err := file.Write(append(data, '\n')); err != nil {
    t.Fatalf("write: %v", err)
  }

  store, err := LoadReplay(file.Name())
  if err != nil {
    t.Fatalf("load replay: %v", err)
  }
  got, ok := store.Lookup("abc123")
  if !ok {
    t.Fatalf("expected replay hit")
  }
  if string(got) == "" {
    t.Fatalf("expected response")
  }
}
