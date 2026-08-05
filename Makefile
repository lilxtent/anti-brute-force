BIN_DIR := bin
SERVER_BIN := $(BIN_DIR)/abf-server
CLI_BIN := $(BIN_DIR)/abf-cli

GOLANGCI_LINT_VERSION := v2.12.2

COMPOSE_FILE := docker-compose.yml
INTEGRATION_COMPOSE_FILE := docker-compose.integration.yml

POSTGRES_USER ?= user
POSTGRES_PASSWORD ?= pass
POSTGRES_DB ?= db
POSTGRES_PORT ?= 5432
POSTGRES_DSN ?= postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@localhost:$(POSTGRES_PORT)/$(POSTGRES_DB)?sslmode=disable
MIGRATIONS_DIR ?= migrations
GOOSE := go run github.com/pressly/goose/v3/cmd/goose@latest

.PHONY: build build-server build-cli run down logs migrate migrate-down migrate-status \
	test test-integration test-integration-local lint lint-fix install-lint tidy clean

## build: compile the server and CLI binaries into ./bin
build: build-server build-cli

build-server:
	go build -o $(SERVER_BIN) ./cmd/server

build-cli:
	go build -o $(CLI_BIN) ./cmd/cli

run:
	docker compose -f $(COMPOSE_FILE) up --build

down:
	docker compose -f $(COMPOSE_FILE) down

logs:
	docker compose -f $(COMPOSE_FILE) logs -f

migrate:
	$(GOOSE) -dir $(MIGRATIONS_DIR) postgres "$(POSTGRES_DSN)" up

migrate-down:
	$(GOOSE) -dir $(MIGRATIONS_DIR) postgres "$(POSTGRES_DSN)" down

migrate-status:
	$(GOOSE) -dir $(MIGRATIONS_DIR) postgres "$(POSTGRES_DSN)" status

test:
	go test ./...

test-integration:
	docker compose -f $(COMPOSE_FILE) -f $(INTEGRATION_COMPOSE_FILE) up --build \
		--abort-on-container-exit --exit-code-from integration-tests integration-tests; \
	status=$$?; \
	docker compose -f $(COMPOSE_FILE) -f $(INTEGRATION_COMPOSE_FILE) down -v; \
	exit $$status

test-integration-local: migrate
	go test -tags integration -count=1 -p 1 ./...

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
