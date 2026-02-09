package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
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
	recordMaxBytes := flag.Int64("record-max-bytes", -1, "record rotation size in bytes (0 disables, -1 uses policy)")
	recordMaxFiles := flag.Int("record-max-files", -1, "record rotation backups to retain (0 keeps none, -1 uses policy/default)")
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

	recordPolicy := config.RecordPolicy{}
	replayPolicy := config.ReplayPolicy{}
	httpPolicy := config.HTTPPolicy{}
	if policy != nil {
		recordPolicy = policy.Record
		replayPolicy = policy.Replay
		httpPolicy = policy.HTTP
	}

	// Rotation config precedence: CLI (if not -1) overrides policy.
	var rotateBytes int64
	var rotateFiles int = -1
	if recordPolicy.MaxBytes != nil {
		rotateBytes = *recordPolicy.MaxBytes
	}
	if recordPolicy.MaxFiles != nil {
		rotateFiles = *recordPolicy.MaxFiles
	}
	if *recordMaxBytes != -1 {
		rotateBytes = *recordMaxBytes
	}
	if *recordMaxFiles != -1 {
		rotateFiles = *recordMaxFiles
	}
	if rotateBytes < 0 || (rotateFiles < -1) {
		logger.Fatalf("invalid record rotation values: max_bytes=%d max_files=%d", rotateBytes, rotateFiles)
	}
	if rotateBytes <= 0 {
		rotateBytes = 0
		rotateFiles = 0
	} else if rotateFiles == -1 {
		rotateFiles = 3
	} else if rotateFiles < 0 {
		rotateFiles = 0
	}

	redactor, err := record.NewRedactor(recordPolicy.RedactKeys, recordPolicy.RedactKeyRegex)
	if err != nil {
		logger.Fatalf("failed to init record redactor: %v", err)
	}
	recorder := record.NewRecorder(*recordPath, redactor, rotateBytes, rotateFiles)
	replay, err := record.LoadReplay(*replayPath, record.ReplayMatch(replayPolicy.Match))
	if err != nil {
		logger.Fatalf("failed to load replay file: %v", err)
	}

	srv := proxy.NewServer(upstreamURL, validator, recorder, replay, *replayStrict, httpPolicy.OriginAllowlist, *maxBody, *timeout, logger)

	httpServer := &http.Server{
		Addr:              *listen,
		Handler:           srv,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger.Printf("listening on %s", *listen)
	logger.Printf("endpoints: POST /rpc, GET /healthz, GET /metricsz")
	if upstreamURL != nil {
		logger.Printf("upstream %s", upstreamURL.String())
	} else {
		logger.Printf("no upstream configured")
	}

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- httpServer.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		logger.Printf("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			logger.Fatalf("shutdown error: %v", err)
		}
		logger.Printf("shutdown complete")
	case err := <-serverErr:
		if err != nil && err != http.ErrServerClosed {
			logger.Fatalf("server error: %v", err)
		}
	}
}
