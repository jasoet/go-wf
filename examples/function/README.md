# Function Workflow Examples

This directory contains examples demonstrating the `go-wf/function` package for orchestrating Go function execution with Temporal.

## Prerequisites

1. **Temporal Server** running locally:
   ```bash
   # Using Temporal CLI (recommended)
   temporal server start-dev

   # Or using Podman Compose
   git clone https://github.com/temporalio/docker-compose.git
   cd docker-compose
   podman-compose up -d
   ```

   Temporal Web UI: http://localhost:8233

2. **Go 1.26+** installed:
   ```bash
   go version
   ```

## Running Examples

All examples use the `//go:build example` build tag. Run with:

```bash
cd examples/function
go run -tags example basic.go
go run -tags example pipeline.go
go run -tags example parallel.go
go run -tags example loop.go
go run -tags example builder.go
```

Each example is self-contained: it creates a function registry, registers handlers, starts a worker, executes the workflow, and prints results.

## How It Works

The function module follows a registry-dispatch pattern:

```
1. Register named handler functions in a Registry
2. Create an activity from the registry
3. Register workflows + activity with a Temporal worker
4. Execute workflows that dispatch to handlers by name
```

```go
// 1. Create registry and register handlers
registry := function.NewRegistry()
registry.Register("my-func", func(ctx context.Context, input function.FunctionInput) (*function.FunctionOutput, error) {
    // Your logic here
    return &function.FunctionOutput{
        Result: map[string]string{"key": "value"},
    }, nil
})

// 2. Create activity from registry
activityFn := activity.NewExecuteFunctionActivity(registry)

// 3. Register with Temporal worker
w := worker.New(client, "function-tasks", worker.Options{})
function.RegisterWorkflows(w)
function.RegisterActivity(w, activityFn)

// 4. Execute workflow
input := payload.FunctionExecutionInput{Name: "my-func", Args: map[string]string{"key": "val"}}
we, _ := client.ExecuteWorkflow(ctx, options, workflow.ExecuteFunctionWorkflow, input)
```

## Handler Signature

All handlers follow this signature:

```go
func(ctx context.Context, input function.FunctionInput) (*function.FunctionOutput, error)
```

**FunctionInput fields:**

| Field | Type | Description |
|-------|------|-------------|
| `Args` | `map[string]string` | Key-value arguments |
| `Data` | `[]byte` | Binary payload |
| `Env` | `map[string]string` | Environment variables |
| `WorkDir` | `string` | Working directory |

**FunctionOutput fields:**

| Field | Type | Description |
|-------|------|-------------|
| `Result` | `map[string]string` | Key-value results |
| `Data` | `[]byte` | Binary output data |

## Example Descriptions

### 1. Basic Function Execution (`basic.go`)

Single function execution with the registry pattern.

**Demonstrates:**
- Creating a function registry
- Registering a handler
- Worker setup with `RegisterWorkflows` + `RegisterActivity`
- Executing `ExecuteFunctionWorkflow`
- Reading `FunctionExecutionOutput` (Name, Success, Duration, Result)

**Use case:** Simple one-off function execution — validation, computation, API calls.

```bash
go run -tags example basic.go
```

---

### 2. Pipeline Workflow (`pipeline.go`)

Sequential function pipeline: validate -> transform -> notify.

**Demonstrates:**
- Registering multiple handlers
- `FunctionPipelineWorkflow` with `PipelineInput`
- `StopOnError: true` — pipeline halts on first failure
- Per-step result inspection (`PipelineOutput.Results`)
- Realistic user onboarding flow

**Use case:** Multi-step data processing where order matters — ETL, onboarding flows, sequential validation.

```bash
go run -tags example pipeline.go
```

---

### 3. Parallel Execution (`parallel.go`)

Concurrent function execution with failure handling.

**Demonstrates:**
- `ParallelFunctionsWorkflow` with `ParallelInput`
- `MaxConcurrency: 3` — limits concurrent executions
- `FailureStrategy: "continue"` — completes all tasks even if some fail
- Result aggregation across parallel tasks
- Simulated API call latency with `time.Sleep`

**Use case:** Independent data fetching, parallel validation, fan-out workloads.

```bash
go run -tags example parallel.go
```

---

### 4. Loop Workflows (`loop.go`)

Iterative function execution with items and parameter combinations.

**Demonstrates 5 patterns:**

| # | Pattern | Workflow | Key Config |
|---|---------|----------|------------|
| 1 | Parallel item loop | `LoopWorkflow` | `Parallel: true`, `MaxConcurrency: 2` |
| 2 | Sequential fail-fast | `LoopWorkflow` | `Parallel: false`, `FailureStrategy: "fail_fast"` |
| 3 | Parameterized matrix | `ParameterizedLoopWorkflow` | `Parameters: {env, region}` |
| 4 | `builder.ForEach` | `LoopWorkflow` | Fluent builder API |
| 5 | `builder.ForEachParam` | `ParameterizedLoopWorkflow` | Fluent builder API |

**Template substitution:** Use `{{item}}`, `{{index}}`, `{{.paramName}}` in Name, Args, Env, and WorkDir fields.

**Use case:** Batch processing, multi-region deployment, matrix builds, tenant sync.

```bash
go run -tags example loop.go
```

---

### 5. Builder API (`builder.go`)

Fluent builder API for composing workflows programmatically.

**Demonstrates 4 patterns:**

| # | Pattern | Builder Method | Build Method |
|---|---------|---------------|--------------|
| 1 | ETL pipeline | `AddInput()` | `BuildPipeline()` |
| 2 | Parallel pre-flight | `Parallel(true)`, `MaxConcurrency()`, `FailFast()` | `BuildParallel()` |
| 3 | Reusable components | `Add(FunctionSource)` | `BuildPipeline()` |
| 4 | Dynamic inputs | `Add(WorkflowSourceFunc)` | `BuildParallel()` |

**Key builder APIs:**
- `NewWorkflowBuilder(name)` — create builder
- `AddInput(FunctionExecutionInput)` — add input directly
- `Add(WorkflowSource)` — add via source interface
- `StopOnError(bool)` — pipeline error handling
- `Parallel(bool)` / `MaxConcurrency(int)` / `FailFast(bool)` — parallel config
- `BuildPipeline()` / `BuildParallel()` / `BuildSingle()` / `Build()` — build output
- `NewFunctionSource(input)` — wrap input as reusable source
- `WorkflowSourceFunc(fn)` — dynamic input generation at build time
- `ForEach(items, template)` — loop builder shortcut
- `ForEachParam(params, template)` — parameterized loop builder shortcut

**Use case:** Programmatic workflow construction, reusable components, dynamic input generation.

```bash
go run -tags example builder.go
```

## Comparison with Docker Examples

The function module has fewer examples because it has a focused scope:

| Feature | Docker | Function |
|---------|:------:|:--------:|
| Single execution | basic.go | basic.go |
| Pipeline | pipeline.go | pipeline.go |
| Parallel | parallel.go | parallel.go |
| Loop / parameterized loop | loop.go | loop.go |
| Builder API | builder.go, builder-advanced.go | builder.go |
| DAG workflow | dag.go | N/A |
| Script templates (bash/python/node) | builder.go | N/A |
| HTTP templates | builder.go | N/A |
| Artifacts (local/S3) | artifacts.go | N/A |
| Data passing between steps | data-passing.go | N/A |
| Operations/lifecycle API | operations.go | N/A |
| Pre-built patterns | patterns-demo.go | N/A |
| Advanced features | advanced.go | N/A |

Docker examples cover container-specific features (templates, artifacts, DAG, lifecycle). Function examples cover the full function API surface — every public type and method is demonstrated.

## Task Queue

All examples use the **`function-tasks`** task queue:

```go
w := worker.New(c, "function-tasks", worker.Options{})
```

## Error Handling

The function activity has two error paths:

| Error Type | Behavior | Example |
|------------|----------|---------|
| Validation/registry error | Returns error — Temporal retries | Missing handler name, unknown function |
| Handler execution error | Captured in output (`Success=false`) — no retry | Business logic failure |

This means handler failures are treated as business results, not infrastructure errors.

## Troubleshooting

### Worker Not Connecting

```bash
# Check Temporal server
temporal server start-dev

# Verify connection
temporal workflow list
```

### Build Tag Error

Always use `-tags example`:
```bash
go run -tags example basic.go
# NOT: go run basic.go
```

### Workflow Stuck in Running

Ensure task queue names match between worker and workflow execution:
```go
// Worker
w := worker.New(c, "function-tasks", worker.Options{})

// Workflow
client.StartWorkflowOptions{TaskQueue: "function-tasks"}
```

### Handler Not Found

Ensure the handler name in `FunctionExecutionInput.Name` matches what was registered:
```go
registry.Register("my-func", handler)  // Registration
input := payload.FunctionExecutionInput{Name: "my-func"}  // Must match
```

## Next Steps

- Review the [function package source](../../function/) for implementation details
- See the [docker examples](../docker/) for container orchestration patterns
- Build custom handlers for your domain logic
- Compose pipelines and parallel workflows using the builder API
