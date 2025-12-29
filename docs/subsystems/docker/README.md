# Docker Helpers

Docker container lifecycle management for FalkorDB knowledge graph with availability detection and readiness polling.

**Documented Version:** v0.13.0

**Last Updated:** 2025-12-29

## Table of Contents

- [Overview](#overview)
- [Design Principles](#design-principles)
- [Key Components](#key-components)
- [Integration Points](#integration-points)
- [Glossary](#glossary)

## Overview

The Docker Helpers subsystem provides container lifecycle management for the FalkorDB graph database. Rather than implementing complex database management, Agentic Memorizer delegates to Docker for running FalkorDB, and this subsystem provides the Go interface for that delegation. The subsystem handles container creation, startup, shutdown, and removal through Docker CLI commands executed as subprocesses.

The subsystem is intentionally minimal and focused: a single container name constant, a StartOptions configuration struct, and six functions covering availability checking, container state inspection, and lifecycle operations. All operations use context timeouts to prevent blocking indefinitely. The design assumes Docker is already installed and running; the subsystem detects availability but does not manage Docker itself.

Key capabilities include:

- **Docker availability detection** - Checks if Docker daemon is running via `docker info`
- **Container state inspection** - Detects whether FalkorDB container exists and is running
- **Container lifecycle** - Start, stop, and remove operations with timeout protection
- **Readiness polling** - Waits for FalkorDB to respond to Redis PING before returning
- **Persistent storage** - Supports bind-mounted data directories for durability
- **Restart policy** - Containers use unless-stopped restart policy for reliability

## Design Principles

### CLI Delegation Pattern

Rather than using Docker client libraries, the subsystem executes Docker CLI commands as subprocesses. This approach eliminates Docker library dependencies, ensures compatibility with the user's installed Docker version, and simplifies debugging (users can run the same commands manually). All Docker interactions go through `exec.Command` with context timeouts.

### Context Timeout Protection

Every Docker operation uses a context with a timeout to prevent indefinite blocking. Availability checks and state inspections use 5-second timeouts. Container creation uses a 60-second timeout to accommodate image pulls. Stop and remove operations use 30-second timeouts. This protects the calling code from hanging if Docker becomes unresponsive.

### Idempotent Operations

Container operations are designed to be idempotent and handle existing state gracefully. StartFalkorDB returns immediately if the container is already running. It attempts to start an existing stopped container before creating a new one. If an existing container fails to start, it removes and recreates rather than failing. This resilience simplifies calling code.

### Readiness Polling

Starting a container and having it accept connections are distinct events. The subsystem polls for readiness after starting, executing `redis-cli ping` inside the container every 500ms until it receives PONG or the context times out. Callers receive confirmation that FalkorDB is ready to accept connections, not just that the container started.

### Single Container Focus

The subsystem manages exactly one container: `memorizer-falkordb`. This constraint simplifies the API and aligns with the application's single-user, single-graph architecture. The container name is a package constant ensuring consistency across all operations.

### Configuration via Struct

Startup options are encapsulated in a StartOptions struct with sensible defaults. Port defaults to 6379, Detach defaults to true, and DataDir is optional. This pattern enables future option additions without breaking existing callers.

## Key Components

### ContainerName Constant

The package defines a single constant `ContainerName = "memorizer-falkordb"` used by all container operations. This ensures consistent naming across availability checks, lifecycle operations, and Docker Compose configurations.

### StartOptions Struct

The StartOptions struct configures FalkorDB container startup with three fields: Port for the Redis protocol port (default 6379), DataDir for persistent storage bind mount (optional), and Detach for background versus foreground execution (default true).

### IsAvailable Function

The IsAvailable function checks if Docker is installed and running by executing `docker info`. Returns true if the command succeeds within a 5-second timeout. Used by CLI commands to provide helpful error messages when Docker is unavailable.

### IsFalkorDBRunning Function

The IsFalkorDBRunning function checks if the FalkorDB container is currently running. Uses `docker inspect` to query container state, returning true only if the container exists and its Running state is true. The port parameter is unused but preserved for API consistency.

### ContainerExists Function

The ContainerExists function checks whether a container with the memorizer-falkordb name exists, regardless of running state. Uses `docker inspect` which succeeds for both running and stopped containers. Used to determine whether to start an existing container or create a new one.

### StartFalkorDB Function

The StartFalkorDB function is the primary entry point for container lifecycle. It first checks if the container is already running (returns immediately if so). If a stopped container exists, it attempts to start it. If that fails, it removes and recreates. For new containers, it builds a docker run command with the specified options: port mapping for Redis (configurable) and browser UI (always 3000), volume mount for data persistence, unless-stopped restart policy, and the latest FalkorDB image. After container creation, it calls waitForReady to confirm FalkorDB accepts connections.

### StopFalkorDB Function

The StopFalkorDB function gracefully stops the running container via `docker stop`. Uses a 30-second timeout for Docker's internal graceful shutdown process. Data remains in the bound volume for the next start.

### RemoveFalkorDB Function

The RemoveFalkorDB function removes the container via `docker rm -f`. The force flag ensures removal even if the container is running. Bound volume data persists after removal.

### waitForReady Function

The waitForReady function polls FalkorDB for readiness by executing `docker exec memorizer-falkordb redis-cli ping` every 500ms. Returns nil when PONG is received, or an error if the context times out. This internal function ensures StartFalkorDB only returns after FalkorDB is accepting connections.

## Integration Points

### Graph Commands

The `memorizer graph start|stop|status` CLI commands use this subsystem for all Docker operations. The start command calls StartFalkorDB with configuration from the config subsystem. The stop command calls StopFalkorDB with optional container removal. The status command calls IsAvailable, ContainerExists, and IsFalkorDBRunning to provide detailed state information.

### Initialization TUI

The terminal UI initialization flow uses IsAvailable and IsFalkorDBRunning to determine which FalkorDB configuration options to present. If Docker is available but FalkorDB isn't running, the TUI offers to start it automatically via StartFalkorDB. This provides a guided setup experience for new users.

### Initialize Command

The `memorizer initialize` command calls IsAvailable and IsFalkorDBRunning during non-interactive setup when `--setup-integrations` is specified. This enables fully automated installation including graph database provisioning.

### Docker Compose

The project includes a docker-compose.yml providing an alternative to the subsystem's direct Docker commands. The compose file uses the same container name and configuration, allowing users to choose their preferred container management approach. Both methods produce compatible containers.

### Configuration System

The config subsystem provides FalkorDB connection settings (host, port, password) used by graph commands when calling StartFalkorDB. The DataDir for persistent storage is derived from the application directory rather than config, ensuring data locality.

## Glossary

**Bind Mount**
A Docker volume type that maps a host directory directly into the container. Used for FalkorDB data persistence at `~/.memorizer/falkordb/`.

**Container State**
The running status of a Docker container: created, running, paused, restarting, removing, exited, or dead. This subsystem primarily distinguishes between running and not-running.

**Docker CLI**
The command-line interface for Docker. This subsystem executes Docker CLI commands as subprocesses rather than using library bindings.

**FalkorDB**
A Redis-compatible graph database used by Agentic Memorizer for knowledge graph storage. Runs in a Docker container managed by this subsystem.

**Readiness**
The state when FalkorDB is not just running but accepting Redis protocol connections. Verified by successful PING/PONG exchange.

**Restart Policy**
Docker configuration determining when containers restart automatically. This subsystem uses `unless-stopped` which restarts on crash or Docker daemon restart, but not on explicit stop.

**Subprocess**
A child process spawned to execute Docker CLI commands. Created via Go's `os/exec` package with context timeouts.
