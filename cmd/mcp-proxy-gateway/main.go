package main

import (
  "flag"
  "log"
  "net/http"
  "net/url"
  "os"
  "time"

  "github.com/sarveshkapre/mcp-proxy-gateway/internal/config"
  "github.com/sarveshkapre/mcp-proxy-gateway/internal/proxy"
  "github.com/sarveshkapre/mcp-proxy-gateway/internal/record"
  "github.com/sarveshkapre/mcp-proxy-gateway/internal/validate"
)

func main() {
  listen := flag.String("listen", ":8080", "listen address")
  upstream := flag.String("upstream", "", "upstream MCP server URL")
  policyPath := flag.String("policy", "", "policy file (yaml/json)")
  recordPath := flag.String("record", "", "record file path (NDJSON)")
  replayPath := flag.String("replay", "", "replay file path (NDJSON)")
  replayStrict := flag.Bool("replay-strict", false, "error on replay miss")
  maxBody := flag.Int64("max-body", 1<<20, "max request/response body in bytes")
  timeout := flag.Duration("timeout", 10*time.Second, "upstream request timeout")
  flag.Parse()

  logger := log.New(os.Stdout, "mcp-proxy-gateway ", log.LstdFlags)

  var upstreamURL *url.URL
  if *upstream != "" {
    parsed, err := url.Parse(*upstream)
    if err != nil {
      logger.Fatalf("invalid upstream URL: %v", err)
    }
    upstreamURL = parsed
  }

  policy, err := config.LoadPolicy(*policyPath)
  if err != nil {
    logger.Fatalf("failed to load policy: %v", err)
  }

  validator, err := validate.New(policy)
  if err != nil {
    logger.Fatalf("failed to init validator: %v", err)
  }

  recorder := record.NewRecorder(*recordPath)
  replay, err := record.LoadReplay(*replayPath)
  if err != nil {
    logger.Fatalf("failed to load replay file: %v", err)
  }

  srv := proxy.NewServer(upstreamURL, validator, recorder, replay, *replayStrict, *maxBody, *timeout, logger)

  httpServer := &http.Server{
    Addr:              *listen,
    Handler:           srv,
    ReadHeaderTimeout: 5 * time.Second,
  }

  logger.Printf("listening on %s", *listen)
  if upstreamURL != nil {
    logger.Printf("upstream %s", upstreamURL.String())
  } else {
    logger.Printf("no upstream configured")
  }
  if err := httpServer.ListenAndServe(); err != nil {
    logger.Fatalf("server error: %v", err)
  }
}
