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
// 1. Build a registry and register handlers
registry := function.NewRegistry()
registry.Register("greet", func(ctx context.Context, input function.FunctionInput) (*function.FunctionOutput, error) {
    name := input.Args["name"]
    return &function.FunctionOutput{
        Result: map[string]string{"greeting": fmt.Sprintf("Hello, %s!", name)},
    }, nil
})

// 2. Build a *job.Definition via the fluent builder
activityFn := activity.NewExecuteFunctionActivity(registry)

def, err := builder.NewFunctionBuilder().
    Name("greet-job").
    Activity(activityFn).
    Single().
    Add(&payload.FunctionExecutionInput{
        Name: "greet",
        Args: map[string]string{"name": "World"},
    }).
    Build()

// 3. Register and execute
w := worker.New(c, def.TaskQueue, worker.Options{})
def.Register(w)
run, err := def.Execute(ctx, c, def.NewInput())
```

See [docs/job-definition.md](../docs/job-definition.md) for the full `*job.Definition` API (lifecycle, scheduling, registry).
