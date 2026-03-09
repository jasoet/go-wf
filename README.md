# go-wf

[![Go Version](https://img.shields.io/badge/Go-1.26+-blue.svg)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Build Status](https://github.com/jasoet/go-wf/actions/workflows/release.yml/badge.svg)](https://github.com/jasoet/go-wf/actions)

Temporal workflow library providing reusable, production-ready workflows for common orchestration patterns.

## Features

- **Docker Workflows** - Execute containers with Temporal orchestration
- **Function Workflows** - Execute registered Go functions with Temporal orchestration
- **Type-Safe Payloads** - Validated input/output structures
- **Production-Ready** - Built-in retries, timeouts, error handling
- **Observable** - Inherits OpenTelemetry from underlying packages
- **Comprehensive Testing** - 85%+ coverage with integration tests
- **Full CI/CD** - Automated releases and quality checks

## Packages

### [workflow](./workflow/)

Generic workflow orchestration core using Go generics:
- **Type-Safe Interfaces** — `TaskInput`/`TaskOutput` constraints for compile-time safety
- **Pipeline** — Sequential task execution with stop-on-error
- **Parallel** — Concurrent task execution with fail-fast/continue
- **Loop** — Iterate over items or parameter combinations
- **Artifacts** — Pluggable artifact storage (local filesystem, MinIO/S3)
- **Extensible** — Implement `TaskInput`/`TaskOutput` to add new activity types

### [docker](./docker/)

Temporal workflows for executing Docker containers with Argo Workflow-like capabilities:

**Core Workflows:**
- **Single Container** - Execute individual containers with wait strategies
- **Pipeline** - Sequential container execution with error handling
- **Parallel** - Concurrent container execution with configurable limits
- **DAG** - Directed Acyclic Graph execution with dependency management

**Builder & Templates:**
- **Fluent Builder API** - Compose complex workflows with chainable methods
- **Container Templates** - Enhanced container execution with functional options
- **Script Templates** - Execute bash, python, node, ruby, or golang scripts
- **HTTP Templates** - HTTP requests, health checks, and webhooks
- **Pre-built Patterns** - CI/CD, fan-out/fan-in, map-reduce, parallel testing

**Advanced Features:**
- **Lifecycle Management** - Submit, wait, watch, cancel, terminate, signal, query workflows
- **Conditional Execution** - When clauses and ContinueOnFail behaviors
- **Resource Management** - CPU, memory, GPU limits
- **Artifacts & Secrets** - Input/output artifacts and secret injection
- **Workflow Parameters** - Template variables for reusable workflows

See [docker/README.md](./docker/README.md) for detailed documentation and [examples/docker/](./examples/docker/) for runnable examples.

### [function](./function/)

Temporal workflows for executing registered Go functions:

**Core Features:**
- **Function Registry** - Register named Go handler functions
- **Single Execution** - Execute individual functions as Temporal activities
- **Pipeline** - Sequential function execution with error handling
- **Parallel** - Concurrent function execution with configurable limits
- **Loop** - Iterate over items or parameter combinations with template substitution

**Builder API:**
- **Fluent Builder** - Compose function workflows with chainable methods
- **Loop Builder** - Item-based and parameterized loop construction
- **WorkflowSource** - Composable function input sources

**Usage:**
```go
// Create and populate registry
registry := function.NewRegistry()
registry.Register("validate", validateHandler)
registry.Register("transform", transformHandler)

// Register with Temporal worker
w := worker.New(client, "task-queue", worker.Options{})
function.RegisterWorkflows(w)
function.RegisterActivity(w, activity.NewExecuteFunctionActivity(registry))

// Build and execute a pipeline
pipeline, _ := builder.NewWorkflowBuilder("my-pipeline").
    AddInput(payload.FunctionExecutionInput{Name: "validate", Args: map[string]string{"path": "/config.yaml"}}).
    AddInput(payload.FunctionExecutionInput{Name: "transform"}).
    StopOnError(true).
    BuildPipeline()
```

See [examples/function/](./examples/function/) for runnable examples.

## Installation

```bash
go get github.com/jasoet/go-wf
```

## Quick Start

### Docker Workflow

```go
package main

import (
    "context"
    "log"

    "github.com/jasoet/go-wf/docker"
    "github.com/jasoet/go-wf/docker/payload"
    "github.com/jasoet/go-wf/docker/workflow"
    "github.com/jasoet/pkg/v2/temporal"
    "go.temporal.io/sdk/client"
    "go.temporal.io/sdk/worker"
)

func main() {
    // Create Temporal client
    c, closer, err := temporal.NewClient(temporal.DefaultConfig())
    if err != nil {
        log.Fatal(err)
    }
    defer c.Close()
    if closer != nil {
        defer closer.Close()
    }

    // Create and start worker
    w := worker.New(c, "docker-tasks", worker.Options{})
    docker.RegisterAll(w)

    go w.Run(nil)
    defer w.Stop()

    // Execute workflow
    input := payload.ContainerExecutionInput{
        Image: "postgres:16-alpine",
        Env: map[string]string{
            "POSTGRES_PASSWORD": "test",
        },
        Ports:      []string{"5432:5432"},
        AutoRemove: true,
    }

    we, _ := c.ExecuteWorkflow(context.Background(),
        client.StartWorkflowOptions{
            ID:        "postgres-setup",
            TaskQueue: "docker-tasks",
        },
        workflow.ExecuteContainerWorkflow,
        input,
    )

    var result payload.ContainerExecutionOutput
    we.Get(context.Background(), &result)
    log.Printf("Container executed: %s", result.ContainerID)
}
```

### Function Workflow

```go
package main

import (
    "context"
    "log"

    fn "github.com/jasoet/go-wf/function"
    fnactivity "github.com/jasoet/go-wf/function/activity"
    "github.com/jasoet/go-wf/function/payload"
    "github.com/jasoet/go-wf/function/workflow"
    "github.com/jasoet/pkg/v2/temporal"
    "go.temporal.io/sdk/client"
    "go.temporal.io/sdk/worker"
)

func main() {
    // Create Temporal client
    c, closer, err := temporal.NewClient(temporal.DefaultConfig())
    if err != nil {
        log.Fatal(err)
    }
    defer c.Close()
    if closer != nil {
        defer closer.Close()
    }

    // Create function registry and register handlers
    registry := fn.NewRegistry()
    registry.Register("greet", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
        return &fn.FunctionOutput{
            Result: map[string]string{"greeting": "Hello, " + input.Args["name"] + "!"},
        }, nil
    })

    // Create and start worker
    w := worker.New(c, "function-tasks", worker.Options{})
    fn.RegisterWorkflows(w)
    fn.RegisterActivity(w, fnactivity.NewExecuteFunctionActivity(registry))

    go w.Run(nil)
    defer w.Stop()

    // Execute workflow
    input := payload.FunctionExecutionInput{
        Name: "greet",
        Args: map[string]string{"name": "Temporal"},
    }

    we, _ := c.ExecuteWorkflow(context.Background(),
        client.StartWorkflowOptions{
            ID:        "greet-example",
            TaskQueue: "function-tasks",
        },
        workflow.ExecuteFunctionWorkflow,
        input,
    )

    var result payload.FunctionExecutionOutput
    we.Get(context.Background(), &result)
    log.Printf("Result: %v", result.Result)
}
```

## Project Structure

```
go-wf/
├── workflow/         # Generic workflow core (interfaces, orchestration)
│   ├── errors/       # Error types and handling
│   └── artifacts/    # Artifact store (local + MinIO)
├── docker/           # Docker container workflows (concrete implementation)
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
├── examples/docker/    # Docker examples (see [README](./examples/docker/README.md))
├── examples/function/  # Function examples (see [README](./examples/function/README.md))
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
- Coverage target: 85%
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

### Code Quality

The project enforces high code quality standards:

- **golangci-lint** - Multiple linters enabled
- **gofumpt** - Stricter formatting than gofmt
- **Test coverage** - 85%+ target
- **Security** - gosec scanning

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make changes with tests
4. Run `task check`
5. Commit with conventional format
6. Submit a pull request

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Author

Deny Prasetyo (@jasoet)
