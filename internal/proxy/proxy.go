package proxy

import (
  "bytes"
  "encoding/json"
  "io"
  "log"
  "net/http"
  "net/url"
  "time"

  "github.com/sarveshkapre/mcp-proxy-gateway/internal/jsonrpc"
  "github.com/sarveshkapre/mcp-proxy-gateway/internal/record"
  "github.com/sarveshkapre/mcp-proxy-gateway/internal/signature"
  "github.com/sarveshkapre/mcp-proxy-gateway/internal/validate"
)

type Server struct {
  upstream     *url.URL
  client       *http.Client
  validator    *validate.Validator
  recorder     *record.Recorder
  replay       *record.ReplayStore
  replayStrict bool
  maxBody      int64
  logger       *log.Logger
}

func NewServer(upstream *url.URL, validator *validate.Validator, recorder *record.Recorder, replay *record.ReplayStore, replayStrict bool, maxBody int64, timeout time.Duration, logger *log.Logger) *Server {
  if maxBody <= 0 {
    maxBody = 1 << 20
  }
  if logger == nil {
    logger = log.Default()
  }
  return &Server{
    upstream:     upstream,
    validator:    validator,
    recorder:     recorder,
    replay:       replay,
    replayStrict: replayStrict,
    maxBody:      maxBody,
    logger:       logger,
    client: &http.Client{
      Timeout: timeout,
    },
  }
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
  if r.Method != http.MethodPost {
    http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
    return
  }
  if r.URL.Path != "/rpc" {
    http.Error(w, "not found", http.StatusNotFound)
    return
  }

  body, err := io.ReadAll(io.LimitReader(r.Body, s.maxBody))
  if err != nil {
    http.Error(w, "failed to read body", http.StatusBadRequest)
    return
  }
  if len(body) == 0 {
    http.Error(w, "empty body", http.StatusBadRequest)
    return
  }

  req := jsonrpc.Request{}
  if err := json.Unmarshal(body, &req); err != nil {
    s.writeJSONRPCError(w, json.RawMessage("null"), jsonrpc.ErrInvalidRequest, "invalid JSON-RPC", nil)
    return
  }
  if err := req.Validate(); err != nil {
    s.writeJSONRPCError(w, req.ID, jsonrpc.ErrInvalidRequest, err.Error(), nil)
    return
  }

  sig, err := signature.FromRequest(&req)
  if err != nil {
    s.writeJSONRPCError(w, req.ID, jsonrpc.ErrInvalidRequest, "unable to compute signature", nil)
    return
  }

  if s.replay != nil {
    if resp, ok := s.replay.Lookup(sig); ok {
      s.writeRawJSON(w, http.StatusOK, resp)
      return
    }
    if s.replayStrict {
      s.writeJSONRPCError(w, req.ID, jsonrpc.ErrServer, "replay miss", nil)
      return
    }
  }

  if req.Method == "tools/call" && s.validator != nil {
    tool, args, err := parseToolCall(req.Params)
    if err != nil {
      s.writeJSONRPCError(w, req.ID, jsonrpc.ErrInvalidParams, "invalid tools/call params", nil)
      return
    }
    decision, err := s.validator.ValidateToolCall(tool, args)
    if err != nil {
      s.writeJSONRPCError(w, req.ID, jsonrpc.ErrServer, "validation error", nil)
      return
    }
    if len(decision.Violations) > 0 && decision.Allowed {
      s.logger.Printf("validation audit: tool=%s violations=%v", tool, decision.Violations)
    }
    if !decision.Allowed {
      s.writeJSONRPCError(w, req.ID, jsonrpc.ErrInvalidParams, "tool call rejected", decision.Violations)
      return
    }
  }

  if s.upstream == nil {
    s.writeJSONRPCError(w, req.ID, jsonrpc.ErrServer, "no upstream configured", nil)
    return
  }

  upstreamResp, status, err := s.forward(body)
  if err != nil {
    s.writeJSONRPCError(w, req.ID, jsonrpc.ErrServer, "upstream error", nil)
    return
  }

  if err := s.recorder.Append(sig, body, upstreamResp); err != nil {
    s.logger.Printf("record append failed: %v", err)
  }

  s.writeRawJSON(w, status, upstreamResp)
}

func parseToolCall(raw json.RawMessage) (string, json.RawMessage, error) {
  if len(raw) == 0 {
    return "", nil, io.ErrUnexpectedEOF
  }
  params := struct {
    Tool      string          `json:"tool"`
    Arguments json.RawMessage `json:"arguments"`
  }{}
  if err := json.Unmarshal(raw, &params); err != nil {
    return "", nil, err
  }
  if params.Tool == "" {
    return "", nil, io.ErrUnexpectedEOF
  }
  return params.Tool, params.Arguments, nil
}

func (s *Server) forward(body []byte) (json.RawMessage, int, error) {
  req, err := http.NewRequest(http.MethodPost, s.upstream.String(), bytes.NewReader(body))
  if err != nil {
    return nil, http.StatusBadGateway, err
  }
  req.Header.Set("Content-Type", "application/json")
  resp, err := s.client.Do(req)
  if err != nil {
    return nil, http.StatusBadGateway, err
  }
  defer resp.Body.Close()
  respBody, err := io.ReadAll(io.LimitReader(resp.Body, s.maxBody))
  if err != nil {
    return nil, http.StatusBadGateway, err
  }
  return json.RawMessage(respBody), resp.StatusCode, nil
}

func (s *Server) writeJSONRPCError(w http.ResponseWriter, id json.RawMessage, code int, message string, data any) {
  resp := jsonrpc.ErrorResponse(id, code, message, data)
  payload, _ := json.Marshal(resp)
  s.writeRawJSON(w, http.StatusOK, payload)
}

func (s *Server) writeRawJSON(w http.ResponseWriter, status int, payload []byte) {
  w.Header().Set("Content-Type", "application/json")
  w.WriteHeader(status)
  _, _ = w.Write(payload)
}
