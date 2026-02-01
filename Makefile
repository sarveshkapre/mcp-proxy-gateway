BINARY_NAME := mcp-proxy-gateway
BIN_DIR := bin
GOLANGCI_LINT := $(shell go env GOPATH)/bin/golangci-lint

.PHONY: setup dev test lint typecheck build check release

setup:
	go mod download

dev:
	go run ./cmd/$(BINARY_NAME) --listen :8080

test:
	go test ./...

lint:
	$(GOLANGCI_LINT) run

typecheck:
	go vet ./...

build:
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(BINARY_NAME) ./cmd/$(BINARY_NAME)

check: lint typecheck test build

release:
	@echo "Release via git tag and GitHub Releases"
