# Getting Started with go-wf

go-wf is a Go library for workflow orchestration built on [Temporal](https://temporal.io). It provides three workflow types: **function** (Go handlers as activities), **container** (Podman/Docker containers as activities), and **datasync** (source-mapper-sink pipelines).

This guide gets you running in under 5 minutes.

## Prerequisites

- **Go 1.26+**
- **Temporal server** — either the dev server (`temporal server start-dev`) or a full deployment
- **Podman** (or Docker) — required for container workflows and the local environment

## Installation

```bash
go get github.com/jasoet/go-wf
```

## Quick Start: Function Workflow

A function workflow registers a Go handler and executes it as a Temporal activity.

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/jasoet/pkg/v2/temporal"
    "go.temporal.io/sdk/client"
    "go.temporal.io/sdk/worker"

    fn "github.com/jasoet/go-wf/function"
    fnactivity "github.com/jasoet/go-wf/function/activity"
    "github.com/jasoet/go-wf/function/payload"
    "github.com/jasoet/go-wf/function/workflow"
)

func main() {
    c, closer, _ := temporal.NewClient(temporal.DefaultConfig())
    defer c.Close()
    if closer != nil { defer closer.Close() }

    // 1. Register a handler
    registry := fn.NewRegistry()
    _ = registry.Register("greet", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
        name := input.Args["name"]
        return &fn.FunctionOutput{
            Result: map[string]string{"greeting": "Hello, " + name + "!"},
        }, nil
    })

    // 2. Create and start a worker
    w := worker.New(c, "function-tasks", worker.Options{})
    fn.RegisterWorkflows(w)
    fn.RegisterActivity(w, fnactivity.NewExecuteFunctionActivity(registry))
    go func() { _ = w.Run(worker.InterruptCh()) }()
    defer w.Stop()
    time.Sleep(time.Second)

    // 3. Execute the workflow
    we, _ := c.ExecuteWorkflow(context.Background(),
        client.StartWorkflowOptions{ID: "greet-example", TaskQueue: "function-tasks"},
        workflow.ExecuteFunctionWorkflow,
        payload.FunctionExecutionInput{Name: "greet", Args: map[string]string{"name": "Temporal"}},
    )

    var result payload.FunctionExecutionOutput
    _ = we.Get(context.Background(), &result)
    log.Printf("Result: %v", result.Result)
}
```

## Quick Start: Container Workflow

A container workflow runs a Podman/Docker container as a Temporal activity.

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/jasoet/pkg/v2/temporal"
    "go.temporal.io/sdk/client"
    "go.temporal.io/sdk/worker"

    "github.com/jasoet/go-wf/container"
    "github.com/jasoet/go-wf/container/payload"
    "github.com/jasoet/go-wf/container/workflow"
)

func main() {
    c, closer, _ := temporal.NewClient(temporal.DefaultConfig())
    defer c.Close()
    if closer != nil { defer closer.Close() }

    // 1. Create and start a worker
    w := worker.New(c, "container-tasks", worker.Options{})
    container.RegisterAll(w)
    go func() { _ = w.Run(worker.InterruptCh()) }()
    defer w.Stop()
    time.Sleep(time.Second)

    // 2. Define and execute the container task
    input := payload.ContainerExecutionInput{
        Image:      "postgres:16-alpine",
        Env:        map[string]string{"POSTGRES_PASSWORD": "test", "POSTGRES_USER": "test"},
        AutoRemove: true,
        Name:       "example-postgres",
        WaitStrategy: payload.WaitStrategyConfig{
            Type: "log", LogMessage: "ready to accept connections",
            StartupTimeout: 30 * time.Second,
        },
    }

    we, _ := c.ExecuteWorkflow(context.Background(),
        client.StartWorkflowOptions{ID: "postgres-example", TaskQueue: "container-tasks"},
        workflow.ExecuteContainerWorkflow, input,
    )

    var result payload.ContainerExecutionOutput
    _ = we.Get(context.Background(), &result)
    log.Printf("Container ID: %s, Exit Code: %d", result.ContainerID, result.ExitCode)
}
```

## Running Examples

List all available examples:

```bash
task example:list
```

Run individual examples (Temporal dev server must be running):

```bash
task example:function -- basic.go
task example:container -- basic.go
task example:datasync -- basic.go
```

Run the full demo (starts Temporal, launches workers, runs all examples):

```bash
task demo
```

Or start the demo environment in the background and run examples interactively:

```bash
task demo:start          # starts Temporal + container worker
task example:function -- basic.go
task example:container -- basic.go
task demo:stop           # clean up
```

## Local Development Environment

The project includes a `compose.yml` with Temporal, PostgreSQL, RustFS (S3-compatible object store), and pre-built workers. Start it with Podman:

```bash
task local:up
```

This brings up:
- **Temporal** at `localhost:7233` (UI at `http://localhost:8233`)
- **RustFS** at `localhost:9000` (console at `http://localhost:9001`, credentials: `rustfsadmin`/`rustfsadmin`)
- **Function worker**, **datasync worker**, and a **trigger** service

To include the container worker (requires a container socket):

```bash
task local:up:all CONTAINER_SOCK=/run/podman/podman.sock
```

Other useful commands:

```bash
task local:logs       # follow logs
task local:trigger    # submit all example workflows
task local:down       # stop everything
task local:clean      # stop and remove volumes
```

## Next Steps

- [Function Workflows](function-workflows.md) — handlers, pipelines, parallel execution, loops
- [Container Workflows](container-workflows.md) — images, wait strategies, DAGs, data passing
- [DataSync Workflows](datasync-workflows.md) — source, mapper, sink pipelines
- [Workflow Patterns](workflow-patterns.md) — common patterns across all workflow types
- [Architecture](architecture.md) — design decisions and module structure
