package proxy

import (
  "bytes"
  "context"
  "encoding/json"
  "errors"
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

var errUpstreamResponseTooLarge = errors.New("upstream response too large")

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
  if r.URL.Path == "/healthz" {
    s.handleHealthz(w, r)
    return
  }

  if r.Method != http.MethodPost {
    http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
    return
  }
  if r.URL.Path != "/rpc" {
    http.Error(w, "not found", http.StatusNotFound)
    return
  }

  r.Body = http.MaxBytesReader(w, r.Body, s.maxBody)
  body, err := io.ReadAll(r.Body)
  if err != nil {
    var maxErr *http.MaxBytesError
    if errors.As(err, &maxErr) {
      s.writeJSONRPCErrorStatus(w, http.StatusRequestEntityTooLarge, json.RawMessage("null"), jsonrpc.ErrInvalidRequest, "request too large", nil)
      return
    }
    http.Error(w, "failed to read body", http.StatusBadRequest)
    return
  }
  trimmed := bytes.TrimSpace(body)
  if len(trimmed) == 0 {
    http.Error(w, "empty body", http.StatusBadRequest)
    return
  }

  if trimmed[0] == '[' {
    s.handleBatch(w, r, trimmed)
    return
  }
  s.handleSingle(w, r, trimmed)
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

func (s *Server) forward(ctx context.Context, body []byte) (json.RawMessage, int, error) {
  req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.upstream.String(), bytes.NewReader(body))
  if err != nil {
    return nil, http.StatusBadGateway, err
  }
  req.Header.Set("Content-Type", "application/json")
  resp, err := s.client.Do(req)
  if err != nil {
    return nil, http.StatusBadGateway, err
  }
  defer resp.Body.Close()
  respBody, err := io.ReadAll(io.LimitReader(resp.Body, s.maxBody+1))
  if err != nil {
    return nil, http.StatusBadGateway, err
  }
  if int64(len(respBody)) > s.maxBody {
    return nil, http.StatusBadGateway, errUpstreamResponseTooLarge
  }
  return json.RawMessage(respBody), resp.StatusCode, nil
}

func (s *Server) writeJSONRPCError(w http.ResponseWriter, id json.RawMessage, code int, message string, data any) {
  s.writeJSONRPCErrorStatus(w, http.StatusOK, id, code, message, data)
}

func (s *Server) writeJSONRPCErrorStatus(w http.ResponseWriter, status int, id json.RawMessage, code int, message string, data any) {
  resp := jsonrpc.ErrorResponse(id, code, message, data)
  payload, _ := json.Marshal(resp)
  s.writeRawJSON(w, status, payload)
}

func (s *Server) writeRawJSON(w http.ResponseWriter, status int, payload []byte) {
  w.Header().Set("Content-Type", "application/json")
  w.WriteHeader(status)
  _, _ = w.Write(payload)
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
  if r.Method != http.MethodGet && r.Method != http.MethodHead {
    http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
    return
  }
  w.Header().Set("Cache-Control", "no-store")
  if r.Method == http.MethodHead {
    w.WriteHeader(http.StatusOK)
    return
  }

  payload, _ := json.Marshal(map[string]any{
    "ok":                true,
    "upstream_configured": s.upstream != nil,
    "record_enabled":     s.recorder != nil,
    "replay_enabled":     s.replay != nil,
  })
  s.writeRawJSON(w, http.StatusOK, payload)
}

func (s *Server) handleSingle(w http.ResponseWriter, r *http.Request, body []byte) {
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
    if resp, ok := s.replay.Lookup(&req, sig); ok {
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

  upstreamResp, status, err := s.forward(r.Context(), body)
  if err != nil {
    if errors.Is(err, errUpstreamResponseTooLarge) {
      s.writeJSONRPCError(w, req.ID, jsonrpc.ErrServer, "upstream response too large", nil)
      return
    }
    s.writeJSONRPCError(w, req.ID, jsonrpc.ErrServer, "upstream error", nil)
    return
  }

  if err := s.recorder.Append(sig, json.RawMessage(body), upstreamResp); err != nil {
    s.logger.Printf("record append failed: %v", err)
  }

  s.writeRawJSON(w, status, upstreamResp)
}

func (s *Server) handleBatch(w http.ResponseWriter, r *http.Request, body []byte) {
  var items []json.RawMessage
  if err := json.Unmarshal(body, &items); err != nil {
    s.writeJSONRPCError(w, json.RawMessage("null"), jsonrpc.ErrInvalidRequest, "invalid JSON-RPC", nil)
    return
  }
  if len(items) == 0 {
    s.writeJSONRPCError(w, json.RawMessage("null"), jsonrpc.ErrInvalidRequest, "invalid JSON-RPC batch", nil)
    return
  }

  responses := make([]json.RawMessage, 0, len(items))
  for _, item := range items {
    itemTrimmed := bytes.TrimSpace(item)
    if len(itemTrimmed) == 0 {
      resp := jsonrpc.ErrorResponse(json.RawMessage("null"), jsonrpc.ErrInvalidRequest, "invalid JSON-RPC", nil)
      payload, _ := json.Marshal(resp)
      responses = append(responses, json.RawMessage(payload))
      continue
    }

    req := jsonrpc.Request{}
    if err := json.Unmarshal(itemTrimmed, &req); err != nil {
      resp := jsonrpc.ErrorResponse(json.RawMessage("null"), jsonrpc.ErrInvalidRequest, "invalid JSON-RPC", nil)
      payload, _ := json.Marshal(resp)
      responses = append(responses, json.RawMessage(payload))
      continue
    }

    if err := req.Validate(); err != nil {
      resp := jsonrpc.ErrorResponse(req.ID, jsonrpc.ErrInvalidRequest, err.Error(), nil)
      payload, _ := json.Marshal(resp)
      responses = append(responses, json.RawMessage(payload))
      continue
    }

    sig, err := signature.FromRequest(&req)
    if err != nil {
      resp := jsonrpc.ErrorResponse(req.ID, jsonrpc.ErrInvalidRequest, "unable to compute signature", nil)
      payload, _ := json.Marshal(resp)
      responses = append(responses, json.RawMessage(payload))
      continue
    }

    if s.replay != nil {
      if resp, ok := s.replay.Lookup(&req, sig); ok {
        if len(req.ID) > 0 {
          responses = append(responses, resp)
        }
        continue
      }
      if s.replayStrict && len(req.ID) > 0 {
        resp := jsonrpc.ErrorResponse(req.ID, jsonrpc.ErrServer, "replay miss", nil)
        payload, _ := json.Marshal(resp)
        responses = append(responses, json.RawMessage(payload))
        continue
      }
    }

    if req.Method == "tools/call" && s.validator != nil {
      tool, args, err := parseToolCall(req.Params)
      if err != nil {
        if len(req.ID) > 0 {
          resp := jsonrpc.ErrorResponse(req.ID, jsonrpc.ErrInvalidParams, "invalid tools/call params", nil)
          payload, _ := json.Marshal(resp)
          responses = append(responses, json.RawMessage(payload))
        }
        continue
      }
      decision, err := s.validator.ValidateToolCall(tool, args)
      if err != nil {
        if len(req.ID) > 0 {
          resp := jsonrpc.ErrorResponse(req.ID, jsonrpc.ErrServer, "validation error", nil)
          payload, _ := json.Marshal(resp)
          responses = append(responses, json.RawMessage(payload))
        }
        continue
      }
      if len(decision.Violations) > 0 && decision.Allowed {
        s.logger.Printf("validation audit: tool=%s violations=%v", tool, decision.Violations)
      }
      if !decision.Allowed {
        if len(req.ID) > 0 {
          resp := jsonrpc.ErrorResponse(req.ID, jsonrpc.ErrInvalidParams, "tool call rejected", decision.Violations)
          payload, _ := json.Marshal(resp)
          responses = append(responses, json.RawMessage(payload))
        }
        continue
      }
    }

    if s.upstream == nil {
      if len(req.ID) > 0 {
        resp := jsonrpc.ErrorResponse(req.ID, jsonrpc.ErrServer, "no upstream configured", nil)
        payload, _ := json.Marshal(resp)
        responses = append(responses, json.RawMessage(payload))
      }
      continue
    }

    upstreamResp, _, err := s.forward(r.Context(), itemTrimmed)
    if err != nil {
      if len(req.ID) > 0 {
        msg := "upstream error"
        if errors.Is(err, errUpstreamResponseTooLarge) {
          msg = "upstream response too large"
        }
        resp := jsonrpc.ErrorResponse(req.ID, jsonrpc.ErrServer, msg, nil)
        payload, _ := json.Marshal(resp)
        responses = append(responses, json.RawMessage(payload))
      }
      continue
    }

    if len(upstreamResp) > 0 {
      if err := s.recorder.Append(sig, json.RawMessage(itemTrimmed), upstreamResp); err != nil {
        s.logger.Printf("record append failed: %v", err)
      }
      if len(req.ID) > 0 {
        responses = append(responses, upstreamResp)
      }
    } else if len(req.ID) > 0 {
      resp := jsonrpc.ErrorResponse(req.ID, jsonrpc.ErrServer, "empty upstream response", nil)
      payload, _ := json.Marshal(resp)
      responses = append(responses, json.RawMessage(payload))
    }
  }

  if len(responses) == 0 {
    w.WriteHeader(http.StatusNoContent)
    return
  }
  payload, _ := json.Marshal(responses)
  s.writeRawJSON(w, http.StatusOK, payload)
}
