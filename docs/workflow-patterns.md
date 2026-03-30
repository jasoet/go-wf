# Workflow Patterns

go-wf provides four orchestration patterns for composing Temporal activities into
workflows. Each pattern is defined as a generic function (or set of types) in the
`workflow/` package, parameterized over `TaskInput` and `TaskOutput` interfaces so
that any concrete implementation (container, function, datasync) can reuse the
same orchestration logic.

This guide explains every pattern, shows when to reach for each one, and
demonstrates how they compose.

## Core Interfaces

All patterns operate on two interface constraints:

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

Any struct that satisfies these interfaces can be used with the generic workflow
functions. The container, function, and datasync packages each provide their own
concrete implementations.

## Pattern Overview

| Pattern | Execution | Best For | Complexity |
|---------|-----------|----------|------------|
| Pipeline | Sequential, one task at a time | Ordered multi-step processes where step N depends on step N-1 | Low |
| Parallel | All tasks launched concurrently | Independent tasks that can safely run at the same time | Low |
| Loop | Repeated execution of a template | Applying the same operation across a list of items or parameter combinations | Medium |
| DAG | Dependency-driven graph execution | Complex workflows where some steps depend on others and some can run in parallel | High |

## Pipeline

A pipeline executes tasks sequentially, one after another. Use it when each step
logically follows the previous one, such as build-then-test or
extract-then-transform-then-load.

### Types

```go
type PipelineInput[I TaskInput, O TaskOutput] struct {
    Tasks       []I  `json:"tasks" validate:"required,min=1"`
    StopOnError bool `json:"stop_on_error"`
    Cleanup     bool `json:"cleanup"`
}

type PipelineOutput[O TaskOutput] struct {
    Results       []O           `json:"results"`
    TotalSuccess  int           `json:"total_success"`
    TotalFailed   int           `json:"total_failed"`
    TotalDuration time.Duration `json:"total_duration"`
}
```

### Behavior

- Tasks execute in array order. Each task runs as a Temporal activity.
- When `StopOnError` is `true`, the pipeline halts on the first failure and
  returns the partial results collected so far.
- When `StopOnError` is `false`, all tasks execute regardless of failures. The
  output counts successes and failures separately.

### Function Signature

```go
func PipelineWorkflow[I TaskInput, O TaskOutput](
    ctx wf.Context,
    input PipelineInput[I, O],
) (*PipelineOutput[O], error)
```

### Example

```go
input := workflow.PipelineInput[MyInput, MyOutput]{
    Tasks: []MyInput{
        {Name: "build", Image: "golang:1.22"},
        {Name: "test",  Image: "golang:1.22"},
        {Name: "push",  Image: "docker:latest"},
    },
    StopOnError: true,
}

output, err := workflow.PipelineWorkflow[MyInput, MyOutput](ctx, input)
fmt.Printf("Success: %d, Failed: %d\n", output.TotalSuccess, output.TotalFailed)
```

## Parallel

Parallel launches all tasks concurrently, then collects results. Use it when
tasks are independent and you want to minimize wall-clock time.

### Types

```go
type ParallelInput[I TaskInput, O TaskOutput] struct {
    Tasks           []I    `json:"tasks" validate:"required,min=1"`
    MaxConcurrency  int    `json:"max_concurrency,omitempty"`
    FailureStrategy string `json:"failure_strategy" validate:"oneof='' continue fail_fast"`
}

type ParallelOutput[O TaskOutput] struct {
    Results       []O           `json:"results"`
    TotalSuccess  int           `json:"total_success"`
    TotalFailed   int           `json:"total_failed"`
    TotalDuration time.Duration `json:"total_duration"`
}
```

### Failure Strategies

Two constants defined in `workflow/helpers.go` control how failures are handled:

| Strategy | Constant | Behavior |
|----------|----------|----------|
| Fail Fast | `"fail_fast"` | Stop collecting results on the first failure and return immediately. |
| Continue | `"continue"` | Collect all results even if some tasks fail. The output reports the failure count. |

An empty string (the default) behaves like Continue.

> **Note:** `MaxConcurrency` is declared but not currently enforced at the
> workflow level. Use Temporal's `MaxConcurrentActivityExecutionSize` worker
> option for concurrency limiting.

### Function Signature

```go
func ParallelWorkflow[I TaskInput, O TaskOutput](
    ctx wf.Context,
    input ParallelInput[I, O],
) (*ParallelOutput[O], error)
```

### Example

```go
input := workflow.ParallelInput[MyInput, MyOutput]{
    Tasks: []MyInput{
        {Name: "lint"},
        {Name: "unit-tests"},
        {Name: "integration-tests"},
    },
    FailureStrategy: workflow.FailureStrategyFailFast,
}

output, err := workflow.ParallelWorkflow[MyInput, MyOutput](ctx, input)
```

## Loop

Loop executes a single task template repeatedly, once per item in a list. It
comes in two variants: simple loop and parameterized loop.

### Simple Loop

Iterates over a list of string items. Each iteration substitutes `{{item}}` and
`{{index}}` into the template using a `Substitutor` function you provide.

```go
type LoopInput[I TaskInput, O TaskOutput] struct {
    Items           []string `json:"items" validate:"required,min=1"`
    Template        I        `json:"template" validate:"required"`
    Parallel        bool     `json:"parallel"`
    MaxConcurrency  int      `json:"max_concurrency,omitempty"`
    FailureStrategy string   `json:"failure_strategy" validate:"oneof='' continue fail_fast"`
}
```

### Parameterized Loop

Iterates over the Cartesian product of multiple named parameter arrays. The
helper `GenerateParameterCombinations` builds every combination. Each parameter
value is available via `{{.paramName}}` or `{{paramName}}` in the template.

```go
type ParameterizedLoopInput[I TaskInput, O TaskOutput] struct {
    Parameters      map[string][]string `json:"parameters" validate:"required,min=1"`
    Template        I                   `json:"template" validate:"required"`
    Parallel        bool                `json:"parallel"`
    MaxConcurrency  int                 `json:"max_concurrency,omitempty"`
    FailureStrategy string              `json:"failure_strategy" validate:"oneof='' continue fail_fast"`
}
```

### Output

Both variants return the same output type:

```go
type LoopOutput[O TaskOutput] struct {
    Results       []O           `json:"results"`
    TotalSuccess  int           `json:"total_success"`
    TotalFailed   int           `json:"total_failed"`
    TotalDuration time.Duration `json:"total_duration"`
    ItemCount     int           `json:"item_count"`
}
```

### The Substitutor Interface

The `Substitutor` is a function type that controls how template variables are
replaced for each iteration:

```go
type Substitutor[I TaskInput] func(template I, item string, index int, params map[string]string) I
```

- `template` — the original task input to clone and modify.
- `item` — the current item string (simple loop) or empty string (parameterized loop).
- `index` — the zero-based iteration index.
- `params` — parameter key-value pairs (parameterized loop) or nil (simple loop).

The helper `SubstituteTemplate` can be used inside a Substitutor to replace
`{{item}}`, `{{index}}`, and `{{.key}}` placeholders in any string field.

### Function Signatures

```go
func LoopWorkflow[I TaskInput, O TaskOutput](
    ctx wf.Context,
    input LoopInput[I, O],
    substitutor Substitutor[I],
) (*LoopOutput[O], error)

func ParameterizedLoopWorkflow[I TaskInput, O TaskOutput](
    ctx wf.Context,
    input ParameterizedLoopInput[I, O],
    substitutor Substitutor[I],
) (*LoopOutput[O], error)
```

### Example: Simple Loop

```go
substitutor := func(tmpl MyInput, item string, index int, _ map[string]string) MyInput {
    copy := tmpl
    copy.Target = item
    return copy
}

input := workflow.LoopInput[MyInput, MyOutput]{
    Items:    []string{"us-east-1", "eu-west-1", "ap-south-1"},
    Template: MyInput{Action: "deploy"},
    Parallel: true,
    FailureStrategy: workflow.FailureStrategyContinue,
}

output, err := workflow.LoopWorkflow[MyInput, MyOutput](ctx, input, substitutor)
```

### Example: Parameterized Loop

```go
input := workflow.ParameterizedLoopInput[MyInput, MyOutput]{
    Parameters: map[string][]string{
        "os":   {"linux", "darwin"},
        "arch": {"amd64", "arm64"},
    },
    Template: MyInput{BuildCmd: "GOOS={{.os}} GOARCH={{.arch}} go build"},
    Parallel: true,
}

// Generates 4 combinations: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64
output, err := workflow.ParameterizedLoopWorkflow[MyInput, MyOutput](ctx, input, substitutor)
```

## DAG (Directed Acyclic Graph)

The DAG pattern executes tasks based on an explicit dependency graph. Nodes
without dependencies (or whose dependencies are already satisfied) can run in
parallel, while dependent nodes wait for their prerequisites.

### Types

```go
type DAGNode[I TaskInput, O TaskOutput] struct {
    Name         string   `json:"name" validate:"required"`
    Input        I        `json:"input" validate:"required"`
    Dependencies []string `json:"dependencies,omitempty"`
}

type DAGInput[I TaskInput, O TaskOutput] struct {
    Nodes       []DAGNode[I, O] `json:"nodes" validate:"required,min=1"`
    FailFast    bool            `json:"fail_fast"`
    MaxParallel int             `json:"max_parallel,omitempty"`
}

type DAGOutput[O TaskOutput] struct {
    Results       map[string]*O   `json:"results"`
    NodeResults   []NodeResult[O] `json:"node_results"`
    TotalSuccess  int             `json:"total_success"`
    TotalFailed   int             `json:"total_failed"`
    TotalDuration time.Duration   `json:"total_duration"`
}
```

### Validation

`DAGInput.Validate()` performs three checks before execution:

1. **No duplicate node names** — every node must have a unique name.
2. **All dependencies exist** — every dependency string must reference a node in
   the graph.
3. **No cycles** — DFS-based cycle detection rejects circular dependencies.

### Concrete DAG Implementations

The generic `workflow/dag.go` defines the types and validation. The actual DAG
execution logic lives in the concrete packages:

- `container/workflow/dag.go` — DAGWorkflow for container tasks, with artifact
  passing between nodes and output extraction via JSONPath/regex.
- `function/workflow/dag.go` — DAGWorkflow for function tasks, with input
  mappings, data mappings, and artifact store integration.

Both implementations share the same execution strategy: recursive DFS that
executes dependencies first, skips already-executed nodes, and respects the
`FailFast` flag.

### Features of Concrete DAG Workflows

- **Output extraction** — nodes can declare outputs extracted from task results
  (via JSONPath or result keys). Downstream nodes reference these outputs through
  input mappings.
- **Artifact passing** — nodes can declare input/output artifacts. The workflow
  engine uploads artifacts after a node completes and downloads them before a
  dependent node starts.
- **Conditional execution** — when `FailFast` is true, a failed dependency
  prevents all downstream nodes from executing.

### Example

```go
input := payload.DAGWorkflowInput{
    Nodes: []payload.DAGNode{
        {Name: "checkout", Function: checkoutInput},
        {Name: "build",    Function: buildInput,    Dependencies: []string{"checkout"}},
        {Name: "lint",     Function: lintInput,     Dependencies: []string{"checkout"}},
        {Name: "test",     Function: testInput,     Dependencies: []string{"build"}},
        {Name: "deploy",   Function: deployInput,   Dependencies: []string{"test", "lint"}},
    },
    FailFast: true,
}
// checkout runs first, then build and lint run in parallel,
// then test waits for build, finally deploy waits for both test and lint.
```

## Pattern Composition

Patterns can be composed by nesting them. Since each workflow function is just a
Temporal workflow, a DAG node can internally run a pipeline, and a pipeline step
can trigger a parallel workflow via child workflows.

Common compositions:

| Outer Pattern | Inner Pattern | Example |
|---------------|---------------|---------|
| DAG | Pipeline | A DAG node runs a multi-step build pipeline |
| DAG | Parallel | A DAG node fans out integration tests across environments |
| Pipeline | Loop | A pipeline step deploys to every region in a loop |
| Pipeline | Parallel | A pipeline step runs independent validations concurrently |

To compose, register the inner pattern as a separate Temporal workflow and invoke
it as a child workflow from the outer pattern's activity or workflow logic.

## Choosing a Pattern

Use this decision guide:

1. **Are all tasks independent with no ordering requirement?** Use **Parallel**.
2. **Must tasks run in a fixed order, one after another?** Use **Pipeline**.
3. **Is it the same task repeated for different inputs?** Use **Loop** (simple or
   parameterized).
4. **Do tasks have complex dependency relationships (some parallel, some
   sequential)?** Use **DAG**.
5. **Not sure?** Start with Pipeline. Refactor to DAG when dependencies become
   non-linear.

## Instrumented Variants

Every pattern has an instrumented wrapper that adds structured logging at workflow
start and completion boundaries. These wrappers delegate to the base workflow
functions and add timing and result-count log entries.

| Base Function | Instrumented Wrapper |
|---------------|---------------------|
| `PipelineWorkflow` | `InstrumentedPipelineWorkflow` |
| `ParallelWorkflow` | `InstrumentedParallelWorkflow` |
| `LoopWorkflow` | `InstrumentedLoopWorkflow` |
| `ParameterizedLoopWorkflow` | `InstrumentedParameterizedLoopWorkflow` |

The instrumented variants are defined in `workflow/otel.go`. Concrete packages
(e.g., `function/workflow/dag_otel.go`) provide their own instrumented DAG
workflows.

For details on observability, tracing, and structured logging, see
[docs/observability.md](observability.md).
