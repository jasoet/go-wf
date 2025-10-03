# go-wf

[![Go Version](https://img.shields.io/badge/Go-1.23+-blue.svg)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Build Status](https://github.com/jasoet/go-wf/actions/workflows/release.yml/badge.svg)](https://github.com/jasoet/go-wf/actions)

Temporal workflow library providing reusable, production-ready workflows for common orchestration patterns.

## Features

- **Docker Workflows** - Execute containers with Temporal orchestration
- **Type-Safe Payloads** - Validated input/output structures
- **Production-Ready** - Built-in retries, timeouts, error handling
- **Observable** - Inherits OpenTelemetry from underlying packages
- **Comprehensive Testing** - 85%+ coverage with integration tests
- **Full CI/CD** - Automated releases and quality checks

## Packages

### [docker](./docker/)

Temporal workflows for executing Docker containers with advanced orchestration:

- **Single Container** - Execute individual containers with wait strategies
- **Pipeline** - Sequential container execution with error handling
- **Parallel** - Concurrent container execution with configurable limits

See [docker/README.md](./docker/README.md) for detailed documentation.

## Installation

```bash
go get github.com/jasoet/go-wf
```

## Quick Start

```go
package main

import (
    "context"
    "log"

    "github.com/jasoet/go-wf/docker"
    "github.com/jasoet/pkg/v2/temporal"
    "go.temporal.io/sdk/client"
    "go.temporal.io/sdk/worker"
)

func main() {
    // Create Temporal client
    c, err := temporal.NewClient(temporal.DefaultConfig())
    if err != nil {
        log.Fatal(err)
    }
    defer c.Close()

    // Create and start worker
    w := worker.New(c, "docker-tasks", worker.Options{})
    docker.RegisterAll(w)

    go w.Run(nil)
    defer w.Stop()

    // Execute workflow
    input := docker.ContainerExecutionInput{
        Image: "postgres:16-alpine",
        Env: map[string]string{
            "POSTGRES_PASSWORD": "test",
        },
        Ports: []string{"5432:5432"},
        AutoRemove: true,
    }

    we, _ := c.ExecuteWorkflow(context.Background(),
        client.StartWorkflowOptions{
            ID:        "postgres-setup",
            TaskQueue: "docker-tasks",
        },
        docker.ExecuteContainerWorkflow,
        input,
    )

    var result docker.ContainerExecutionOutput
    we.Get(context.Background(), &result)
    log.Printf("Container executed: %s", result.ContainerID)
}
```

## Project Structure

```
go-wf/
├── docker/           # Docker container workflows
├── docs/             # Project templates and documentation
├── .github/          # GitHub Actions workflows
├── Taskfile.yml      # Task automation
└── README.md         # This file
```

## 🤖 AI Agent Instructions

**Repository Type:** Library

**Critical Setup:**
- Go 1.23+
- Docker for integration tests
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
- Unit tests: `task test`
- Integration tests: `task test:integration` (requires Docker)
- All tests: `task test:all`
- Test files: `*_test.go` (unit), `integration_test.go` (integration)

**Quality Standards:**
- Zero golangci-lint errors
- gofumpt formatting
- Cyclomatic complexity < 20
- Line length < 190 characters
- All exported functions documented

**Development Commands:**
```bash
task                   # List all available tasks
task test              # Run unit tests with coverage
task test:integration  # Run integration tests (Docker required)
task test:all          # Run all tests with combined coverage
task lint              # Run golangci-lint
task fmt               # Format code with gofumpt
task check             # Run tests + lint
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

**Template Documentation:**
See [docs/](./docs/) for project setup templates and guides:
- [PROJECT_TEMPLATE.md](./docs/PROJECT_TEMPLATE.md) - Complete project structure guide
- [AI_PROJECT_SETUP.md](./docs/AI_PROJECT_SETUP.md) - AI agent setup instructions
- [REUSABLE_INFRASTRUCTURE.md](./docs/REUSABLE_INFRASTRUCTURE.md) - CI/CD and tooling guide

## Development

### Prerequisites

- Go 1.23 or higher
- Docker (for integration tests)
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
# Unit tests only
task test

# Integration tests (requires Docker)
task test:integration

# All tests with coverage report
task test:all
# Open output/coverage-all.html to view coverage
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
