# Container Package

Temporal workflows for executing Docker containers with advanced orchestration patterns. This package provides Argo Workflow-like capabilities using Temporal, including builder patterns, pre-built workflow templates, and full lifecycle management.

## Key Features

- **Single, Pipeline, Parallel, and DAG workflows** for container execution
- **Fluent Builder API** with container, script, and HTTP templates
- **Wait strategies** — log, port, HTTP, and health-check based readiness detection
- **Pre-built patterns** — CI/CD, fan-out/fan-in, map-reduce, parallel testing
- **Workflow lifecycle** — submit, wait, watch, cancel, terminate, signal, query
- **Resource management** — CPU, memory, GPU limits, artifacts, and secrets

## Documentation

- [Container Workflows Guide](../docs/container-workflows.md) — comprehensive usage guide with examples
- [Architecture](../docs/architecture.md) — how this package fits in the overall system
- [Workflow Patterns](../docs/workflow-patterns.md) — orchestration patterns (pipeline, parallel, DAG)
- [Getting Started](../docs/getting-started.md) — quick start guide

## Quick Example

```go
input := container.ContainerExecutionInput{
    Image:      "postgres:16-alpine",
    Env:        map[string]string{"POSTGRES_PASSWORD": "test"},
    Ports:      []string{"5432:5432"},
    WaitStrategy: container.WaitStrategyConfig{
        Type:       "log",
        LogMessage: "ready to accept connections",
    },
    AutoRemove: true,
}

we, _ := c.ExecuteWorkflow(ctx,
    client.StartWorkflowOptions{ID: "pg", TaskQueue: "container-tasks"},
    container.ExecuteContainerWorkflow, input)
```

See [examples/container/](../examples/container/) for complete working examples.
