package record

import (
  "bytes"
  "encoding/json"
  "os"
  "testing"

  "github.com/sarveshkapre/mcp-proxy-gateway/internal/jsonrpc"
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

  store, err := LoadReplay(file.Name(), ReplayMatchSignature)
  if err != nil {
    t.Fatalf("load replay: %v", err)
  }
  reqObj := jsonrpc.Request{}
  if err := json.Unmarshal(entry.Request, &reqObj); err != nil {
    t.Fatalf("unmarshal request: %v", err)
  }
  got, ok := store.Lookup(&reqObj, "abc123")
  if !ok {
    t.Fatalf("expected replay hit")
  }
  if string(got) == "" {
    t.Fatalf("expected response")
  }
}

func TestReplayStoreLargeLine(t *testing.T) {
  file, err := os.CreateTemp(t.TempDir(), "records-*.ndjson")
  if err != nil {
    t.Fatalf("temp file: %v", err)
  }
  defer file.Close()

  large := make([]byte, 200*1024)
  for i := range large {
    large[i] = 'a'
  }

  entry := Entry{
    Time:      "2024-01-01T00:00:00Z",
    Signature: "bigline",
    Request:   json.RawMessage(`{"jsonrpc":"2.0"}`),
    Response:  json.RawMessage(append([]byte(`{"jsonrpc":"2.0","result":"`), append(large, []byte(`"}`)...)...)),
  }
  data, err := json.Marshal(entry)
  if err != nil {
    t.Fatalf("marshal: %v", err)
  }
  if _, err := file.Write(append(data, '\n')); err != nil {
    t.Fatalf("write: %v", err)
  }

  store, err := LoadReplay(file.Name(), ReplayMatchSignature)
  if err != nil {
    t.Fatalf("load replay: %v", err)
  }
  reqObj := jsonrpc.Request{}
  if err := json.Unmarshal(entry.Request, &reqObj); err != nil {
    t.Fatalf("unmarshal request: %v", err)
  }
  if _, ok := store.Lookup(&reqObj, "bigline"); !ok {
    t.Fatalf("expected replay hit")
  }
}

func TestReplayMatchByMethod(t *testing.T) {
  file, err := os.CreateTemp(t.TempDir(), "records-*.ndjson")
  if err != nil {
    t.Fatalf("temp file: %v", err)
  }
  defer file.Close()

  req := json.RawMessage(`{"jsonrpc":"2.0","id":1,"method":"ping"}`)
  entry := Entry{
    Time:      "2024-01-01T00:00:00Z",
    Signature: "abc123",
    Request:   req,
    Response:  json.RawMessage(`{"jsonrpc":"2.0","id":1,"result":{"ok":true}}`),
  }
  data, err := json.Marshal(entry)
  if err != nil {
    t.Fatalf("marshal: %v", err)
  }
  if _, err := file.Write(append(data, '\n')); err != nil {
    t.Fatalf("write: %v", err)
  }

  store, err := LoadReplay(file.Name(), ReplayMatchMethod)
  if err != nil {
    t.Fatalf("load replay: %v", err)
  }
  reqObj := jsonrpc.Request{}
  if err := json.Unmarshal(req, &reqObj); err != nil {
    t.Fatalf("unmarshal request: %v", err)
  }
  got, ok := store.Lookup(&reqObj, "")
  if !ok || len(got) == 0 {
    t.Fatalf("expected replay hit by method")
  }
}

func TestReplayMatchByTool(t *testing.T) {
  file, err := os.CreateTemp(t.TempDir(), "records-*.ndjson")
  if err != nil {
    t.Fatalf("temp file: %v", err)
  }
  defer file.Close()

  req := json.RawMessage(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"tool":"web.search","arguments":{"query":"hi"}}}`)
  entry := Entry{
    Time:      "2024-01-01T00:00:00Z",
    Signature: "def456",
    Request:   req,
    Response:  json.RawMessage(`{"jsonrpc":"2.0","id":1,"result":{"ok":true}}`),
  }
  data, err := json.Marshal(entry)
  if err != nil {
    t.Fatalf("marshal: %v", err)
  }
  if _, err := file.Write(append(data, '\n')); err != nil {
    t.Fatalf("write: %v", err)
  }

  store, err := LoadReplay(file.Name(), ReplayMatchTool)
  if err != nil {
    t.Fatalf("load replay: %v", err)
  }
  reqObj := jsonrpc.Request{}
  if err := json.Unmarshal(req, &reqObj); err != nil {
    t.Fatalf("unmarshal request: %v", err)
  }
  got, ok := store.Lookup(&reqObj, "")
  if !ok || len(got) == 0 {
    t.Fatalf("expected replay hit by tool")
  }
}

func TestRecorder_RedactsBeforeWriting(t *testing.T) {
  file, err := os.CreateTemp(t.TempDir(), "records-*.ndjson")
  if err != nil {
    t.Fatalf("temp file: %v", err)
  }
  _ = file.Close()

  redactor, err := NewRedactor([]string{"token"}, nil)
  if err != nil {
    t.Fatalf("new redactor: %v", err)
  }

  rec := NewRecorder(file.Name(), redactor)
  if err := rec.Append("sig", json.RawMessage(`{"token":"abc"}`), json.RawMessage(`{"ok":true,"token":"def"}`)); err != nil {
    t.Fatalf("append: %v", err)
  }

  data, err := os.ReadFile(file.Name())
  if err != nil {
    t.Fatalf("read: %v", err)
  }
  if !bytes.Contains(data, []byte(`"token":"[REDACTED]"`)) {
    t.Fatalf("expected redaction in file, got=%s", string(data))
  }
  if bytes.Contains(data, []byte(`"token":"abc"`)) || bytes.Contains(data, []byte(`"token":"def"`)) {
    t.Fatalf("expected secrets to be removed, got=%s", string(data))
  }
}
