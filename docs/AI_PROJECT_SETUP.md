# AI Agent Project Setup Instructions

This document provides step-by-step instructions for AI agents (like Claude Code) to automatically set up a new Go project following the patterns and standards from `github.com/jasoet/pkg`.

## AI Agent Directive

**Role:** You are setting up a production-grade Go project with comprehensive testing, CI/CD, and quality tooling.

**Standards:**
- Test coverage: 80%+ (target 85%)
- Zero golangci-lint errors
- Conventional commits
- Semantic versioning
- Testcontainer-based integration tests

## Setup Checklist

### Phase 1: Project Initialization

```bash
# 1. Create project directory
mkdir -p /path/to/newproject
cd /path/to/newproject

# 2. Initialize Go module
go mod init github.com/yourusername/projectname

# 3. Set Go version
go mod edit -go=1.23
```

### Phase 2: Copy Infrastructure Files

Execute these commands to copy core infrastructure:

```bash
# Set source and destination
export SOURCE="/path/to/github.com/jasoet/pkg"
export DEST="."

# Copy core files
cp $SOURCE/Taskfile.yml $DEST/
cp $SOURCE/.golangci.yml $DEST/
cp $SOURCE/.releaserc.json $DEST/
cp $SOURCE/.gitignore $DEST/
cp $SOURCE/TESTING.md $DEST/

# Copy GitHub workflows
mkdir -p .github/workflows
cp $SOURCE/.github/workflows/release.yml $DEST/.github/workflows/
cp $SOURCE/.github/workflows/claude.yml $DEST/.github/workflows/

# Create output directory (gitignored)
mkdir -p output
```

### Phase 3: Create Project Documentation

#### 3.1 Create README.md

```markdown
# Project Name

[![Go Version](https://img.shields.io/badge/Go-1.23+-blue.svg)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Build Status](https://github.com/USERNAME/PROJECT/actions/workflows/release.yml/badge.svg)](https://github.com/USERNAME/PROJECT/actions)

[One-line project description]

## Features

- Feature 1
- Feature 2
- Feature 3

## Installation

```bash
go get github.com/yourusername/projectname
```

## Quick Start

```go
package main

import (
    "github.com/yourusername/projectname/package1"
)

func main() {
    // Quick example
}
```

## 🤖 AI Agent Instructions

**Repository Type:** [Library/Service/CLI Tool]

**Critical Setup:**
- Go 1.23+
- Docker for integration tests

**Architecture:**
- [Describe architecture pattern]

**Testing Strategy:**
- Coverage target: 85%
- Unit tests: `task test`
- Integration tests: `task test:integration` (requires Docker)
- All tests: `task test:all`

**Development Commands:**
```bash
task test              # Unit tests
task test:integration  # Integration tests
task test:all          # All tests with coverage
task lint              # Run linter
task fmt               # Format code
task check             # Full quality check
```

## License

MIT License - see [LICENSE](LICENSE) file for details.
```

#### 3.2 Create LICENSE

```text
MIT License

Copyright (c) [YEAR] [COPYRIGHT HOLDER]

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

### Phase 4: Customize Configuration

#### 4.1 Update .golangci.yml

Find and replace project-specific values:

```yaml
# Line 47: Update local-prefixes
goimports:
  local-prefixes: github.com/yourusername/projectname

# Line 90: Update acronyms for your project
revive:
  rules:
    - name: var-naming
      arguments:
        - ["ID", "URL", "HTTP", "API", "JSON"]  # Add your acronyms
```

#### 4.2 Customize Taskfile.yml

Add project-specific tasks if needed:

```yaml
# Add after standard tasks
tasks:
  # ... standard tasks ...

  build:
    desc: Build the application
    cmds:
      - go build -o bin/app ./cmd/app

  run:
    desc: Run the application
    cmds:
      - go run ./cmd/app
```

### Phase 5: Create First Package

#### 5.1 Package Structure

```bash
# Create package directory
mkdir -p mypackage

# Create package files
touch mypackage/mypackage.go
touch mypackage/mypackage_test.go
touch mypackage/integration_test.go
touch mypackage/README.md

# Create examples
mkdir -p mypackage/examples
touch mypackage/examples/README.md
touch mypackage/examples/basic.go
```

#### 5.2 Package Implementation Template

**mypackage/mypackage.go:**

```go
package mypackage

// Config holds configuration for MyPackage
type Config struct {
    Option1 string
    Option2 int
}

// Client is the main client for MyPackage
type Client struct {
    config Config
}

// New creates a new Client with the given config
func New(cfg Config) *Client {
    return &Client{
        config: cfg,
    }
}

// DoSomething performs the main operation
func (c *Client) DoSomething() error {
    // Implementation
    return nil
}
```

#### 5.3 Unit Test Template

**mypackage/mypackage_test.go:**

```go
package mypackage

import (
    "testing"
)

func TestNew(t *testing.T) {
    tests := []struct {
        name   string
        config Config
        want   *Client
    }{
        {
            name: "creates client with config",
            config: Config{
                Option1: "test",
                Option2: 42,
            },
            want: &Client{
                config: Config{
                    Option1: "test",
                    Option2: 42,
                },
            },
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := New(tt.config)
            if got.config.Option1 != tt.want.config.Option1 {
                t.Errorf("New() Option1 = %v, want %v", got.config.Option1, tt.want.config.Option1)
            }
        })
    }
}

func TestClient_DoSomething(t *testing.T) {
    c := New(Config{})

    err := c.DoSomething()
    if err != nil {
        t.Errorf("DoSomething() error = %v", err)
    }
}
```

#### 5.4 Integration Test Template

**mypackage/integration_test.go:**

```go
//go:build integration

package mypackage_test

import (
    "context"
    "testing"

    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/wait"
)

func TestIntegration_DoSomething(t *testing.T) {
    ctx := context.Background()

    // Setup container
    req := testcontainers.ContainerRequest{
        Image:        "nginx:alpine",
        ExposedPorts: []string{"80/tcp"},
        WaitingFor:   wait.ForHTTP("/").WithPort("80/tcp"),
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

    // Test logic
    // ...
}
```

#### 5.5 Example Code Template

**mypackage/examples/basic.go:**

```go
//go:build example

package main

import (
    "fmt"
    "log"

    "github.com/yourusername/projectname/mypackage"
)

func main() {
    // Create config
    cfg := mypackage.Config{
        Option1: "example",
        Option2: 123,
    }

    // Create client
    client := mypackage.New(cfg)

    // Use client
    if err := client.DoSomething(); err != nil {
        log.Fatal(err)
    }

    fmt.Println("Success!")
}
```

#### 5.6 Package README Template

**mypackage/README.md:**

```markdown
# MyPackage

Brief description of what this package does.

## Features

- Feature 1
- Feature 2

## Installation

```bash
go get github.com/yourusername/projectname/mypackage
```

## Quick Start

```go
package main

import "github.com/yourusername/projectname/mypackage"

func main() {
    cfg := mypackage.Config{
        Option1: "value",
        Option2: 42,
    }

    client := mypackage.New(cfg)
    client.DoSomething()
}
```

## Examples

See [examples/](./examples/) directory for complete examples.

Run examples:
```bash
go run -tags=example ./mypackage/examples/basic.go
```

## Testing

```bash
# Unit tests
go test ./mypackage/...

# Integration tests
go test -tags=integration ./mypackage/...
```
```

### Phase 6: Install Dependencies

```bash
# Install test dependencies
go get github.com/testcontainers/testcontainers-go@latest
go get github.com/stretchr/testify/assert@latest

# Install development tools
task tools

# Tidy dependencies
go mod tidy
```

### Phase 7: Verify Setup

```bash
# Format code
task fmt

# Run tests
task test

# Run linter
task lint

# Full check
task check
```

### Phase 8: Initialize Git Repository

```bash
# Initialize git
git init

# Add all files
git add .

# Initial commit (conventional format)
git commit -m "chore: initialize project with standard infrastructure"

# Add remote
git remote add origin https://github.com/yourusername/projectname.git

# Push
git branch -M main
git push -u origin main
```

### Phase 9: Configure GitHub

#### 9.1 Repository Settings

1. Go to repository Settings → Branches
2. Add branch protection for `main`:
   - Require pull request reviews
   - Require status checks to pass
   - Include administrators

#### 9.2 GitHub Secrets

Add required secrets for CI/CD:

1. **For Semantic Release:**
   - `GITHUB_TOKEN` (automatically available)

2. **For Claude Code (optional):**
   - `ANTHROPIC_API_KEY` or `CLAUDE_CODE_OAUTH_TOKEN`

3. **For Other Integrations:**
   - Add as needed for your project

#### 9.3 Enable GitHub Actions

1. Go to Actions tab
2. Enable workflows
3. First push will trigger release workflow

### Phase 10: Create Development Branch

```bash
# Create development branch for first feature
git checkout -b feat/initial-implementation

# Make changes
# ...

# Commit with conventional format
git add .
git commit -m "feat(mypackage): implement core functionality"

# Push
git push -u origin feat/initial-implementation

# Create PR
gh pr create --title "feat(mypackage): implement core functionality" \
  --body "Initial implementation of core package functionality"
```

## AI Agent Workflow Template

When setting up a project, follow this sequence:

```
1. INITIALIZE
   ↓
2. COPY INFRASTRUCTURE
   ↓
3. CREATE DOCUMENTATION
   ↓
4. CUSTOMIZE CONFIG
   ↓
5. CREATE PACKAGES
   ↓
6. INSTALL DEPENDENCIES
   ↓
7. VERIFY
   ↓
8. GIT SETUP
   ↓
9. GITHUB CONFIG
   ↓
10. FEATURE BRANCH
```

## Validation Checklist

Before considering setup complete, verify:

- [ ] `task test` passes
- [ ] `task lint` shows zero errors
- [ ] `task fmt` applied
- [ ] README.md has AI instructions
- [ ] All packages have README.md
- [ ] Integration tests use testcontainers
- [ ] Examples have `//go:build example` tag
- [ ] GitHub Actions configured
- [ ] Semantic release configured
- [ ] First commit uses conventional format

## Common Customizations

### For Library Projects

```yaml
# Taskfile.yml additions
tasks:
  examples:
    desc: Run all examples
    cmds:
      - go run -tags=example ./pkg1/examples/...
      - go run -tags=example ./pkg2/examples/...
```

### For Service Projects

```yaml
# Taskfile.yml additions
tasks:
  run:
    desc: Run service locally
    cmds:
      - go run ./cmd/service

  docker:build:
    desc: Build Docker image
    cmds:
      - docker build -t myservice:latest .
```

### For CLI Projects

```yaml
# Taskfile.yml additions
tasks:
  build:
    desc: Build CLI
    cmds:
      - go build -o bin/cli ./cmd/cli

  install:
    desc: Install CLI
    cmds:
      - go install ./cmd/cli
```

## Troubleshooting

### Setup Fails at Task Installation

```bash
# Install task manually
npm install -g @go-task/cli
# or
brew install go-task/tap/go-task
```

### Linter Reports Errors

```bash
# Auto-fix issues
golangci-lint run --fix ./...

# Update to project standards
task fmt
```

### Tests Fail

```bash
# Check Docker for integration tests
docker info

# Run only unit tests first
go test ./...

# Then integration tests
go test -tags=integration ./...
```

### Import Path Issues

Ensure all imports use the correct module path from `go.mod`:
```go
import "github.com/yourusername/projectname/package"
```

## Post-Setup Tasks

After initial setup is complete:

1. **Add more packages** following the same structure
2. **Write comprehensive tests** (aim for 85% coverage)
3. **Create examples** for all packages
4. **Set up branch protection** on GitHub
5. **Configure dependabot** for security updates
6. **Add badges** to README.md
7. **Create CONTRIBUTING.md** if open source

## Success Criteria

The project setup is successful when:

✅ All tests pass: `task test:all`
✅ Zero lint errors: `task lint`
✅ Coverage ≥ 80%: Check `output/coverage-all.html`
✅ CI/CD works: Push triggers release workflow
✅ Documentation complete: README with AI instructions
✅ Examples runnable: `go run -tags=example ./*/examples/*.go`

## Next Steps for AI Agent

After completing setup:

1. Implement core package functionality
2. Write comprehensive unit tests
3. Add integration tests with testcontainers
4. Create runnable examples
5. Generate package documentation
6. Verify all quality checks pass
7. Create initial release

Remember: Follow conventional commits for automatic versioning! 🚀
