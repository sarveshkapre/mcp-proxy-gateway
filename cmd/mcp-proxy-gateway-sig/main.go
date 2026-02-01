package main

import (
  "encoding/json"
  "flag"
  "fmt"
  "io"
  "os"

  "github.com/sarveshkapre/mcp-proxy-gateway/internal/jsonrpc"
  "github.com/sarveshkapre/mcp-proxy-gateway/internal/signature"
)

func main() {
  file := flag.String("file", "", "path to JSON-RPC request (defaults to stdin)")
  flag.Parse()

  var data []byte
  var err error
  if *file == "" {
    data, err = io.ReadAll(os.Stdin)
  } else {
    data, err = os.ReadFile(*file)
  }
  if err != nil {
    fail("read input", err)
  }

  req := jsonrpc.Request{}
  if err := json.Unmarshal(data, &req); err != nil {
    fail("parse JSON", err)
  }
  if err := req.Validate(); err != nil {
    fail("validate JSON-RPC", err)
  }

  sig, err := signature.FromRequest(&req)
  if err != nil {
    fail("compute signature", err)
  }
  fmt.Println(sig)
}

func fail(msg string, err error) {
  fmt.Fprintf(os.Stderr, "%s: %v\n", msg, err)
  os.Exit(1)
}
