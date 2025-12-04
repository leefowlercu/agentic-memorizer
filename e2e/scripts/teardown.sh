#!/bin/bash
# E2E Test Teardown Script
# Cleans up the test environment

set -e

echo "Tearing down E2E test environment..."

cd "$(dirname "$0")/.."

# Stop all services
echo "Stopping Docker services..."
docker-compose down

# Optionally remove volumes
if [ "$1" = "--clean-volumes" ]; then
    echo "Removing Docker volumes..."
    docker-compose down -v
fi

# Clean test artifacts
echo "Cleaning test artifacts..."
rm -rf ./fixtures/.cache

echo "✅ E2E test environment cleaned up"
