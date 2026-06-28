# Makefile for go-stock project

.PHONY: all test test-integration lint build

all: lint test build

lint:
	go vet ./...
	staticcheck ./...
	golangci-lint run

# Unit tests only (excluding integration)
test:
	go test -cover ./... -run "^Test" -skip "integration"

# Integration tests (requires Chrome)
test-integration:
	go test -tags=integration ./... -v

build:
	wails build -clean -skipbindings
