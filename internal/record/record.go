package record

import (
  "bufio"
  "encoding/json"
  "errors"
  "os"
  "sync"
  "time"

  "github.com/sarveshkapre/mcp-proxy-gateway/internal/jsonrpc"
)

type Entry struct {
  Time      string          `json:"time"`
  Signature string          `json:"signature"`
  Request   json.RawMessage `json:"request"`
  Response  json.RawMessage `json:"response"`
}

type Recorder struct {
  path string
  mu   sync.Mutex
  redactor *Redactor
}

func NewRecorder(path string, redactor *Redactor) *Recorder {
  if path == "" {
    return nil
  }
  return &Recorder{path: path, redactor: redactor}
}

func (r *Recorder) Append(signature string, request, response json.RawMessage) error {
  if r == nil {
    return nil
  }
  r.mu.Lock()
  defer r.mu.Unlock()

  file, err := os.OpenFile(r.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
  if err != nil {
    return err
  }
  defer file.Close()

  req := request
  resp := response
  if r.redactor != nil {
    var err error
    req, err = r.redactor.Apply(request)
    if err != nil {
      return err
    }
    resp, err = r.redactor.Apply(response)
    if err != nil {
      return err
    }
  }

  entry := Entry{
    Time:      time.Now().UTC().Format(time.RFC3339Nano),
    Signature: signature,
    Request:   req,
    Response:  resp,
  }
  data, err := json.Marshal(entry)
  if err != nil {
    return err
  }
  _, err = file.Write(append(data, '\n'))
  return err
}

type ReplayStore struct {
  match      ReplayMatch
  bySignature map[string]json.RawMessage
  byMethod    map[string]json.RawMessage
  byTool      map[string]json.RawMessage
}

type ReplayMatch string

const (
  ReplayMatchSignature ReplayMatch = "signature"
  ReplayMatchMethod    ReplayMatch = "method"
  ReplayMatchTool      ReplayMatch = "tool"
)

func LoadReplay(path string, match ReplayMatch) (*ReplayStore, error) {
  if path == "" {
    return nil, nil
  }
  if match == "" {
    match = ReplayMatchSignature
  }
  file, err := os.Open(path)
  if err != nil {
    return nil, err
  }
  defer file.Close()

  store := &ReplayStore{
    match:       match,
    bySignature: map[string]json.RawMessage{},
    byMethod:    map[string]json.RawMessage{},
    byTool:      map[string]json.RawMessage{},
  }
  scanner := bufio.NewScanner(file)
  // Entries can be large (request + response bodies). Increase the scanner limit
  // to avoid failing on valid recordings.
  scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
  for scanner.Scan() {
    line := scanner.Bytes()
    if len(line) == 0 {
      continue
    }
    entry := Entry{}
    if err := json.Unmarshal(line, &entry); err != nil {
      return nil, err
    }
    if entry.Signature == "" || len(entry.Response) == 0 {
      continue
    }
    if _, exists := store.bySignature[entry.Signature]; !exists {
      store.bySignature[entry.Signature] = entry.Response
    }

    if len(entry.Request) == 0 {
      continue
    }
    req := jsonrpc.Request{}
    if err := json.Unmarshal(entry.Request, &req); err != nil {
      continue
    }
    if req.Method != "" {
      if _, exists := store.byMethod[req.Method]; !exists {
        store.byMethod[req.Method] = entry.Response
      }
    }
    if req.Method == "tools/call" {
      tool, err := extractToolName(req.Params)
      if err == nil && tool != "" {
        if _, exists := store.byTool[tool]; !exists {
          store.byTool[tool] = entry.Response
        }
      }
    }
  }
  if err := scanner.Err(); err != nil {
    return nil, err
  }
  return store, nil
}

func (r *ReplayStore) Lookup(req *jsonrpc.Request, signature string) (json.RawMessage, bool) {
  if r == nil {
    return nil, false
  }
  switch r.match {
  case ReplayMatchMethod:
    if req == nil || req.Method == "" {
      return nil, false
    }
    resp, ok := r.byMethod[req.Method]
    return resp, ok
  case ReplayMatchTool:
    if req == nil || req.Method != "tools/call" {
      return nil, false
    }
    tool, err := extractToolName(req.Params)
    if err != nil || tool == "" {
      return nil, false
    }
    resp, ok := r.byTool[tool]
    return resp, ok
  default:
    if signature == "" {
      return nil, false
    }
    resp, ok := r.bySignature[signature]
    return resp, ok
  }
}

func extractToolName(params json.RawMessage) (string, error) {
  if len(params) == 0 {
    return "", errors.New("missing params")
  }
  var data struct {
    Tool string `json:"tool"`
  }
  if err := json.Unmarshal(params, &data); err != nil {
    return "", err
  }
  if data.Tool == "" {
    return "", errors.New("missing tool")
  }
  return data.Tool, nil
}
