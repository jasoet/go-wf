# Function Activity Module Design

## Overview

A new top-level `function/` module, parallel to `docker/`, that lets users register named Go functions and orchestrate them using the existing generic workflow core (pipeline, parallel, loop).

## Architecture

```
go-wf/
├── workflow/          # Generic core (unchanged)
├── docker/            # Docker container activities (unchanged)
├── function/          # NEW — Go function activities
│   ├── activity/      # Function dispatcher activity
│   ├── builder/       # Fluent builder API
│   ├── payload/       # FunctionExecutionInput/Output
│   ├── workflow/      # Thin workflow wrappers
│   ├── registry.go    # Function registry (name → handler)
│   └── worker.go      # RegisterAll(w, registry)
```

## Execution Model

**Function registry pattern.** Users register named Go handler functions with a `Registry`. The single Temporal activity (`ExecuteFunctionActivity`) looks up the function by name and calls it.

No middleware, no plugins, no embedded interpreters.

## Registry

```go
// function/registry.go
type Handler func(ctx context.Context, input FunctionInput) (*FunctionOutput, error)

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

type Registry struct {
    mu       sync.RWMutex
    handlers map[string]Handler
}

func NewRegistry() *Registry
func (r *Registry) Register(name string, handler Handler)
func (r *Registry) Get(name string) (Handler, error)
func (r *Registry) Has(name string) bool
```

## Payloads

```go
// function/payload/payload.go
type FunctionExecutionInput struct {
    Name    string            `json:"name" validate:"required"`
    Args    map[string]string `json:"args,omitempty"`
    Data    []byte            `json:"data,omitempty"`
    Env     map[string]string `json:"env,omitempty"`
    WorkDir string            `json:"work_dir,omitempty"`
    Timeout time.Duration     `json:"timeout,omitempty"`
    Labels  map[string]string `json:"labels,omitempty"`
}

func (i *FunctionExecutionInput) Validate() error
func (i *FunctionExecutionInput) ActivityName() string  // returns "ExecuteFunctionActivity"

type FunctionExecutionOutput struct {
    Name       string            `json:"name"`
    Success    bool              `json:"success"`
    Error      string            `json:"error,omitempty"`
    Result     map[string]string `json:"result,omitempty"`
    Data       []byte            `json:"data,omitempty"`
    Duration   time.Duration     `json:"duration"`
    StartedAt  time.Time         `json:"started_at"`
    FinishedAt time.Time         `json:"finished_at"`
}

func (o FunctionExecutionOutput) IsSuccess() bool
func (o FunctionExecutionOutput) GetError() string
```

Pipeline, parallel, and loop payloads follow the same pattern as docker:

```go
type PipelineInput struct {
    Functions   []FunctionExecutionInput `json:"functions" validate:"required,min=1"`
    StopOnError bool                     `json:"stop_on_error"`
}

type PipelineOutput struct {
    Results       []FunctionExecutionOutput `json:"results"`
    TotalSuccess  int                       `json:"total_success"`
    TotalFailed   int                       `json:"total_failed"`
    TotalDuration time.Duration             `json:"total_duration"`
}

// Similar for ParallelInput/Output, LoopInput/Output, ParameterizedLoopInput
```

## Activity

```go
// function/activity/function.go
func NewExecuteFunctionActivity(registry *Registry) func(ctx context.Context, input payload.FunctionExecutionInput) (*payload.FunctionExecutionOutput, error)
```

A closure over the registry. Execution flow:
1. Validate input.
2. Look up handler by `input.Name`.
3. Set env vars and working directory if specified.
4. Call handler with `FunctionInput` mapped from payload.
5. Wrap result into `FunctionExecutionOutput` with timing and success/error.

## Workflows

Thin wrappers delegating to the generic core with `[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput]`:

```go
// function/workflow/
func ExecuteFunctionWorkflow(ctx, input) (*FunctionExecutionOutput, error)
func FunctionPipelineWorkflow(ctx, input) (*PipelineOutput, error)
func ParallelFunctionsWorkflow(ctx, input) (*ParallelOutput, error)
func LoopWorkflow(ctx, input) (*LoopOutput, error)
func ParameterizedLoopWorkflow(ctx, input) (*LoopOutput, error)
```

Loop workflows include a `functionSubstitutor()` that applies `{{item}}`, `{{index}}`, `{{.param}}` substitution to `Name`, `Args`, `Env`, and `WorkDir`.

## Builder

```go
// function/builder/builder.go
type WorkflowBuilder struct { ... }

func NewWorkflowBuilder(name string, opts ...BuilderOption) *WorkflowBuilder

.Add(source WorkflowSource)
.AddInput(input FunctionExecutionInput)
.StopOnError(bool)
.Parallel(bool)
.FailFast(bool)
.MaxConcurrency(int)

.BuildPipeline() (*payload.PipelineInput, error)
.BuildParallel() (*payload.ParallelInput, error)
.BuildSingle() (*payload.FunctionExecutionInput, error)
```

`WorkflowSource` interface with `ToInput() payload.FunctionExecutionInput`.

## Registration

```go
// function/worker.go
func RegisterWorkflows(w worker.Worker)
func RegisterActivities(w worker.Worker, registry *Registry)
func RegisterAll(w worker.Worker, registry *Registry)
```

Note: `RegisterActivities` and `RegisterAll` take the registry as a parameter since the activity is a closure.

## Scope Exclusions

- **No templates** — function inputs are already simple, unlike verbose container config.
- **No patterns** — patterns are domain-specific; hard to define generic ones for arbitrary Go functions.
- **No middleware/hooks** — YAGNI. Users handle cross-cutting concerns inside their handler functions.

## Usage Example

```go
// Create registry and register handlers
registry := function.NewRegistry()

registry.Register("validate-config", func(ctx context.Context, input function.FunctionInput) (*function.FunctionOutput, error) {
    configPath := input.Args["path"]
    // ... validate config ...
    return &function.FunctionOutput{
        Result: map[string]string{"valid": "true"},
    }, nil
})

registry.Register("transform-data", func(ctx context.Context, input function.FunctionInput) (*function.FunctionOutput, error) {
    // ... transform ...
    return &function.FunctionOutput{
        Result: map[string]string{"records": "150"},
    }, nil
})

// Register with Temporal worker
w := worker.New(client, "task-queue", worker.Options{})
function.RegisterAll(w, registry)

// Build a pipeline using the builder
pipeline, _ := builder.NewWorkflowBuilder("my-pipeline").
    AddInput(payload.FunctionExecutionInput{Name: "validate-config", Args: map[string]string{"path": "/etc/app.yaml"}}).
    AddInput(payload.FunctionExecutionInput{Name: "transform-data", Args: map[string]string{"source": "input.csv"}}).
    StopOnError(true).
    BuildPipeline()

// Execute
we, _ := client.ExecuteWorkflow(ctx, options, workflow.FunctionPipelineWorkflow, pipeline)
```
