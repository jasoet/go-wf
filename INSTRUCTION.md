# Project Instructions

<!-- This file is the single source of truth for AI assistants working on this project. -->
<!-- AI: Read this file at the start of every session. Update it when conventions, -->
<!-- architecture, or key paths change. Also keep README.md in sync. -->
<!-- If Obsidian MCP is available, check the vault for additional knowledge, best -->
<!-- practices, and reference notes. Ask the user for the vault path if needed. -->

## Project Overview

go-wf — a Go library providing a generic workflow orchestration core with Docker container, Go function, and data synchronization activity support, built on Temporal. The `workflow/` package defines type-safe interfaces (`TaskInput`/`TaskOutput`) using Go generics for pipeline, parallel, loop, and single-task execution. The `container/` package is a concrete implementation that wires Docker container activities into the generic core. The `function/` package provides a function registry pattern where named Go handler functions are dispatched as Temporal activities. The `datasync/` package provides generic `Source[T] -> Mapper[T,U] -> Sink[U]` data synchronization pipelines as Temporal workflows. Built with Go 1.26+, uses `github.com/jasoet/pkg/v2` as the base library. Features include a fluent builder API, container/script/HTTP templates, artifact storage, and lifecycle management.

**Repository Type:** Library (Go module)
**Module:** `github.com/jasoet/go-wf`
**Key Dependencies:** Temporal SDK, testcontainers-go, pkg/v2, validator/v10, aws-sdk-go-v2

## ABSOLUTE RULE — Git Authorship

**NEVER add AI (Claude, Copilot, or any AI) as co-author, committer, or contributor in git commits.**
Only the user's registered email may appear in commits. This is company policy — commits with AI
authorship WILL BE REJECTED. Do not use `--author`, `Co-authored-by`, or any other mechanism to
attribute commits to AI. This applies to ALL commits, including those made by tools and subagents.

## Conventions

- **Python**: Always use `uv` (never pip, conda, pipenv). All scripts must be Python, no bash scripts.
- **Node.js**: Always use `bun`/`bunx` (never node, npm, npx).
- **Commands**: Always use `task <name>` to run commands. Run `task --list` to discover available tasks. If a command is important or repeated but has no task, suggest adding it to `Taskfile.yml` with a proper description.
- **Brainstorming**: When the user starts a new topic or plans something, always use the brainstorming skill first. If unsure whether to brainstorm, ask the user.
- **Superpowers**: Ensure superpowers skills are installed and use them. Brainstorm before features, TDD for implementation, systematic-debugging for bugs.
- **README.md**: Human-readable project documentation. Update when user-facing behavior changes.
- **INSTRUCTION.md**: AI-readable project context (this file). Update when project conventions, architecture, or key paths change.
- **Scripts**: For complex automation, create proper UV projects (Python) or Bun projects (TypeScript) with `pyproject.toml`/`package.json`. Always `.gitignore` generated files (`.venv/`, `node_modules/`, `__pycache__/`).
- **Containers**: Always use Podman (never Docker). Use `podman` and `podman-compose` instead of `docker` and `docker-compose`. Migrate any existing Docker references to Podman.
- **Commits**: Use [Conventional Commits](https://www.conventionalcommits.org/). Format: `<type>(<scope>): <description>`. Types: `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`, `perf`, `ci`.
- **Branching**: Create a new branch for each feature or fix (`feat/...`, `fix/...`). Create PR (GitHub, use `gh`) or MR (GitLab, use `glab`) when ready. Always squash commits on merge. Use `gh`/`glab` to check PR/MR status and CI pipeline results before merging.

## Key Paths

| Path | Purpose |
|------|---------|
| `workflow/` | Generic workflow core (interfaces, orchestration logic) |
| `workflow/errors/` | Error types and handling |
| `workflow/artifacts/` | Artifact store (local + S3) |
| `workflow/testutil/` | Shared test helpers (Temporal testcontainer) |
| `container/` | Container workflows (concrete implementation) |
| `container/activity/` | Temporal activities for container execution |
| `container/builder/` | Fluent builder API for workflows |
| `container/patterns/` | Pre-built patterns (CI/CD, loop, parallel) |
| `container/payload/` | Type-safe payload structs |
| `container/template/` | Container, script, and HTTP templates |
| `container/workflow/` | Workflow implementations (container, pipeline, parallel, DAG, loop) |
| `function/` | Go function activities (concrete implementation) |
| `function/activity/` | Temporal activity for function dispatch |
| `function/builder/` | Fluent builder API for function workflows (including DAG) |
| `function/patterns/` | Pre-built patterns (pipeline, parallel, loop, DAG) |
| `function/payload/` | Type-safe payload structs for functions (including DAG) |
| `function/workflow/` | Workflow implementations (function, pipeline, parallel, loop, DAG) |
| `datasync/` | Generic data sync core (Source, Sink, Mapper, Job, Runner) |
| `datasync/activity/` | SyncData activity with OTel instrumentation |
| `datasync/builder/` | Fluent builder for Job construction |
| `datasync/payload/` | Temporal payload types (SyncExecutionInput/Output) |
| `datasync/workflow/` | Sync workflow function, registration, scheduling helpers |
| `workflow/otel.go` | Instrumented workflow orchestration wrappers |
| `container/activity/otel.go` | Container activity OTel spans + metrics |
| `function/activity/otel.go` | Function activity OTel spans + metrics |
| `workflow/artifacts/otel.go` | Instrumented artifact store decorator |
| `compose.yml` | Podman compose for local Temporal infrastructure |
| `examples/container/` | Container example code (build tag: `//go:build example`) |
| `examples/function/` | Function example code (build tag: `//go:build example`) |
| `examples/function/worker/` | Shared function worker for all examples |
| `examples/trigger/` | Trigger CLI for submitting and scheduling workflows |
| `docs/plans/` | New implementation plans |
| `docs/plans/archived/` | Completed implementation plans |
| `Taskfile.yml` | All project commands |
| `.claude/` | Claude Code hooks, scripts, and skills |
| `.github/workflows/` | GitHub Actions CI/CD |
| `INSTRUCTION.md` | AI context (this file) |
| `README.md` | Human documentation |

## Taskfile Commands

| Task | Description |
|------|-------------|
| `task` | List all available tasks |
| `task test` | Run all tests (unit + integration) with coverage — container engine required |
| `task test:unit` | Run unit tests only (fast, no container engine required) |
| `task test:integration` | Run integration tests only (container engine required) |
| `task container:check` | Check Docker or Podman availability and daemon status |
| `task lint` | Run golangci-lint |
| `task check` | Run all checks (test + lint) |
| `task tools` | Install development tools (golangci-lint, gofumpt, goimports) |
| `task fmt` | Format all Go files (goimports with local prefix + gofumpt) |
| `task ci:test` | Run unit tests for CI (no coverage HTML) |
| `task ci:lint` | Run golangci-lint for CI |
| `task ci:check` | Run all CI checks (test + lint) |
| `task release` | Run semantic-release (CI only) |
| `task release:proxy-warmup` | Warm Go module proxy with latest tag |
| `task test:pkg` | Run unit tests for a specific package (`task test:pkg -- ./function/workflow/...`) |
| `task test:run` | Run a specific test by name (`task test:run -- -run TestName ./package/...`) |
| `task test:coverage` | Show coverage for a specific package (`task test:coverage -- ./function/workflow/...`) |
| `task example:container` | Run a container example (`task example:container -- basic.go`) |
| `task example:container:worker` | Start the container example worker (listens on container-tasks queue) |
| `task example:function` | Run a function example (`task example:function -- basic.go`) |
| `task example:list` | List all available example files |
| `task demo` | Start Temporal, run all examples, watch at http://localhost:8233 |
| `task demo:start` | Start Temporal + container worker in background for manual example running |
| `task demo:stop` | Stop Temporal + container worker started by `demo:start` |
| `task local:up` | Start local infrastructure (Temporal, PostgreSQL, RustFS) via podman-compose |
| `task local:down` | Stop local infrastructure |
| `task local:clean` | Stop infrastructure and remove volumes |
| `task local:workers` | Start container and function workers in background |
| `task local:workers:stop` | Stop background workers |
| `task local:trigger` | Submit all example workflows once |
| `task local:schedule` | Create recurring workflow schedules |
| `task local:schedule:clean` | Remove all workflow schedules |
| `task local:start` | Start everything (infra + workers + trigger + schedules) |
| `task local:stop` | Stop everything (schedules + workers + infra) |
| `task clean` | Clean build artifacts |

## Architecture

Multi-layer architecture organized as package-per-feature:

**Generic Workflow Core (`workflow/`)**
- Defines `TaskInput` and `TaskOutput` interface constraints using Go generics
- Provides generic orchestration: pipeline, parallel, loop, and single-task execution
- Activity dispatch via `ActivityName()` (string-based, not function reference) for Temporal compatibility
- Artifact storage abstraction (local filesystem, S3-compatible storage)
- Error types shared across all implementations

**Container Module (`container/`)** — concrete implementation
- **Activities** wrap `github.com/jasoet/pkg/v2/docker` for container execution
- **Payloads** implement `TaskInput`/`TaskOutput` interfaces with validated structs (`go-playground/validator`)
- **Workflows** register with Temporal workers via `container.RegisterAll(w)`, using generic core for orchestration
- **Builder** provides a fluent API to compose container → pipeline → parallel → DAG
- **Templates** (container, script, HTTP) generate payload structs from higher-level config
- **Patterns** are pre-built workflow compositions (CI/CD pipelines, fan-out/fan-in, etc.)

**Function Module (`function/`)** — concrete implementation
- **Registry** maps named Go handler functions (`func(ctx, FunctionInput) (*FunctionOutput, error)`) for dispatch
- **Activity** dispatches to registered handlers via closure over the registry
- **Payloads** implement `TaskInput`/`TaskOutput` interfaces with validated structs
- **Workflows** register with Temporal workers, using generic core for orchestration (pipeline, parallel, loop, DAG)
- **Builder** provides a fluent API to compose function → pipeline → parallel → loop → DAG
- **Patterns** are pre-built workflow compositions (ETL, fan-out/fan-in, batch processing, CI/CD DAG, etc.)

**DataSync Module (`datasync/`)** — concrete implementation
- **Core interfaces** (`Source[T]`, `Sink[U]`, `Mapper[T,U]`) define the generic data pipeline
- **Helpers** (`RecordMapper`, `InsertIfAbsentSink`, `IdentityMapper`, `MapperFunc`) cover common patterns
- **Job** bundles Source + Mapper + Sink + schedule into a deployable unit
- **Runner** provides in-process test execution (no Temporal needed)
- **Activity** runs the Source→Mapper→Sink pipeline with full OTel instrumentation (`go_wf.datasync.*` metrics)
- **Workflow** wraps the activity in a Temporal workflow with scheduling support
- **Builder** provides a fluent API for Job construction
- **Payloads** (`SyncExecutionInput`/`SyncExecutionOutput`) implement `TaskInput`/`TaskOutput` for composition with Pipeline, Parallel, and DAG

**Observability (`jasoet/pkg/v2/otel`)**
- Activities get full OTel spans + metrics via `Layers.StartService` (container: `go_wf.container.task.*`, function: `go_wf.function.task.*`, datasync: `go_wf.datasync.*`)
- Workflow orchestration has structured logging wrappers at pipeline/parallel/loop boundaries
- Artifact store uses `InstrumentedStore` decorator with `Layers.StartRepository` (metrics: `go_wf.artifact.operation.*`)
- All instrumentation is opt-in via `otel.ContextWithConfig()` — zero overhead when disabled
- Three-signal correlation: traces, logs, and metrics share the same trace context

## Testing Strategy

- **Coverage target**: 85%+
- **Unit tests**: `*_test.go` — no build tags, no external dependencies, fast. Uses Temporal `TestWorkflowEnvironment` with mocked activities.
- **Integration tests**: `*_integration_test.go` — `//go:build integration` tag, uses testcontainers, requires Docker/Podman
- **Example code**: `//go:build example` tag in `examples/`
- **Assertion library**: `github.com/stretchr/testify` (assert + require)
- **Pattern**: Table-driven tests preferred
- **Run unit only**: `task test:unit` (fast, no container engine)
- **Run integration only**: `task test:integration` (container engine required)
- **Run all**: `task test` (unit + integration, container engine required)

## Quality Standards

- Zero golangci-lint errors
- gofumpt formatting enforced
- goimports with local prefix `github.com/jasoet/go-wf` (3-group imports: stdlib, third-party, local)
- Cyclomatic complexity < 20
- Function length < 100 lines / 50 statements (`funlen`)
- Code duplication threshold: 100 tokens (`dupl`)
- Line length < 190 characters
- All exported functions documented
- Comments on declarations must end in a period (`godot`)
- Security: gosec scanning, no hardcoded secrets, directory permissions ≤ 0o750, file permissions ≤ 0o600

## Go: Base Library (pkg/v2)

All Go projects use `github.com/jasoet/pkg/v2` as the base library.
Read the following files from `~/Documents/Go/pkg/` for full details:
- `README.md`: Available packages and quick start
- `CLAUDE.md`: Architecture patterns, testing strategy, development commands
- `PROJECT_TEMPLATE.md`: Recommended project structure and wiring patterns

Key patterns:
- Functional Options for all configuration (`WithXxx()` functions)
- OTelConfig injected via functional options, never serialized (`yaml:"-" mapstructure:"-"`)
- LayerContext for automatic span + logger correlation
- No-op provider pattern: OTel gracefully defaults to no-op when nil
