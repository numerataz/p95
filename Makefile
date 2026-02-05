.PHONY: all build build-api build-tui run-api run-tui test clean migrate migrate-down dev docker-up docker-down lint

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Binary names
API_BINARY=sixtyseven-api
TUI_BINARY=sixtyseven

# Directories
CMD_API=./cmd/api
CMD_TUI=./cmd/tui
BUILD_DIR=./build

# Database
DATABASE_URL?=postgres://sixtyseven:sixtyseven@localhost:5432/sixtyseven?sslmode=disable

all: build

# Build commands
build: build-api build-tui

build-api:
	@echo "Building API server..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(API_BINARY) $(CMD_API)

build-tui:
	@echo "Building TUI..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(TUI_BINARY) $(CMD_TUI)

# Run commands
run-api:
	@echo "Starting API server..."
	$(GOCMD) run $(CMD_API)

run-tui:
	@echo "Starting TUI..."
	$(GOCMD) run $(CMD_TUI)

# Development
dev:
	@echo "Starting development environment..."
	docker-compose -f deployments/docker/docker-compose.yml up -d db redis
	@sleep 3
	@make migrate
	@make run-api

# Test
test:
	@echo "Running tests..."
	$(GOTEST) -v -race ./...

test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# Database migrations
migrate:
	@echo "Running migrations..."
	@for f in migrations/*.up.sql; do \
		echo "Applying $$f..."; \
		psql $(DATABASE_URL) -f $$f; \
	done

migrate-down:
	@echo "Rolling back migrations..."
	@for f in $$(ls -r migrations/*.down.sql); do \
		echo "Rolling back $$f..."; \
		psql $(DATABASE_URL) -f $$f; \
	done

# Docker
docker-up:
	@echo "Starting Docker containers..."
	docker-compose -f deployments/docker/docker-compose.yml up -d

docker-down:
	@echo "Stopping Docker containers..."
	docker-compose -f deployments/docker/docker-compose.yml down

docker-build:
	@echo "Building Docker images..."
	docker-compose -f deployments/docker/docker-compose.yml build

# Clean
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out coverage.html

# Dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

# Lint
lint:
	@echo "Running linter..."
	golangci-lint run ./...

# Generate
generate:
	@echo "Running go generate..."
	$(GOCMD) generate ./...

# Python SDK
sdk-install:
	@echo "Installing Python SDK in development mode..."
	cd sdk/python && pip install -e .

sdk-test:
	@echo "Running Python SDK tests..."
	cd sdk/python && pytest tests/ -v

sdk-build:
	@echo "Building Python SDK..."
	cd sdk/python && python -m build

# Help
help:
	@echo "Available targets:"
	@echo "  build        - Build all binaries"
	@echo "  build-api    - Build API server"
	@echo "  build-tui    - Build TUI application"
	@echo "  run-api      - Run API server"
	@echo "  run-tui      - Run TUI application"
	@echo "  dev          - Start development environment"
	@echo "  test         - Run tests"
	@echo "  migrate      - Run database migrations"
	@echo "  docker-up    - Start Docker containers"
	@echo "  docker-down  - Stop Docker containers"
	@echo "  clean        - Clean build artifacts"
	@echo "  deps         - Download dependencies"
	@echo "  lint         - Run linter"
	@echo "  sdk-install  - Install Python SDK"
	@echo "  sdk-test     - Run Python SDK tests"
