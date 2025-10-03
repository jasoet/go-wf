# Reusable Infrastructure Guide

This guide explains how to reuse the CI/CD setup, Taskfile, testing infrastructure, and quality tooling from the `github.com/jasoet/pkg` project in your new projects.

## Table of Contents

- [Quick Start](#quick-start)
- [Taskfile Setup](#taskfile-setup)
- [CI/CD Workflows](#cicd-workflows)
- [Quality Tooling](#quality-tooling)
- [Testing Infrastructure](#testing-infrastructure)
- [Release Automation](#release-automation)
- [Customization Guide](#customization-guide)

## Quick Start

### Prerequisites

```bash
# Install required tools
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install mvdan.cc/gofumpt@latest
npm install -g @go-task/cli  # or: brew install go-task/tap/go-task

# For semantic release
npm install -g semantic-release @semantic-release/github conventional-changelog-conventionalcommits
```

### Copy Core Files

```bash
# From pkg project to your new project
cp Taskfile.yml /path/to/yourproject/
cp .golangci.yml /path/to/yourproject/
cp .releaserc.json /path/to/yourproject/
cp -r .github /path/to/yourproject/
cp TESTING.md /path/to/yourproject/
```

## Taskfile Setup

### Core Taskfile.yml

The Taskfile provides consistent task automation across projects. Copy the entire file and customize as needed.

**Essential Tasks:**

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

### Adding Project-Specific Tasks

Extend the Taskfile with your project needs:

```yaml
tasks:
  # ... standard tasks above ...

  build:
    desc: Build the application
    cmds:
      - go build -o bin/myapp ./cmd/myapp

  run:
    desc: Run the application
    cmds:
      - go run ./cmd/myapp

  docker:build:
    desc: Build Docker image
    cmds:
      - docker build -t myapp:latest .

  docker:up:
    desc: Start Docker services
    cmds:
      - docker-compose up -d

  docker:down:
    desc: Stop Docker services
    cmds:
      - docker-compose down
```

## CI/CD Workflows

### GitHub Actions Setup

#### 1. Release Workflow (.github/workflows/release.yml)

This workflow runs tests and creates releases using semantic-release:

```yaml
name: Release

on:
  push:
    branches:
      - main
      - master

jobs:
  test:
    name: Test
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.23.0'
          check-latest: true

      - name: Run tests
        run: go test -v ./...

  release:
    name: Release
    needs: test
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4
        with:
          fetch-depth: 0
          persist-credentials: false

      - name: Setup Node.js
        uses: actions/setup-node@v4
        with:
          node-version: '20'

      - name: Install dependencies
        run: npm install -g semantic-release @semantic-release/github conventional-changelog-conventionalcommits

      - name: Release
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: npx semantic-release
```

**Customization Points:**
- Go version: Update `go-version` to your minimum supported version
- Test command: Add integration tests if needed
- Branch names: Adjust if using different branch strategy

#### 2. Claude Code Assistant (Optional)

Enable AI-powered PR assistance:

```yaml
name: Claude PR Assistant

on:
  issue_comment:
    types: [ created ]
  pull_request_review_comment:
    types: [ created ]

jobs:
  claude-code-action:
    if: contains(github.event.comment.body, '@claude')
    runs-on: ubuntu-latest
    permissions:
      contents: read
      pull-requests: read
      issues: read
      id-token: write
    steps:
      - name: Checkout repository
        uses: actions/checkout@v4

      - name: Run Claude PR Action
        uses: anthropics/claude-code-action@beta
        with:
          anthropic_api_key: ${{ secrets.ANTHROPIC_API_KEY }}
          timeout_minutes: "60"
```

**Setup:**
1. Add `ANTHROPIC_API_KEY` to GitHub secrets
2. Or use OAuth: `CLAUDE_CODE_OAUTH_TOKEN`
3. Tag with `@claude` in PR comments to trigger

### Additional Workflow Examples

#### Pull Request Checks

Create `.github/workflows/pr.yml`:

```yaml
name: Pull Request

on:
  pull_request:
    branches: [ main, master ]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.23'

      - name: Run tests
        run: go test -race -coverprofile=coverage.txt -covermode=atomic ./...

      - name: Upload coverage
        uses: codecov/codecov-action@v3
        with:
          files: ./coverage.txt

  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.23'

      - name: golangci-lint
        uses: golangci/golangci-lint-action@v3
        with:
          version: latest
```

## Quality Tooling

### golangci-lint Configuration

Copy `.golangci.yml` and customize for your project:

#### Essential Linters

```yaml
linters:
  enable:
    # Code correctness
    - govet
    - errcheck
    - staticcheck
    - unused
    - gosimple
    - ineffassign
    - typecheck

    # Security
    - gosec

    # Performance
    - prealloc
    - unconvert

    # Style
    - gofmt
    - goimports
    - gofumpt
    - revive
    - misspell

    # Bug prevention
    - bodyclose
    - rowserrcheck
    - sqlclosecheck
```

#### Project-Specific Settings

```yaml
linters-settings:
  revive:
    rules:
      - name: var-naming
        arguments:
          - ["ID", "URL", "HTTP", "API"]  # Your project acronyms

  goimports:
    local-prefixes: github.com/yourusername/yourproject

  lll:
    line-length: 190  # Adjust based on team preference

  gocyclo:
    min-complexity: 20  # Complexity threshold
```

#### Test Exclusions

```yaml
issues:
  exclude-rules:
    - path: _test\.go
      linters:
        - gosec
        - goconst
        - lll
        - errcheck
        - gocyclo
```

### Code Formatting

#### gofumpt

More strict formatting than gofmt:

```bash
# Install
go install mvdan.cc/gofumpt@latest

# Format
task fmt
# or
gofumpt -l -w .
```

#### Pre-commit Hook (Optional)

Create `.git/hooks/pre-commit`:

```bash
#!/bin/sh
task fmt
task lint
task test
```

Make executable:
```bash
chmod +x .git/hooks/pre-commit
```

## Testing Infrastructure

### Testcontainers Setup

#### Installation

```go
// go.mod
require (
    github.com/testcontainers/testcontainers-go v0.26.0
)
```

#### Integration Test Pattern

```go
//go:build integration

package mypackage_test

import (
    "context"
    "testing"

    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/wait"
)

func setupTestContainer(t *testing.T) testcontainers.Container {
    ctx := context.Background()

    req := testcontainers.ContainerRequest{
        Image:        "postgres:16-alpine",
        ExposedPorts: []string{"5432/tcp"},
        Env: map[string]string{
            "POSTGRES_PASSWORD": "test",
            "POSTGRES_USER":     "test",
            "POSTGRES_DB":       "test",
        },
        WaitingFor: wait.ForLog("database system is ready to accept connections"),
    }

    container, err := testcontainers.GenericContainer(ctx,
        testcontainers.GenericContainerRequest{
            ContainerRequest: req,
            Started:          true,
        })

    if err != nil {
        t.Fatal(err)
    }

    t.Cleanup(func() {
        if err := container.Terminate(ctx); err != nil {
            t.Logf("failed to terminate container: %s", err)
        }
    })

    return container
}
```

### Test Structure

```
package/
├── package.go
├── package_test.go              # Unit tests (no build tag)
├── integration_test.go          # //go:build integration
├── helpers_test.go              # Test utilities
└── testdata/
    ├── input.json
    └── expected.json
```

### Coverage Reporting

The Taskfile automatically generates coverage reports:

```bash
# Unit tests
task test
# → output/coverage.html

# Integration tests
task test:integration
# → output/coverage-integration.html

# All tests
task test:all
# → output/coverage-all.html
```

## Release Automation

### Semantic Release Configuration

Copy `.releaserc.json`:

```json
{
  "branches": [
    "main",
    "master"
  ],
  "plugins": [
    [
      "@semantic-release/commit-analyzer",
      {
        "preset": "conventionalcommits",
        "releaseRules": [
          { "type": "docs", "scope": "README", "release": "patch" },
          { "type": "refactor", "release": "patch" },
          { "type": "style", "release": "patch" },
          { "type": "chore", "release": "patch" },
          { "type": "test", "release": "patch" }
        ]
      }
    ],
    [
      "@semantic-release/release-notes-generator",
      {
        "preset": "conventionalcommits"
      }
    ],
    [
      "@semantic-release/github",
      {
        "assets": []
      }
    ]
  ]
}
```

### Version Bumping Rules

| Commit Type | Example | Version Bump |
|-------------|---------|--------------|
| `fix:` | `fix(auth): resolve token expiry` | Patch (1.0.x) |
| `feat:` | `feat(api): add user endpoints` | Minor (1.x.0) |
| `BREAKING CHANGE:` | `feat!: redesign API` | Major (x.0.0) |
| `docs:`, `chore:` | `docs: update README` | Patch (1.0.x) |

### Release Process

1. Commits to `main` trigger release workflow
2. semantic-release analyzes commits
3. Determines version bump
4. Creates GitHub release
5. Generates changelog

**Manual release:**
```bash
npm install -g semantic-release @semantic-release/github conventional-changelog-conventionalcommits
export GITHUB_TOKEN=your_token
npx semantic-release
```

## Customization Guide

### For Library Projects

1. Keep the standard Taskfile
2. Use integration tests with testcontainers
3. Focus on test coverage (80%+ target)
4. Enable all quality linters

### For Service Projects

Add to Taskfile:

```yaml
tasks:
  run:
    desc: Run the service locally
    cmds:
      - go run ./cmd/service

  docker:build:
    desc: Build Docker image
    cmds:
      - docker build -t myservice:{{.VERSION}} .

  deploy:dev:
    desc: Deploy to development
    cmds:
      - kubectl apply -f k8s/dev/
```

### For CLI Projects

Add to Taskfile:

```yaml
tasks:
  build:
    desc: Build CLI for all platforms
    cmds:
      - GOOS=linux GOARCH=amd64 go build -o dist/cli-linux-amd64 ./cmd/cli
      - GOOS=darwin GOARCH=amd64 go build -o dist/cli-darwin-amd64 ./cmd/cli
      - GOOS=windows GOARCH=amd64 go build -o dist/cli-windows-amd64.exe ./cmd/cli

  install:
    desc: Install CLI locally
    cmds:
      - go install ./cmd/cli
```

## Best Practices

### 1. Always Use Task

```bash
# Good
task test
task lint

# Avoid
go test ./...
golangci-lint run
```

### 2. Keep Infrastructure Updated

```bash
# Update from source project
cd /path/to/pkg
git pull

# Copy updated files
cp Taskfile.yml /path/to/yourproject/
cp .golangci.yml /path/to/yourproject/
```

### 3. Customize, Don't Replace

- Start with standard configuration
- Add project-specific tasks/settings
- Document customizations in README

### 4. Test Before Committing

```bash
task check  # Runs test + lint
```

## Troubleshooting

### Task Not Found

```bash
# Install task
npm install -g @go-task/cli
# or
brew install go-task/tap/go-task
```

### Linter Issues

```bash
# Install/update linter
task tools

# Run with fixes
golangci-lint run --fix ./...
```

### CI/CD Failures

1. Check GitHub Actions logs
2. Run tests locally: `task test:all`
3. Verify secrets are configured
4. Check Go version matches CI

### Testcontainers Issues

1. Ensure Docker is running
2. Check Docker resources (4GB+ RAM)
3. Clean up: `docker system prune`

## Next Steps

1. ✅ Copy infrastructure files to your project
2. ✅ Customize Taskfile for your needs
3. ✅ Set up GitHub secrets for CI/CD
4. ✅ Configure golangci-lint for your project
5. ✅ Test the setup: `task test:all`
6. ✅ Commit with conventional format
7. ✅ Push and verify CI/CD pipeline

Your project now has production-grade infrastructure! 🚀
