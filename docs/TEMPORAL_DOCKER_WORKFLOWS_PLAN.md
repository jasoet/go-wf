# Temporal Docker Workflows - Implementation Plan

This document outlines the architecture and implementation plan for reusable Temporal workflows that execute Docker containers.

## Overview

**Goal:** Create a standalone library of pre-built Temporal workflows and activities for executing Docker containers with proper payload validation, observability, and error handling.

**Dependencies:**
- `github.com/jasoet/pkg/v2/docker` - Docker executor
- `github.com/jasoet/pkg/v2/temporal` - Temporal utilities
- `go.temporal.io/sdk` - Temporal SDK

## Project Decision: Separate Repository

### Repository: `temporal-docker-workflows`

**Reasons for separation:**
1. Will contain many ready-to-use workflows in the future
2. Different release cycle than core pkg utilities
3. Other projects will use docker executor as dependency
4. Focused scope for workflow templates
5. Independent versioning and maintenance

### Module Path
```
github.com/yourusername/temporal-docker-workflows
```

## Project Structure

```
temporal-docker-workflows/
├── .github/
│   └── workflows/
│       ├── release.yml              # Semantic release automation
│       └── claude.yml               # Claude Code PR assistant
├── .golangci.yml                    # Linter configuration (from pkg)
├── .releaserc.json                  # Semantic release config (from pkg)
├── .gitignore                       # Standard Go gitignore (from pkg)
├── Taskfile.yml                     # Task automation (from pkg)
├── README.md                        # Main documentation
├── TESTING.md                       # Testing guide (from pkg)
├── LICENSE                          # MIT License
├── go.mod
├── go.sum
├── payloads.go                      # Type-safe input/output structures
├── payloads_test.go
├── workflows.go                     # Pre-built workflow templates
├── workflows_test.go
├── activities.go                    # Docker container activities
├── activities_test.go
├── worker.go                        # Worker registration helpers
├── worker_test.go
├── integration_test.go              # //go:build integration
└── examples/
    ├── README.md
    ├── basic.go                     # //go:build example
    ├── pipeline.go                  # //go:build example
    ├── parallel.go                  # //go:build example
    └── worker/
        └── main.go                  # Example worker setup
```

## Core Components

### 1. Payloads (`payloads.go`)

Type-safe, validated input/output structures with JSON serialization:

```go
package temporaldocker

import (
    "time"
    "github.com/go-playground/validator/v10"
)

// ContainerExecutionInput defines input for single container execution
type ContainerExecutionInput struct {
    // Required fields
    Image        string            `json:"image" validate:"required"`

    // Optional configuration
    Command      []string          `json:"command,omitempty"`
    Entrypoint   []string          `json:"entrypoint,omitempty"`
    Env          map[string]string `json:"env,omitempty"`
    Ports        []string          `json:"ports,omitempty"`
    Volumes      map[string]string `json:"volumes,omitempty"`
    WorkDir      string            `json:"work_dir,omitempty"`
    User         string            `json:"user,omitempty"`

    // Wait strategy
    WaitStrategy WaitStrategyConfig `json:"wait_strategy,omitempty"`

    // Timeouts
    StartTimeout time.Duration     `json:"start_timeout,omitempty"`
    RunTimeout   time.Duration     `json:"run_timeout,omitempty"`

    // Cleanup
    AutoRemove   bool              `json:"auto_remove"`

    // Metadata
    Name         string            `json:"name,omitempty"`
    Labels       map[string]string `json:"labels,omitempty"`
}

// WaitStrategyConfig defines container readiness check
type WaitStrategyConfig struct {
    Type           string        `json:"type" validate:"oneof=log port http healthy"`
    LogMessage     string        `json:"log_message,omitempty"`
    Port           string        `json:"port,omitempty"`
    HTTPPath       string        `json:"http_path,omitempty"`
    HTTPStatus     int           `json:"http_status,omitempty"`
    StartupTimeout time.Duration `json:"startup_timeout,omitempty"`
}

// ContainerExecutionOutput defines output from container execution
type ContainerExecutionOutput struct {
    ContainerID string            `json:"container_id"`
    Name        string            `json:"name,omitempty"`
    ExitCode    int               `json:"exit_code"`
    Stdout      string            `json:"stdout,omitempty"`
    Stderr      string            `json:"stderr,omitempty"`
    Endpoint    string            `json:"endpoint,omitempty"`
    Ports       map[string]string `json:"ports,omitempty"`
    StartedAt   time.Time         `json:"started_at"`
    FinishedAt  time.Time         `json:"finished_at"`
    Duration    time.Duration     `json:"duration"`
    Success     bool              `json:"success"`
    Error       string            `json:"error,omitempty"`
}

// PipelineInput defines sequential container execution
type PipelineInput struct {
    Containers  []ContainerExecutionInput `json:"containers" validate:"required,min=1"`
    StopOnError bool                      `json:"stop_on_error"`
    Cleanup     bool                      `json:"cleanup"` // Cleanup after each step
}

// PipelineOutput defines pipeline execution results
type PipelineOutput struct {
    Results       []ContainerExecutionOutput `json:"results"`
    TotalSuccess  int                        `json:"total_success"`
    TotalFailed   int                        `json:"total_failed"`
    TotalDuration time.Duration              `json:"total_duration"`
}

// ParallelInput defines parallel container execution
type ParallelInput struct {
    Containers      []ContainerExecutionInput `json:"containers" validate:"required,min=1"`
    MaxConcurrency  int                       `json:"max_concurrency,omitempty"` // 0 = unlimited
    FailureStrategy string                    `json:"failure_strategy" validate:"oneof=continue fail_fast"`
}

// ParallelOutput defines parallel execution results
type ParallelOutput struct {
    Results       []ContainerExecutionOutput `json:"results"`
    TotalSuccess  int                        `json:"total_success"`
    TotalFailed   int                        `json:"total_failed"`
    TotalDuration time.Duration              `json:"total_duration"`
}

// Validate validates input using struct tags
func (i *ContainerExecutionInput) Validate() error {
    validate := validator.New()
    return validate.Struct(i)
}
```

### 2. Workflows (`workflows.go`)

Pre-built workflow templates:

```go
package temporaldocker

import (
    "fmt"
    "time"

    "go.temporal.io/sdk/temporal"
    "go.temporal.io/sdk/workflow"
)

// ExecuteContainerWorkflow runs a single container and returns results
func ExecuteContainerWorkflow(ctx workflow.Context, input ContainerExecutionInput) (*ContainerExecutionOutput, error) {
    logger := workflow.GetLogger(ctx)
    logger.Info("Starting container execution workflow",
        "image", input.Image,
        "name", input.Name)

    // Validate input
    if err := input.Validate(); err != nil {
        return nil, fmt.Errorf("invalid input: %w", err)
    }

    // Set default timeout if not specified
    timeout := input.RunTimeout
    if timeout == 0 {
        timeout = 10 * time.Minute
    }

    // Activity options with retry policy
    ao := workflow.ActivityOptions{
        StartToCloseTimeout: timeout,
        RetryPolicy: &temporal.RetryPolicy{
            InitialInterval:    time.Second,
            BackoffCoefficient: 2.0,
            MaximumInterval:    time.Minute,
            MaximumAttempts:    3,
        },
    }
    ctx = workflow.WithActivityOptions(ctx, ao)

    // Execute container activity
    var output ContainerExecutionOutput
    err := workflow.ExecuteActivity(ctx, StartContainerActivity, input).Get(ctx, &output)
    if err != nil {
        logger.Error("Container execution failed", "error", err)
        return nil, err
    }

    logger.Info("Container execution completed",
        "success", output.Success,
        "exitCode", output.ExitCode,
        "duration", output.Duration)

    return &output, nil
}

// ContainerPipelineWorkflow executes containers sequentially
func ContainerPipelineWorkflow(ctx workflow.Context, input PipelineInput) (*PipelineOutput, error) {
    logger := workflow.GetLogger(ctx)
    logger.Info("Starting container pipeline workflow", "steps", len(input.Containers))

    startTime := workflow.Now(ctx)
    output := &PipelineOutput{
        Results: make([]ContainerExecutionOutput, 0, len(input.Containers)),
    }

    for i, containerInput := range input.Containers {
        stepName := containerInput.Name
        if stepName == "" {
            stepName = fmt.Sprintf("step-%d", i+1)
        }

        logger.Info("Executing pipeline step",
            "step", i+1,
            "name", stepName,
            "image", containerInput.Image)

        // Execute step
        var result ContainerExecutionOutput
        err := workflow.ExecuteActivity(ctx, StartContainerActivity, containerInput).Get(ctx, &result)

        output.Results = append(output.Results, result)

        if err != nil || !result.Success {
            output.TotalFailed++
            logger.Error("Pipeline step failed",
                "step", i+1,
                "name", stepName,
                "error", err)

            if input.StopOnError {
                output.TotalDuration = workflow.Now(ctx).Sub(startTime)
                return output, fmt.Errorf("pipeline stopped at step %d: %w", i+1, err)
            }
            continue
        }

        output.TotalSuccess++
        logger.Info("Pipeline step completed",
            "step", i+1,
            "name", stepName,
            "duration", result.Duration)
    }

    output.TotalDuration = workflow.Now(ctx).Sub(startTime)

    logger.Info("Pipeline workflow completed",
        "success", output.TotalSuccess,
        "failed", output.TotalFailed,
        "totalDuration", output.TotalDuration)

    return output, nil
}

// ParallelContainersWorkflow executes multiple containers in parallel
func ParallelContainersWorkflow(ctx workflow.Context, input ParallelInput) (*ParallelOutput, error) {
    logger := workflow.GetLogger(ctx)
    logger.Info("Starting parallel containers workflow",
        "containers", len(input.Containers),
        "maxConcurrency", input.MaxConcurrency)

    startTime := workflow.Now(ctx)

    // Execute containers in parallel
    futures := make([]workflow.Future, len(input.Containers))

    for i, containerInput := range input.Containers {
        // Use child workflow for better isolation
        futures[i] = workflow.ExecuteActivity(ctx, StartContainerActivity, containerInput)
    }

    // Collect results
    output := &ParallelOutput{
        Results: make([]ContainerExecutionOutput, 0, len(input.Containers)),
    }

    for i, future := range futures {
        var result ContainerExecutionOutput
        err := future.Get(ctx, &result)

        output.Results = append(output.Results, result)

        if err != nil || !result.Success {
            output.TotalFailed++

            if input.FailureStrategy == "fail_fast" {
                output.TotalDuration = workflow.Now(ctx).Sub(startTime)
                return output, fmt.Errorf("parallel execution failed at container %d: %w", i, err)
            }
        } else {
            output.TotalSuccess++
        }
    }

    output.TotalDuration = workflow.Now(ctx).Sub(startTime)

    logger.Info("Parallel workflow completed",
        "success", output.TotalSuccess,
        "failed", output.TotalFailed,
        "totalDuration", output.TotalDuration)

    return output, nil
}
```

### 3. Activities (`activities.go`)

Docker container execution using pkg/v2/docker:

```go
package temporaldocker

import (
    "context"
    "fmt"
    "time"

    "github.com/jasoet/pkg/v2/docker"
    "go.temporal.io/sdk/activity"
)

// StartContainerActivity starts a container, waits for completion, and returns results
func StartContainerActivity(ctx context.Context, input ContainerExecutionInput) (*ContainerExecutionOutput, error) {
    logger := activity.GetLogger(ctx)
    logger.Info("Starting container", "image", input.Image, "name", input.Name)

    startTime := time.Now()

    // Build docker executor options
    opts := []docker.Option{
        docker.WithImage(input.Image),
        docker.WithAutoRemove(input.AutoRemove),
    }

    if input.Name != "" {
        opts = append(opts, docker.WithName(input.Name))
    }

    if len(input.Command) > 0 {
        opts = append(opts, docker.WithCmd(input.Command...))
    }

    if len(input.Entrypoint) > 0 {
        opts = append(opts, docker.WithEntrypoint(input.Entrypoint...))
    }

    if len(input.Env) > 0 {
        opts = append(opts, docker.WithEnvMap(input.Env))
    }

    if len(input.Ports) > 0 {
        for _, port := range input.Ports {
            opts = append(opts, docker.WithPorts(port))
        }
    }

    if len(input.Volumes) > 0 {
        opts = append(opts, docker.WithVolumes(input.Volumes))
    }

    if input.WorkDir != "" {
        opts = append(opts, docker.WithWorkDir(input.WorkDir))
    }

    if input.User != "" {
        opts = append(opts, docker.WithUser(input.User))
    }

    if len(input.Labels) > 0 {
        for k, v := range input.Labels {
            opts = append(opts, docker.WithLabel(k, v))
        }
    }

    // Add wait strategy if configured
    if input.WaitStrategy.Type != "" {
        waitStrategy := buildWaitStrategy(input.WaitStrategy)
        opts = append(opts, docker.WithWaitStrategy(waitStrategy))
    }

    // Create executor
    exec, err := docker.New(opts...)
    if err != nil {
        return &ContainerExecutionOutput{
            Name:       input.Name,
            StartedAt:  startTime,
            FinishedAt: time.Now(),
            Success:    false,
            Error:      err.Error(),
        }, err
    }

    // Start container
    if err := exec.Start(ctx); err != nil {
        return &ContainerExecutionOutput{
            Name:       input.Name,
            StartedAt:  startTime,
            FinishedAt: time.Now(),
            Success:    false,
            Error:      err.Error(),
        }, err
    }

    // Ensure cleanup
    defer exec.Terminate(ctx)

    // Get container info
    containerID := exec.GetContainerID()
    logger.Info("Container started", "containerID", containerID)

    // Wait for completion
    exitCode, err := exec.Wait(ctx)
    finishTime := time.Now()

    // Collect logs
    stdout, _ := exec.GetStdout(ctx)
    stderr, _ := exec.GetStderr(ctx)

    // Get endpoint if ports exposed
    var endpoint string
    var ports map[string]string
    if len(input.Ports) > 0 {
        endpoint, _ = exec.Endpoint(ctx, input.Ports[0])
        ports, _ = exec.GetAllPorts(ctx)
    }

    output := &ContainerExecutionOutput{
        ContainerID: containerID,
        Name:        input.Name,
        ExitCode:    exitCode,
        Stdout:      stdout,
        Stderr:      stderr,
        Endpoint:    endpoint,
        Ports:       ports,
        StartedAt:   startTime,
        FinishedAt:  finishTime,
        Duration:    finishTime.Sub(startTime),
        Success:     exitCode == 0 && err == nil,
    }

    if err != nil {
        output.Error = err.Error()
        logger.Error("Container execution failed", "error", err, "exitCode", exitCode)
        return output, err
    }

    logger.Info("Container completed",
        "exitCode", exitCode,
        "duration", output.Duration)

    return output, nil
}

// buildWaitStrategy converts config to docker wait strategy
func buildWaitStrategy(cfg WaitStrategyConfig) docker.WaitStrategy {
    timeout := cfg.StartupTimeout
    if timeout == 0 {
        timeout = 60 * time.Second
    }

    switch cfg.Type {
    case "log":
        return docker.WaitForLog(cfg.LogMessage).WithStartupTimeout(timeout)
    case "port":
        return docker.WaitForPort(cfg.Port)
    case "http":
        status := cfg.HTTPStatus
        if status == 0 {
            status = 200
        }
        return docker.WaitForHTTP(cfg.Port, cfg.HTTPPath, status)
    case "healthy":
        return docker.WaitForHealthy()
    default:
        return docker.WaitForLog("").WithStartupTimeout(timeout)
    }
}
```

### 4. Worker Helper (`worker.go`)

```go
package temporaldocker

import (
    "go.temporal.io/sdk/worker"
)

// RegisterWorkflows registers all docker workflows with a worker
func RegisterWorkflows(w worker.Worker) {
    w.RegisterWorkflow(ExecuteContainerWorkflow)
    w.RegisterWorkflow(ContainerPipelineWorkflow)
    w.RegisterWorkflow(ParallelContainersWorkflow)
}

// RegisterActivities registers all docker activities with a worker
func RegisterActivities(w worker.Worker) {
    w.RegisterActivity(StartContainerActivity)
}

// RegisterAll registers both workflows and activities
func RegisterAll(w worker.Worker) {
    RegisterWorkflows(w)
    RegisterActivities(w)
}
```

## Usage Examples

### Example 1: Basic Container Execution

```go
package main

import (
    "context"
    "log"

    "github.com/jasoet/pkg/v2/temporal"
    temporaldocker "github.com/yourusername/temporal-docker-workflows"
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
    temporaldocker.RegisterAll(w)

    go w.Run(nil)
    defer w.Stop()

    // Execute workflow
    input := temporaldocker.ContainerExecutionInput{
        Image: "postgres:16-alpine",
        Env: map[string]string{
            "POSTGRES_PASSWORD": "test",
            "POSTGRES_USER":     "test",
            "POSTGRES_DB":       "test",
        },
        Ports: []string{"5432:5432"},
        WaitStrategy: temporaldocker.WaitStrategyConfig{
            Type:       "log",
            LogMessage: "ready to accept connections",
        },
        AutoRemove: true,
    }

    we, err := c.ExecuteWorkflow(context.Background(),
        client.StartWorkflowOptions{
            ID:        "postgres-setup",
            TaskQueue: "docker-tasks",
        },
        temporaldocker.ExecuteContainerWorkflow,
        input,
    )
    if err != nil {
        log.Fatal(err)
    }

    var result temporaldocker.ContainerExecutionOutput
    if err := we.Get(context.Background(), &result); err != nil {
        log.Fatal(err)
    }

    log.Printf("Container executed: %s, endpoint: %s", result.ContainerID, result.Endpoint)
}
```

### Example 2: Container Pipeline

```go
// Sequential execution: build → test → deploy
input := temporaldocker.PipelineInput{
    StopOnError: true,
    Containers: []temporaldocker.ContainerExecutionInput{
        {
            Name:  "build",
            Image: "golang:1.23",
            Command: []string{"go", "build", "./..."},
            Volumes: map[string]string{
                "/app": "/workspace",
            },
        },
        {
            Name:  "test",
            Image: "golang:1.23",
            Command: []string{"go", "test", "./..."},
            Volumes: map[string]string{
                "/app": "/workspace",
            },
        },
        {
            Name:  "deploy",
            Image: "alpine/k8s:1.28",
            Command: []string{"kubectl", "apply", "-f", "deployment.yaml"},
        },
    },
}

we, _ := client.ExecuteWorkflow(ctx,
    client.StartWorkflowOptions{
        ID:        "build-pipeline",
        TaskQueue: "docker-tasks",
    },
    temporaldocker.ContainerPipelineWorkflow,
    input,
)
```

### Example 3: Parallel Container Execution

```go
// Run multiple test suites in parallel
input := temporaldocker.ParallelInput{
    MaxConcurrency: 3,
    FailureStrategy: "continue",
    Containers: []temporaldocker.ContainerExecutionInput{
        {
            Name:  "unit-tests",
            Image: "golang:1.23",
            Command: []string{"go", "test", "./internal/..."},
        },
        {
            Name:  "integration-tests",
            Image: "golang:1.23",
            Command: []string{"go", "test", "-tags=integration", "./..."},
        },
        {
            Name:  "e2e-tests",
            Image: "cypress/included:13.0.0",
            Command: []string{"cypress", "run"},
        },
    },
}

we, _ := client.ExecuteWorkflow(ctx,
    client.StartWorkflowOptions{
        ID:        "test-suite",
        TaskQueue: "docker-tasks",
    },
    temporaldocker.ParallelContainersWorkflow,
    input,
)
```

## Testing Strategy

### Unit Tests
- Test payload validation
- Test workflow logic (mocked activities)
- Test activity logic (mocked docker)
- Target: 85% coverage

### Integration Tests
- Use testcontainers for Temporal server
- Use real docker executor
- Test end-to-end workflows
- Test error scenarios

```go
//go:build integration

package temporaldocker_test

import (
    "context"
    "testing"
    "time"

    temporaldocker "github.com/yourusername/temporal-docker-workflows"
    "github.com/testcontainers/testcontainers-go"
)

func TestIntegration_ExecuteContainer(t *testing.T) {
    // Setup Temporal container
    ctx := context.Background()

    req := testcontainers.ContainerRequest{
        Image: "temporalio/auto-setup:latest",
        ExposedPorts: []string{"7233/tcp"},
    }

    container, _ := testcontainers.GenericContainer(ctx,
        testcontainers.GenericContainerRequest{
            ContainerRequest: req,
            Started: true,
        })
    defer container.Terminate(ctx)

    // Create client, worker, execute workflow
    // Test assertions
}
```

## Infrastructure Setup

Use the documentation from `docs/AI_PROJECT_SETUP.md` to:

1. **Copy infrastructure files:**
   - Taskfile.yml
   - .golangci.yml
   - .releaserc.json
   - .github/workflows/

2. **Set up project:**
   - Initialize go module
   - Add dependencies
   - Create package structure
   - Write tests

3. **Configure CI/CD:**
   - GitHub Actions for testing
   - Semantic release for versioning
   - Code quality checks

## Benefits

✅ **Type-safe** - Validated payloads prevent runtime errors
✅ **Reusable** - Pre-built workflows ready to use
✅ **Observable** - Inherits OTel from docker & temporal packages
✅ **Tested** - Comprehensive unit and integration tests
✅ **Easy to use** - Simple registration and execution
✅ **Flexible** - Extensible for custom workflows
✅ **Production-ready** - Error handling, retries, timeouts

## Future Enhancements

1. **Additional Workflows:**
   - Multi-stage builds
   - Blue-green deployments
   - Canary deployments
   - Database migrations

2. **Advanced Features:**
   - Container networking
   - Volume management
   - Resource limits
   - Health check strategies

3. **Integrations:**
   - Kubernetes deployment
   - Cloud provider APIs
   - CI/CD platforms
   - Monitoring systems

## Implementation Checklist

- [ ] Set up project using AI_PROJECT_SETUP.md
- [ ] Implement payloads.go with validation
- [ ] Implement workflows.go
- [ ] Implement activities.go
- [ ] Implement worker.go
- [ ] Write unit tests (85% coverage target)
- [ ] Write integration tests
- [ ] Create examples
- [ ] Write documentation
- [ ] Set up CI/CD
- [ ] First release (v1.0.0)

## Getting Started

To create this project automatically:

```bash
# Provide docs/AI_PROJECT_SETUP.md to your AI agent
# Specify:
# - Project name: temporal-docker-workflows
# - Project type: Library
# - Dependencies: pkg/v2/docker, pkg/v2/temporal

# AI agent will:
# 1. Create project structure
# 2. Copy infrastructure files
# 3. Set up testing framework
# 4. Configure CI/CD
# 5. Initialize git repository
```

---

**Ready to build?** Use `docs/AI_PROJECT_SETUP.md` to automate the entire setup! 🚀
