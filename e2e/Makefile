.PHONY: help setup build test test-quick test-verbose test-cli test-daemon test-mcp test-integrations test-config test-graph test-semantic test-embeddings test-e2e test-facts test-integrations-facts clean clean-volumes logs shell

help: ## Show this help message
	@echo "E2E Test Suite - Agentic Memorizer"
	@echo ""
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-20s %s\n", $$1, $$2}'

setup: ## Build test environment
	@echo "Building test environment..."
	docker compose build

build: setup ## Build test binary
	@echo "Building test binary..."
	docker compose run --rm test-runner make build

test: setup ## Run full E2E test suite
	@echo "Starting FalkorDB..."
	docker compose up -d falkordb
	@echo "Waiting for FalkorDB to be healthy..."
	@sh -c 'for i in 1 2 3 4 5 6 7 8 9 10; do docker compose exec -T falkordb redis-cli ping 2>/dev/null | grep -q PONG && break || sleep 3; done'
	@echo "Running E2E tests..."
	docker compose run --rm test-runner go test -tags=e2e -v -timeout 60m ./e2e/tests/...
	@echo "Stopping services and cleaning volumes..."
	docker compose down -v

test-quick: setup ## Run quick smoke tests only
	@echo "Starting FalkorDB..."
	docker compose up -d falkordb
	@echo "Waiting for FalkorDB..."
	@sh -c 'for i in 1 2 3 4 5 6 7 8 9 10; do docker compose exec -T falkordb redis-cli ping 2>/dev/null | grep -q PONG && break || sleep 3; done'
	@echo "Running smoke tests..."
	docker compose run --rm test-runner go test -tags=e2e -v -timeout 5m -run "^TestSmoke" ./e2e/tests/...
	@echo "Stopping services and cleaning volumes..."
	docker compose down -v

test-verbose: setup ## Run tests with verbose output
	@echo "Starting FalkorDB..."
	docker compose up -d falkordb
	@echo "Waiting for FalkorDB..."
	@sh -c 'for i in 1 2 3 4 5 6 7 8 9 10; do docker compose exec -T falkordb redis-cli ping 2>/dev/null | grep -q PONG && break || sleep 3; done'
	@echo "Running tests (verbose)..."
	docker compose run --rm test-runner go test -tags=e2e -v -timeout 30m -count=1 ./e2e/tests/...
	@echo "Stopping services and cleaning volumes..."
	docker compose down -v

test-cli: setup ## Run CLI command tests only
	@echo "Starting FalkorDB..."
	docker compose up -d falkordb
	@echo "Waiting for FalkorDB..."
	@sh -c 'for i in 1 2 3 4 5 6 7 8 9 10; do docker compose exec -T falkordb redis-cli ping 2>/dev/null | grep -q PONG && break || sleep 3; done'
	@echo "Running CLI tests..."
	docker compose run --rm test-runner go test -tags=e2e -v -timeout 10m ./e2e/tests/cli_test.go
	@echo "Stopping services and cleaning volumes..."
	docker compose down -v

test-daemon: setup ## Run daemon lifecycle tests only
	@echo "Starting FalkorDB..."
	docker compose up -d falkordb
	@echo "Waiting for FalkorDB..."
	@sh -c 'for i in 1 2 3 4 5 6 7 8 9 10; do docker compose exec -T falkordb redis-cli ping 2>/dev/null | grep -q PONG && break || sleep 3; done'
	@echo "Running daemon tests..."
	docker compose run --rm test-runner go test -tags=e2e -v -timeout 10m ./e2e/tests/daemon_test.go
	@echo "Stopping services and cleaning volumes..."
	docker compose down -v

test-mcp: setup ## Run MCP server tests only
	@echo "Starting FalkorDB..."
	docker compose up -d falkordb
	@echo "Waiting for FalkorDB..."
	@sh -c 'for i in 1 2 3 4 5 6 7 8 9 10; do docker compose exec -T falkordb redis-cli ping 2>/dev/null | grep -q PONG && break || sleep 3; done'
	@echo "Running MCP tests..."
	docker compose run --rm test-runner go test -tags=e2e -v -timeout 10m ./e2e/tests/mcp_test.go
	@echo "Stopping services and cleaning volumes..."
	docker compose down -v

test-integrations: setup ## Run integration adapter tests only
	@echo "Starting FalkorDB..."
	docker compose up -d falkordb
	@echo "Waiting for FalkorDB..."
	@sh -c 'for i in 1 2 3 4 5 6 7 8 9 10; do docker compose exec -T falkordb redis-cli ping 2>/dev/null | grep -q PONG && break || sleep 3; done'
	@echo "Running integration tests..."
	docker compose run --rm test-runner go test -tags=e2e -v -timeout 10m ./e2e/tests/integration_test.go
	@echo "Stopping services and cleaning volumes..."
	docker compose down -v

test-config: setup ## Run configuration tests only
	@echo "Starting FalkorDB..."
	docker compose up -d falkordb
	@echo "Waiting for FalkorDB..."
	@sh -c 'for i in 1 2 3 4 5 6 7 8 9 10; do docker compose exec -T falkordb redis-cli ping 2>/dev/null | grep -q PONG && break || sleep 3; done'
	@echo "Running config tests..."
	docker compose run --rm test-runner go test -tags=e2e -v -timeout 10m ./e2e/tests/config_test.go
	@echo "Stopping services and cleaning volumes..."
	docker compose down -v

test-graph: setup ## Run FalkorDB graph tests only
	@echo "Starting FalkorDB..."
	docker compose up -d falkordb
	@echo "Waiting for FalkorDB..."
	@sh -c 'for i in 1 2 3 4 5 6 7 8 9 10; do docker compose exec -T falkordb redis-cli ping 2>/dev/null | grep -q PONG && break || sleep 3; done'
	@echo "Running graph tests..."
	docker compose run --rm test-runner go test -tags=e2e -v -timeout 10m ./e2e/tests/graph_test.go
	@echo "Stopping services and cleaning volumes..."
	docker compose down -v

test-semantic: setup ## Run semantic provider tests only
	@echo "Starting FalkorDB..."
	docker compose up -d falkordb
	@echo "Waiting for FalkorDB..."
	@sh -c 'for i in 1 2 3 4 5 6 7 8 9 10; do docker compose exec -T falkordb redis-cli ping 2>/dev/null | grep -q PONG && break || sleep 3; done'
	@echo "Running semantic provider tests..."
	docker compose run --rm test-runner go test -tags=e2e -v -timeout 10m ./e2e/tests/semantic_providers_test.go
	@echo "Stopping services and cleaning volumes..."
	docker compose down -v

test-embeddings: setup ## Run embeddings provider tests only
	@echo "Starting FalkorDB..."
	docker compose up -d falkordb
	@echo "Waiting for FalkorDB..."
	@sh -c 'for i in 1 2 3 4 5 6 7 8 9 10; do docker compose exec -T falkordb redis-cli ping 2>/dev/null | grep -q PONG && break || sleep 3; done'
	@echo "Running embeddings provider tests..."
	docker compose run --rm test-runner go test -tags=e2e -v -timeout 10m ./e2e/tests/embeddings_providers_test.go
	@echo "Stopping services and cleaning volumes..."
	docker compose down -v

test-e2e: setup ## Run end-to-end workflow tests only
	@echo "Starting FalkorDB..."
	docker compose up -d falkordb
	@echo "Waiting for FalkorDB..."
	@sh -c 'for i in 1 2 3 4 5 6 7 8 9 10; do docker compose exec -T falkordb redis-cli ping 2>/dev/null | grep -q PONG && break || sleep 3; done'
	@echo "Running E2E workflow tests..."
	docker compose run --rm test-runner go test -tags=e2e -v -timeout 10m ./e2e/tests/e2e_test.go
	@echo "Stopping services and cleaning volumes..."
	docker compose down -v

test-facts: setup ## Run facts command tests only
	@echo "Starting FalkorDB..."
	docker compose up -d falkordb
	@echo "Waiting for FalkorDB..."
	@sh -c 'for i in 1 2 3 4 5 6 7 8 9 10; do docker compose exec -T falkordb redis-cli ping 2>/dev/null | grep -q PONG && break || sleep 3; done'
	@echo "Running facts tests..."
	docker compose run --rm test-runner go test -tags=e2e -v -timeout 10m ./e2e/tests/facts_test.go
	@echo "Stopping services and cleaning volumes..."
	docker compose down -v

test-integrations-facts: setup ## Run integration facts tests only
	@echo "Starting FalkorDB..."
	docker compose up -d falkordb
	@echo "Waiting for FalkorDB..."
	@sh -c 'for i in 1 2 3 4 5 6 7 8 9 10; do docker compose exec -T falkordb redis-cli ping 2>/dev/null | grep -q PONG && break || sleep 3; done'
	@echo "Running integration facts tests..."
	docker compose run --rm test-runner go test -tags=e2e -v -timeout 10m ./e2e/tests/integrations_facts_test.go
	@echo "Stopping services and cleaning volumes..."
	docker compose down -v

clean: ## Stop services and clean up
	@echo "Stopping services..."
	docker compose down
	@echo "Cleaning test artifacts..."
	rm -rf ./fixtures/.cache

clean-volumes: ## Stop services and remove volumes
	@echo "Stopping services and removing volumes..."
	docker compose down -v
	@echo "Cleaning test artifacts..."
	rm -rf ./fixtures/.cache

logs: ## Show logs from running services
	docker compose logs -f

shell: setup ## Open shell in test-runner container
	@echo "Starting FalkorDB..."
	docker compose up -d falkordb
	@echo "Opening shell in test-runner..."
	docker compose run --rm test-runner /bin/bash
