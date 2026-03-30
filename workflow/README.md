# Workflow Package

Generic orchestration primitives built on Temporal. Defines the `TaskInput` and `TaskOutput` interfaces that all workflow task types must satisfy, and provides reusable, fully-generic workflow patterns.

## Key Features

- **TaskInput / TaskOutput interfaces** — common contract for all task payloads
- **ExecuteTaskWorkflow** — run a single task as a Temporal activity
- **PipelineWorkflow** — sequential execution with optional stop-on-error
- **ParallelWorkflow** — concurrent execution with configurable concurrency limit
- **LoopWorkflow** — execute a task template for each item in a list
- **DAG types** — DAGInput, DAGNode, DAGOutput for graph execution with dependency edges
- **Fully generic** — parameterized on `[I TaskInput, O TaskOutput]` so concrete packages plug in their own types

## Documentation

- [Architecture](../docs/architecture.md) — how this package fits in the overall system
- [Workflow Patterns](../docs/workflow-patterns.md) — detailed guide to orchestration patterns
- [Getting Started](../docs/getting-started.md) — quick start guide

## Quick Example

```go
// Build a pipeline of tasks
input := workflow.PipelineInput[MyInput, MyOutput]{
    Tasks:       []MyInput{task1, task2, task3},
    StopOnError: true,
}

we, _ := c.ExecuteWorkflow(ctx, opts,
    workflow.PipelineWorkflow[MyInput, MyOutput], input)
```
