# Architecture Guide

This document describes the architecture of **go-wf**, a Go library for workflow orchestration
built on [Temporal](https://temporal.io). It is intended for Go developers integrating with the
library and for AI agents navigating the codebase.

## High-Level Overview

go-wf is organized in four layers. Each layer depends only on layers below it.

```
┌─────────────────────────────────────────────────────────┐
│  Layer 4 — Workers & Operations                         │
│  Worker registration (RegisterAll), workflow submit,    │
│  cancel, watch, schedule                                │
├─────────────────────────────────────────────────────────┤
│  Layer 3 — Builder / Pattern APIs                       │
│  Fluent builders, pre-built patterns (CI/CD, ETL, etc.) │
│  container/builder  function/builder  datasync/builder  │
│  container/patterns function/patterns                   │
├─────────────────────────────────────────────────────────┤
│  Layer 2 — Concrete Implementations                     │
│  container/   function/   datasync/                     │
│  Payloads, activities, workflow wrappers                 │
├─────────────────────────────────────────────────────────┤
│  Layer 1 — Generic Workflow Core (workflow/)             │
│  TaskInput/TaskOutput constraints, orchestration logic,  │
│  Store, Artifacts, Errors, OTel wrappers                │
└─────────────────────────────────────────────────────────┘
```

## Package Relationship Map

```
workflow/                          ← Generic core (no concrete types)
├── store/                         ← RawStore, Store[T], Codec[T]
├── artifacts/                     ← Legacy artifact store (DAG)
├── errors/                        ← Shared error types
└── testutil/                      ← Temporal testcontainer helpers

container/                         ← Concrete: Docker/Podman execution
├── activity/                      ← StartContainerActivity + OTel
├── payload/                       ← TaskInput/TaskOutput impl structs
├── workflow/                      ← Workflow funcs calling workflow.*
├── builder/                       ← Fluent API
├── patterns/                      ← Pre-built compositions
└── template/                      ← Container, script, HTTP templates

function/                          ← Concrete: Go function dispatch
├── activity/                      ← ExecuteFunctionActivity + OTel
├── payload/                       ← TaskInput/TaskOutput impl structs
├── workflow/                      ← Workflow funcs calling workflow.*
├── builder/                       ← Fluent API (incl. DAG)
└── patterns/                      ← Pre-built compositions

datasync/                          ← Concrete: Source→Mapper→Sink
├── activity/                      ← SyncData activity + OTel
├── payload/                       ← SyncExecutionInput/Output
├── workflow/                      ← Sync workflow + scheduling
└── builder/                       ← Fluent Job builder
```

**Dependency flow (imports go downward only):**

```
container/builder ──→ container/workflow ──→ workflow/
container/workflow ──→ container/activity ──→ container/payload
function/builder  ──→ function/workflow  ──→ workflow/
function/workflow  ──→ function/activity  ──→ function/payload
datasync/workflow  ──→ datasync/activity  ──→ datasync/ (core interfaces)
All payloads       ──→ workflow/ (satisfy TaskInput/TaskOutput)
```

## Data Flow: Workflow Execution

This is the path a single workflow execution takes, using the function module as an example.

```
1. Client submits workflow
   │
   ▼
2. Temporal Server schedules workflow on a task queue
   │
   ▼
3. Worker picks up workflow
   │  (registered via function.RegisterAll)
   │
   ▼
4. Workflow function runs (e.g., FunctionPipelineWorkflow)
   │  Delegates to workflow.PipelineWorkflow[I, O]()
   │
   ▼
5. Generic core iterates tasks, for each:
   │  calls wf.ExecuteActivity(ctx, task.ActivityName(), task)
   │           ▲                      ▲
   │           │                      │
   │     Temporal SDK          String-based dispatch
   │                          (returns "ExecuteFunctionActivity")
   ▼
6. Temporal schedules activity on the same task queue
   │
   ▼
7. Worker picks up activity
   │  (registered as "ExecuteFunctionActivity")
   │
   ▼
8. Activity function executes
   │  function/activity: looks up handler in Registry by name
   │  container/activity: calls Docker/Podman via pkg/v2
   │
   ▼
9. Result (TaskOutput) returned to workflow
   │  Workflow aggregates results into PipelineOutput/ParallelOutput/etc.
   │
   ▼
10. Workflow completes, result stored in Temporal
```

## Key Design Decisions

### String-Based Activity Dispatch

Temporal requires activity functions to be registered by name on workers. Go generics cannot be
used as direct function references across the Temporal serialization boundary. go-wf solves this
with the `ActivityName()` method on `TaskInput`:

```go
// In the generic core — no knowledge of concrete types
err := wf.ExecuteActivity(ctx, task.ActivityName(), task).Get(ctx, &result)
```

Each concrete payload returns a fixed string:
- Container payloads return `"StartContainerActivity"`
- Function payloads return `"ExecuteFunctionActivity"`
- DataSync payloads return `"SyncDataActivity"`

This allows the generic orchestration code in `workflow/` to dispatch activities without importing
any concrete implementation package.

### Dual Type Parameters (I and O)

Orchestration types carry both an input type `I` and an output type `O`:

```go
type PipelineInput[I TaskInput, O TaskOutput] struct {
    Tasks       []I  `json:"tasks"`
    StopOnError bool `json:"stop_on_error"`
}

type PipelineOutput[O TaskOutput] struct {
    Results       []O           `json:"results"`
    TotalSuccess  int           `json:"total_success"`
    TotalFailed   int           `json:"total_failed"`
    TotalDuration time.Duration `json:"total_duration"`
}
```

The `O` parameter on `PipelineInput` is needed so the generic workflow function can deserialize
activity results into the correct concrete output type. Without it, the workflow would need
runtime type assertions.

### Generic Type Constraints (not concrete types)

`TaskInput` and `TaskOutput` are **interface constraints**, not concrete interfaces. They define
what methods a type must have to be used as a workflow task:

```go
type TaskInput interface {
    Validate() error
    ActivityName() string
}

type TaskOutput interface {
    IsSuccess() bool
    GetError() string
}
```

Any struct satisfying these constraints can be plugged into the orchestration functions. This is
how `container/`, `function/`, and `datasync/` all share the same pipeline/parallel/loop/DAG
logic without the core knowing about any of them.

### Function Registry Pattern

The function module uses a registry to map string names to Go handler functions:

```go
type Handler func(ctx context.Context, input FunctionInput) (*FunctionOutput, error)

type Registry struct {
    handlers map[string]Handler
}
```

At startup, the application registers handlers:
```go
registry := function.NewRegistry()
registry.Register("my-transform", myTransformHandler)
```

The single `ExecuteFunctionActivity` looks up the handler by the name carried in the payload,
calls it, and wraps the result. This keeps Temporal's activity surface area minimal (one
registered activity) while supporting an unlimited number of named functions.

## Type System

### TaskInput / TaskOutput Constraints

These are the two constraints that every concrete implementation must satisfy:

| Constraint   | Methods                              | Purpose                            |
|-------------|--------------------------------------|------------------------------------|
| `TaskInput`  | `Validate() error`                  | Pre-flight validation              |
|              | `ActivityName() string`             | Temporal activity dispatch key     |
| `TaskOutput` | `IsSuccess() bool`                  | Check execution outcome            |
|              | `GetError() string`                 | Human-readable error description   |

### Orchestration Input/Output Types

All orchestration types are generic over `[I TaskInput, O TaskOutput]`:

| Type                          | Execution Model                         |
|-------------------------------|-----------------------------------------|
| `PipelineInput[I, O]`        | Sequential, with optional stop-on-error |
| `ParallelInput[I, O]`        | Fan-out all tasks simultaneously        |
| `LoopInput[I, O]`            | Template + items, sequential or parallel|
| `ParameterizedLoopInput[I,O]`| Template + parameter matrix             |
| `DAGInput[I, O]`             | Dependency graph with topological exec  |

Each has a corresponding output type (`PipelineOutput[O]`, `ParallelOutput[O]`, `LoopOutput[O]`,
`DAGOutput[O]`) that aggregates per-task results with success/failure counts and total duration.

### Validation Chain

All input types implement `Validate()`. Composite types validate recursively:
- `PipelineInput.Validate()` validates the struct, then calls `Validate()` on each task.
- `DAGInput.Validate()` checks for duplicate node names, missing dependencies, cycles (DFS),
  then validates each node's input.

## Store Architecture

The `workflow/store/` package provides a layered storage abstraction:

```
┌──────────────────────────┐
│  Store[T]  (typed API)   │   Save(key, T) / Load(key) -> T
│  ┌────────────────────┐  │
│  │  Codec[T]          │  │   Encode(T) -> Reader / Decode(Reader) -> T
│  │  (JSON, Bytes)     │  │
│  └────────────────────┘  │
├──────────────────────────┤
│  RawStore  (byte API)    │   Upload(key, Reader) / Download(key) -> ReadCloser
│  (Local FS, S3)          │
└──────────────────────────┘
```

### Interfaces

**RawStore** — byte-level persistence with string keys:
```go
type RawStore interface {
    Upload(ctx, key string, data io.Reader) error
    Download(ctx, key string) (io.ReadCloser, error)
    Delete(ctx, key string) error
    Exists(ctx, key string) (bool, error)
    List(ctx, prefix string) ([]string, error)
    Close() error
}
```

**Codec[T]** — serialization strategy:
```go
type Codec[T any] interface {
    Encode(value T) (io.Reader, error)
    Decode(reader io.Reader) (T, error)
}
```

**Store[T]** — typed storage combining RawStore + Codec:
```go
type Store[T any] interface {
    Save(ctx, key string, value T) error
    Load(ctx, key string) (T, error)
    Delete(ctx, key string) error
    Exists(ctx, key string) (bool, error)
    List(ctx, prefix string) ([]string, error)
    Close() error
}
```

### Implementations

| Component        | Implementation                          |
|------------------|-----------------------------------------|
| RawStore         | `LocalStore` (filesystem), `S3Store`    |
| Codec            | `JSONCodec[T]`, `BytesCodec`            |
| Store[T]         | `TypedStore[T]` (composes RawStore + Codec) |
| Convenience      | `NewJSONStore[T](raw)`, `NewBytesStore(raw)` |

### Instrumentation

`InstrumentedStore` wraps any `RawStore` with OpenTelemetry spans and metrics
(`go_wf.artifact.operation.*`), activated via `otel.ContextWithConfig()`.

## How Temporal Is Used

### Activities

Each concrete module registers exactly **one** Temporal activity:

| Module     | Activity Name               | What It Does                        |
|------------|-----------------------------|-------------------------------------|
| container  | `StartContainerActivity`    | Runs a Docker/Podman container      |
| function   | `ExecuteFunctionActivity`   | Dispatches to a registry handler    |
| datasync   | `SyncDataActivity`          | Runs Source -> Mapper -> Sink cycle  |

Activities are registered by string name (not function reference) using
`RegisterActivityWithOptions` with explicit `Name` in `RegisterOptions`.

### Workflows

Workflow functions are thin wrappers that instantiate the generic core with concrete types:

```
container/workflow/pipeline.go:
  func ContainerPipelineWorkflow(ctx, input) =
      workflow.PipelineWorkflow[ContainerInput, ContainerOutput](ctx, input)

function/workflow/pipeline.go:
  func FunctionPipelineWorkflow(ctx, input) =
      workflow.PipelineWorkflow[FunctionInput, FunctionOutput](ctx, input)
```

Each module registers these wrapper functions as Temporal workflows.

### Workers

Worker registration follows a consistent pattern across modules:

**Container:**
```go
w := worker.New(client, "container-tasks", worker.Options{})
container.RegisterAll(w)  // registers all workflows + activities
```

**Function:**
```go
registry := function.NewRegistry()
// ... register handlers ...
activityFn := activity.NewExecuteFunctionActivity(registry)

w := worker.New(client, "function-tasks", worker.Options{})
function.RegisterAll(w, activityFn)  // needs the activity closure
```

The function module differs because its activity closes over a `Registry` instance, so the
caller must construct the activity function and pass it in.

### Task Queues

Each module typically uses its own task queue (e.g., `"container-tasks"`,
`"function-tasks"`). This allows independent scaling of workers per workload type.

### Error Handling Strategy

Activities distinguish between **infrastructure errors** and **business logic failures**:

- Infrastructure errors (validation failure, registry lookup miss) — return a Go error, causing
  Temporal to retry according to the retry policy.
- Business logic failures (handler returns error, container exits non-zero) — captured in the
  output struct (`Success=false`, `Error` set), returned with `nil` error. Temporal does **not**
  retry these. The workflow sees the failure via `TaskOutput.IsSuccess()`.

## DataSync Module

The datasync module follows a different pattern from container/function. Instead of
TaskInput/TaskOutput constraints driving individual tasks, it defines a three-stage pipeline:

```
Source[T] ──→ Mapper[T, U] ──→ Sink[U]
  Fetch()       Map()           Write()
```

**Core interfaces:**
```go
type Source[T any] interface {
    Name() string
    Fetch(ctx context.Context) ([]T, error)
}

type Mapper[T any, U any] interface {
    Map(ctx context.Context, records []T) ([]U, error)
}

type Sink[U any] interface {
    Name() string
    Write(ctx context.Context, records []U) (WriteResult, error)
}
```

A `Job[T, U]` bundles Source + Mapper + Sink with scheduling and retry configuration. The
`Runner[T, U]` executes the pipeline in-process (useful for testing), while the Temporal
activity wraps the same pipeline with full observability.

DataSync payloads (`SyncExecutionInput`/`SyncExecutionOutput`) also implement
`TaskInput`/`TaskOutput`, so sync jobs can be composed into Pipeline, Parallel, and DAG
orchestrations alongside container and function tasks.

## Observability

All three modules support OpenTelemetry instrumentation:

| Module     | Metric Prefix              | Activation                     |
|------------|----------------------------|--------------------------------|
| container  | `go_wf.container.task.*`   | `otel.ContextWithConfig(ctx)`  |
| function   | `go_wf.function.task.*`    | `otel.ContextWithConfig(ctx)`  |
| datasync   | `go_wf.datasync.*`         | `otel.ContextWithConfig(ctx)`  |
| store      | `go_wf.artifact.operation.*` | `InstrumentedStore` wrapper  |

Instrumentation is opt-in. When no OTel config is present in the context, all instrumentation
is zero-overhead no-ops. When enabled, activities emit traces, structured logs, and metrics with
shared trace context for full correlation.
