.PHONY: build install test clean uninstall help coverage coverage-html

BINARY_NAME=agentic-memorizer
INSTALL_DIR=$(HOME)/.local/bin
INSTALL_PATH=$(INSTALL_DIR)/$(BINARY_NAME)
CONFIG_NAME=config.yaml
CONFIG_DIR=$(HOME)/.agentic-memorizer
CONFIG_PATH=$(CONFIG_DIR)/$(CONFIG_NAME)

help: ## Show this help message
	@echo "Agentic Memorizer - Build and Installation"
	@echo ""
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

build: ## Build the binary
	@echo "Building $(BINARY_NAME)..."
	@go build -o $(BINARY_NAME) .
	@echo "✅ Build complete: ./$(BINARY_NAME)"

install: build ## Build and install to ~/.local/bin/
	@echo "Installing to $(INSTALL_PATH)..."
	@mkdir -p $(INSTALL_DIR)
	@cp $(BINARY_NAME) $(INSTALL_PATH)
	@chmod +x $(INSTALL_PATH)
	@echo "✅ Installed successfully to $(INSTALL_PATH)"
	@echo ""
	@if [ ! -f $(CONFIG_PATH) ]; then \
		echo "📝 Creating default config..."; \
		mkdir -p $(CONFIG_DIR); \
		cp config.yaml.example $(CONFIG_PATH); \
		echo "✅ Config created at $(CONFIG_PATH)"; \
		echo ""; \
		echo "⚠️  IMPORTANT: Edit $(CONFIG_PATH) and add your Claude API key"; \
		echo ""; \
	else \
		echo "ℹ️  Config already exists at $(CONFIG_PATH)"; \
		echo ""; \
	fi
	@echo "Next steps:"
	@echo "  1. Edit config: $(CONFIG_PATH)"
	@echo "  2. Set your ANTHROPIC_API_KEY or add api_key to config"
	@echo "  3. Configure SessionStart hook in ~/.claude/settings.json"
	@echo "  4. Test: $(INSTALL_PATH)"

test: ## Run tests
	@echo "Running tests..."
	@go test -v ./...

clean: ## Remove build artifacts
	@echo "Cleaning..."
	@rm -f $(BINARY_NAME)
	@rm -f coverage.out coverage.html
	@echo "✅ Clean complete"

uninstall: ## Remove installed binary
	@echo "Uninstalling from $(INSTALL_PATH)..."
	@rm -f $(INSTALL_PATH)
	@echo "✅ Uninstalled (config and cache preserved)"
	@echo ""
	@echo "To remove config: rm $(CONFIG_PATH)"
	@echo "To remove cache: check cache_dir in your config"

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
