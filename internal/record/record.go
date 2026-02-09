package record

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
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
	path     string
	mu       sync.Mutex
	redactor *Redactor

	maxBytes int64
	maxFiles int
}

func NewRecorder(path string, redactor *Redactor, maxBytes int64, maxFiles int) *Recorder {
	if path == "" {
		return nil
	}
	return &Recorder{
		path:     path,
		redactor: redactor,
		maxBytes: maxBytes,
		maxFiles: maxFiles,
	}
}

func (r *Recorder) Append(signature string, request, response json.RawMessage) error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()

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

	// Rotate before opening the file so we never append to a file that should have
	// rolled over.
	if err := r.maybeRotate(int64(len(data) + 1)); err != nil {
		return err
	}

	file, err := os.OpenFile(r.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Write(append(data, '\n'))
	return err
}

func (r *Recorder) maybeRotate(nextWriteBytes int64) error {
	if r == nil || r.maxBytes <= 0 {
		return nil
	}
	if r.maxFiles < 0 {
		return errors.New("record max_files must be >= 0")
	}

	st, err := os.Stat(r.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	// If the file is empty, allow the write even if it exceeds maxBytes to avoid
	// rotating on every append for a single oversized entry.
	if st.Size() == 0 {
		return nil
	}
	if st.Size()+nextWriteBytes <= r.maxBytes {
		return nil
	}

	// No backups requested: delete the active file and start fresh.
	if r.maxFiles == 0 {
		if err := os.Remove(r.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}

	// Shift backups: path.(N-1)->path.N ... path.1->path.2, then path->path.1.
	last := fmt.Sprintf("%s.%d", r.path, r.maxFiles)
	if err := os.Remove(last); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for i := r.maxFiles - 1; i >= 1; i-- {
		src := fmt.Sprintf("%s.%d", r.path, i)
		dst := fmt.Sprintf("%s.%d", r.path, i+1)
		if err := renameReplace(src, dst); err != nil {
			return err
		}
	}
	if err := renameReplace(r.path, fmt.Sprintf("%s.%d", r.path, 1)); err != nil {
		return err
	}
	return nil
}

func renameReplace(src, dst string) error {
	if err := os.Remove(dst); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Rename(src, dst); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return nil
}

type ReplayStore struct {
	match       ReplayMatch
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
