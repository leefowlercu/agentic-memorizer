.PHONY: build install test test-integration test-all clean uninstall help coverage coverage-html lint fmt vet check test-race daemon-start daemon-stop daemon-status daemon-logs clean-cache validate-config release-major release-minor release-patch

BINARY_NAME=agentic-memorizer
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

help: ## Show this help message
	@echo "Agentic Memorizer - Build and Installation"
	@echo ""
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

build: ## Build the binary with version information
	@echo "Building $(BINARY_NAME) $(VERSION)..."
	@go build -ldflags "$(LDFLAGS)" -o $(BINARY_NAME) .
	@echo "✅ Build complete: ./$(BINARY_NAME) $(VERSION)"

install: build ## Install the binary with version information
	@echo "Installing $(VERSION) to $(INSTALL_PATH)..."
	@mkdir -p $(INSTALL_DIR)
	@$(INSTALL_PATH) daemon stop 2>/dev/null || true
	@sleep 1
	@mv $(BINARY_NAME) $(INSTALL_PATH).tmp && mv $(INSTALL_PATH).tmp $(INSTALL_PATH)
	@chmod +x $(INSTALL_PATH)
	@echo "✅ Installed $(VERSION) to $(INSTALL_PATH)"
	@echo ""
	@echo "Verify installation:"
	@$(INSTALL_PATH) version
	@echo ""
	@echo "To restart the daemon, run: $(INSTALL_PATH) daemon start"

test: ## Run unit tests only
	@echo "Running unit tests..."
	@go test -v ./...

test-integration: ## Run integration tests only
	@echo "Running integration tests..."
	@go test -tags=integration -v ./...

test-all: test test-integration ## Run all tests (unit + integration)
	@echo ""
	@echo "✅ All tests passed"

test-race: ## Run tests with race detector
	@echo "Running tests with race detector..."
	@go test -race -v ./...
	@echo "✅ Race detection complete"

clean: ## Remove build artifacts
	@echo "Cleaning..."
	@rm -f $(BINARY_NAME)
	@rm -f coverage.out coverage.html
	@echo "✅ Clean complete"

clean-cache: ## Remove cache files only
	@echo "Removing cache files..."
	@rm -rf $(HOME)/.agentic-memorizer/.cache/*
	@echo "✅ Cache cleaned"

uninstall: ## Remove installed binary
	@echo "Uninstalling from $(INSTALL_PATH)..."
	@rm -f $(INSTALL_PATH)
	@echo "✅ Uninstalled (config, cache, and logs preserved)"

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy
	@echo "✅ Dependencies updated"

coverage: ## Run tests with coverage report
	@echo "Running tests with coverage..."
	@go test -coverprofile=coverage.out ./...
	@go tool cover -func=coverage.out
	@echo ""
	@echo "✅ Coverage report generated: coverage.out"
	@echo "Run 'make coverage-html' to view HTML report"

coverage-html: coverage ## Generate and open HTML coverage report
	@echo "Generating HTML coverage report..."
	@go tool cover -html=coverage.out -o coverage.html
	@echo "✅ HTML coverage report generated: coverage.html"
	@if command -v open > /dev/null 2>&1; then \
		open coverage.html; \
	elif command -v xdg-open > /dev/null 2>&1; then \
		xdg-open coverage.html; \
	else \
		echo "Open coverage.html in your browser to view the report"; \
	fi

lint: ## Run golangci-lint
	@echo "Running linter..."
	@if command -v golangci-lint > /dev/null 2>&1; then \
		golangci-lint run ./...; \
		echo "✅ Lint complete"; \
	else \
		echo "⚠️  golangci-lint not installed. Install from: https://golangci-lint.run/usage/install/"; \
		exit 1; \
	fi

fmt: ## Format code with gofmt
	@echo "Formatting code..."
	@gofmt -s -w .
	@echo "✅ Code formatted"

vet: ## Run go vet
	@echo "Running go vet..."
	@go vet ./...
	@echo "✅ Vet complete"

check: fmt vet test-all ## Run all code quality checks (format, vet, all tests)
	@echo ""
	@echo "✅ All checks passed"

daemon-start: build ## Build and start daemon
	@echo "Starting daemon..."
	@./$(BINARY_NAME) daemon start
	@echo "✅ Daemon started"

daemon-stop: ## Stop running daemon
	@echo "Stopping daemon..."
	@./$(BINARY_NAME) daemon stop
	@echo "✅ Daemon stopped"

daemon-status: ## Check daemon status
	@./$(BINARY_NAME) daemon status

daemon-logs: ## Tail daemon logs
	@echo "Tailing daemon logs (Ctrl+C to exit)..."
	@tail -f $(HOME)/.agentic-memorizer/daemon.log

validate-config: build ## Validate configuration file
	@echo "Validating configuration..."
	@./$(BINARY_NAME) config validate
	@echo "✅ Configuration valid"

release-major: ## Bump major version, update VERSION file, create git tag
	@echo "Current version: $(CURRENT_VERSION)"
	@NEW_VERSION=$$(echo $(CURRENT_VERSION) | awk -F. '{print $$1+1".0.0"}'); \
	echo "Bumping to major version: $$NEW_VERSION"; \
	echo $$NEW_VERSION > $(VERSION_FILE); \
	git add $(VERSION_FILE); \
	git commit -m "release: bump version to $$NEW_VERSION"; \
	git tag -a v$$NEW_VERSION -m "Release v$$NEW_VERSION"; \
	echo "✅ Version bumped to $$NEW_VERSION and tagged as v$$NEW_VERSION"; \
	echo ""; \
	echo "Next steps:"; \
	echo "  git push origin master"; \
	echo "  git push origin v$$NEW_VERSION"

release-minor: ## Bump minor version, update VERSION file, create git tag
	@echo "Current version: $(CURRENT_VERSION)"
	@NEW_VERSION=$$(echo $(CURRENT_VERSION) | awk -F. '{print $$1"."$$2+1".0"}'); \
	echo "Bumping to minor version: $$NEW_VERSION"; \
	echo $$NEW_VERSION > $(VERSION_FILE); \
	git add $(VERSION_FILE); \
	git commit -m "release: bump version to $$NEW_VERSION"; \
	git tag -a v$$NEW_VERSION -m "Release v$$NEW_VERSION"; \
	echo "✅ Version bumped to $$NEW_VERSION and tagged as v$$NEW_VERSION"; \
	echo ""; \
	echo "Next steps:"; \
	echo "  git push origin master"; \
	echo "  git push origin v$$NEW_VERSION"

release-patch: ## Bump patch version, update VERSION file, create git tag
	@echo "Current version: $(CURRENT_VERSION)"
	@NEW_VERSION=$$(echo $(CURRENT_VERSION) | awk -F. '{print $$1"."$$2"."$$3+1}'); \
	echo "Bumping to patch version: $$NEW_VERSION"; \
	echo $$NEW_VERSION > $(VERSION_FILE); \
	git add $(VERSION_FILE); \
	git commit -m "release: bump version to $$NEW_VERSION"; \
	git tag -a v$$NEW_VERSION -m "Release v$$NEW_VERSION"; \
	echo "✅ Version bumped to $$NEW_VERSION and tagged as v$$NEW_VERSION"; \
	echo ""; \
	echo "Next steps:"; \
	echo "  git push origin master"; \
	echo "  git push origin v$$NEW_VERSION"
