# GitHub Actions Workflows

This directory contains CI/CD workflows for the Agentic Memorizer project.

## Workflows

### 1. CI (`ci.yml`)

**Triggers:** Push to master/main, pull requests

**Jobs:**
- **Lint** - Code formatting and static analysis
  - `gofmt` formatting check
  - `go vet` static analysis
  - `golangci-lint` (best-effort)

- **Unit Tests** - Fast unit tests
  - Standard test suite
  - Race detector tests

- **Integration Tests** - Integration test suite
  - Tests with external dependencies

- **Coverage** - Code coverage reporting
  - Generates coverage reports
  - Uploads coverage artifacts
  - Checks coverage threshold (currently disabled)

- **Build** - Multi-platform builds
  - Ubuntu and macOS builds
  - Version verification
  - Binary artifact upload

**Duration:** ~5-10 minutes

---

### 2. E2E Tests (`e2e-tests.yml`)

**Triggers:** Push to master/main, pull requests, manual dispatch

**Jobs:**
- **E2E** - Full end-to-end test suite
  - Docker Compose environment setup
  - FalkorDB integration tests
  - CLI command tests
  - Daemon lifecycle tests
  - File processing tests
  - MCP server tests
  - Graph operations tests
  - Integration framework tests

- **E2E Quick** - Fast smoke tests
  - Quick validation of core functionality
  - Faster feedback for PRs

**Features:**
- Docker layer caching for faster builds
- Test log artifact upload
- 30-minute timeout for full suite
- 10-minute timeout for quick tests

**Duration:**
- Full suite: ~30-40 minutes
- Quick tests: ~5-10 minutes

---

## Workflow Dependencies

```
┌─────────────┐
│  ci.yml     │  ← Runs on every push/PR
│  (5-10 min) │     Fast feedback loop
└─────────────┘

┌─────────────┐
│ e2e-tests   │  ← Runs on every push/PR
│ (30-40 min) │     Comprehensive validation
└─────────────┘
```

---

## Local Testing

### Test workflows before pushing

```bash
# Install act (GitHub Actions runner)
brew install act  # macOS
# or download from: https://github.com/nektos/act

# Run CI workflow locally
act -j lint
act -j test
act -j build

# Run E2E quick tests
act -j e2e-quick

# Note: Full E2E tests require Docker and may not work with act
```

### Run tests manually

```bash
# Run all CI checks
make check        # format + vet + test
make test-race    # with race detector
make coverage     # with coverage report

# Run E2E tests
make test-e2e       # full suite
make test-e2e-quick # smoke tests

# Run specific E2E test suites
cd e2e && make test-cli
cd e2e && make test-daemon
cd e2e && make test-graph
```

---

## Troubleshooting

### Workflow fails on cache

If Docker cache is corrupted:
1. Go to Actions → Select failed workflow
2. Click "Re-run jobs" → "Re-run all jobs"
3. Cache will be rebuilt

### E2E tests timeout

- Check FalkorDB container health in logs
- Verify Docker Compose is running
- Check test logs in artifacts

---

## Adding New Workflows

When adding new workflows:

1. **Follow naming conventions**
   - Use lowercase with hyphens: `my-workflow.yml`
   - Use descriptive names: `security-scan.yml`

2. **Set appropriate timeouts**
   - Fast workflows: 5-10 minutes
   - E2E tests: 30-40 minutes
   - Never exceed 60 minutes

3. **Add caching when possible**
   - Go module cache
   - Docker layer cache
   - Build artifacts

4. **Upload artifacts for debugging**
   - Test logs
   - Coverage reports
   - Binary artifacts

5. **Document in this README**
   - Purpose and triggers
   - Duration estimate
   - Prerequisites

---

## Status Badges

Add to main README.md:

```markdown
![CI Status](https://github.com/leefowlercu/agentic-memorizer/workflows/CI/badge.svg)
![E2E Tests](https://github.com/leefowlercu/agentic-memorizer/workflows/E2E%20Tests/badge.svg)
```

---

## References

- [GitHub Actions Documentation](https://docs.github.com/en/actions)
- [Docker Buildx Cache](https://github.com/docker/build-push-action/blob/master/docs/advanced/cache.md)
