# Docker Package

Temporal workflows for executing Docker containers with advanced orchestration patterns.

## Features

- **Single Container Execution** - Run individual Docker containers with full configuration
- **Pipeline Workflows** - Sequential container execution with error handling
- **Parallel Execution** - Run multiple containers concurrently
- **Wait Strategies** - Log, port, HTTP, and health check based readiness detection
- **Type-Safe Payloads** - Validated input/output structures
- **Production-Ready** - Built-in retries, timeouts, and error handling

## Installation

```bash
go get github.com/jasoet/go-wf/docker
```

## Quick Start

### Basic Container Execution

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
    c, _ := temporal.NewClient(temporal.DefaultConfig())
    defer c.Close()

    // Create worker
    w := worker.New(c, "docker-tasks", worker.Options{})
    docker.RegisterAll(w)
    go w.Run(nil)
    defer w.Stop()

    // Execute container
    input := docker.ContainerExecutionInput{
        Image: "postgres:16-alpine",
        Env: map[string]string{
            "POSTGRES_PASSWORD": "test",
        },
        Ports: []string{"5432:5432"},
        WaitStrategy: docker.WaitStrategyConfig{
            Type:       "log",
            LogMessage: "ready to accept connections",
        },
        AutoRemove: true,
    }

    we, _ := c.ExecuteWorkflow(context.Background(),
        client.StartWorkflowOptions{
            ID:        "postgres-setup",
            TaskQueue: "docker-tasks",
        },
        docker.ExecuteContainerWorkflow,
        input)

    var result docker.ContainerExecutionOutput
    we.Get(context.Background(), &result)
    log.Printf("Container: %s, Success: %v", result.ContainerID, result.Success)
}
```

## Workflows

### ExecuteContainerWorkflow

Executes a single Docker container with optional wait strategies.

**Input:** `ContainerExecutionInput`
**Output:** `ContainerExecutionOutput`

### ContainerPipelineWorkflow

Executes containers sequentially, optionally stopping on first failure.

**Input:** `PipelineInput`
**Output:** `PipelineOutput`

```go
input := docker.PipelineInput{
    StopOnError: true,
    Containers: []docker.ContainerExecutionInput{
        {Image: "golang:1.23", Command: []string{"go", "build"}},
        {Image: "golang:1.23", Command: []string{"go", "test"}},
    },
}
```

### ParallelContainersWorkflow

Executes multiple containers in parallel with configurable failure strategies.

**Input:** `ParallelInput`
**Output:** `ParallelOutput`

```go
input := docker.ParallelInput{
    FailureStrategy: "continue", // or "fail_fast"
    Containers: []docker.ContainerExecutionInput{
        {Image: "alpine:latest", Name: "task1"},
        {Image: "nginx:alpine", Name: "task2"},
    },
}
```

## Wait Strategies

### Log-based

Wait for specific log message:

```go
WaitStrategy: docker.WaitStrategyConfig{
    Type:           "log",
    LogMessage:     "ready to accept connections",
    StartupTimeout: 30 * time.Second,
}
```

### Port-based

Wait for port to be available:

```go
WaitStrategy: docker.WaitStrategyConfig{
    Type: "port",
    Port: "5432",
}
```

### HTTP-based

Wait for HTTP endpoint to return specific status:

```go
WaitStrategy: docker.WaitStrategyConfig{
    Type:       "http",
    Port:       "80",
    HTTPPath:   "/health",
    HTTPStatus: 200,
}
```

### Health check

Wait for container health check to pass:

```go
WaitStrategy: docker.WaitStrategyConfig{
    Type: "healthy",
}
```

## Configuration Options

### ContainerExecutionInput

| Field | Type | Description |
|-------|------|-------------|
| `Image` | string | Docker image (required) |
| `Command` | []string | Override container command |
| `Entrypoint` | []string | Override entrypoint |
| `Env` | map[string]string | Environment variables |
| `Ports` | []string | Port mappings ("host:container") |
| `Volumes` | map[string]string | Volume mounts |
| `WorkDir` | string | Working directory |
| `User` | string | User to run as |
| `WaitStrategy` | WaitStrategyConfig | Readiness check |
| `RunTimeout` | time.Duration | Execution timeout (default: 10m) |
| `AutoRemove` | bool | Remove container after execution |
| `Name` | string | Container name |
| `Labels` | map[string]string | Container labels |

## Testing

```bash
# Unit tests
go test ./docker/...

# Integration tests (requires Docker)
go test -tags=integration ./docker/...

# Coverage
go test -coverprofile=coverage.out ./docker/...
```

## Examples

See [examples/](./examples/) directory for complete working examples:

- `basic.go` - Simple container execution
- `pipeline.go` - Sequential workflow
- `parallel.go` - Parallel execution
- `worker/main.go` - Worker setup

Run examples:

```bash
go run -tags=example ./docker/examples/basic.go
```

## Error Handling

All workflows implement automatic retries with exponential backoff:

- Initial interval: 1s
- Backoff coefficient: 2.0
- Maximum interval: 1m
- Maximum attempts: 3

## Dependencies

- `github.com/jasoet/pkg/v2/docker` - Docker executor
- `github.com/jasoet/pkg/v2/temporal` - Temporal utilities
- `go.temporal.io/sdk` - Temporal SDK
- `github.com/go-playground/validator/v10` - Input validation

## License

MIT License - see [LICENSE](../LICENSE) file for details.
