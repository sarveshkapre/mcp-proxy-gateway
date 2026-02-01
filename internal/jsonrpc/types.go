package jsonrpc

import (
  "encoding/json"
  "errors"
)

const (
  ErrInvalidRequest = -32600
  ErrMethodNotFound = -32601
  ErrInvalidParams  = -32602
  ErrInternal       = -32603
  ErrServer         = -32000
)

var (
  ErrEmptyMethod = errors.New("method is required")
)

type Request struct {
  JSONRPC string          `json:"jsonrpc"`
  ID      json.RawMessage `json:"id,omitempty"`
  Method  string          `json:"method"`
  Params  json.RawMessage `json:"params,omitempty"`
}

func (r *Request) Validate() error {
  if r.JSONRPC != "2.0" {
    return errors.New("jsonrpc must be 2.0")
  }
  if r.Method == "" {
    return ErrEmptyMethod
  }
  return nil
}

type Response struct {
  JSONRPC string          `json:"jsonrpc"`
  ID      json.RawMessage `json:"id,omitempty"`
  Result  any             `json:"result,omitempty"`
  Error   *ErrorObject    `json:"error,omitempty"`
}

type ErrorObject struct {
  Code    int    `json:"code"`
  Message string `json:"message"`
  Data    any    `json:"data,omitempty"`
}

func ErrorResponse(id json.RawMessage, code int, message string, data any) Response {
  if len(id) == 0 {
    id = json.RawMessage("null")
  }
  return Response{
    JSONRPC: "2.0",
    ID:      id,
    Error: &ErrorObject{
      Code:    code,
      Message: message,
      Data:    data,
    },
  }
}
