# Container Runtime

Container runtime abstraction for FalkorDB lifecycle management supporting both Docker and Podman with availability detection and readiness polling.

**Documented Version:** v0.14.0

**Last Updated:** 2025-12-30

## Table of Contents

- [Overview](#overview)
- [Design Principles](#design-principles)
- [Key Components](#key-components)
- [Integration Points](#integration-points)
- [Glossary](#glossary)

## Overview

The Container Runtime subsystem provides an abstraction layer for managing the FalkorDB graph database container across multiple container runtimes. Rather than coupling to a specific runtime, the subsystem supports both Docker and Podman through a unified interface. This enables users to choose their preferred container runtime while maintaining consistent application behavior.

The subsystem detects available runtimes, handles runtime-specific networking differences, and provides idempotent lifecycle operations. Docker uses standard port mapping (`-p host:container`) while Podman uses host networking (`--network=host`) to ensure localhost connectivity. All operations use context timeouts to prevent blocking indefinitely.

Key capabilities include:

- **Multi-runtime support** - Docker and Podman with runtime-specific configurations
- **Runtime detection** - Checks if Docker and/or Podman daemons are running
- **Container state inspection** - Detects whether FalkorDB container exists and is running
- **Container lifecycle** - Start and create operations with timeout protection
- **Readiness polling** - Waits for FalkorDB to respond to Redis PING before returning
- **Persistent storage** - Supports bind-mounted data directories for durability
- **Restart policy** - Containers use unless-stopped restart policy for reliability

## Design Principles

### Runtime Abstraction Pattern

The subsystem defines a Runtime type with constants (RuntimeDocker, RuntimePodman, RuntimeNone) enabling runtime-agnostic code. Functions accept the Runtime parameter to specify which runtime to use. The GetRuntime function provides human-readable names for display. This abstraction centralizes runtime-specific logic while exposing a consistent interface.

### CLI Delegation Pattern

Rather than using container client libraries, the subsystem executes CLI commands as subprocesses. This approach eliminates library dependencies, ensures compatibility with the user's installed runtime version, and simplifies debugging (users can run the same commands manually). All container interactions go through `exec.Command` with context timeouts.

### Runtime-Specific Networking

Docker and Podman have different networking defaults. Docker containers use bridged networking requiring explicit port mapping (`-p 6379:6379`). Podman on some platforms (particularly macOS and rootless Linux) has connectivity issues with bridged networking. The subsystem uses `--network=host` for Podman, eliminating port mapping complexity and ensuring localhost connectivity works reliably.

### Context Timeout Protection

Every container operation uses a context with a timeout to prevent indefinite blocking. Availability checks and state inspections use 5-second timeouts. Container creation uses a 60-second timeout to accommodate image pulls. This protects the calling code from hanging if a runtime becomes unresponsive.

### Idempotent Operations

Container operations are designed to be idempotent and handle existing state gracefully. StartFalkorDB returns immediately if the container is already running. It attempts to start an existing stopped container before creating a new one. If an existing container fails to start, it removes and recreates rather than failing. This resilience simplifies calling code.

### Readiness Polling

Starting a container and having it accept connections are distinct events. The subsystem polls for readiness after starting, executing `redis-cli ping` inside the container every 500ms until it receives PONG or the context times out. Callers receive confirmation that FalkorDB is ready to accept connections, not just that the container started.

### Single Container Focus

The subsystem manages exactly one container: `memorizer-falkordb`. This constraint simplifies the API and aligns with the application's single-user, single-graph architecture. The container name is a package constant ensuring consistency across all operations regardless of which runtime is used.

## Key Components

### Runtime Type

The Runtime type is a string alias with three constants: RuntimeDocker (`"docker"`), RuntimePodman (`"podman"`), and RuntimeNone (`""`). Functions accept Runtime parameters to specify the target runtime. RuntimeNone indicates no runtime is available or selected.

### ContainerName Constant

The package defines a single constant `ContainerName = "memorizer-falkordb"` used by all container operations in both Docker and Podman. This ensures consistent naming across availability checks, lifecycle operations, and external configuration.

### StartOptions Struct

The StartOptions struct configures FalkorDB container startup with three fields: Port for the Redis protocol port (default 6379), DataDir for persistent storage bind mount (optional), and Detach for background versus foreground execution (default true).

### IsDockerAvailable Function

The IsDockerAvailable function checks if Docker is installed and running by executing `docker info`. Returns true if the command succeeds within a 5-second timeout. Used by CLI commands and TUI to determine available runtime options.

### IsPodmanAvailable Function

The IsPodmanAvailable function checks if Podman is installed and running by executing `podman info`. Returns true if the command succeeds within a 5-second timeout. Used by CLI commands and TUI to determine available runtime options.

### GetRuntime Function

The GetRuntime function returns a human-readable display name for a Runtime value: "Docker" for RuntimeDocker, "Podman" for RuntimePodman, and empty string for RuntimeNone. Used in TUI messages and CLI output.

### IsFalkorDBRunning Function

The IsFalkorDBRunning function checks if the FalkorDB container is running in any available runtime. It checks Docker first, then Podman. Returns true if the container is running in either runtime. The port parameter is unused but preserved for API consistency.

### IsFalkorDBRunningIn Function

The IsFalkorDBRunningIn function checks if the FalkorDB container is running in a specific runtime. Uses the runtime's inspect command to query container state, returning true only if the container exists and its Running state is true.

### containerExistsIn Function

The containerExistsIn function checks whether a container with the memorizer-falkordb name exists in a specific runtime, regardless of running state. Uses the runtime's inspect command which succeeds for both running and stopped containers.

### StartFalkorDB Function

The StartFalkorDB function is the primary entry point for container lifecycle. It accepts a Runtime parameter specifying which runtime to use (returns error for RuntimeNone). It first checks if the container is already running (returns immediately if so). If a stopped container exists, it attempts to start it. If that fails, it removes and recreates. For new containers, it builds runtime-specific arguments via buildDockerArgs or buildPodmanArgs. After container creation, it calls waitForReady to confirm FalkorDB accepts connections.

### buildDockerArgs Function

The buildDockerArgs function constructs Docker-specific run arguments: port mapping for Redis (`-p hostPort:6379`) and browser UI (`-p 3000:3000`), volume mount for data persistence (optional), unless-stopped restart policy, and the latest FalkorDB image.

### buildPodmanArgs Function

The buildPodmanArgs function constructs Podman-specific run arguments: `--network=host` for localhost connectivity (no port mapping needed), volume mount for data persistence (optional), unless-stopped restart policy, and the latest FalkorDB image.

### waitForReady Function

The waitForReady function polls FalkorDB for readiness by executing `redis-cli ping` inside the container every 500ms. Returns nil when PONG is received, or an error if the context times out. This internal function ensures StartFalkorDB only returns after FalkorDB is accepting connections.

## Integration Points

### Initialization TUI

The terminal UI initialization flow uses IsDockerAvailable, IsPodmanAvailable, and IsFalkorDBRunning to determine which FalkorDB configuration options to present. Based on available runtimes, it shows appropriate options:
- Both available: "Start FalkorDB in Docker", "Start FalkorDB in Podman", plus manual options
- Docker only: "Start FalkorDB in Docker" plus manual options
- Podman only: "Start FalkorDB in Podman" plus manual options
- Neither: Manual configuration options only

The TUI tracks the selected runtime and passes it to StartFalkorDB when the user confirms.

### Initialize Command

The `memorizer initialize` command provides `--start-falkordb-docker` and `--start-falkordb-podman` flags for unattended setup. These flags are mutually exclusive. The command validates that the specified runtime is available before attempting to start FalkorDB.

### Docker Compose

The project includes a docker-compose.yml providing an alternative to the subsystem's direct CLI commands. The compose file uses the same container name and configuration, allowing users to choose their preferred container management approach. Both methods produce compatible containers.

### Configuration System

The config subsystem provides FalkorDB connection settings (host, port, password) used by the initialization TUI when calling StartFalkorDB. The DataDir for persistent storage is derived from the application directory rather than config, ensuring data locality.

## Glossary

**Bind Mount**
A container volume type that maps a host directory directly into the container. Used for FalkorDB data persistence at `~/.memorizer/falkordb/`.

**Container Runtime**
Software that runs containers: Docker or Podman. This subsystem abstracts over both runtimes.

**Container State**
The running status of a container: created, running, paused, restarting, removing, exited, or dead. This subsystem primarily distinguishes between running and not-running.

**Docker**
A popular container runtime using the Docker daemon. This subsystem uses Docker CLI commands executed as subprocesses.

**FalkorDB**
A Redis-compatible graph database used by Agentic Memorizer for knowledge graph storage. Runs in a container managed by this subsystem.

**Host Networking**
A container networking mode where the container shares the host's network namespace. Used by Podman via `--network=host` for reliable localhost connectivity.

**Podman**
A daemonless container runtime compatible with Docker images. This subsystem uses `--network=host` for Podman to ensure localhost connectivity.

**Port Mapping**
Docker's method of exposing container ports on the host via `-p hostPort:containerPort`. Not used for Podman which uses host networking instead.

**Readiness**
The state when FalkorDB is not just running but accepting Redis protocol connections. Verified by successful PING/PONG exchange.

**Restart Policy**
Container configuration determining when containers restart automatically. This subsystem uses `unless-stopped` which restarts on crash or daemon restart, but not on explicit stop.

**Subprocess**
A child process spawned to execute container CLI commands. Created via Go's `os/exec` package with context timeouts.
