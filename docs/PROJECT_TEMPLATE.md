# Go Project Template Guide

This guide explains how to set up a new Go project following the same structure, patterns, and quality standards as the `github.com/jasoet/pkg` utility library.

## Table of Contents

- [Project Structure](#project-structure)
- [Core Setup Files](#core-setup-files)
- [Package Organization](#package-organization)
- [Testing Strategy](#testing-strategy)
- [Quality Standards](#quality-standards)
- [Documentation Standards](#documentation-standards)
- [Development Workflow](#development-workflow)

## Project Structure

### Standard Directory Layout

```
project-root/
├── .github/
│   └── workflows/
│       ├── release.yml          # CI/CD for releases
│       └── claude.yml           # Optional: Claude Code PR assistant
├── .gitignore                   # Standard Go gitignore
├── .golangci.yml               # Linter configuration
├── .releaserc.json             # Semantic release configuration
├── Taskfile.yml                # Task automation
├── README.md                   # Main documentation with AI instructions
├── TESTING.md                  # Testing guide
├── MAINTAINING.md              # Maintenance guide
├── VERSIONING_GUIDE.md         # Version management (if needed)
├── go.mod                      # Go module file
├── go.sum                      # Go dependencies
├── output/                     # Test coverage reports (gitignored)
├── dist/                       # Build artifacts (gitignored)
├── package1/
│   ├── README.md               # Package documentation
│   ├── package1.go             # Main implementation
│   ├── package1_test.go        # Unit tests
│   ├── integration_test.go     # Integration tests (build tag)
│   └── examples/
│       ├── README.md
│       └── basic.go            # Example code (build tag)
├── package2/
│   └── ...                     # Same structure
└── docs/
    └── ...                     # Additional documentation
```

## Core Setup Files

### 1. go.mod

```go
module github.com/yourusername/yourproject

go 1.23

require (
    // Your dependencies
)
```

**Key Points:**
- Use descriptive module name
- Set minimum Go version (1.23+ recommended)
- Keep dependencies minimal and well-vetted

### 2. .gitignore

Use the standard Go gitignore from this project:
- Excludes: `vendor/`, `output/`, `dist/`, `.idea/`, IDE files
- Custom additions: `*.db`, `*.log`, `.temporal/`, `.env.prod`, `.secret`

### 3. README.md Structure

```markdown
# Project Name

[![Go Version](https://img.shields.io/badge/Go-1.23+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Build Status](badge-url)](build-url)

Brief description of the project.

## Features

- Feature 1
- Feature 2
- Feature 3

## Installation

```bash
go get github.com/yourusername/yourproject
```

## Quick Start

[Minimal working example]

## Package Documentation

[List packages with descriptions]

## 🤖 AI Agent Instructions

**Repository Type:** [Library/Service/Tool]

**Critical Setup:**
- [Setup requirements]

**Architecture:**
- [Key architectural patterns]

**Key Development Patterns:**
- [Important patterns to follow]

**Testing Strategy:**
- Coverage: [target]%
- Unit tests: [command]
- Integration tests: [command]

[More details...]
```

### 4. Taskfile.yml

Essential tasks to include:

```yaml
version: '3'

tasks:
  default:
    desc: List all available tasks
    silent: true
    cmds:
      - task --list

  test:
    desc: Run unit tests with coverage
    silent: true
    cmds:
      - mkdir -p output
      - go test -race -count=1 -coverprofile=output/coverage.out -covermode=atomic ./...
      - go tool cover -html=output/coverage.out -o output/coverage.html
      - 'echo "✓ Coverage: output/coverage.html"'

  test:integration:
    desc: Run integration tests (Docker required)
    silent: true
    cmds:
      - mkdir -p output
      - go test -race -count=1 -coverprofile=output/coverage-integration.out -covermode=atomic -tags=integration -timeout=15m ./...
      - go tool cover -html=output/coverage-integration.out -o output/coverage-integration.html

  test:all:
    desc: Run all tests with combined coverage
    silent: true
    cmds:
      - mkdir -p output
      - go test -race -count=1 -coverprofile=output/coverage-all.out -covermode=atomic -tags=integration -timeout=15m ./...
      - go tool cover -html=output/coverage-all.out -o output/coverage-all.html
      - go tool cover -func=output/coverage-all.out | grep total

  lint:
    desc: Run golangci-lint
    silent: true
    cmds:
      - golangci-lint run ./...

  check:
    desc: Run all checks (test + lint)
    silent: true
    cmds:
      - task: test
      - task: lint

  tools:
    desc: Install development tools
    silent: true
    cmds:
      - go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
      - go install mvdan.cc/gofumpt@latest
      - echo "✓ Tools installed"

  fmt:
    desc: Format all Go files
    cmds:
      - gofumpt -l -w .

  clean:
    desc: Clean build artifacts
    silent: true
    cmds:
      - rm -rf dist output
```

## Package Organization

### Package Structure Rules

1. **One package per directory**
2. **Clear separation of concerns**
3. **Self-contained with minimal dependencies**
4. **Each package has:**
   - Main implementation files
   - Comprehensive tests
   - README.md documentation
   - Examples directory

### Example Package Layout

```
mypackage/
├── README.md                   # Package documentation
├── mypackage.go               # Main implementation
├── config.go                  # Configuration types
├── types.go                   # Type definitions
├── mypackage_test.go          # Unit tests
├── integration_test.go        # Integration tests (//go:build integration)
├── testdata/                  # Test fixtures
│   └── sample.yaml
└── examples/
    ├── README.md
    ├── basic.go               # //go:build example
    └── advanced.go            # //go:build example
```

### Build Tags

Use build tags to separate code:

```go
//go:build integration
// or
//go:build example
```

**Standard tags:**
- No tag: Unit tests and main code
- `integration`: Integration tests requiring Docker/external services
- `example`: Example code excluded from builds

## Testing Strategy

### Test Categories

1. **Unit Tests** (no build tag)
   - Fast, no external dependencies
   - High coverage target (80%+)
   - Use table-driven tests
   - Mock external dependencies

2. **Integration Tests** (`//go:build integration`)
   - Use testcontainers for dependencies
   - Test with real services
   - Automatic cleanup
   - Longer timeout (15m)

### Test File Naming

- `*_test.go` - Unit tests
- `integration_test.go` - Integration tests
- `helpers_test.go` - Test utilities

### Test Organization

```go
func TestFeatureName(t *testing.T) {
    tests := []struct {
        name    string
        input   Input
        want    Output
        wantErr bool
    }{
        {
            name:    "success case",
            input:   Input{},
            want:    Output{},
            wantErr: false,
        },
        {
            name:    "error case",
            input:   Input{},
            want:    Output{},
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := FeatureName(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if !reflect.DeepEqual(got, tt.want) {
                t.Errorf("got %v, want %v", got, tt.want)
            }
        })
    }
}
```

### Integration Test Pattern with Testcontainers

```go
//go:build integration

package mypackage_test

import (
    "context"
    "testing"

    "github.com/testcontainers/testcontainers-go"
)

func TestIntegrationFeature(t *testing.T) {
    ctx := context.Background()

    // Setup container
    req := testcontainers.ContainerRequest{
        Image:        "postgres:16-alpine",
        ExposedPorts: []string{"5432/tcp"},
        Env: map[string]string{
            "POSTGRES_PASSWORD": "test",
        },
    }

    container, err := testcontainers.GenericContainer(ctx,
        testcontainers.GenericContainerRequest{
            ContainerRequest: req,
            Started:          true,
        })
    if err != nil {
        t.Fatal(err)
    }
    defer container.Terminate(ctx)

    // Run tests
    // ...
}
```

## Quality Standards

### Linting Configuration (.golangci.yml)

Key linters to enable:
- `govet`, `errcheck`, `staticcheck` - Code correctness
- `gosec` - Security checks
- `gofmt`, `goimports`, `gofumpt` - Code formatting
- `revive` - Style guide
- `prealloc`, `unconvert` - Performance
- `bodyclose`, `rowserrcheck` - Resource management

### Code Quality Targets

- **Test Coverage:** 80%+ (85%+ ideal)
- **Cyclomatic Complexity:** < 20
- **Line Length:** < 190 characters
- **Go Report Card:** A+ grade

### Security Practices

1. **No hardcoded secrets** - Use environment variables
2. **Input validation** - Validate all external inputs
3. **Error handling** - Check all errors
4. **Dependency updates** - Regular security updates
5. **gosec compliance** - Zero high-severity issues

## Documentation Standards

### Package README.md

Each package must have a README with:

```markdown
# Package Name

Brief description.

## Features

- Feature list

## Installation

```bash
go get github.com/yourusername/project/package
```

## Quick Start

[Minimal example]

## API Reference

### Functions

**FunctionName**
```go
func FunctionName(param Type) (Result, error)
```
Description and usage.

## Examples

See [examples/](./examples/) directory.

## Testing

[Test coverage and commands]
```

### Code Comments

- **Exported functions/types:** Must have godoc comments
- **Complex logic:** Explain the "why", not the "what"
- **Examples:** Include example code in godoc

```go
// NewClient creates a new HTTP client with default configuration.
// It initializes connection pooling, timeout settings, and retry logic.
//
// Example:
//
//  client := NewClient(config)
//  defer client.Close()
func NewClient(cfg Config) *Client {
    // ...
}
```

## Development Workflow

### 1. Local Development

```bash
# Install tools
task tools

# Run tests during development
task test

# Format code
task fmt

# Full check before commit
task check
```

### 2. Pre-commit Checklist

- [ ] Run `task fmt`
- [ ] Run `task test`
- [ ] Run `task lint`
- [ ] Update documentation if needed
- [ ] Add/update tests for changes
- [ ] Follow conventional commit format

### 3. Commit Message Format

Use [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <description>

[optional body]

[optional footer]
```

**Types:** `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`, `perf`, `ci`

Examples:
```
feat(docker): add wait strategy for HTTP health checks
fix(temporal): resolve worker shutdown race condition
docs(README): update installation instructions
test(db): add integration tests for PostgreSQL
```

### 4. Release Process

The project uses semantic-release for automated versioning:

1. Commits following conventional format trigger releases
2. `fix:` → patch version (1.0.x)
3. `feat:` → minor version (1.x.0)
4. `BREAKING CHANGE:` → major version (x.0.0)
5. Changelog generated automatically

## Next Steps

1. Copy this structure to your new project
2. Customize go.mod, README.md, and package names
3. Set up CI/CD using workflows from `.github/workflows/`
4. Configure golangci-lint with `.golangci.yml`
5. Add your first package following the structure
6. Set up semantic-release with `.releaserc.json`

## Additional Resources

- [Effective Go](https://golang.org/doc/effective_go.html)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Standard Go Project Layout](https://github.com/golang-standards/project-layout)
- [Testcontainers for Go](https://golang.testcontainers.org/)
- [Conventional Commits](https://www.conventionalcommits.org/)
