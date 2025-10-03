# Testing Guide

This document explains the testing strategy and practices for the go-wf project.

## Overview

The project uses a comprehensive testing approach with two types of tests:

1. **Unit Tests** - Fast, isolated tests with mocked dependencies
2. **Integration Tests** - Tests with real dependencies using testcontainers

## Test Coverage Target

- **Minimum:** 80%
- **Target:** 85%+
- **Goal:** Comprehensive coverage of all critical paths

## Running Tests

### Unit Tests

Run fast unit tests without external dependencies:

```bash
task test
```

This will:
- Run all tests without build tags
- Generate coverage report
- Create HTML coverage report at `output/coverage.html`

### Integration Tests

Run integration tests with Docker containers:

```bash
task test:integration
```

Requirements:
- Docker must be running
- Sufficient resources (4GB+ RAM recommended)
- Network connectivity for pulling images

### All Tests

Run complete test suite:

```bash
task test:all
```

This runs both unit and integration tests with combined coverage report.

View coverage:
```bash
# Open in browser
open output/coverage-all.html

# Or view in terminal
go tool cover -func=output/coverage-all.out
```

## Test Organization

### File Structure

```
package/
├── package.go                # Implementation
├── package_test.go           # Unit tests (no build tag)
├── integration_test.go       # Integration tests (//go:build integration)
├── helpers_test.go           # Test utilities
└── testdata/                 # Test fixtures
    └── sample.json
```

### Build Tags

#### Unit Tests
No build tag required - runs by default:

```go
package mypackage

import "testing"

func TestMyFunction(t *testing.T) {
    // Unit test
}
```

#### Integration Tests
Use `//go:build integration` tag:

```go
//go:build integration

package mypackage_test

import (
    "testing"
    "context"
    "github.com/testcontainers/testcontainers-go"
)

func TestIntegration_MyFeature(t *testing.T) {
    // Integration test with containers
}
```

## Writing Tests

### Table-Driven Tests

Use table-driven tests for comprehensive coverage:

```go
func TestMyFunction(t *testing.T) {
    tests := []struct {
        name    string
        input   Input
        want    Output
        wantErr bool
    }{
        {
            name: "valid input",
            input: Input{Value: "test"},
            want: Output{Result: "expected"},
            wantErr: false,
        },
        {
            name: "invalid input",
            input: Input{Value: ""},
            want: Output{},
            wantErr: true,
        },
        {
            name: "edge case - nil",
            input: Input{},
            want: Output{},
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := MyFunction(tt.input)

            if (err != nil) != tt.wantErr {
                t.Errorf("MyFunction() error = %v, wantErr %v", err, tt.wantErr)
                return
            }

            if !reflect.DeepEqual(got, tt.want) {
                t.Errorf("MyFunction() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

### Integration Tests with Testcontainers

Use testcontainers for real dependency testing:

```go
//go:build integration

package mypackage_test

import (
    "context"
    "testing"

    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/wait"
)

func TestIntegration_DatabaseOperations(t *testing.T) {
    ctx := context.Background()

    // Setup container
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

    // Cleanup
    t.Cleanup(func() {
        if err := container.Terminate(ctx); err != nil {
            t.Logf("failed to terminate container: %s", err)
        }
    })

    // Get connection details
    host, err := container.Host(ctx)
    if err != nil {
        t.Fatal(err)
    }

    port, err := container.MappedPort(ctx, "5432")
    if err != nil {
        t.Fatal(err)
    }

    // Run tests with real database
    // ...
}
```

### Test Helpers

Create reusable test utilities:

```go
// helpers_test.go
package mypackage

import "testing"

func setupTestDB(t *testing.T) *DB {
    t.Helper()

    db := NewDB(":memory:")

    t.Cleanup(func() {
        db.Close()
    })

    return db
}

func assertEqual(t *testing.T, got, want interface{}) {
    t.Helper()

    if !reflect.DeepEqual(got, want) {
        t.Errorf("got %v, want %v", got, want)
    }
}
```

## Test Best Practices

### 1. Test Naming

- Use descriptive test names
- Format: `TestFunctionName_Scenario`
- Table test cases: describe the scenario

```go
func TestNewClient_ValidConfig(t *testing.T) { }
func TestNewClient_InvalidConfig(t *testing.T) { }
```

### 2. Test Independence

- Tests should not depend on each other
- Use `t.Cleanup()` for resource cleanup
- Don't share state between tests

### 3. Error Testing

Always test error cases:

```go
tests := []struct {
    name    string
    wantErr bool
    errMsg  string
}{
    {
        name:    "success",
        wantErr: false,
    },
    {
        name:    "invalid input",
        wantErr: true,
        errMsg:  "invalid input",
    },
}
```

### 4. Edge Cases

Test boundary conditions:

- Empty strings
- Nil values
- Zero values
- Maximum values
- Negative numbers
- Concurrent access

### 5. Test Coverage

Run with coverage to identify gaps:

```bash
task test:all
open output/coverage-all.html
```

Focus on:
- Critical business logic
- Error handling paths
- Edge cases
- Public API surface

## Mocking

### Using Interfaces

Define interfaces for dependencies:

```go
type Storage interface {
    Get(key string) (string, error)
    Set(key, value string) error
}

type Client struct {
    storage Storage
}
```

Create mock implementations:

```go
type mockStorage struct {
    data map[string]string
    err  error
}

func (m *mockStorage) Get(key string) (string, error) {
    if m.err != nil {
        return "", m.err
    }
    return m.data[key], nil
}

func (m *mockStorage) Set(key, value string) error {
    if m.err != nil {
        return m.err
    }
    m.data[key] = value
    return nil
}
```

Use in tests:

```go
func TestClient_Get(t *testing.T) {
    mock := &mockStorage{
        data: map[string]string{"key": "value"},
    }

    client := &Client{storage: mock}

    got, err := client.Get("key")
    if err != nil {
        t.Fatal(err)
    }

    if got != "value" {
        t.Errorf("got %v, want %v", got, "value")
    }
}
```

## Test Data

### Using testdata/

Store test fixtures in `testdata/`:

```
package/
└── testdata/
    ├── valid.json
    ├── invalid.json
    └── sample.yaml
```

Load in tests:

```go
func TestParseConfig(t *testing.T) {
    data, err := os.ReadFile("testdata/valid.json")
    if err != nil {
        t.Fatal(err)
    }

    cfg, err := ParseConfig(data)
    // Test cfg
}
```

## Continuous Integration

Tests run automatically on:

1. **Pull Requests** - All tests must pass
2. **Main branch pushes** - Full test suite + release
3. **Local development** - Run before committing

CI configuration: `.github/workflows/release.yml`

## Troubleshooting

### Tests Fail Locally

```bash
# Clean build cache
go clean -testcache

# Run with verbose output
go test -v ./...

# Run specific test
go test -v -run TestMyFunction ./package
```

### Integration Tests Fail

```bash
# Check Docker is running
docker info

# Check available resources
docker system df

# Clean up containers
docker system prune -a
```

### Coverage Not Generated

```bash
# Ensure output directory exists
mkdir -p output

# Run with explicit coverage flags
go test -coverprofile=output/coverage.out ./...
```

## Resources

- [Go Testing Package](https://golang.org/pkg/testing/)
- [Testcontainers for Go](https://golang.testcontainers.org/)
- [Table Driven Tests](https://github.com/golang/go/wiki/TableDrivenTests)
- [Go Code Coverage](https://go.dev/blog/cover)

## Summary

✅ Write table-driven tests
✅ Use testcontainers for integration tests
✅ Aim for 85%+ coverage
✅ Test error cases and edge conditions
✅ Keep tests independent and fast
✅ Run `task check` before committing
