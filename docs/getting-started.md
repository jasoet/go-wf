# Getting Started with go-wf

go-wf is a Go library for workflow orchestration built on [Temporal](https://temporal.io). It provides three workflow types: **function** (Go handlers as activities), **container** (Podman/Docker containers as activities), and **datasync** (source-mapper-sink pipelines).

This guide gets you running in under 5 minutes.

## Prerequisites

- **Go 1.26+**
- **Temporal server** — either the dev server (`temporal server start-dev`) or a full deployment
- **Podman** (or Docker) — required for container workflows and the local environment

## Installation

```bash
go get github.com/jasoet/go-wf/v2
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
    "go.temporal.io/sdk/worker"

    fn "github.com/jasoet/go-wf/v2/function"
    fnactivity "github.com/jasoet/go-wf/v2/function/activity"
    fnbuilder "github.com/jasoet/go-wf/v2/function/builder"
    "github.com/jasoet/go-wf/v2/function/payload"
)

func main() {
    c, err := temporal.NewClient(temporal.DefaultConfig())
    if err != nil {
        log.Fatal(err)
    }
    defer c.Close()

    // 1. Register a handler
    registry := fn.NewRegistry()
    _ = registry.Register("greet", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
        name := input.Args["name"]
        return &fn.FunctionOutput{
            Result: map[string]string{"greeting": "Hello, " + name + "!"},
        }, nil
    })

    // 2. Build a job Definition
    def, err := fnbuilder.NewWorkflowBuilder[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput]().
        Name("greet-job").
        Single().
        Activity(fnactivity.NewExecuteFunctionActivity(registry)).
        Build()
    if err != nil {
        log.Fatal(err)
    }

    // 3. Register on a worker and start
    w := worker.New(c, def.TaskQueue, worker.Options{})
    def.Register(w)
    go func() { _ = w.Run(worker.InterruptCh()) }()
    defer w.Stop()
    time.Sleep(time.Second)

    // 4. Execute the workflow
    run, err := def.Execute(context.Background(), c,
        &payload.FunctionExecutionInput{Name: "greet", Args: map[string]string{"name": "Temporal"}},
    )
    if err != nil {
        log.Fatal(err)
    }

    var result payload.FunctionExecutionOutput
    _ = run.Get(context.Background(), &result)
    log.Printf("Result: %v", result.Result)
}
```

`def.TaskQueue` defaults to `"function-<name>"` when not overridden with `.TaskQueue(...)`. For more details see [Function Workflows](function-workflows.md) and [docs/job-definition.md](job-definition.md).

## Quick Start: Container Workflow

A container workflow runs a Podman/Docker container as a Temporal activity.

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/jasoet/pkg/v2/temporal"
    "go.temporal.io/sdk/worker"

    cbuilder "github.com/jasoet/go-wf/v2/container/builder"
    "github.com/jasoet/go-wf/v2/container/payload"
)

func main() {
    c, err := temporal.NewClient(temporal.DefaultConfig())
    if err != nil {
        log.Fatal(err)
    }
    defer c.Close()

    // 1. Build a job Definition
    def, err := cbuilder.NewWorkflowBuilder().
        Name("postgres-example").
        Single().
        AddInput(payload.ContainerExecutionInput{
            Image:      "postgres:16-alpine",
            Env:        map[string]string{"POSTGRES_PASSWORD": "test", "POSTGRES_USER": "test"},
            AutoRemove: true,
            Name:       "example-postgres",
            WaitStrategy: payload.WaitStrategyConfig{
                Type: "log", LogMessage: "ready to accept connections",
                StartupTimeout: 30 * time.Second,
            },
        }).
        Build()
    if err != nil {
        log.Fatal(err)
    }

    // 2. Register on a worker and start
    w := worker.New(c, def.TaskQueue, worker.Options{})
    def.Register(w)
    go func() { _ = w.Run(worker.InterruptCh()) }()
    defer w.Stop()
    time.Sleep(time.Second)

    // 3. Execute the workflow
    run, err := def.Execute(context.Background(), c, def.NewInput())
    if err != nil {
        log.Fatal(err)
    }

    var result payload.ContainerExecutionOutput
    _ = run.Get(context.Background(), &result)
    log.Printf("Container ID: %s, Exit Code: %d", result.ContainerID, result.ExitCode)
}
```

`def.TaskQueue` defaults to `"container-<name>"`. For more details see [Container Workflows](container-workflows.md) and [docs/job-definition.md](job-definition.md).

## Quick Start: Chunked DataSync

For large datasets that must be processed in partitions (e.g., by date range), use `datasync/chunk.ChunkedSync` instead of the plain datasync builder. It walks partitions sequentially, supports cursor-based resume across executions, and issues `ContinueAsNew` when the partition list is too large for one workflow history.

```go
import (
    "github.com/jasoet/go-wf/v2/datasync/chunk"
    "github.com/jasoet/pkg/v2/temporal/job"
)

def, err := chunk.NewChunkedSync[OrderRow, OrderRecord, time.Time]("orders-sync").
    Partitioner(datePartitioner).
    Fetcher(orderFetcher).
    Mapper(orderMapper).
    Sink(orderSink).
    WithTracker(cursorStore).
    MaxPartitionsPerExecution(50).
    ScheduleEvery(6 * time.Hour).
    Build()
```

`def` is a standard `*job.Definition` — use `def.Register(w)` and `def.Execute(...)` as with any other builder. For the full API see [DataSync Workflows](datasync-workflows.md).

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

The workers in this stack are created via `github.com/jasoet/go-wf/v2/worker`.New, which enables Temporal Worker Versioning (Worker Deployments, Pinned behavior) when `TEMPORAL_DEPLOYMENT_NAME` and `TEMPORAL_BUILD_ID` are set — the compose stack sets both (`BUILD_ID` env overrides the default `dev`). In-flight executions finish on the worker build they started with, so redeploying workflow code never breaks replay; without the env vars, the helper behaves exactly like the SDK's `worker.New`. See [Architecture](architecture.md#workflow-versioning) for details.

## Next Steps

- [Function Workflows](function-workflows.md) — handlers, pipelines, parallel execution, loops
- [Container Workflows](container-workflows.md) — images, wait strategies, DAGs, data passing
- [DataSync Workflows](datasync-workflows.md) — source, mapper, sink pipelines
- [Workflow Patterns](workflow-patterns.md) — common patterns across all workflow types
- [Architecture](architecture.md) — design decisions and module structure
