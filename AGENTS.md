# Repository Guidelines

## Project Structure & Module Organization
- `cmd/` CLI entrypoints organized by domain (`daemon`, `remember`, `list`, etc.).
- `internal/` contains subsystems: `daemon` (orchestration/health), `analysis` (queue/workers), `watcher` (fsnotify), `walker` (path traversal), `graph` (FalkorDB client), `events` (bus), `providers` (semantic/embeddings), `cache`, `cleaner`, `mcp`, `metrics`, `registry` (SQLite). Tests live beside implementations.
- `GRACEFUL_DEGRADATION_*.md` and `SPEC.md` capture architecture and degradation policies.

## Build, Test, and Development Commands
- `make build` – compile `memorizer` binary.
- `make install` – build and install to `~/.local/bin`.
- `make test` / `make test-race` – run all tests (race detector optional).
- `make lint` / `make lint-fix` – run golangci-lint (with optional autofix).
- `memorizer daemon start` – run the daemon locally; `memorizer daemon status` for health.

## Coding Style & Naming Conventions
- Go code, tabs for indentation; keep files ASCII unless data demands otherwise.
- Prefer interface-first design (`Graph`, `Bus`, `Watcher`, `Queue`), functional options for constructors (`WithX`), and registries for plug-ins.
- Use `fmt.Errorf("context; %w", err)` (semicolon) for error wrapping.
- Structured logging via `log/slog` with `.With("component", ...)` context.
- Health/status detail keys are lower_snake_case; avoid renaming existing keys.

## Testing Guidelines
- Standard library `testing` only; table-driven tests favored.
- Place tests next to code in `internal/...`. Use `t.Cleanup` for resource cleanup and `t.Setenv` for isolation.
- Run `go test ./...` before submit; use `make test-race` for concurrency-sensitive changes.

## Commit & Pull Request Guidelines
- Commit messages: short imperative summary (e.g., "refactor: add supervisor restart signals").
- PRs should describe behavior changes, note degradation/restart policy impacts, and link issues if applicable. Include screenshots/log excerpts only when UI/UX or CLI output changes.

## Security & Configuration Tips
- Config via `config.yaml`/env; prefer `config.ExpandPath` for user paths.
- Sensitive providers (API keys) must be checked with `Available()` before use; keep in env, not repo.
- Event bus and registry paths default under `~/.config/memorizer`; avoid mixing durable queues with the registry DB.

## Architecture Overview
- Daemon orchestrator builds components via a registry, starts persistent components with supervisors and health polling, and runs jobs (initial walk/rebuild) via a `JobRunner` abstraction.
- Event-driven flow: watcher → bus → queue/workers → graph; cleaner/mcp/metrics subscribe as needed; degraded modes prioritize availability over consistency.
