BIN_DIR := bin
SERVER_BIN := $(BIN_DIR)/abf-server
CLI_BIN := $(BIN_DIR)/abf-cli

GOLANGCI_LINT_VERSION := v2.12.2

.PHONY: build build-server build-cli run down test test-integration lint lint-fix install-lint tidy clean

## build: compile the server and CLI binaries into ./bin
build: build-server build-cli

build-server:
	go build -o $(SERVER_BIN) ./cmd/server

build-cli:
	go build -o $(CLI_BIN) ./cmd/cli

run:
	docker compose up --build

down:
	docker compose down

test:
	go test ./...

## test-integration: run integration tests (needs postgres, e.g. `docker compose up -d postgres`)
test-integration:
	go test -tags integration ./...

lint:
	golangci-lint run --build-tags integration ./...

lint-fix:
	golangci-lint run --build-tags integration --fix ./...

install-lint:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

tidy:
	go mod tidy

clean:
	rm -rf $(BIN_DIR)
