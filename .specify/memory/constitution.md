<!--
SYNC IMPACT REPORT
==================
Version change: 0.0.0 → 1.0.0 (MAJOR - initial constitution ratification)

Modified principles: N/A (initial creation)

Added sections:
  - Core Principles (7 principles)
  - Technology Constraints
  - Development Workflow
  - Governance

Removed sections: N/A (initial creation)

Templates requiring updates:
  - .specify/templates/plan-template.md: ✅ Already aligned (Constitution Check section exists)
  - .specify/templates/spec-template.md: ✅ Already aligned (prioritized user stories, requirements)
  - .specify/templates/tasks-template.md: ✅ Already aligned (TDD references, parallel execution)

Follow-up TODOs: None
-->

# Agentic Memorizer Constitution

## Core Principles

### I. Library-First Design

All features MUST begin as standalone, independently testable libraries before CLI exposure.
Libraries MUST be self-contained with clear boundaries and no external state dependencies.
Each library MUST have a single, well-defined purpose - organizational-only libraries are prohibited.
Public library interfaces MUST be stable; breaking changes require MAJOR version increments.

**Rationale**: Library-first enables unit testing in isolation, promotes reuse, and enforces
clean separation between business logic and I/O concerns.

### II. CLI Interface Contract

Every library capability MUST be exposed via CLI using text-based protocols.
Input flows through stdin and command arguments; output goes to stdout; errors go to stderr.
All CLI commands MUST support both JSON and human-readable output formats via flags.
Exit codes MUST follow Unix conventions: 0 for success, non-zero for specific error categories.

**Rationale**: Text-based I/O enables composition with other Unix tools, scriptability,
and consistent debugging. Structured output (JSON) enables programmatic consumption.

### III. Test-Driven Development (NON-NEGOTIABLE)

TDD is mandatory for all production code: tests written → tests fail → implementation → tests pass.
The Red-Green-Refactor cycle MUST be strictly followed without exception.
No implementation code may be written before a failing test exists for that behavior.
Test coverage MUST include unit, integration, and contract tests as appropriate.

**Rationale**: TDD catches defects early, produces better-designed code, and ensures
every feature has verification. Skipping TDD creates technical debt.

### IV. Unix Philosophy Compliance

Each component MUST do one thing well - no feature creep or scope expansion.
Data exchange MUST use text streams and standard formats (JSON, YAML, plain text).
Components MUST be designed for composition via pipes and standard I/O.
Silence is golden: output only when necessary; support --quiet and --verbose flags.

**Rationale**: Unix philosophy has proven effective for building maintainable, composable
systems over decades. Adherence enables interoperability with the broader ecosystem.

### V. Separation of Concerns

Distinct responsibilities MUST reside in separate packages/modules with clear boundaries.
Business logic MUST NOT contain I/O, persistence, or presentation concerns.
Each layer (data, service, presentation) MUST only depend on layers below it.
Cross-cutting concerns (logging, metrics, auth) MUST be implemented as middleware/decorators.

**Rationale**: Clean separation enables independent testing, evolution, and replacement
of components without cascading changes.

### VI. Composable Subsystems with Loose Coupling

Subsystems MUST communicate through well-defined interfaces, never implementation details.
Dependencies MUST be injected, not instantiated internally, to enable testing and swapping.
Shared state between subsystems is prohibited; use explicit message passing instead.
Interface contracts MUST be versioned; implementations MAY change freely.

**Rationale**: Loose coupling reduces change propagation, enables parallel development,
and allows subsystems to evolve independently.

### VII. Convention over Configuration

Sensible defaults MUST be provided for all configuration options.
Project structure MUST follow established conventions (Go project layout per CLAUDE.md).
Naming conventions MUST be consistent and predictable across the codebase.
Configuration overrides MUST be explicit and discoverable, never hidden.

**Rationale**: Conventions reduce cognitive load, enable tooling, and make codebases
immediately navigable for new contributors.

## Technology Constraints

**Language**: Go (latest stable version)
**CLI Framework**: Cobra with standards per CLAUDE.md (PreRunE validation, variable-based flags)
**Logging**: log/slog library exclusively
**Testing**: Go standard library testing package with table-driven tests
**Project Layout**: Golang Standards Project Layout (excluding pkg/ directory)
**Data Formats**: JSON for structured data, YAML for configuration

All external dependencies MUST be justified and documented.
Prefer standard library solutions over third-party packages when functionality is equivalent.

## Development Workflow

**Code Review Requirements**:
- All changes require review before merge
- Reviewers MUST verify constitution compliance
- TDD evidence (test-first commits) MUST be visible in history

**Quality Gates**:
- All tests MUST pass before merge
- Linting (golangci-lint or equivalent) MUST pass with zero warnings
- No new TODO comments without linked tracking issues

**Commit Standards**:
- Conventional commit format, lowercase, single line
- No co-authoring or generation tool mentions in commit messages

## Governance

This constitution supersedes all other development practices and style guides.
Violations MUST be remediated before code can be merged.
Complexity that violates principles MUST be explicitly justified in design documents.

**Amendment Process**:
1. Propose changes in a dedicated PR with rationale
2. All active contributors must review and approve
3. Migration plan required for breaking changes
4. Update CONSTITUTION_VERSION following semantic versioning

**Versioning Policy**:
- MAJOR: Principle removal, redefinition, or backward-incompatible governance change
- MINOR: New principle added or existing principle materially expanded
- PATCH: Clarifications, typo fixes, non-semantic refinements

**Compliance Review**:
All PRs MUST include a Constitution Check confirming adherence to applicable principles.

**Version**: 1.0.0 | **Ratified**: 2026-01-09 | **Last Amended**: 2026-01-09
