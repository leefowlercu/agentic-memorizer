#!/bin/bash
# E2E Test Teardown Script
# Cleans up the test environment

set -e

echo "Tearing down E2E test environment..."

cd "$(dirname "$0")/.."

# Stop all services and remove volumes by default
echo "Stopping Docker services and removing volumes..."
if [ "$1" = "--keep-volumes" ]; then
    echo "Note: Keeping volumes (use without --keep-volumes to remove)"
    docker-compose down
else
    docker-compose down -v
fi

# Clean test artifacts
echo "Cleaning test artifacts..."
rm -rf ./fixtures/.cache

echo "✅ E2E test environment cleaned up"
