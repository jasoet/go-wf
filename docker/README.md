# Docker Package

Temporal workflows for executing Docker containers with advanced orchestration patterns. This package provides Argo Workflow-like capabilities using Temporal, with builder patterns, pre-built workflow templates, and lifecycle management.

## Features

### Core Workflows
- **Single Container Execution** - Run individual Docker containers with full configuration
- **Pipeline Workflows** - Sequential container execution with error handling
- **Parallel Execution** - Run multiple containers concurrently with configurable limits
- **DAG Workflows** - Directed Acyclic Graph execution with dependency management
- **Wait Strategies** - Log, port, HTTP, and health check based readiness detection

### Builder & Templates
- **Fluent Builder API** - Compose complex workflows with readable, chainable methods
- **Container Templates** - Enhanced container execution with functional options
- **Script Templates** - Execute bash, python, node, ruby, or golang scripts
- **HTTP Templates** - Make HTTP requests, health checks, and webhook calls
- **Pre-built Patterns** - CI/CD, fan-out/fan-in, map-reduce, parallel testing, multi-region deployment

### Advanced Features
- **Workflow Lifecycle** - Submit, wait, watch, cancel, terminate, signal, and query workflows
- **Conditional Execution** - When clauses and ContinueOnFail behaviors
- **Resource Management** - CPU, memory, and GPU limits
- **Artifacts & Secrets** - Input/output artifacts and secret injection
- **Workflow Parameters** - Template variables for reusable workflows
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

## Builder API

The builder API provides a fluent interface for constructing complex workflows from composable parts.

### Basic Pipeline

```go
import (
    "github.com/jasoet/go-wf/docker/builder"
    "github.com/jasoet/go-wf/docker/template"
)

// Build a simple CI/CD pipeline
input, err := builder.NewWorkflowBuilder("ci-cd").
    Add(template.NewContainer("build", "golang:1.23",
        template.WithCommand("go", "build", "-o", "app"),
        template.WithWorkDir("/workspace"))).
    Add(template.NewContainer("test", "golang:1.23",
        template.WithCommand("go", "test", "./..."),
        template.WithWorkDir("/workspace"))).
    Add(template.NewContainer("deploy", "deployer:v1",
        template.WithCommand("deploy.sh"))).
    StopOnError(true).
    BuildPipeline()
```

### Parallel Execution

```go
// Execute multiple tasks in parallel
input, err := builder.NewWorkflowBuilder("parallel-tasks").
    Add(template.NewContainer("task1", "alpine:latest",
        template.WithCommand("echo", "Task 1"))).
    Add(template.NewContainer("task2", "alpine:latest",
        template.WithCommand("echo", "Task 2"))).
    Add(template.NewContainer("task3", "alpine:latest",
        template.WithCommand("echo", "Task 3"))).
    Parallel(true).
    FailFast(false).
    MaxConcurrency(2).
    BuildParallel()
```

### Exit Handlers

```go
// Add cleanup or notification tasks
notify := template.NewHTTPWebhook("notify",
    "https://hooks.slack.com/services/...",
    `{"text": "Pipeline completed"}`)

input, err := builder.NewWorkflowBuilder("pipeline-with-notify").
    Add(template.NewContainer("main-task", "alpine:latest")).
    AddExitHandler(notify).
    BuildPipeline()
```

## Template Types

### Container Template

Enhanced container execution with fluent options:

```go
container := template.NewContainer("postgres", "postgres:16-alpine",
    template.WithEnv("POSTGRES_PASSWORD", "secret"),
    template.WithEnv("POSTGRES_DB", "mydb"),
    template.WithPorts("5432:5432"),
    template.WithVolume("/data", "/var/lib/postgresql/data"),
    template.WithWaitForLog("ready to accept connections"),
    template.WithWorkDir("/app"),
    template.WithUser("postgres"))
```

### Script Template

Execute inline scripts in various languages:

```go
// Bash script
bash := template.NewBashScript("setup",
    `#!/bin/bash
    set -e
    echo "Setting up environment..."
    apt-get update && apt-get install -y curl
    curl -fsSL https://get.docker.com | sh`,
    template.WithScriptEnv("DEBIAN_FRONTEND", "noninteractive"))

// Python script
python := template.NewPythonScript("analyze",
    `import sys
    import json

    data = json.load(sys.stdin)
    result = sum(data['values'])
    print(f"Total: {result}")`,
    template.WithScriptImage("python:3.12-slim"))

// Node.js script
node := template.NewNodeScript("build",
    `const fs = require('fs');
    console.log('Building assets...');
    // Build logic here`)

// Ruby script
ruby := template.NewRubyScript("migrate",
    `require 'json'
    puts "Running migrations..."`)

// Go script
golang := template.NewGoScript("tool",
    `package main
    import "fmt"
    func main() {
        fmt.Println("Running tool...")
    }`)
```

### HTTP Template

Make HTTP requests, health checks, and webhook calls:

```go
// Health check
health := template.NewHTTPHealthCheck("check-api",
    "https://api.example.com/health",
    template.WithHTTPMethod("GET"),
    template.WithHTTPExpectedStatus(200),
    template.WithHTTPTimeout(30))

// Webhook notification
webhook := template.NewHTTPWebhook("notify",
    "https://hooks.slack.com/services/T00/B00/XXX",
    `{"text": "Deployment completed"}`,
    template.WithHTTPHeader("Content-Type", "application/json"))

// Custom HTTP request
request := template.NewHTTP("api-call",
    template.WithHTTPURL("https://api.example.com/data"),
    template.WithHTTPMethod("POST"),
    template.WithHTTPBody(`{"key": "value"}`),
    template.WithHTTPHeader("Authorization", "Bearer token"),
    template.WithHTTPExpectedStatus(201))
```

## Pre-built Workflow Patterns

The package includes battle-tested workflow patterns for common use cases.

### CI/CD Patterns

```go
import "github.com/jasoet/go-wf/docker/patterns"

// Build → Test → Deploy pipeline
input, err := patterns.BuildTestDeploy(
    "golang:1.23",
    "golang:1.23",
    "deployer:v1")

// With health check after deployment
input, err := patterns.BuildTestDeployWithHealthCheck(
    "golang:1.23",
    "deployer:v1",
    "https://myapp.com/health")

// With Slack notification
input, err := patterns.BuildTestDeployWithNotification(
    "golang:1.23",
    "deployer:v1",
    "https://hooks.slack.com/services/...",
    `{"text": "Deploy complete"}`)

// Multi-environment deployment
input, err := patterns.MultiEnvironmentDeploy(
    "deployer:v1",
    []string{"staging", "production"})
```

### Parallel Patterns

```go
// Fan-out/Fan-in: Execute multiple tasks in parallel
input, err := patterns.FanOutFanIn(
    "alpine:latest",
    []string{"task-1", "task-2", "task-3"})

// Parallel data processing
input, err := patterns.ParallelDataProcessing(
    "processor:v1",
    []string{"data-1.csv", "data-2.csv", "data-3.csv"},
    "process.sh")

// Parallel test suite
input, err := patterns.ParallelTestSuite(
    "golang:1.23",
    map[string]string{
        "unit":        "go test ./internal/...",
        "integration": "go test ./tests/integration/...",
        "e2e":         "go test ./tests/e2e/...",
    })

// Multi-region deployment
input, err := patterns.ParallelDeployment(
    "deployer:v1",
    []string{"us-west", "us-east", "eu-central"})

// Map-Reduce workflow
input, err := patterns.MapReduce(
    "alpine:latest",
    []string{"file1.txt", "file2.txt", "file3.txt"},
    "wc -w",
    "awk '{sum+=$1} END {print sum}'")
```

## DAG Workflows

Execute containers in a dependency graph where execution order is determined by dependencies:

```go
import "github.com/jasoet/go-wf/docker"

// Define nodes with dependencies
input := docker.DAGWorkflowInput{
    Nodes: []docker.DAGNode{
        {
            Name: "checkout",
            Container: docker.ExtendedContainerInput{
                ContainerExecutionInput: docker.ContainerExecutionInput{
                    Image:   "alpine/git",
                    Command: []string{"git", "clone", "..."},
                },
            },
        },
        {
            Name: "build",
            Container: docker.ExtendedContainerInput{
                ContainerExecutionInput: docker.ContainerExecutionInput{
                    Image:   "golang:1.23",
                    Command: []string{"go", "build"},
                },
            },
            Dependencies: []string{"checkout"},
        },
        {
            Name: "test-unit",
            Container: docker.ExtendedContainerInput{
                ContainerExecutionInput: docker.ContainerExecutionInput{
                    Image:   "golang:1.23",
                    Command: []string{"go", "test", "./..."},
                },
            },
            Dependencies: []string{"build"},
        },
        {
            Name: "test-integration",
            Container: docker.ExtendedContainerInput{
                ContainerExecutionInput: docker.ContainerExecutionInput{
                    Image:   "golang:1.23",
                    Command: []string{"go", "test", "-tags=integration", "./..."},
                },
            },
            Dependencies: []string{"build"},
        },
        {
            Name: "deploy",
            Container: docker.ExtendedContainerInput{
                ContainerExecutionInput: docker.ContainerExecutionInput{
                    Image:   "deployer:v1",
                    Command: []string{"deploy.sh"},
                },
            },
            Dependencies: []string{"test-unit", "test-integration"},
        },
    },
    FailFast: true,
}

// Execute DAG workflow
we, _ := c.ExecuteWorkflow(ctx,
    client.StartWorkflowOptions{
        ID:        "dag-workflow",
        TaskQueue: "docker-tasks",
    },
    docker.DAGWorkflow,
    input)

var output docker.DAGWorkflowOutput
we.Get(ctx, &output)
```

## Workflow Lifecycle Management

Manage workflow execution lifecycle with helper functions:

### Submit and Wait

```go
import "github.com/jasoet/go-wf/docker"

// Submit workflow and wait for completion
status, err := docker.SubmitAndWait(ctx, temporalClient, input, "docker-queue", 10*time.Minute)
if err != nil {
    log.Printf("Workflow failed: %v", err)
} else {
    log.Printf("Workflow completed: %+v", status.Result)
}
```

### Submit Async

```go
// Submit workflow without waiting
status, err := docker.SubmitWorkflow(ctx, temporalClient, input, "docker-queue")
log.Printf("Workflow started: %s (RunID: %s)", status.WorkflowID, status.RunID)

// Check status later
status, err = docker.GetWorkflowStatus(ctx, temporalClient, workflowID, runID)
```

### Watch Workflow

```go
// Stream workflow updates
updates := make(chan *docker.WorkflowStatus)
go docker.WatchWorkflow(ctx, temporalClient, workflowID, runID, updates)

for status := range updates {
    log.Printf("Status: %s", status.Status)
}
```

### Cancel/Terminate

```go
// Cancel workflow (allows graceful cleanup)
err := docker.CancelWorkflow(ctx, temporalClient, workflowID, runID)

// Terminate workflow (immediate stop)
err := docker.TerminateWorkflow(ctx, temporalClient, workflowID, runID, "reason")
```

### Signal/Query

```go
// Send signal to workflow
err := docker.SignalWorkflow(ctx, temporalClient, workflowID, runID, "pause", nil)

// Query workflow state
var state string
err := docker.QueryWorkflow(ctx, temporalClient, workflowID, runID, "status", &state)
```

## Advanced Features

### Workflow Parameters

Use template variables for reusable workflows:

```go
input := docker.ContainerExecutionInput{
    Image:   "alpine:latest",
    Command: []string{"echo", "{{.version}}"},
    Env: map[string]string{
        "VERSION":     "{{.version}}",
        "ENVIRONMENT": "{{.env}}",
    },
}

params := []docker.WorkflowParameter{
    {Name: "version", Value: "v1.2.3"},
    {Name: "env", Value: "production"},
}

output, err := docker.WorkflowWithParameters(ctx, input, params)
```

### Resource Limits

Configure CPU, memory, and GPU limits:

```go
node := docker.DAGNode{
    Name: "ml-training",
    Container: docker.ExtendedContainerInput{
        ContainerExecutionInput: docker.ContainerExecutionInput{
            Image: "ml-trainer:latest",
        },
        Resources: &docker.ResourceLimits{
            CPURequest:    "2000m",
            CPULimit:      "4000m",
            MemoryRequest: "4Gi",
            MemoryLimit:   "8Gi",
            GPUCount:      1,
        },
    },
}
```

### Conditional Execution

Control execution flow with conditions:

```go
node := docker.DAGNode{
    Name: "deploy-production",
    Container: docker.ExtendedContainerInput{
        ContainerExecutionInput: docker.ContainerExecutionInput{
            Image: "deployer:v1",
        },
        Conditional: &docker.ConditionalBehavior{
            When:           "{{steps.test.exitCode}} == 0",
            ContinueOnFail: false,
        },
    },
}
```

### Artifacts

Define input and output artifacts:

```go
container := docker.ExtendedContainerInput{
    ContainerExecutionInput: docker.ContainerExecutionInput{
        Image: "builder:v1",
    },
    InputArtifacts: []docker.Artifact{
        {Name: "source", Path: "/src", Type: "directory"},
    },
    OutputArtifacts: []docker.Artifact{
        {Name: "binary", Path: "/output/app", Type: "file"},
        {Name: "logs", Path: "/logs", Type: "archive"},
    },
}
```

### Secrets

Inject secrets as environment variables:

```go
container := docker.ExtendedContainerInput{
    ContainerExecutionInput: docker.ContainerExecutionInput{
        Image: "app:v1",
    },
    Secrets: []docker.SecretReference{
        {Name: "db-creds", Key: "password", EnvVar: "DB_PASSWORD"},
        {Name: "api-keys", Key: "github", EnvVar: "GITHUB_TOKEN"},
    },
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

### DAGWorkflow

Executes containers in a directed acyclic graph with dependency management.

**Input:** `DAGWorkflowInput`
**Output:** `DAGWorkflowOutput`

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

## Argo Workflows Comparison

This package provides **~70% feature parity** with Argo Workflows, offering similar capabilities with a type-safe Go API instead of YAML. See [ARGO_COMPARISON.md](./ARGO_COMPARISON.md) for detailed feature comparison.

### Key Advantages over Argo Workflows
- **Type-Safe API** - Compile-time validation vs YAML
- **Simpler Developer Experience** - Fluent Go API
- **No Kubernetes Required** - Works with plain Docker
- **Better Local Development** - Easy testing and debugging
- **Pre-built Patterns** - Ready-to-use CI/CD workflows
- **Temporal Benefits** - Built-in observability, retries, versioning

### Argo-equivalent Features
| Argo Workflows | go-wf/docker | Example |
|---------------|--------------|---------|
| Container Template | `template.NewContainer` | `basic.go` |
| Script Template | `template.NewBashScript`, etc. | `builder.go` |
| DAG | `DAGWorkflow` | `dag.go` |
| Steps (Sequential) | `ContainerPipelineWorkflow` | `pipeline.go` |
| Steps (Parallel) | `ParallelContainersWorkflow` | `parallel.go` |
| Loops (withItems) | `LoopWorkflow` | `loop.go` |
| Loops (withParam) | `ParameterizedLoopWorkflow` | `loop.go` |
| Parameters | `WorkflowParameter` | `advanced.go` |
| Resource Limits | `ResourceLimits` | `advanced.go` |
| Conditionals | `ConditionalBehavior` | `advanced.go` |
| Exit Handlers | `AddExitHandler` | `builder.go` |
| Data Passing | `OutputDefinition`, `InputMapping` | `data-passing.go` |
| Artifacts | `Artifact`, `ArtifactStore` | `artifacts.go` |
| Retries | Temporal retry policies | All examples |
| Lifecycle Ops | `Submit`, `Cancel`, `Watch`, etc. | See operations.go |

### Notable Gaps
- **Sidecars** - Not yet supported (K8s-specific)
- **Suspend/Resume** - Use Temporal signals as workaround

See [ARGO_COMPARISON.md](./ARGO_COMPARISON.md) for complete details and workarounds.
See [ROADMAP.md](./ROADMAP.md) for implementation plans to close these gaps.

## Examples

See [examples/docker/](../examples/docker/) directory for complete working examples with comprehensive documentation.

### Core Examples
- `basic.go` - Single container execution with wait strategies
- `pipeline.go` - Sequential workflow (Build → Test → Deploy)
- `parallel.go` - Parallel execution with concurrency control
- `worker/main.go` - Worker setup and registration

### Advanced Workflows
- `dag.go` - Complex DAG workflow with dependencies (Argo DAG equivalent)
- `loop.go` - Loop patterns: withItems, withParam, matrix builds, batch processing
- `data-passing.go` - Explicit data passing between steps with JSONPath and regex extraction
- `artifacts.go` - Artifact storage and retrieval (local and Minio/S3)

### Builder & Templates
- `builder.go` - Builder patterns with script templates and HTTP operations
- `advanced.go` - Parameters, resource limits, conditionals, wait strategies

### Quick Start

```bash
# Start Temporal server
temporal server start-dev

# In another terminal, run worker
cd examples/docker/worker
go run -tags example main.go

# In another terminal, run any example
cd examples/docker
go run -tags example basic.go
go run -tags example dag.go
go run -tags example loop.go
go run -tags example data-passing.go
go run -tags example artifacts.go
go run -tags example builder.go
go run -tags example advanced.go
```

For detailed documentation, see [examples/docker/README.md](../examples/docker/README.md).

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
