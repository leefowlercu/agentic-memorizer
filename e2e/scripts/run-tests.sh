#!/bin/bash
# E2E Test Runner Script
# Main script for running E2E tests

set -e

# Parse arguments
VERBOSE=""
SUITE=""
TIMEOUT="30m"

while [[ $# -gt 0 ]]; do
    case $1 in
        -v|--verbose)
            VERBOSE="-v"
            shift
            ;;
        -s|--suite)
            SUITE="$2"
            shift 2
            ;;
        -t|--timeout)
            TIMEOUT="$2"
            shift 2
            ;;
        *)
            echo "Unknown option: $1"
            echo "Usage: $0 [-v|--verbose] [-s|--suite SUITE] [-t|--timeout TIMEOUT]"
            echo ""
            echo "Suites: cli, daemon, mcp, integrations, config, graph, e2e, all"
            exit 1
            ;;
    esac
done

cd "$(dirname "$0")/.."

# Ensure environment is set up
if ! docker compose ps falkordb | grep -q "Up"; then
    echo "Starting FalkorDB..."
    docker compose up -d falkordb
    timeout 30 sh -c 'until docker compose exec -T falkordb redis-cli ping 2>/dev/null | grep -q PONG; do sleep 1; done'
fi

# Determine test path
TEST_PATH="./e2e/tests/..."
if [ -n "$SUITE" ]; then
    case $SUITE in
        all)
            TEST_PATH="./e2e/tests/..."
            ;;
        *)
            TEST_PATH="./e2e/tests/${SUITE}_test.go"
            ;;
    esac
fi

echo "Running E2E tests: $TEST_PATH"
echo "Timeout: $TIMEOUT"
echo ""

# Run tests
docker compose run --rm test-runner \
    go test -tags=e2e $VERBOSE -timeout "$TIMEOUT" "$TEST_PATH"

EXIT_CODE=$?

if [ $EXIT_CODE -eq 0 ]; then
    echo ""
    echo "✅ All tests passed"
else
    echo ""
    echo "❌ Tests failed with exit code $EXIT_CODE"
fi

exit $EXIT_CODE
