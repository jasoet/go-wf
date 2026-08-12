# Function Workflows

Function workflows dispatch named Go functions as Temporal activities. Unlike
[container workflows](container-workflows.md) (which spawn containers), function
workflows call registered Go handlers directly inside the worker process. This
makes them faster and simpler but requires that handler code is compiled into the
worker binary.

For generic pattern concepts (pipeline, parallel, loop, DAG) see
[Workflow Patterns](workflow-patterns.md).

## Handler Signature

Every handler must match the `function.Handler` type:

```go
type Handler func(ctx context.Context, input FunctionInput) (*FunctionOutput, error)
```

`FunctionInput` carries key-value `Args`, an opaque `Data` byte slice, optional
`Env` variables, and a `WorkDir` path. `FunctionOutput` returns a `Result` map
and optional `Data` bytes.

```go
type FunctionInput struct {
    Args    map[string]string
    Data    []byte
    Env     map[string]string
    WorkDir string
}

type FunctionOutput struct {
    Result map[string]string
    Data   []byte
}
```

## Registry

`Registry` is a thread-safe map from function name to `Handler`. The activity
looks up handlers by name at runtime.

```go
registry := function.NewRegistry()

err := registry.Register("greet", func(ctx context.Context, in function.FunctionInput) (*function.FunctionOutput, error) {
    name := in.Args["name"]
    return &function.FunctionOutput{
        Result: map[string]string{"message": "Hello, " + name},
    }, nil
})

handler, err := registry.Get("greet")   // retrieve by name
exists := registry.Has("greet")          // check existence
```

`Register` returns an error if a handler with the same name already exists.
`Get` returns an error if the name is not found.

## Activity

The `function/activity` package provides `NewExecuteFunctionActivity`, which
creates a Temporal activity function backed by a registry:

```go
import "github.com/jasoet/go-wf/v2/function/activity"

activityFn := activity.NewExecuteFunctionActivity(registry)
```

The activity validates the input, looks up the handler by name, calls it with
panic recovery, and returns a `FunctionExecutionOutput`. Validation and lookup
errors cause Temporal retries; handler errors are captured in the output
(`Success=false`) without returning an error, so Temporal does **not** retry
business-logic failures.

## Payload Types

`function/payload` defines the wire types used by workflows and activities.
The root `function` package re-exports them as type aliases for convenience.

| Alias (in `function`) | Original (in `function/payload`) |
|---|---|
| `FunctionExecutionInput` | `payload.FunctionExecutionInput` |
| `FunctionExecutionOutput` | `payload.FunctionExecutionOutput` |
| `PipelineInput` | `payload.PipelineInput` |
| `PipelineOutput` | `payload.PipelineOutput` |
| `ParallelInput` | `payload.ParallelInput` |
| `ParallelOutput` | `payload.ParallelOutput` |
| `LoopInput` | `payload.LoopInput` |
| `ParameterizedLoopInput` | `payload.ParameterizedLoopInput` |
| `LoopOutput` | `payload.LoopOutput` |

DAG-specific aliases:

| Alias | Original |
|---|---|
| `FunctionDAGNode` | `payload.FunctionDAGNode` |
| `DAGWorkflowInput` | `payload.DAGWorkflowInput` |
| `OutputMapping` | `payload.OutputMapping` |
| `FunctionInputMapping` | `payload.FunctionInputMapping` |
| `DataMapping` | `payload.DataMapping` |
| `FunctionNodeResult` | `payload.FunctionNodeResult` |
| `FunctionDAGWorkflowOutput` | `payload.FunctionDAGWorkflowOutput` |

`FunctionExecutionInput` includes `Name`, `Args`, `Data`, `Env`, `WorkDir`,
`Timeout`, and `Labels`. Names must match `[a-zA-Z][a-zA-Z0-9_-]*` (template
placeholders like `{{item}}` are allowed and validated at execution time).

## Builder API

The `function/builder` package provides a fluent API that produces a `*job.Definition`
ready for registration and execution. Using the builder is preferred over constructing
workflow inputs manually.

### WorkflowBuilder

`NewWorkflowBuilder[I, O]()` is generic. `NewFunctionBuilder()` is a convenience
alias pre-specialized for `*payload.FunctionExecutionInput` / `payload.FunctionExecutionOutput`.

```go
import (
    "github.com/jasoet/go-wf/v2/function/builder"
    "github.com/jasoet/go-wf/v2/function/activity"
    "github.com/jasoet/pkg/v2/temporal/job"
)

activityFn := activity.NewExecuteFunctionActivity(registry)

def, err := builder.NewFunctionBuilder().
    Name("my-pipeline").
    Activity(activityFn).
    Pipeline().           // select execution mode
    Add(&payload.FunctionExecutionInput{Name: "step-a", Args: map[string]string{"key": "val"}}).
    Add(&payload.FunctionExecutionInput{Name: "step-b"}).
    StopOnError(true).
    Build()               // returns (*job.Definition, error)
if err != nil {
    log.Fatal(err)
}
```

**Identity setters (required before `Build`):**

| Method | Description |
|---|---|
| `.Name(string)` | Job name — also used as workflow ID prefix |
| `.Activity(fn)` | The activity function returned by `activity.NewExecuteFunctionActivity` |
| `.TaskQueue(string)` | Override task queue (default: `"function-<name>"`) |

**Mode setters (pick exactly one before calling `Build`):**

| Method | Registered Temporal workflow |
|---|---|
| `.Pipeline()` | `FunctionPipelineWorkflow` — sequential, stop-on-error |
| `.Parallel()` | `ParallelFunctionsWorkflow` — concurrent |
| `.Single()` | `ExecuteFunctionWorkflow` — single function |

**Configuration:** `StopOnError(bool)`, `FailFast(bool)`, `MaxConcurrency(int)`.

**`.Build()` returns `(*job.Definition, error)`.** Consume the Definition like this:

```go
w := worker.New(c, def.TaskQueue, worker.Options{})
def.Register(w)               // calls fn.RegisterAll internally; idempotent

run, err := def.Execute(ctx, c, def.NewInput())
```

`def.Register(w)` calls `fn.RegisterAll(w, activityFn)` via idempotent helpers.
`fn.RegisterAll` is safe to call multiple times — repeated registrations for an
already-registered (worker, type) pair are silently ignored.

### LoopBuilder

`LoopBuilder` produces a `*job.Definition` for item-based or parameterized loops.
`Build()` returns `(*job.Definition, error)`.

```go
// Simple item loop — {{item}} in template args is replaced per iteration
def, err := builder.ForEach(
    []string{"file1.csv", "file2.csv"},
    payload.FunctionExecutionInput{
        Name: "process-file",
        Args: map[string]string{"file": "{{item}}"},
    },
).Name("process-files").Activity(activityFn).Parallel(true).Build()

// Parameterized loop — {{.key}} placeholders are replaced with cross-product values
def, err := builder.ForEachParam(
    map[string][]string{
        "environment": {"dev", "staging"},
        "region":      {"us-west", "eu-central"},
    },
    payload.FunctionExecutionInput{
        Name: "deploy-service",
        Args: map[string]string{
            "environment": "{{.environment}}",
            "region":      "{{.region}}",
        },
    },
).Name("multi-region-deploy").Activity(activityFn).Parallel(true).FailFast(true).Build()
```

Convenience constructors: `NewFunctionLoopBuilder(items)` and
`NewFunctionParameterizedLoopBuilder(params)`.

`BuildLoop()` and `BuildParameterizedLoop()` return raw input structs (without a
Definition), for callers that manage Temporal options manually.

### DAG Builder

The DAG builder constructs a `DAGWorkflowInput` with dependency edges and data
mapping between nodes.

```go
dagInput, err := builder.NewDAGBuilder("ci-pipeline").
    AddNodeWithInput("compile", payload.FunctionExecutionInput{
        Name: "compile",
    }).
    WithOutputMapping("compile", payload.OutputMapping{
        Name:      "artifact",
        ResultKey: "artifact",
    }).
    AddNodeWithInput("test", payload.FunctionExecutionInput{
        Name: "run-tests",
    }, "compile").                           // depends on compile
    AddNodeWithInput("publish", payload.FunctionExecutionInput{
        Name: "publish-artifact",
    }, "test").
    WithInputMapping("publish", payload.FunctionInputMapping{
        Name: "artifact_path",
        From: "compile.artifact",            // node.output format
    }).
    FailFast(true).
    MaxParallel(4).
    BuildDAG()
```

**Data mapping between nodes:**

- `OutputMapping` — captures a value from a node's `Result` map under a named
  output. Fields: `Name`, `ResultKey`, `Default`.
- `FunctionInputMapping` — maps a previous node's named output into the current
  node's `Args`. The `From` field uses `"node-name.output-name"` format. Fields:
  `Name`, `From`, `Default`, `Required`.
- `DataMapping` — passes the raw `Data` bytes from one node to another. Set via
  `WithDataMapping(nodeName, fromNode)`.

DAG validation checks for duplicate node names, missing dependency references,
and circular dependencies (DFS-based cycle detection).

## Pre-built Patterns

The `function/patterns` package provides ready-made workflow constructors.

### Pipeline Patterns

```go
import "github.com/jasoet/go-wf/v2/function/patterns"

// 3-step ETL pipeline
input, err := patterns.ETLPipeline("s3://bucket/data", "json", "postgres://db/table")

// Validate -> Transform -> Notify
input, err := patterns.ValidateTransformNotify("user@example.com", "report", "#alerts")

// Deploy to multiple environments sequentially
input, err := patterns.MultiEnvironmentDeploy("v1.2.3", []string{"staging", "production"})
```

### Parallel Patterns

```go
// Fan-out/fan-in across named functions
input, err := patterns.FanOutFanIn([]string{"task-1", "task-2", "task-3"})

// Health check across services with fail-fast
input, err := patterns.ParallelHealthCheck([]string{"api", "database", "cache"}, "production")
```

### Loop Patterns

```go
// Batch process files in parallel (continue on failure)
input, err := patterns.BatchProcess([]string{"a.csv", "b.csv"}, "process-file")

// Sequential database migrations (fail-fast)
input, err := patterns.SequentialMigration([]string{"001_create_users.sql", "002_add_index.sql"})

// Cross-product deploy across environments and regions
input, err := patterns.MultiRegionDeploy(
    []string{"dev", "staging", "prod"},
    []string{"us-west", "us-east"},
    "v1.2.3",
)

// Hyperparameter sweep with concurrency limit
input, err := patterns.ParameterSweep(
    map[string][]string{
        "learning_rate": {"0.001", "0.01"},
        "batch_size":    {"32", "64"},
    },
    "train-model", 5,
)
```

### DAG Patterns

```go
// ETL with parallel validate + extract, then transform, then load
input, err := patterns.ETLWithValidation("database", "parquet", "warehouse")

// CI pipeline: compile -> (test + lint) -> publish with output/input mapping
input, err := patterns.CIPipeline()
```

## Worker Setup

### Builder-based (preferred)

When you use `function.WorkflowBuilder` or `function.LoopBuilder`, the resulting
`*job.Definition` handles registration automatically:

```go
import (
    fn "github.com/jasoet/go-wf/v2/function"
    "github.com/jasoet/go-wf/v2/function/activity"
    "github.com/jasoet/go-wf/v2/function/builder"
    "github.com/jasoet/pkg/v2/temporal"
    "go.temporal.io/sdk/worker"
)

registry := fn.NewRegistry()
// ... register handlers ...
activityFn := activity.NewExecuteFunctionActivity(registry)

def, err := builder.NewFunctionBuilder().
    Name("greet-pipeline").
    Activity(activityFn).
    Pipeline().
    Add(&fn.FunctionExecutionInput{Name: "greet"}).
    Build()
if err != nil {
    log.Fatal(err)
}

c, err := temporal.NewClient(temporal.DefaultConfig())
if err != nil {
    log.Fatal(err)
}
defer c.Close()

w := worker.New(c, def.TaskQueue, worker.Options{})
def.Register(w)   // calls fn.RegisterAll internally; idempotent
w.Run(worker.InterruptCh())
```

`def.Register(w)` calls `fn.RegisterAll(w, activityFn)` via idempotent helpers.
`fn.RegisterAll` is safe to call multiple times — repeated (worker, type) pairs
are silently deduped.

### Manual registration

For lower-level use or when registering DAG workflows separately:

```go
import (
    fn "github.com/jasoet/go-wf/v2/function"
    "github.com/jasoet/go-wf/v2/function/activity"
)

registry := fn.NewRegistry()
// ... register handlers ...

activityFn := activity.NewExecuteFunctionActivity(registry)

// Register everything at once:
fn.RegisterAll(worker, activityFn)

// Or register separately:
fn.RegisterWorkflows(worker)       // registers all function workflow types
fn.RegisterActivity(worker, activityFn)
```

`WorkflowRegistrar` is the interface that `worker` must satisfy (it matches
Temporal's `worker.Worker`):

```go
type WorkflowRegistrar interface {
    RegisterWorkflow(w interface{})
    RegisterActivityWithOptions(a interface{}, options activity.RegisterOptions)
}
```

`RegisterWorkflows` registers these workflow functions:
- `ExecuteFunctionWorkflow` — single function execution
- `FunctionPipelineWorkflow` — sequential pipeline
- `ParallelFunctionsWorkflow` — parallel execution
- `LoopWorkflow` — item-based loop
- `ParameterizedLoopWorkflow` — parameter cross-product loop
- `InstrumentedDAGWorkflow` — DAG execution with optional OTel tracing

### OpenTelemetry Instrumentation

Call `fn.SetActivityInstrumenter(wrapper)` during initialization to wrap the
activity with OpenTelemetry spans. This must be called once before
`RegisterActivity`; subsequent calls are ignored. See
[Observability](observability.md) for details.

See [Job Definition](job-definition.md) for the full `*job.Definition` API.
