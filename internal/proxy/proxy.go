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
	"strings"
	"sync/atomic"
	"time"

	"github.com/sarveshkapre/mcp-proxy-gateway/internal/jsonrpc"
	"github.com/sarveshkapre/mcp-proxy-gateway/internal/record"
	"github.com/sarveshkapre/mcp-proxy-gateway/internal/signature"
	"github.com/sarveshkapre/mcp-proxy-gateway/internal/validate"
)

var errUpstreamResponseTooLarge = errors.New("upstream response too large")

type Server struct {
	upstream        *url.URL
	client          *http.Client
	validator       *validate.Validator
	recorder        *record.Recorder
	replay          *record.ReplayStore
	replayStrict    bool
	maxBody         int64
	originAllowlist map[string]struct{}
	logger          *log.Logger
	metrics         *proxyMetrics
}

type proxyMetrics struct {
	requestsTotal          atomic.Uint64
	batchItemsTotal        atomic.Uint64
	replayHitsTotal        atomic.Uint64
	replayMissesTotal      atomic.Uint64
	validationRejectsTotal atomic.Uint64
	upstreamErrorsTotal    atomic.Uint64
	latencyLE5ms           atomic.Uint64
	latencyLE20ms          atomic.Uint64
	latencyLE100ms         atomic.Uint64
	latencyLE500ms         atomic.Uint64
	latencyLE1000ms        atomic.Uint64
	latencyGT1000ms        atomic.Uint64
}

func newProxyMetrics() *proxyMetrics {
	return &proxyMetrics{}
}

func (m *proxyMetrics) incRequests() {
	if m == nil {
		return
	}
	m.requestsTotal.Add(1)
}

func (m *proxyMetrics) addBatchItems(n int) {
	if m == nil || n <= 0 {
		return
	}
	m.batchItemsTotal.Add(uint64(n))
}

func (m *proxyMetrics) incReplayHit() {
	if m == nil {
		return
	}
	m.replayHitsTotal.Add(1)
}

func (m *proxyMetrics) incReplayMiss() {
	if m == nil {
		return
	}
	m.replayMissesTotal.Add(1)
}

func (m *proxyMetrics) incValidationReject() {
	if m == nil {
		return
	}
	m.validationRejectsTotal.Add(1)
}

func (m *proxyMetrics) incUpstreamError() {
	if m == nil {
		return
	}
	m.upstreamErrorsTotal.Add(1)
}

func (m *proxyMetrics) observeLatency(d time.Duration) {
	if m == nil {
		return
	}
	ms := d.Milliseconds()
	switch {
	case ms <= 5:
		m.latencyLE5ms.Add(1)
	case ms <= 20:
		m.latencyLE20ms.Add(1)
	case ms <= 100:
		m.latencyLE100ms.Add(1)
	case ms <= 500:
		m.latencyLE500ms.Add(1)
	case ms <= 1000:
		m.latencyLE1000ms.Add(1)
	default:
		m.latencyGT1000ms.Add(1)
	}
}

func (m *proxyMetrics) snapshot() map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return map[string]any{
		"requests_total":           m.requestsTotal.Load(),
		"batch_items_total":        m.batchItemsTotal.Load(),
		"replay_hits_total":        m.replayHitsTotal.Load(),
		"replay_misses_total":      m.replayMissesTotal.Load(),
		"validation_rejects_total": m.validationRejectsTotal.Load(),
		"upstream_errors_total":    m.upstreamErrorsTotal.Load(),
		"latency_buckets_ms": map[string]uint64{
			"le_5":    m.latencyLE5ms.Load(),
			"le_20":   m.latencyLE20ms.Load(),
			"le_100":  m.latencyLE100ms.Load(),
			"le_500":  m.latencyLE500ms.Load(),
			"le_1000": m.latencyLE1000ms.Load(),
			"gt_1000": m.latencyGT1000ms.Load(),
			"total":   m.latencyLE5ms.Load() + m.latencyLE20ms.Load() + m.latencyLE100ms.Load() + m.latencyLE500ms.Load() + m.latencyLE1000ms.Load() + m.latencyGT1000ms.Load(),
		},
	}
}

func NewServer(upstream *url.URL, validator *validate.Validator, recorder *record.Recorder, replay *record.ReplayStore, replayStrict bool, originAllowlist []string, maxBody int64, timeout time.Duration, logger *log.Logger) *Server {
	if maxBody <= 0 {
		maxBody = 1 << 20
	}
	if logger == nil {
		logger = log.Default()
	}

	var originAllow map[string]struct{}
	for _, origin := range originAllowlist {
		origin = strings.TrimSpace(origin)
		if origin == "" {
			continue
		}
		if originAllow == nil {
			originAllow = map[string]struct{}{}
		}
		originAllow[origin] = struct{}{}
	}
	return &Server{
		upstream:        upstream,
		validator:       validator,
		recorder:        recorder,
		replay:          replay,
		replayStrict:    replayStrict,
		maxBody:         maxBody,
		originAllowlist: originAllow,
		logger:          logger,
		metrics:         newProxyMetrics(),
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
	if r.URL.Path == "/metricsz" {
		s.handleMetricsz(w, r)
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
	if origin := r.Header.Get("Origin"); origin != "" && len(s.originAllowlist) > 0 {
		if _, ok := s.originAllowlist[origin]; !ok {
			http.Error(w, "origin not allowed", http.StatusForbidden)
			return
		}
	}
	s.metrics.incRequests()

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

func isNotification(req *jsonrpc.Request) bool {
	if req == nil {
		return false
	}
	return len(req.ID) == 0
}

func withResponseID(rawResp, id json.RawMessage) (json.RawMessage, error) {
	if len(id) == 0 {
		return rawResp, nil
	}
	payload := map[string]json.RawMessage{}
	if err := json.Unmarshal(rawResp, &payload); err != nil {
		return nil, err
	}
	payload["id"] = id
	out, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(out), nil
}

func wantsEventStream(r *http.Request) bool {
	if r == nil {
		return false
	}
	accept := r.Header.Get("Accept")
	return strings.Contains(strings.ToLower(accept), "text/event-stream")
}

func isEventStreamContentType(ct string) bool {
	if ct == "" {
		return false
	}
	ct = strings.ToLower(strings.TrimSpace(ct))
	if idx := strings.Index(ct, ";"); idx >= 0 {
		ct = strings.TrimSpace(ct[:idx])
	}
	return ct == "text/event-stream"
}

func (s *Server) doUpstream(ctx context.Context, in *http.Request, body []byte, includeAccept bool) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.upstream.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	// Only forward a small allowlist of headers to avoid becoming an implicit
	// generic HTTP proxy. Add more on demand with explicit documentation.
	if in != nil {
		if v := in.Header.Get("Authorization"); v != "" {
			req.Header.Set("Authorization", v)
		}
		if includeAccept {
			if v := in.Header.Get("Accept"); v != "" {
				req.Header.Set("Accept", v)
			}
		}
	}

	return s.client.Do(req)
}

func (s *Server) readUpstreamJSON(resp *http.Response) (json.RawMessage, error) {
	if resp == nil || resp.Body == nil {
		return nil, errors.New("missing upstream response body")
	}
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, s.maxBody+1))
	if err != nil {
		return nil, err
	}
	if int64(len(respBody)) > s.maxBody {
		return nil, errUpstreamResponseTooLarge
	}
	return json.RawMessage(respBody), nil
}

type flushingResponseWriter struct {
	w http.ResponseWriter
}

func (f flushingResponseWriter) Write(p []byte) (int, error) {
	n, err := f.w.Write(p)
	if fl, ok := f.w.(http.Flusher); ok {
		fl.Flush()
	}
	return n, err
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
		"ok":                  true,
		"upstream_configured": s.upstream != nil,
		"record_enabled":      s.recorder != nil,
		"replay_enabled":      s.replay != nil,
	})
	s.writeRawJSON(w, http.StatusOK, payload)
}

func (s *Server) handleMetricsz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	payload, _ := json.Marshal(s.metrics.snapshot())
	s.writeRawJSON(w, http.StatusOK, payload)
}

func (s *Server) handleSingle(w http.ResponseWriter, r *http.Request, body []byte) {
	start := time.Now()
	defer func() {
		s.metrics.observeLatency(time.Since(start))
	}()

	req := jsonrpc.Request{}
	if err := json.Unmarshal(body, &req); err != nil {
		s.writeJSONRPCError(w, json.RawMessage("null"), jsonrpc.ErrInvalidRequest, "invalid JSON-RPC", nil)
		return
	}
	if err := req.Validate(); err != nil {
		s.writeJSONRPCError(w, req.ID, jsonrpc.ErrInvalidRequest, err.Error(), nil)
		return
	}
	notification := isNotification(&req)

	sig, err := signature.FromRequest(&req)
	if err != nil {
		s.writeJSONRPCError(w, req.ID, jsonrpc.ErrInvalidRequest, "unable to compute signature", nil)
		return
	}

	if s.replay != nil {
		if resp, ok := s.replay.Lookup(&req, sig); ok {
			s.metrics.incReplayHit()
			if notification {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			replayResp, err := withResponseID(resp, req.ID)
			if err != nil {
				s.writeJSONRPCError(w, req.ID, jsonrpc.ErrServer, "invalid replay response", nil)
				return
			}
			s.writeRawJSON(w, http.StatusOK, replayResp)
			return
		}
		s.metrics.incReplayMiss()
		if s.replayStrict {
			if notification {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			s.writeJSONRPCError(w, req.ID, jsonrpc.ErrServer, "replay miss", nil)
			return
		}
	}

	if req.Method == "tools/call" && s.validator != nil {
		tool, args, err := parseToolCall(req.Params)
		if err != nil {
			s.metrics.incValidationReject()
			if notification {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			s.writeJSONRPCError(w, req.ID, jsonrpc.ErrInvalidParams, "invalid tools/call params", nil)
			return
		}
		decision, err := s.validator.ValidateToolCall(tool, args)
		if err != nil {
			if notification {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			s.writeJSONRPCError(w, req.ID, jsonrpc.ErrServer, "validation error", nil)
			return
		}
		if len(decision.Violations) > 0 && decision.Allowed {
			s.logger.Printf("validation audit: tool=%s violations=%v", tool, decision.Violations)
		}
		if !decision.Allowed {
			s.metrics.incValidationReject()
			if notification {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			s.writeJSONRPCError(w, req.ID, jsonrpc.ErrInvalidParams, "tool call rejected", decision.Violations)
			return
		}
	}

	if s.upstream == nil {
		if notification {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		s.writeJSONRPCError(w, req.ID, jsonrpc.ErrServer, "no upstream configured", nil)
		return
	}

	upstreamHTTPResp, err := s.doUpstream(r.Context(), r, body, wantsEventStream(r))
	if err != nil {
		s.metrics.incUpstreamError()
		if notification {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		s.writeJSONRPCError(w, req.ID, jsonrpc.ErrServer, "upstream error", nil)
		return
	}
	defer upstreamHTTPResp.Body.Close()

	// If the upstream chooses SSE (or other streaming) we pass it through as-is.
	if isEventStreamContentType(upstreamHTTPResp.Header.Get("Content-Type")) {
		if notification {
			// Notifications never return a response body.
			w.WriteHeader(http.StatusNoContent)
			return
		}
		// Recording/replay of streamed responses is intentionally skipped.
		ct := upstreamHTTPResp.Header.Get("Content-Type")
		if ct == "" {
			ct = "text/event-stream"
		}
		w.Header().Set("Content-Type", ct)
		if v := upstreamHTTPResp.Header.Get("Cache-Control"); v != "" {
			w.Header().Set("Cache-Control", v)
		} else {
			w.Header().Set("Cache-Control", "no-store")
		}
		w.WriteHeader(upstreamHTTPResp.StatusCode)

		n, copyErr := io.Copy(flushingResponseWriter{w: w}, io.LimitReader(upstreamHTTPResp.Body, s.maxBody+1))
		if copyErr != nil {
			s.metrics.incUpstreamError()
			s.logger.Printf("upstream stream copy failed: %v", copyErr)
		}
		if n > s.maxBody {
			s.metrics.incUpstreamError()
			s.logger.Printf("upstream stream truncated at max-body=%d bytes", s.maxBody)
		}
		return
	}

	upstreamResp, err := s.readUpstreamJSON(upstreamHTTPResp)
	status := upstreamHTTPResp.StatusCode
	if err != nil {
		s.metrics.incUpstreamError()
		if notification {
			w.WriteHeader(http.StatusNoContent)
			return
		}
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

	if notification {
		w.WriteHeader(http.StatusNoContent)
		return
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
	s.metrics.addBatchItems(len(items))

	responses := make([]json.RawMessage, 0, len(items))
	for _, item := range items {
		func(rawItem json.RawMessage) {
			itemStart := time.Now()
			defer func() {
				s.metrics.observeLatency(time.Since(itemStart))
			}()

			itemTrimmed := bytes.TrimSpace(rawItem)
			if len(itemTrimmed) == 0 {
				resp := jsonrpc.ErrorResponse(json.RawMessage("null"), jsonrpc.ErrInvalidRequest, "invalid JSON-RPC", nil)
				payload, _ := json.Marshal(resp)
				responses = append(responses, json.RawMessage(payload))
				return
			}

			req := jsonrpc.Request{}
			if err := json.Unmarshal(itemTrimmed, &req); err != nil {
				resp := jsonrpc.ErrorResponse(json.RawMessage("null"), jsonrpc.ErrInvalidRequest, "invalid JSON-RPC", nil)
				payload, _ := json.Marshal(resp)
				responses = append(responses, json.RawMessage(payload))
				return
			}

			if err := req.Validate(); err != nil {
				resp := jsonrpc.ErrorResponse(req.ID, jsonrpc.ErrInvalidRequest, err.Error(), nil)
				payload, _ := json.Marshal(resp)
				responses = append(responses, json.RawMessage(payload))
				return
			}

			sig, err := signature.FromRequest(&req)
			if err != nil {
				resp := jsonrpc.ErrorResponse(req.ID, jsonrpc.ErrInvalidRequest, "unable to compute signature", nil)
				payload, _ := json.Marshal(resp)
				responses = append(responses, json.RawMessage(payload))
				return
			}

			if s.replay != nil {
				if resp, ok := s.replay.Lookup(&req, sig); ok {
					s.metrics.incReplayHit()
					if len(req.ID) > 0 {
						replayResp, err := withResponseID(resp, req.ID)
						if err != nil {
							resp := jsonrpc.ErrorResponse(req.ID, jsonrpc.ErrServer, "invalid replay response", nil)
							payload, _ := json.Marshal(resp)
							responses = append(responses, json.RawMessage(payload))
						} else {
							responses = append(responses, replayResp)
						}
					}
					return
				}
				s.metrics.incReplayMiss()
				if s.replayStrict && len(req.ID) > 0 {
					resp := jsonrpc.ErrorResponse(req.ID, jsonrpc.ErrServer, "replay miss", nil)
					payload, _ := json.Marshal(resp)
					responses = append(responses, json.RawMessage(payload))
					return
				}
			}

			if req.Method == "tools/call" && s.validator != nil {
				tool, args, err := parseToolCall(req.Params)
				if err != nil {
					s.metrics.incValidationReject()
					if len(req.ID) > 0 {
						resp := jsonrpc.ErrorResponse(req.ID, jsonrpc.ErrInvalidParams, "invalid tools/call params", nil)
						payload, _ := json.Marshal(resp)
						responses = append(responses, json.RawMessage(payload))
					}
					return
				}
				decision, err := s.validator.ValidateToolCall(tool, args)
				if err != nil {
					if len(req.ID) > 0 {
						resp := jsonrpc.ErrorResponse(req.ID, jsonrpc.ErrServer, "validation error", nil)
						payload, _ := json.Marshal(resp)
						responses = append(responses, json.RawMessage(payload))
					}
					return
				}
				if len(decision.Violations) > 0 && decision.Allowed {
					s.logger.Printf("validation audit: tool=%s violations=%v", tool, decision.Violations)
				}
				if !decision.Allowed {
					s.metrics.incValidationReject()
					if len(req.ID) > 0 {
						resp := jsonrpc.ErrorResponse(req.ID, jsonrpc.ErrInvalidParams, "tool call rejected", decision.Violations)
						payload, _ := json.Marshal(resp)
						responses = append(responses, json.RawMessage(payload))
					}
					return
				}
			}

			if s.upstream == nil {
				if len(req.ID) > 0 {
					resp := jsonrpc.ErrorResponse(req.ID, jsonrpc.ErrServer, "no upstream configured", nil)
					payload, _ := json.Marshal(resp)
					responses = append(responses, json.RawMessage(payload))
				}
				return
			}

			upstreamHTTPResp, err := s.doUpstream(r.Context(), r, itemTrimmed, false)
			if err != nil {
				s.metrics.incUpstreamError()
				if len(req.ID) > 0 {
					resp := jsonrpc.ErrorResponse(req.ID, jsonrpc.ErrServer, "upstream error", nil)
					payload, _ := json.Marshal(resp)
					responses = append(responses, json.RawMessage(payload))
				}
				return
			}
			defer upstreamHTTPResp.Body.Close()

			upstreamResp, err := s.readUpstreamJSON(upstreamHTTPResp)
			if err != nil {
				s.metrics.incUpstreamError()
				if len(req.ID) > 0 {
					msg := "upstream error"
					if errors.Is(err, errUpstreamResponseTooLarge) {
						msg = "upstream response too large"
					}
					resp := jsonrpc.ErrorResponse(req.ID, jsonrpc.ErrServer, msg, nil)
					payload, _ := json.Marshal(resp)
					responses = append(responses, json.RawMessage(payload))
				}
				return
			}

			if len(upstreamResp) > 0 {
				if err := s.recorder.Append(sig, json.RawMessage(itemTrimmed), upstreamResp); err != nil {
					s.logger.Printf("record append failed: %v", err)
				}
				if len(req.ID) > 0 {
					responses = append(responses, upstreamResp)
				}
				return
			}
			s.metrics.incUpstreamError()
			if len(req.ID) > 0 {
				resp := jsonrpc.ErrorResponse(req.ID, jsonrpc.ErrServer, "empty upstream response", nil)
				payload, _ := json.Marshal(resp)
				responses = append(responses, json.RawMessage(payload))
			}
		}(item)
	}

	if len(responses) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	payload, _ := json.Marshal(responses)
	s.writeRawJSON(w, http.StatusOK, payload)
}
