# Agentic Memorizer Development Guidelines

Auto-generated from all feature plans. Last updated: 2026-01-09

## Active Technologies
- Go 1.25.5 + github.com/spf13/viper (configuration), github.com/spf13/cobra (CLI - existing) (001-app-config-subsystem)
- Go 1.25.5 + log/slog (stdlib), github.com/spf13/viper (via config subsystem) (002-logging-subsystem)
- File-based logging to `~/.config/memorizer/memorizer.log` (default) (002-logging-subsystem)
- Go 1.25.5 (per existing project) (003-version-build-info)
- N/A (compile-time embedded data only) (003-version-build-info)
- Go 1.25.5 + github.com/spf13/cobra (CLI), github.com/spf13/viper (config), github.com/go-chi/chi/v5 (HTTP router), net/http (HTTP server) (004-core-daemon-subsystem)
- PID file at `~/.config/memorizer/daemon.pid` (004-core-daemon-subsystem)
- YAML configuration file (~/.config/memorizer/config.yaml) (005-initialize-command-tui)

## Project Structure

```text
src/
tests/
```

## Commands

# Add commands for Go 1.25.5

## Code Style

Go 1.25.5: Follow standard conventions

## Recent Changes
- 005-initialize-command-tui: Added Go 1.25.5
- 004-core-daemon-subsystem: Added Go 1.25.5 + github.com/spf13/cobra (CLI), github.com/spf13/viper (config), github.com/go-chi/chi/v5 (HTTP router), net/http (HTTP server)
- 003-version-build-info: Added Go 1.25.5 (per existing project)


<!-- MANUAL ADDITIONS START -->
<!-- MANUAL ADDITIONS END -->
