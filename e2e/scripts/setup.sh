#!/bin/bash
# E2E Test Setup Script
# Prepares the environment for running E2E tests

set -e

echo "Setting up E2E test environment..."

# Build the test binary
echo "Building agentic-memorizer binary..."
cd "$(dirname "$0")/../.."
make build

# Ensure Docker is running
if ! docker info > /dev/null 2>&1; then
    echo "Error: Docker is not running. Please start Docker first."
    exit 1
fi

# Build test runner image
echo "Building test runner Docker image..."
cd e2e
docker compose build test-runner

# Start FalkorDB
echo "Starting FalkorDB..."
docker compose up -d falkordb

# Wait for FalkorDB to be ready
echo "Waiting for FalkorDB to be healthy..."
timeout 30 sh -c 'until docker compose exec -T falkordb redis-cli ping 2>/dev/null | grep -q PONG; do sleep 1; done' || {
    echo "Error: FalkorDB failed to start"
    docker compose logs falkordb
    exit 1
}

echo "✅ E2E test environment is ready"
echo ""
echo "Run tests with:"
echo "  make test         # Full test suite"
echo "  make test-quick   # Quick smoke tests"
echo "  make test-cli     # CLI tests only"
