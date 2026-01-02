.PHONY: build install test test-integration test-all test-e2e test-e2e-quick clean uninstall help coverage coverage-html lint fmt vet check test-race ci-lint ci-test ci-all daemon-start daemon-stop daemon-status daemon-logs clean-cache validate-config goreleaser-check goreleaser-snapshot release-check release-major release-minor release-patch

BINARY_NAME=memorizer
INSTALL_DIR=$(HOME)/.local/bin
INSTALL_PATH=$(INSTALL_DIR)/$(BINARY_NAME)

# Version information for builds
VERSION_FILE=internal/version/VERSION
CURRENT_VERSION=$(shell cat $(VERSION_FILE) 2>/dev/null || echo "0.0.0")
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X github.com/leefowlercu/agentic-memorizer/internal/version.Version=$(VERSION) \
           -X github.com/leefowlercu/agentic-memorizer/internal/version.GitCommit=$(GIT_COMMIT) \
           -X github.com/leefowlercu/agentic-memorizer/internal/version.BuildDate=$(BUILD_DATE)

# Colors for output
COLOR_RESET := \033[0m
COLOR_BOLD := \033[1m
COLOR_GREEN := \033[32m
COLOR_YELLOW := \033[33m
COLOR_BLUE := \033[34m

help: ## Show this help message
	@echo "Memorizer - Build and Installation"
	@echo ""
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

build: ## Build the binary with version information
	@echo "$(COLOR_BLUE)$(COLOR_BOLD)Building $(BINARY_NAME) $(VERSION)...$(COLOR_RESET)"
	@go build -ldflags "$(LDFLAGS)" -o $(BINARY_NAME) .
	@echo "$(COLOR_GREEN)Build complete: ./$(BINARY_NAME) $(VERSION) ✓$(COLOR_RESET)"

install: build ## Install the binary with version information
	@echo "$(COLOR_BLUE)$(COLOR_BOLD)Installing $(VERSION) to $(INSTALL_PATH)...$(COLOR_RESET)"
	@mkdir -p $(INSTALL_DIR)
	@$(INSTALL_PATH) daemon stop 2>/dev/null || true
	@sleep 1
	@mv $(BINARY_NAME) $(INSTALL_PATH).tmp && mv $(INSTALL_PATH).tmp $(INSTALL_PATH)
	@chmod +x $(INSTALL_PATH)
	@echo "$(COLOR_GREEN)Installed $(VERSION) to $(INSTALL_PATH) ✓$(COLOR_RESET)"
	@echo "Verify installation:"
	@$(INSTALL_PATH) version
	@echo "To restart the daemon, run: $(INSTALL_PATH) daemon start"

test: ## Run unit tests only
	@echo "$(COLOR_BLUE)$(COLOR_BOLD)Running unit tests...$(COLOR_RESET)"
	@go test -race -v ./...

test-integration: build ## Run integration tests only
	@echo "$(COLOR_BLUE)$(COLOR_BOLD)Running integration tests...$(COLOR_RESET)"
	@go test -race -tags=integration -v ./...

test-all: test test-integration ## Run all non-e2e tests (unit + integration, with race detector)
	@echo "$(COLOR_GREEN)All tests passed ✓$(COLOR_RESET)"

test-quick: ## Run unit tests without race detector (faster)
	@echo "$(COLOR_BLUE)$(COLOR_BOLD)Running quick unit tests...$(COLOR_RESET)"
	@go test -v ./...

test-e2e: build ## Run E2E tests
	@echo "$(COLOR_BLUE)$(COLOR_BOLD)Running E2E tests...$(COLOR_RESET)"
	@cd e2e && $(MAKE) test

test-e2e-quick: build ## Run quick E2E smoke tests
	@echo "$(COLOR_BLUE)$(COLOR_BOLD)Running E2E smoke tests...$(COLOR_RESET)"
	@cd e2e && $(MAKE) test-quick

test-race: test test-integration ## Alias for test-all (race detector now enabled by default)
	@echo "$(COLOR_GREEN)Race detection complete ✓$(COLOR_RESET)"

clean: ## Remove build artifacts
	@echo "$(COLOR_YELLOW)Cleaning...$(COLOR_RESET)"
	@rm -f $(BINARY_NAME)
	@rm -f coverage.out coverage.html
	@rm -rf dist/
	@echo "$(COLOR_GREEN)Clean complete ✓$(COLOR_RESET)"

clean-cache: ## Remove cache files only
	@echo "$(COLOR_YELLOW)Removing cache files...$(COLOR_RESET)"
	@rm -rf $(HOME)/.memorizer/.cache/*
	@echo "$(COLOR_GREEN)Cache cleaned ✓$(COLOR_RESET)"

uninstall: ## Remove installed binary
	@echo "$(COLOR_YELLOW)Uninstalling from $(INSTALL_PATH)...$(COLOR_RESET)"
	@rm -f $(INSTALL_PATH)
	@echo "$(COLOR_GREEN)Uninstalled (config, cache, and logs preserved) ✓$(COLOR_RESET)"

deps: ## Download dependencies
	@echo "$(COLOR_BLUE)$(COLOR_BOLD)Downloading dependencies...$(COLOR_RESET)"
	@go mod download
	@go mod tidy
	@echo "$(COLOR_GREEN)Dependencies updated ✓$(COLOR_RESET)"

coverage: ## Run tests with coverage report
	@echo "$(COLOR_BLUE)$(COLOR_BOLD)Running tests with coverage...$(COLOR_RESET)"
	@go test -coverprofile=coverage.out ./...
	@go tool cover -func=coverage.out
	@echo "$(COLOR_GREEN)Coverage report generated: coverage.out ✓$(COLOR_RESET)"
	@echo "Run 'make coverage-html' to view HTML report"

coverage-html: coverage ## Generate and open HTML coverage report
	@echo "$(COLOR_BLUE)$(COLOR_BOLD)Generating HTML coverage report...$(COLOR_RESET)"
	@go tool cover -html=coverage.out -o coverage.html
	@echo "$(COLOR_GREEN)HTML coverage report generated: coverage.html ✓$(COLOR_RESET)"
	@if command -v open > /dev/null 2>&1; then \
		open coverage.html; \
	elif command -v xdg-open > /dev/null 2>&1; then \
		xdg-open coverage.html; \
	else \
		echo "Open coverage.html in your browser to view the report"; \
	fi

lint: ## Run golangci-lint
	@echo "$(COLOR_BLUE)$(COLOR_BOLD)Running linter...$(COLOR_RESET)"
	@if command -v golangci-lint > /dev/null 2>&1; then \
		golangci-lint run ./...; \
		echo "$(COLOR_GREEN)Lint complete ✓$(COLOR_RESET)"; \
	else \
		echo "$(COLOR_YELLOW)golangci-lint not installed. Install from: https://golangci-lint.run/usage/install/ ⚠$(COLOR_RESET)"; \
		exit 1; \
	fi

fmt: ## Format code with gofmt
	@echo "$(COLOR_BLUE)$(COLOR_BOLD)Formatting code...$(COLOR_RESET)"
	@gofmt -s -w .
	@echo "$(COLOR_GREEN)Code formatted ✓$(COLOR_RESET)"

vet: ## Run go vet
	@echo "$(COLOR_BLUE)$(COLOR_BOLD)Running go vet...$(COLOR_RESET)"
	@go vet ./...
	@echo "$(COLOR_GREEN)Vet complete ✓$(COLOR_RESET)"

check: fmt vet test-all ## Run all code quality checks (format, vet, all tests)
	@echo "$(COLOR_GREEN)All checks passed ✓$(COLOR_RESET)"

ci-lint: ## Run CI linting checks locally
	@echo "$(COLOR_BLUE)$(COLOR_BOLD)Running CI linting checks...$(COLOR_RESET)"
	@if [ -n "$$(gofmt -l .)" ]; then \
		echo "Go code is not formatted:"; \
		gofmt -d .; \
		exit 1; \
	fi
	@$(MAKE) vet
	@echo "$(COLOR_GREEN)CI linting checks passed ✓$(COLOR_RESET)"

ci-test: ## Run CI test suite locally
	@echo "$(COLOR_BLUE)$(COLOR_BOLD)Running CI test suite...$(COLOR_RESET)"
	@$(MAKE) test
	@$(MAKE) test-race
	@echo "$(COLOR_GREEN)CI test suite passed ✓$(COLOR_RESET)"

ci-all: ci-lint ci-test test-integration ## Run all CI checks locally
	@echo "$(COLOR_GREEN)All CI checks passed ✓$(COLOR_RESET)"

daemon-start: build ## Build and start daemon
	@echo "$(COLOR_BLUE)$(COLOR_BOLD)Starting daemon...$(COLOR_RESET)"
	@./$(BINARY_NAME) daemon start
	@echo "$(COLOR_GREEN)Daemon started ✓$(COLOR_RESET)"

daemon-stop: ## Stop running daemon
	@echo "$(COLOR_YELLOW)Stopping daemon...$(COLOR_RESET)"
	@./$(BINARY_NAME) daemon stop
	@echo "$(COLOR_GREEN)Daemon stopped ✓$(COLOR_RESET)"

daemon-status: ## Check daemon status
	@./$(BINARY_NAME) daemon status

daemon-logs: ## Tail daemon logs
	@echo "$(COLOR_BLUE)$(COLOR_BOLD)Tailing daemon logs (Ctrl+C to exit)...$(COLOR_RESET)"
	@tail -f $(HOME)/.memorizer/daemon.log

validate-config: build ## Validate configuration file
	@echo "$(COLOR_BLUE)$(COLOR_BOLD)Validating configuration...$(COLOR_RESET)"
	@./$(BINARY_NAME) config validate
	@echo "$(COLOR_GREEN)Configuration valid ✓$(COLOR_RESET)"

goreleaser-check: ## Validate Goreleaser configuration
	@echo "$(COLOR_BLUE)$(COLOR_BOLD)Validating Goreleaser configuration...$(COLOR_RESET)"
	@if ! command -v goreleaser &> /dev/null; then \
		echo "Error: goreleaser is not installed"; \
		echo "Install with: go install github.com/goreleaser/goreleaser/v2@latest"; \
		exit 1; \
	fi
	@goreleaser check
	@echo "$(COLOR_GREEN)Goreleaser configuration is valid ✓$(COLOR_RESET)"

goreleaser-snapshot: goreleaser-check ## Test release locally without publishing
	@echo "$(COLOR_BLUE)$(COLOR_BOLD)Building snapshot release...$(COLOR_RESET)"
	@goreleaser release --snapshot --clean --skip=publish
	@echo "$(COLOR_GREEN)Snapshot build complete in dist/ ✓$(COLOR_RESET)"

release-check: ## Verify release prerequisites
	@echo "$(COLOR_BLUE)$(COLOR_BOLD)Checking release prerequisites...$(COLOR_RESET)"
	@if ! command -v goreleaser &> /dev/null; then \
		echo "Error: goreleaser is not installed"; \
		echo "Install with: go install github.com/goreleaser/goreleaser/v2@latest"; \
		exit 1; \
	fi
	@if [ -z "$(GITHUB_TOKEN)" ]; then \
		echo "$(COLOR_YELLOW)Warning: GITHUB_TOKEN environment variable not set$(COLOR_RESET)"; \
		echo "GoReleaser needs this to create GitHub releases"; \
		echo "See README.md for setup instructions"; \
	fi
	@if [ -n "$$(git status --porcelain)" ]; then \
		echo "Error: Working directory is not clean"; \
		git status --short; \
		exit 1; \
	fi
	@echo "$(COLOR_GREEN)Release prerequisites satisfied ✓$(COLOR_RESET)"

release-major: release-check ## Prepare major version release
	@read -p "Release tagline (e.g., 'Breaking Changes & New Architecture'): " tagline; \
	./scripts/bump-version.sh major; \
	RELEASE_TAGLINE="$$tagline" $(MAKE) release-prep VERSION=$$(cat .next-version)

release-minor: release-check ## Prepare minor version release
	@read -p "Release tagline (e.g., 'MCP Server Implementation'): " tagline; \
	./scripts/bump-version.sh minor; \
	RELEASE_TAGLINE="$$tagline" $(MAKE) release-prep VERSION=$$(cat .next-version)

release-patch: release-check ## Prepare patch version release
	@read -p "Release tagline (e.g., 'Bug Fixes & Performance'): " tagline; \
	./scripts/bump-version.sh patch; \
	RELEASE_TAGLINE="$$tagline" $(MAKE) release-prep VERSION=$$(cat .next-version)

release-prep: ## Prepare release (internal target, use release-{major,minor,patch})
	@if [ -z "$(VERSION)" ]; then \
		echo "Error: VERSION not specified"; \
		exit 1; \
	fi
	@./scripts/prepare-release.sh $(VERSION)
