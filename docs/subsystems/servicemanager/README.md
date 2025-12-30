# Service Manager

Platform-specific service integration for systemd (Linux) and launchd (macOS) enabling automatic daemon startup and system-level supervision.

**Documented Version:** v0.14.0

**Last Updated:** 2025-12-29

## Table of Contents

- [Overview](#overview)
- [Design Principles](#design-principles)
- [Key Components](#key-components)
- [Integration Points](#integration-points)
- [Glossary](#glossary)

## Overview

The Service Manager subsystem generates platform-specific service configuration files that enable the daemon to run as a system-managed background process. On Linux systems, it produces systemd unit files; on macOS, it produces launchd property list (plist) files. Both platforms support user-level and system-level installation, with appropriate security settings and restart policies.

The subsystem handles platform detection, binary path resolution, configuration file generation, and installation with clear user instructions. It enables the daemon to start automatically at login or boot, restart on failure, and integrate with system logging facilities.

Key capabilities include:

- **Platform Detection** - Automatic identification of Linux or macOS runtime environment
- **Binary Path Resolution** - Locates the memorizer binary via executable path, common paths, or PATH
- **Systemd Integration** - Generates unit files with sd_notify support, security hardening, and restart policies
- **Launchd Integration** - Generates plist files with KeepAlive, ThrottleInterval, and environment configuration
- **User and System Modes** - Supports both user-level (no root) and system-level (root required) installation
- **Installation Instructions** - Provides complete setup commands for each platform and mode

## Design Principles

### Platform-Specific Templates

Each platform has its own configuration format and conventions. Systemd uses INI-style unit files with sections like [Unit], [Service], and [Install]. Launchd uses XML property lists with keys for program arguments, environment, and process settings. Rather than attempting a unified abstraction, the subsystem provides dedicated generators for each format that leverage platform-specific features.

### User-Level Preference

User-level installation is preferred because it requires no root privileges and runs the daemon in the user's security context. The systemd user units live in `~/.config/systemd/user/` and use `WantedBy=default.target`. The launchd agents live in `~/Library/LaunchAgents/` and run when the user logs in. System-level installation is available when persistent operation is required regardless of user login state.

### Security Hardening

Generated service configurations include security settings appropriate for the platform. Systemd unit files include `NoNewPrivileges=true` to prevent privilege escalation and `PrivateTmp=true` for temporary file isolation. Launchd plists set `ProcessType=Background` for appropriate CPU scheduling. Both platforms configure restart-on-failure with reasonable delays to prevent crash loops.

### Automatic Restart

Both platforms configure the service to restart automatically after failure. Systemd uses `Restart=on-failure` with `RestartSec=5s`. Launchd uses `KeepAlive` with `SuccessfulExit=false` and `ThrottleInterval=30` seconds. This ensures the daemon recovers from transient failures without requiring manual intervention.

### Binary Path Resolution

The subsystem must locate the memorizer binary to include its absolute path in service configurations. It tries multiple locations in order: the currently executing binary path (resolved through symlinks), the common installation path (`~/.local/bin/memorizer`), and finally the system PATH. This allows the generated configuration to work regardless of how the user installed the application.

## Key Components

### Platform Detection

The servicemanager.go file provides platform detection via `DetectPlatform()` which returns `PlatformLinux`, `PlatformDarwin`, or `PlatformUnknown` based on `runtime.GOOS`. The `IsPlatformSupported()` function indicates whether service manager integration is available. The `GetBinaryPath()` function locates the memorizer binary through multiple resolution strategies.

### Systemd Module

The systemd.go file provides systemd unit file generation for Linux. The `SystemdConfig` struct holds binary path, user, home directory, and log file settings. `GenerateUserUnit()` produces a user-level unit file for `default.target`, while `GenerateSystemUnit()` produces a system-level unit file for `multi-user.target`. Both include Type=notify for sd_notify integration, restart policies, timeout configuration, and security hardening.

Installation functions include `GetUserUnitPath()` returning `~/.config/systemd/user/memorizer.service`, `GetSystemUnitPath()` returning `/etc/systemd/system/memorizer.service`, and `InstallUserUnit()` for writing user-level unit files. Instruction generators provide complete setup commands for both modes.

### Launchd Module

The launchd.go file provides plist generation for macOS. The `LaunchdConfig` struct holds binary path, user, home directory, and log file settings. `GeneratePlist()` produces an XML plist with the service label (derived from username), program arguments, working directory, KeepAlive settings, log paths, environment variables, and throttle interval.

Installation functions include `GetUserAgentPath()` returning `~/Library/LaunchAgents/com.{user}.memorizer.plist`, `GetSystemDaemonPath()` returning `/Library/LaunchDaemons/com.{user}.memorizer.plist`, and `InstallUserAgent()` for writing user-level agent files. Instruction generators provide complete launchctl commands for both modes.

## Integration Points

### CLI Commands

The `daemon systemctl` command uses `GenerateUserUnit()` and `GenerateSystemUnit()` to produce systemd configuration with installation instructions. The `daemon launchctl` command uses `GeneratePlist()` to produce launchd configuration with installation instructions. Both commands use `GetBinaryPath()` to locate the binary and display appropriate instructions based on the `--system` flag.

### Daemon Subsystem

The generated service configurations invoke `memorizer daemon start` as the main process. The daemon supports sd_notify on systemd via Type=notify, signaling readiness after initialization. On launchd, the daemon runs as a long-lived process with KeepAlive ensuring restart on unexpected termination.

### Configuration Subsystem

The service configurations include environment variable settings to ensure the HOME variable is set correctly, which is necessary for config file discovery at `~/.memorizer/config.yaml`. The working directory is set to the user's home directory for consistent path resolution.

### Logging Subsystem

Systemd captures stdout/stderr to the journal, accessible via `journalctl --user -u memorizer` for user-level or `journalctl -u memorizer` for system-level. Launchd directs stdout and stderr to the configured log file path, typically within `~/.memorizer/logs/`.

## Glossary

**Launchd**
The macOS service management framework that handles starting, stopping, and supervising daemon processes. Uses XML plist files for configuration and the launchctl command for control.

**Plist**
Property list file, an XML format used by macOS for configuration. Launchd plists define service properties like program path, environment, and restart behavior.

**sd_notify**
The systemd notification protocol allowing services to signal startup completion. When Type=notify is set, systemd waits for the service to send READY=1 before considering it started.

**Systemd**
The Linux service manager that handles starting, stopping, and supervising system services. Uses unit files for configuration and the systemctl command for control.

**Unit File**
Systemd configuration file defining a service, socket, timer, or other managed resource. Service unit files have sections for dependencies ([Unit]), execution ([Service]), and installation ([Install]).

**User-Level Service**
A service that runs in the user's security context without root privileges. Managed via `systemctl --user` on Linux or user LaunchAgents on macOS.
