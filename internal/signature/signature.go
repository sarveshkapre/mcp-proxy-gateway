package signature

import (
  "crypto/sha256"
  "encoding/hex"
  "encoding/json"

  "github.com/sarveshkapre/mcp-proxy-gateway/internal/jsonrpc"
)

type toolCallParams struct {
  Tool      string          `json:"tool"`
  Arguments json.RawMessage `json:"arguments"`
}

type sigInput struct {
  Method    string          `json:"method"`
  Tool      string          `json:"tool,omitempty"`
  Arguments json.RawMessage `json:"arguments,omitempty"`
  Params    json.RawMessage `json:"params,omitempty"`
}

func FromRequest(req *jsonrpc.Request) (string, error) {
  input := sigInput{Method: req.Method}
  if req.Method == "tools/call" {
    params := toolCallParams{}
    if len(req.Params) > 0 {
      if err := json.Unmarshal(req.Params, &params); err != nil {
        return "", err
      }
      input.Tool = params.Tool
      if len(params.Arguments) > 0 {
        normalized, err := normalizeJSON(params.Arguments)
        if err != nil {
          return "", err
        }
        input.Arguments = normalized
      }
    }
  } else if len(req.Params) > 0 {
    normalized, err := normalizeJSON(req.Params)
    if err != nil {
      return "", err
    }
    input.Params = normalized
  }

  payload, err := json.Marshal(input)
  if err != nil {
    return "", err
  }
  sum := sha256.Sum256(payload)
  return hex.EncodeToString(sum[:]), nil
}

func normalizeJSON(raw json.RawMessage) (json.RawMessage, error) {
  var v any
  if err := json.Unmarshal(raw, &v); err != nil {
    return nil, err
  }
  normalized, err := json.Marshal(v)
  if err != nil {
    return nil, err
  }
  return json.RawMessage(normalized), nil
}
