# Container Package

Temporal workflows for executing Docker containers with advanced orchestration patterns. This package provides Argo Workflow-like capabilities using Temporal, including builder patterns, pre-built workflow templates, and full lifecycle management.

## Key Features

- **Single, Pipeline, Parallel, and DAG workflows** for container execution
- **Fluent Builder API** with container, script, and HTTP templates
- **Wait strategies** — log, port, HTTP, and health-check based readiness detection
- **Pre-built patterns** — CI/CD, fan-out/fan-in, map-reduce, parallel testing
- **Workflow lifecycle** — submit, wait, watch, cancel, terminate, signal, query
- **Resource management** — CPU, memory, GPU limits, artifacts, and `secret://` env references (resolved worker-side; see [docs/security.md](../docs/security.md))

## Documentation

- [Container Workflows Guide](../docs/container-workflows.md) — comprehensive usage guide with examples
- [Architecture](../docs/architecture.md) — how this package fits in the overall system
- [Workflow Patterns](../docs/workflow-patterns.md) — orchestration patterns (pipeline, parallel, DAG)
- [Getting Started](../docs/getting-started.md) — quick start guide

## Quick Example

### Builder path (preferred)

```go
def, err := builder.NewWorkflowBuilder().
    Name("pg-start").
    Single().
    AddInput(payload.ContainerExecutionInput{
        Image:      "postgres:16-alpine",
        Env:        map[string]string{"POSTGRES_PASSWORD": "test"},
        Ports:      []string{"5432:5432"},
        WaitStrategy: payload.WaitStrategyConfig{
            Type:       "log",
            LogMessage: "ready to accept connections",
        },
        AutoRemove: true,
    }).
    Build()

// def is a *job.Definition — register it with a worker and execute it.
w := worker.New(c, def.TaskQueue, worker.Options{})
def.Register(w)
run, err := def.Execute(ctx, c, def.NewInput())
```

See [docs/job-definition.md](../docs/job-definition.md) for the full `*job.Definition` API (lifecycle, scheduling, registry).

### Low-level path

```go
input := payload.ContainerExecutionInput{
    Image:      "postgres:16-alpine",
    Env:        map[string]string{"POSTGRES_PASSWORD": "test"},
    Ports:      []string{"5432:5432"},
    WaitStrategy: payload.WaitStrategyConfig{
        Type:       "log",
        LogMessage: "ready to accept connections",
    },
    AutoRemove: true,
}

we, _ := c.ExecuteWorkflow(ctx,
    client.StartWorkflowOptions{ID: "pg", TaskQueue: "container-pg-start"},
    container.ExecuteContainerWorkflow, input)
```

See [examples/container/](../examples/container/) for complete working examples.
