# Agentic Memorizer Makefile

# Version from internal/version/VERSION file
VERSION := $(shell cat internal/version/VERSION)

# Git commit with optional -dirty suffix
GIT_DIRTY := $(shell git diff --quiet 2>/dev/null || echo "-dirty")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")$(GIT_DIRTY)

# Build date in ISO 8601 format
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Linker flags for version injection
LDFLAGS := -X github.com/leefowlercu/agentic-memorizer/internal/version.gitCommit=$(GIT_COMMIT) \
           -X github.com/leefowlercu/agentic-memorizer/internal/version.buildDate=$(BUILD_DATE)

# Binary name
BINARY := memorizer

# Spec. and Plan files
SPEC_FILE := SPEC.md
PLAN_FILE := PLAN.md

# Install directory
INSTALL_DIR := $(HOME)/.local/bin

# ANSI color codes
GREEN  := \033[0;32m
YELLOW := \033[0;33m
RED    := \033[0;31m
NC     := \033[0m
CHECK  := \xE2\x9C\x93

.PHONY: build build-nocolor install install-nocolor clean clean-nocolor test test-nocolor test-race test-race-nocolor

# Build with colored output
build:
	@printf "Building $(BINARY)..."
	@go build -ldflags "$(LDFLAGS)" -o $(BINARY) .
	@printf " $(GREEN)$(CHECK)$(NC)\n"

# Build without colors (for CI)
build-nocolor:
	@printf "Building $(BINARY)..."
	@go build -ldflags "$(LDFLAGS)" -o $(BINARY) .
	@printf " $(CHECK)\n"

# Build and install to ~/.local/bin
install: build
	@printf "Installing to $(INSTALL_DIR)..."
	@mkdir -p $(INSTALL_DIR)
	@cp $(BINARY) $(INSTALL_DIR)/
	@printf " $(GREEN)$(CHECK)$(NC)\n"

# Install without colors (for CI)
install-nocolor: build-nocolor
	@printf "Installing to $(INSTALL_DIR)..."
	@mkdir -p $(INSTALL_DIR)
	@cp $(BINARY) $(INSTALL_DIR)/
	@printf " $(CHECK)\n"

# Clean build artifacts
clean:
	@printf "Cleaning..."
	@rm -f $(BINARY) $(SPEC_FILE) $(PLAN_FILE)
	@printf " $(GREEN)$(CHECK)$(NC)\n"

clean-nocolor:
	@printf "Cleaning..."
	@rm -f $(BINARY) $(SPEC_FILE) $(PLAN_FILE)
	@printf " $(CHECK)\n"

# Run tests
test:
	@printf "Running tests..."
	@go test ./... -v
	@printf " $(GREEN)$(CHECK)$(NC)\n"

test-nocolor:
	@printf "Running tests..."
	@go test ./... -v
	@printf " $(CHECK)\n"

# Run tests with race detector
test-race:
	@printf "Running tests with race detector..."
	@go test -race ./... -v
	@printf " $(GREEN)$(CHECK)$(NC)\n"

test-race-nocolor:
	@printf "Running tests with race detector..."
	@go test -race ./... -v
	@printf " $(CHECK)\n"
