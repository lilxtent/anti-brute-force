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
	#docker compose up --build

down:
	docker compose down

test:
	go test ./...

## lint: run golangci-lint
lint:
	golangci-lint run ./...

## lint-fix: run golangci-lint and auto-fix issues
lint-fix:
	golangci-lint run --fix ./...

## install-lint: install the pinned golangci-lint version
install-lint:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

## tidy: sync go.mod/go.sum
tidy:
	go mod tidy

## clean: remove build artifacts
clean:
	rm -rf $(BIN_DIR)
