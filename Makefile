BINARY_NAME := mcp-proxy-gateway
BIN_DIR := bin
GOLANGCI_LINT := $(shell go env GOPATH)/bin/golangci-lint
GOLANGCI_LINT_VERSION := v1.64.8
GOFILES := $(shell find . -name '*.go' -not -path './vendor/*')

.PHONY: setup dev fmt fmtcheck test lint typecheck build check smoke release

$(GOLANGCI_LINT):
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

setup:
	go mod download

dev:
	go run ./cmd/$(BINARY_NAME) --listen :8080

fmt:
	gofmt -w $(GOFILES)

fmtcheck:
	@unformatted="$$(gofmt -l $(GOFILES))"; \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt needed on:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi

test:
	go test ./...

lint: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) run

typecheck:
	go vet ./...

build:
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(BINARY_NAME) ./cmd/$(BINARY_NAME)

check: fmtcheck lint typecheck test build

smoke:
	./scripts/smoke.sh

release:
	@echo "Release via git tag and GitHub Releases"
