# KConduit Makefile
# Build and development tasks for KConduit

# Variables
BINARY_NAME=kconduit
MAIN_PATH=cmd/kconduit/main.go
GO=go
GOFLAGS=-v
LDFLAGS=-s -w
BUILD_DIR=build
# docker-compose v1 is end-of-life; v2 ships as a docker subcommand.
DOCKER_COMPOSE=docker compose
TEST_COMPOSE_FILE=tests/docker-compose.yaml
ACLS_COMPOSE_FILE=tests/docker-compose-acls.yaml

# Bootstrap addresses for the two test clusters, taken from the advertised
# EXTERNAL/BROKER_HOST listeners in the compose files above. They are the ports
# published to this machine, so they only work from the host — a client running
# inside a container has to use the in-network listener instead.
PLAIN_BROKERS=localhost:19094,localhost:29094,localhost:39094
ACL_BROKERS=localhost:29092,localhost:39092,localhost:49092

# Credentials for the ACL cluster, matching tests/kafka_jaas.conf. Passed via
# the environment because --sasl-password is deprecated.
ACL_SASL_USERNAME=admin
ACL_SASL_PASSWORD=admin-secret

# Version information
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Build flags with version info
FULL_LDFLAGS=-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT) $(LDFLAGS)

# Default target
.DEFAULT_GOAL := help

# Colors for output
CYAN=\033[0;36m
GREEN=\033[0;32m
RED=\033[0;31m
YELLOW=\033[0;33m
NC=\033[0m # No Color

## help: Display this help message
.PHONY: help
help:
	@echo "$(CYAN)KConduit Makefile$(NC)"
	@echo "$(GREEN)Available targets:$(NC)"
	@awk '/^##@/ { printf "\n$(CYAN)%s$(NC)\n", substr($$0, 5); next } \
	      /^## [a-zA-Z_-]+:/ { sep = index($$0, ":"); \
	        printf "  $(YELLOW)%-22s$(NC) %s\n", substr($$0, 4, sep - 4), substr($$0, sep + 2) }' $(MAKEFILE_LIST)

##@ Build

## build: Build the binary for current platform
.PHONY: build
build:
	@echo "$(GREEN)Building $(BINARY_NAME)...$(NC)"
	$(GO) build $(GOFLAGS) -ldflags "$(FULL_LDFLAGS)" -o $(BINARY_NAME) $(MAIN_PATH)
	@echo "$(GREEN)✓ Build complete: ./$(BINARY_NAME)$(NC)"

## build-all: Build for multiple platforms
.PHONY: build-all
build-all: build-linux build-darwin build-windows

## build-linux: Build for Linux (amd64)
.PHONY: build-linux
build-linux:
	@echo "$(GREEN)Building for Linux...$(NC)"
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) -ldflags "$(FULL_LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PATH)
	@echo "$(GREEN)✓ Linux build complete$(NC)"

## build-darwin: Build for macOS (amd64 and arm64)
.PHONY: build-darwin
build-darwin:
	@echo "$(GREEN)Building for macOS...$(NC)"
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 $(GO) build $(GOFLAGS) -ldflags "$(FULL_LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_PATH)
	GOOS=darwin GOARCH=arm64 $(GO) build $(GOFLAGS) -ldflags "$(FULL_LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_PATH)
	@echo "$(GREEN)✓ macOS builds complete$(NC)"

## build-windows: Build for Windows (amd64)
.PHONY: build-windows
build-windows:
	@echo "$(GREEN)Building for Windows...$(NC)"
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 $(GO) build $(GOFLAGS) -ldflags "$(FULL_LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PATH)
	@echo "$(GREEN)✓ Windows build complete$(NC)"

## install: Install the binary to GOPATH/bin
.PHONY: install
install:
	@echo "$(GREEN)Installing $(BINARY_NAME)...$(NC)"
	$(GO) install $(GOFLAGS) -ldflags "$(FULL_LDFLAGS)" $(MAIN_PATH)
	@echo "$(GREEN)✓ Installed to $(GOPATH)/bin/$(BINARY_NAME)$(NC)"

##@ Development

## run: Build and run the application against the default broker
.PHONY: run
run: build
	@echo "$(GREEN)Running $(BINARY_NAME)...$(NC)"
	./$(BINARY_NAME)

## run-dev: Run against the ACL cluster with debug logging to /tmp/k.log
.PHONY: run-dev
run-dev: build
	@echo "$(GREEN)Running $(BINARY_NAME) against the ACL cluster (debug)...$(NC)"
	KCONDUIT_SASL_PASSWORD=$(ACL_SASL_PASSWORD) ./$(BINARY_NAME) -b $(ACL_BROKERS) \
		--sasl \
		--sasl-protocol SASL_PLAINTEXT \
		--sasl-mechanism PLAIN \
		--sasl-username $(ACL_SASL_USERNAME) \
		--log-level debug --log-file /tmp/k.log

## run-debug: Run with debug logging
.PHONY: run-debug
run-debug: build
	@echo "$(GREEN)Running $(BINARY_NAME) with debug logging...$(NC)"
	./$(BINARY_NAME) --log-level debug


## fmt: Format Go code
.PHONY: fmt
fmt:
	@echo "$(GREEN)Formatting code...$(NC)"
	$(GO) fmt ./...
	@echo "$(GREEN)✓ Code formatted$(NC)"

## vet: Run go vet
.PHONY: vet
vet:
	@echo "$(GREEN)Running go vet...$(NC)"
	$(GO) vet ./...
	@echo "$(GREEN)✓ Vet complete$(NC)"

## lint: Run golangci-lint
.PHONY: lint
lint:
	@command -v golangci-lint > /dev/null 2>&1 || (echo "$(RED)golangci-lint not found. Install from https://golangci-lint.run/usage/install/$(NC)" && exit 1)
	@echo "$(GREEN)Running linter...$(NC)"
	golangci-lint run --config .golangci.yml ./cmd/... ./pkg/...
	@echo "$(GREEN)✓ Linting complete$(NC)"

## tidy: Tidy and verify module dependencies
.PHONY: tidy
tidy:
	@echo "$(GREEN)Tidying dependencies...$(NC)"
	$(GO) mod tidy
	$(GO) mod verify
	@echo "$(GREEN)✓ Dependencies tidied$(NC)"

##@ Testing

## test: Run all tests
.PHONY: test
test:
	@echo "$(GREEN)Running tests...$(NC)"
	$(GO) test -v -race -cover ./...
	@echo "$(GREEN)✓ Tests complete$(NC)"

## test-short: Run short tests only
.PHONY: test-short
test-short:
	@echo "$(GREEN)Running short tests...$(NC)"
	$(GO) test -short -v ./...
	@echo "$(GREEN)✓ Short tests complete$(NC)"

## test-coverage: Run tests with coverage report
.PHONY: test-coverage
test-coverage:
	@echo "$(GREEN)Running tests with coverage...$(NC)"
	$(GO) test -v -race -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)✓ Coverage report generated: coverage.html$(NC)"

## benchmark: Run benchmarks
.PHONY: benchmark
benchmark:
	@echo "$(GREEN)Running benchmarks...$(NC)"
	$(GO) test -bench=. -benchmem ./...
	@echo "$(GREEN)✓ Benchmarks complete$(NC)"

##@ Kafka Environment

## kafka-up: Start the plaintext Kafka cluster (no ACLs)
.PHONY: kafka-up
kafka-up:
	@if docker ps --format '{{.Names}}' | grep -qx broker-1; then \
		echo "$(RED)The ACL cluster is running and publishes the same ports (29094, 39094, 8088).$(NC)"; \
		echo "$(YELLOW)Run 'make kafka-acls-down' first.$(NC)"; \
		exit 1; \
	fi
	@echo "$(GREEN)Starting the plaintext Kafka cluster...$(NC)"
	$(DOCKER_COMPOSE) -f $(TEST_COMPOSE_FILE) up -d
	@echo "$(GREEN)✓ Cluster started on $(PLAIN_BROKERS)$(NC)"
	@echo "$(YELLOW)Run 'make run-plain' to connect to it$(NC)"

## kafka-down: Stop local Kafka cluster
.PHONY: kafka-down
kafka-down:
	@echo "$(GREEN)Stopping Kafka cluster...$(NC)"
	$(DOCKER_COMPOSE) -f $(TEST_COMPOSE_FILE) down
	@echo "$(GREEN)✓ Kafka cluster stopped$(NC)"

## kafka-logs: View Kafka cluster logs
.PHONY: kafka-logs
kafka-logs:
	$(DOCKER_COMPOSE) -f $(TEST_COMPOSE_FILE) logs -f

## kafka-clean: Stop cluster and remove volumes
.PHONY: kafka-clean
kafka-clean:
	@echo "$(RED)Removing Kafka cluster and volumes...$(NC)"
	$(DOCKER_COMPOSE) -f $(TEST_COMPOSE_FILE) down -v
	@echo "$(GREEN)✓ Kafka cluster and volumes removed$(NC)"

## kafka-acls-up: Start the SASL Kafka cluster (ACLs enabled)
.PHONY: kafka-acls-up
kafka-acls-up:
	@if docker ps --format '{{.Names}}' | grep -qx broker1; then \
		echo "$(RED)The plaintext cluster is running and publishes the same ports (29094, 39094, 8088).$(NC)"; \
		echo "$(YELLOW)Run 'make kafka-down' first.$(NC)"; \
		exit 1; \
	fi
	@echo "$(GREEN)Starting the SASL Kafka cluster...$(NC)"
	$(DOCKER_COMPOSE) -f $(ACLS_COMPOSE_FILE) up -d
	@echo "$(GREEN)✓ Cluster started on $(ACL_BROKERS)$(NC)"
	@echo "$(YELLOW)Run 'make run-acls' to connect to it$(NC)"

## kafka-acls-down: Stop the SASL Kafka cluster
.PHONY: kafka-acls-down
kafka-acls-down:
	@echo "$(GREEN)Stopping the SASL Kafka cluster...$(NC)"
	$(DOCKER_COMPOSE) -f $(ACLS_COMPOSE_FILE) down
	@echo "$(GREEN)✓ SASL Kafka cluster stopped$(NC)"

## kafka-acls-logs: View SASL Kafka cluster logs
.PHONY: kafka-acls-logs
kafka-acls-logs:
	$(DOCKER_COMPOSE) -f $(ACLS_COMPOSE_FILE) logs -f

## kafka-acls-clean: Stop the SASL cluster and remove volumes
.PHONY: kafka-acls-clean
kafka-acls-clean:
	@echo "$(RED)Removing the SASL Kafka cluster and volumes...$(NC)"
	$(DOCKER_COMPOSE) -f $(ACLS_COMPOSE_FILE) down -v
	@echo "$(GREEN)✓ SASL Kafka cluster and volumes removed$(NC)"

##@ Running against a test cluster

## run-plain: Run KConduit against the plaintext cluster (no ACLs)
.PHONY: run-plain
run-plain: build
	@echo "$(GREEN)Connecting to the plaintext cluster on $(PLAIN_BROKERS)...$(NC)"
	./$(BINARY_NAME) -b $(PLAIN_BROKERS)

## run-acls: Run KConduit against the SASL cluster (ACLs enabled)
.PHONY: run-acls
run-acls: build
	@echo "$(GREEN)Connecting to the SASL cluster on $(ACL_BROKERS)...$(NC)"
	KCONDUIT_SASL_PASSWORD=$(ACL_SASL_PASSWORD) ./$(BINARY_NAME) -b $(ACL_BROKERS) \
		--sasl \
		--sasl-protocol SASL_PLAINTEXT \
		--sasl-mechanism PLAIN \
		--sasl-username $(ACL_SASL_USERNAME)

## run-local: Alias for run-plain, kept for existing muscle memory
.PHONY: run-local
run-local: run-plain

##@ AI Testing

## test-ai-openai: Test with OpenAI (requires OPENAI_API_KEY)
.PHONY: test-ai-openai
test-ai-openai: build
	@test -n "$$OPENAI_API_KEY" || (echo "$(RED)Error: OPENAI_API_KEY not set$(NC)" && exit 1)
	@echo "$(GREEN)Testing with OpenAI...$(NC)"
	./$(BINARY_NAME) -b $(PLAIN_BROKERS) --ai-engine openai --ai-model gpt-3.5-turbo

## test-ai-gemini: Test with Gemini (requires GEMINI_API_KEY)
.PHONY: test-ai-gemini
test-ai-gemini: build
	@test -n "$$GEMINI_API_KEY" || (echo "$(RED)Error: GEMINI_API_KEY not set$(NC)" && exit 1)
	@echo "$(GREEN)Testing with Gemini...$(NC)"
	./$(BINARY_NAME) -b $(PLAIN_BROKERS) --ai-engine gemini --ai-model gemini-3.1-pro-preview

## test-ai-ollama: Test with Ollama (requires ollama to be running)
.PHONY: test-ai-ollama
test-ai-ollama: build
	@echo "$(GREEN)Testing with Ollama (ensure ollama is running)...$(NC)"
	./$(BINARY_NAME) -b $(PLAIN_BROKERS) --ai-engine ollama --ai-model llama2

##@ Cleanup

## clean: Remove built binaries and build directory
.PHONY: clean
clean:
	@echo "$(RED)Cleaning build artifacts...$(NC)"
	rm -f $(BINARY_NAME)
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html
	@echo "$(GREEN)✓ Clean complete$(NC)"

## clean-all: Clean everything including dependencies and cache
.PHONY: clean-all
clean-all: clean kafka-clean
	@echo "$(RED)Cleaning Go cache...$(NC)"
	$(GO) clean -cache -testcache -modcache
	@echo "$(GREEN)✓ Deep clean complete$(NC)"

##@ Release

## version: Display version information
.PHONY: version
version:
	@echo "Version: $(VERSION)"
	@echo "Build Time: $(BUILD_TIME)"
	@echo "Git Commit: $(GIT_COMMIT)"

## release: Create release builds for all platforms
.PHONY: release
release: clean build-all
	@echo "$(GREEN)Creating release archives...$(NC)"
	@mkdir -p $(BUILD_DIR)/release
	cd $(BUILD_DIR) && tar czf release/$(BINARY_NAME)-$(VERSION)-linux-amd64.tar.gz $(BINARY_NAME)-linux-amd64
	cd $(BUILD_DIR) && tar czf release/$(BINARY_NAME)-$(VERSION)-darwin-amd64.tar.gz $(BINARY_NAME)-darwin-amd64
	cd $(BUILD_DIR) && tar czf release/$(BINARY_NAME)-$(VERSION)-darwin-arm64.tar.gz $(BINARY_NAME)-darwin-arm64
	cd $(BUILD_DIR) && zip release/$(BINARY_NAME)-$(VERSION)-windows-amd64.zip $(BINARY_NAME)-windows-amd64.exe
	@echo "$(GREEN)✓ Release builds created in $(BUILD_DIR)/release/$(NC)"

## docker-build: Build Docker image
.PHONY: docker-build
docker-build:
	@echo "$(GREEN)Building Docker image...$(NC)"
	docker build -t kconduit:$(VERSION) -t kconduit:latest .
	@echo "$(GREEN)✓ Docker image built: kconduit:$(VERSION)$(NC)"

##@ Documentation

## docs: Generate documentation
.PHONY: docs
docs:
	@echo "$(GREEN)Generating documentation...$(NC)"
	$(GO) doc -all ./... > docs.txt
	@echo "$(GREEN)✓ Documentation generated: docs.txt$(NC)"

.PHONY: all
all: clean build test
