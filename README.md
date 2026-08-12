# go-wf

[![Go Version](https://img.shields.io/badge/Go-1.26+-blue.svg)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Build Status](https://github.com/jasoet/go-wf/v2/actions/workflows/release.yml/badge.svg)](https://github.com/jasoet/go-wf/v2/actions)

Temporal workflow library providing reusable, production-ready workflows for common orchestration patterns.

## Features

- **Container Workflows** - Execute containers (Podman/Docker) with Temporal orchestration
- **Function Workflows** - Execute registered Go functions with Temporal orchestration
- **DataSync Workflows** - Generic Source/Mapper/Sink data synchronization pipelines, including partitioned/chunked sync via `datasync/chunk`
- **Type-Safe Payloads** - Validated input/output structures
- **Unified Builder API** - All builders produce `*job.Definition` (from `github.com/jasoet/pkg/v2/temporal/job`), providing a single type for registration, execution, scheduling, and lifecycle management
- **Production-Ready** - Built-in retries, timeouts, error handling
- **Observable** - Built-in OpenTelemetry instrumentation (traces, logs, metrics) with zero overhead when disabled
- **Comprehensive Testing** - 91.2% total coverage (unit + integration); CI enforces a minimum of 85%
- **Full CI/CD** - Automated releases and quality checks

## job.Definition — Unified Workflow Handle

Every builder in go-wf (`container/builder`, `function/builder`, `datasync/builder`, `datasync/chunk`) returns a `*job.Definition` from its `Build()` method. A `*job.Definition` is the single type you need for all per-job operations:

```go
import "github.com/jasoet/pkg/v2/temporal/job"

def, err := container.NewWorkflowBuilder().Name("deploy").Single().Add(myContainer).Build()
if err != nil { log.Fatal(err) }

// Register workflows + activities on a worker (idempotent)
def.Register(w)

// Start a workflow run
run, err := def.Execute(ctx, c, def.NewInput())

// Manage lifecycle
def.Cancel(ctx, c, wfID, runID, "reason")
def.Describe(ctx, c, wfID, runID)
def.ListRuns(ctx, c, opts)

// Schedule management
def.ApplySchedule(ctx, c, opts)
def.PauseSchedule(ctx, c, scheduleID, "reason")
```

A `job.Registry` aggregates multiple Definitions, allowing you to register all jobs at once and dispatch by name:

```go
registry := job.NewRegistry()
registry.Add(ordersJobDef)
registry.Add(usersJobDef)
registry.RegisterAll(w) // registers every Definition on the worker

registry.MustGet("orders").Execute(ctx, c, input)
```

## Documentation

| Guide | Description |
|-------|-------------|
| [Architecture](./docs/architecture.md) | Architecture and design |
| [Getting Started](./docs/getting-started.md) | Quick start guide |
| [Workflow Patterns](./docs/workflow-patterns.md) | Orchestration patterns (pipeline, parallel, loop, DAG) |
| [Container Workflows](./docs/container-workflows.md) | Container workflow guide |
| [Function Workflows](./docs/function-workflows.md) | Function workflow guide |
| [DataSync Workflows](./docs/datasync-workflows.md) | Data synchronization guide |
| [Store](./docs/store.md) | Store API (RawStore, Store[T]) |
| [Security](./docs/security.md) | Trust boundaries and secret handling |
| [Observability](./docs/observability.md) | OpenTelemetry integration |
| [Contributing](./docs/contributing.md) | Development and contribution guide |

## Packages

### [workflow](./workflow/)

Generic workflow orchestration core using Go generics with type-safe `TaskInput`/`TaskOutput` constraints. Provides pipeline, parallel, loop, and single-task execution patterns, plus pluggable artifact storage (local + S3). See [Workflow Patterns](./docs/workflow-patterns.md) for details.

### [container](./container/)

Temporal workflows for executing containers with Argo Workflow-like capabilities: single container, pipeline, parallel, and DAG execution. Includes a fluent builder API, container/script/HTTP templates, and pre-built patterns. See [Container Workflows](./docs/container-workflows.md) for details.

### [function](./function/)

Temporal workflows for executing registered Go functions: function registry, pipeline, parallel, loop, and DAG execution with a fluent builder API and pre-built patterns. See [Function Workflows](./docs/function-workflows.md) for details.

### [datasync](./datasync/)

Generic data synchronization workflows using a `Source[T] -> Mapper[T,U] -> Sink[U]` pipeline with helpers for record mapping, deduplication, and Temporal scheduling. See [DataSync Workflows](./docs/datasync-workflows.md) for details.

### [datasync/chunk](./datasync/chunk/)

Partitioned sync builder for large datasets. `ChunkedSync` walks a sequence of partitions (e.g., date ranges), runs fetch/map/write per partition with cursor-based resume and `ContinueAsNew` for long partition lists. See [DataSync Workflows](./docs/datasync-workflows.md) for details.

## Observability

Built-in OpenTelemetry instrumentation (traces, logs, metrics) with zero overhead when disabled. See [Observability](./docs/observability.md) for details.

## Installation

```bash
go get github.com/jasoet/go-wf/v2
```

## Quick Start

### Container Workflow

```go
package main

import (
    "context"
    "log"

    "github.com/jasoet/go-wf/v2/container"
    "github.com/jasoet/go-wf/v2/container/builder"
    "github.com/jasoet/go-wf/v2/container/payload"
    "github.com/jasoet/pkg/v2/temporal"
    "go.temporal.io/sdk/worker"
)

func main() {
    // Create Temporal client
    c, err := temporal.NewClient(temporal.DefaultConfig())
    if err != nil {
        log.Fatal(err)
    }
    defer c.Close()

    // Build a job Definition (recommended pattern)
    def, err := builder.NewWorkflowBuilder().
        Name("postgres-setup").
        Single().
        AddInput(payload.ContainerExecutionInput{
            Image: "postgres:16-alpine",
            Env: map[string]string{
                "POSTGRES_PASSWORD": "test", // see docs/security.md for secret:// references
            },
            Ports:      []string{"5432:5432"},
            AutoRemove: true,
        }).
        Build()
    if err != nil {
        log.Fatal(err)
    }

    // Register and start worker
    w := worker.New(c, def.TaskQueue, worker.Options{})
    def.Register(w)
    go func() { _ = w.Run(worker.InterruptCh()) }()
    defer w.Stop()

    // Execute workflow
    run, err := def.Execute(context.Background(), c, def.NewInput())
    if err != nil {
        log.Fatal(err)
    }

    var result payload.ContainerExecutionOutput
    _ = run.Get(context.Background(), &result)
    log.Printf("Container executed: %s", result.ContainerID)
}
```

For callers that register workflows manually, `container.RegisterAll(w)` is also available (idempotent — safe to call multiple times).

### Function Workflow

```go
package main

import (
    "context"
    "log"

    fn "github.com/jasoet/go-wf/v2/function"
    fnactivity "github.com/jasoet/go-wf/v2/function/activity"
    fnbuilder "github.com/jasoet/go-wf/v2/function/builder"
    "github.com/jasoet/go-wf/v2/function/payload"
    "github.com/jasoet/pkg/v2/temporal"
    "go.temporal.io/sdk/worker"
)

func main() {
    // Create Temporal client
    c, err := temporal.NewClient(temporal.DefaultConfig())
    if err != nil {
        log.Fatal(err)
    }
    defer c.Close()

    // Register a handler
    registry := fn.NewRegistry()
    _ = registry.Register("greet", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
        return &fn.FunctionOutput{
            Result: map[string]string{"greeting": "Hello, " + input.Args["name"] + "!"},
        }, nil
    })

    // Build a job Definition
    def, err := fnbuilder.NewWorkflowBuilder[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput]().
        Name("greet-job").
        Single().
        Activity(fnactivity.NewExecuteFunctionActivity(registry)).
        Build()
    if err != nil {
        log.Fatal(err)
    }

    // Register and start worker
    w := worker.New(c, def.TaskQueue, worker.Options{})
    def.Register(w)
    go func() { _ = w.Run(worker.InterruptCh()) }()
    defer w.Stop()

    // Execute workflow
    run, err := def.Execute(context.Background(), c,
        &payload.FunctionExecutionInput{
            Name: "greet",
            Args: map[string]string{"name": "Temporal"},
        },
    )
    if err != nil {
        log.Fatal(err)
    }

    var result payload.FunctionExecutionOutput
    _ = run.Get(context.Background(), &result)
    log.Printf("Result: %v", result.Result)
}
```

## Project Structure

```
go-wf/
├── workflow/         # Generic workflow core (interfaces, orchestration)
│   ├── errors/       # Error types and handling
│   └── artifacts/    # Artifact store (local + S3)
├── container/           # Container workflows (concrete implementation)
│   ├── activity/     # Temporal activities for container execution
│   ├── builder/      # Fluent builder API
│   ├── patterns/     # Pre-built patterns (CI/CD, loop, parallel)
│   ├── payload/      # Type-safe payload structs
│   ├── template/     # Container, script, HTTP templates
│   └── workflow/     # Workflow implementations
├── function/         # Go function activities (concrete implementation)
│   ├── activity/     # Temporal activity for function dispatch
│   ├── builder/      # Fluent builder API
│   ├── payload/      # Type-safe payload structs
│   └── workflow/     # Workflow implementations
├── datasync/         # Generic data sync (Source → Mapper → Sink)
│   ├── activity/     # SyncData activity with OTel
│   ├── builder/      # Fluent builder for Job construction
│   ├── chunk/        # Partitioned/chunked sync builder (ChunkedSync, DateChunkedSync)
│   ├── internal/
│   │   └── heartbeat/ # Shared heartbeat helpers (used by activity and chunk)
│   ├── payload/      # Temporal payload types
│   └── workflow/     # Workflow function and registration
├── examples/container/    # Container examples (see [README](./examples/container/README.md))
├── examples/function/  # Function examples (see [README](./examples/function/README.md))
├── docs/               # Project documentation (architecture, guides, API)
├── docs/plans/         # Implementation plans (archived/)
├── .github/          # GitHub Actions workflows
├── Taskfile.yml      # Task automation
└── README.md         # This file
```

## AI Agent Instructions

**Repository Type:** Library

**Critical Setup:**
- Go 1.26+
- Docker or Podman for integration tests
- Task CLI for automation

**Architecture:**
- Go module-based library
- Package-per-feature organization
- Testcontainer-based integration tests
- Semantic versioning with conventional commits

**Key Development Patterns:**
1. **Always read files before editing** - Use Read tool first
2. **Follow existing code style** - gofumpt formatting enforced
3. **Write table-driven tests** - All tests use table-driven pattern
4. **Integration tests use testcontainers** - Tag with `//go:build integration`
5. **Examples use build tags** - Tag with `//go:build example`
6. **Security first** - No hardcoded secrets, validate inputs

**Testing Strategy:**
- Coverage: 91.2% measured (unit + integration); CI enforces a minimum of 85% via `scripts/check-coverage.sh`
- Unit tests: `task test:unit` (fast, no container engine)
- Integration tests: `task test:integration` (requires Docker/Podman)
- All tests: `task test` (unit + integration, requires Docker/Podman)
- Test files: `*_test.go` (unit), `*_integration_test.go` with `//go:build integration` tag

**Quality Standards:**
- Zero golangci-lint errors
- gofumpt formatting
- Cyclomatic complexity < 20
- Line length < 190 characters
- All exported functions documented

**Development Commands:**
```bash
task                   # List all available tasks
task test:unit         # Run unit tests only (fast, no container engine)
task test              # Run all tests with coverage (container engine required)
task test:integration  # Run integration tests only (container engine required)
task lint              # Run golangci-lint
task fmt               # Format code (goimports + gofumpt)
task check             # Run all checks (test + lint)
task tools             # Install development tools
task clean             # Clean build artifacts
```

**Commit Message Format:**
Use conventional commits for automatic versioning:
- `feat:` - New feature (minor version bump)
- `fix:` - Bug fix (patch version bump)
- `BREAKING CHANGE:` - Breaking change (major version bump)
- `docs:`, `test:`, `chore:`, `refactor:` - Patch version bump

**Package Structure:**
```
packagename/
├── README.md              # Package documentation
├── packagename.go         # Main implementation
├── packagename_test.go    # Unit tests
├── integration_test.go    # Integration tests (//go:build integration)
└── examples/
    ├── README.md
    └── basic.go           # Example code (//go:build example)
```

**File Organization Rules:**
1. One package per directory
2. Keep packages focused and cohesive
3. Minimize inter-package dependencies
4. Export only what's necessary

**Error Handling:**
- Always check errors
- Use meaningful error messages
- Wrap errors with context
- Don't panic in library code

**Before Committing:**
- [ ] `task fmt` - Format code
- [ ] `task test` - Run unit tests
- [ ] `task lint` - Check code quality
- [ ] Update tests for changes
- [ ] Update documentation if needed
- [ ] Follow conventional commit format

**Workflow for New Features:**
1. Create feature branch: `git checkout -b feat/feature-name`
2. Implement with tests (TDD preferred)
3. Run `task check` to verify
4. Commit with conventional format
5. Push and create PR
6. Wait for CI/CD to pass

## Development

### Prerequisites

- Go 1.26 or higher
- Docker or Podman (for integration tests)
- Task CLI

### Setup

```bash
# Install development tools
task tools

# Run tests
task test

# Run linter
task lint

# Format code
task fmt
```

### Testing

```bash
# Unit tests only (fast, no container engine)
task test:unit

# Integration tests only (requires Docker/Podman)
task test:integration

# All tests with coverage report (requires Docker/Podman)
task test
# Open output/coverage.html to view coverage
```

### Local Environment

Run all examples with persistent Temporal infrastructure for manual workflow inspection via the UI:

```bash
# One command to start everything (Temporal, workers, trigger workflows, schedules)
task local:start

# Stop everything
task local:stop

# Clean up (remove all data volumes)
task local:clean
```

**Services:**
- Temporal UI: http://localhost:8233
- RustFS Console: http://localhost:9001 (rustfsadmin/rustfsadmin)
- Temporal gRPC: localhost:7233
- PostgreSQL: localhost:5432 (temporal/temporal)

**Prerequisites:** Podman and podman-compose installed.

**Individual commands for advanced usage:**

| Command | Description |
|---------|-------------|
| `task local:up` | Start infrastructure only |
| `task local:down` | Stop infrastructure |
| `task local:workers` | Start workers in background |
| `task local:workers:stop` | Stop workers |
| `task local:trigger` | Submit all workflows once |
| `task local:schedule` | Create recurring schedules |
| `task local:schedule:clean` | Remove schedules |

### Running Examples

All examples require a running Temporal server. Use the built-in demo tasks:

```bash
# Run all examples automatically (starts Temporal + worker, runs 16 examples, cleans up)
task demo

# Or run examples interactively:
task demo:start                         # Start Temporal + container worker in background
task example:function -- basic.go       # Run a function example
task example:container -- pipeline.go   # Run a container example
task example:list                       # List all available examples
task demo:stop                         # Stop Temporal + worker when done
```

Temporal Web UI is available at http://localhost:8233 to watch workflow executions.

See [examples/container/README.md](./examples/container/README.md) and [examples/function/README.md](./examples/function/README.md) for detailed example descriptions.

### Code Quality

The project enforces high code quality standards:

- **golangci-lint** - Multiple linters enabled
- **gofumpt** - Stricter formatting than gofmt
- **Test coverage** - 85%+ target
- **Security** - gosec scanning

## Contributing

See [Contributing Guide](./docs/contributing.md) for development setup, testing, and contribution workflow.

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Author

Deny Prasetyo (@jasoet)
