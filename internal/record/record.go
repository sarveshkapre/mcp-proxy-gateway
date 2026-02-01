package record

import (
  "bufio"
  "encoding/json"
  "os"
  "sync"
  "time"
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
}

func NewRecorder(path string) *Recorder {
  if path == "" {
    return nil
  }
  return &Recorder{path: path}
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

  entry := Entry{
    Time:      time.Now().UTC().Format(time.RFC3339Nano),
    Signature: signature,
    Request:   request,
    Response:  response,
  }
  data, err := json.Marshal(entry)
  if err != nil {
    return err
  }
  _, err = file.Write(append(data, '\n'))
  return err
}

type ReplayStore struct {
  entries map[string]json.RawMessage
}

func LoadReplay(path string) (*ReplayStore, error) {
  if path == "" {
    return nil, nil
  }
  file, err := os.Open(path)
  if err != nil {
    return nil, err
  }
  defer file.Close()

  store := &ReplayStore{entries: map[string]json.RawMessage{}}
  scanner := bufio.NewScanner(file)
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
    store.entries[entry.Signature] = entry.Response
  }
  if err := scanner.Err(); err != nil {
    return nil, err
  }
  return store, nil
}

func (r *ReplayStore) Lookup(signature string) (json.RawMessage, bool) {
  if r == nil {
    return nil, false
  }
  resp, ok := r.entries[signature]
  return resp, ok
}
