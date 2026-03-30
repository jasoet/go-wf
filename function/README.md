# Function Package

Temporal workflow activities for dispatching arbitrary Go functions. Uses a registry to map function names to implementations at runtime, executing them as Temporal activities that compose with the generic orchestration patterns in the `workflow` package.

## Key Features

- **Function Registry** — register named Go functions for Temporal dispatch
- **Type-safe payloads** — implements `workflow.TaskInput` and `workflow.TaskOutput`
- **Composable** — use with Pipeline, Parallel, Loop, and DAG workflows
- **Builder API** — fluent construction of function workflow inputs
- **Pre-built patterns** — common function orchestration patterns included

## Documentation

- [Function Workflows Guide](../docs/function-workflows.md) — comprehensive usage guide with examples
- [Architecture](../docs/architecture.md) — how this package fits in the overall system
- [Workflow Patterns](../docs/workflow-patterns.md) — orchestration patterns
- [Getting Started](../docs/getting-started.md) — quick start guide

## Quick Example

```go
// Register a handler
registry := function.NewRegistry()
registry.Register("greet", func(ctx context.Context, input function.FunctionInput) (*function.FunctionOutput, error) {
    name := input.Args["name"]
    return &function.FunctionOutput{
        Result: map[string]string{"greeting": fmt.Sprintf("Hello, %s!", name)},
    }, nil
})

// Execute via Temporal
input := function.FunctionExecutionInput{
    Name: "greet",
    Args: map[string]string{"name": "World"},
}
```
