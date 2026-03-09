# Design: Extract Generic Workflow Core

**Date:** 2026-03-06
**Status:** Approved

## Problem

All workflow orchestration logic (pipeline, parallel, DAG, loop) lives under `docker/` but has no actual Docker dependency. Only `docker/activity/container.go` interacts with Docker. The library should be generic so users can plug in any activity (Docker, Kubernetes, SSH, shell, etc.).

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Module strategy | Single `go.mod` at root | Simpler; unused packages don't affect compilation or binary |
| Type safety | Go generics (`[I TaskInput, O TaskOutput]`) | Compile-time safety, better IDE experience, self-documenting API |
| Activity resolution | `ActivityName()` on `TaskInput` interface | Simple, works with Temporal's name-based dispatch, useful in history |
| Templates & patterns | Stay in `docker/` | They're container-specific; generic patterns can be added later if needed |
| Migration approach | Bottom-up extraction | Build generic core first, then rewire docker as adapter |
| Core location | `workflow/` package (not root) | Keeps root directory clean for non-source directories |

## Generic Core Interfaces

```go
// workflow/task.go
package workflow

type TaskInput interface {
    Validate() error
    ActivityName() string
}

type TaskOutput interface {
    IsSuccess() bool
    GetError() string
}
```

Generic composite types in `workflow/payload/`:

```go
type PipelineInput[I TaskInput] struct {
    Tasks       []I    `json:"tasks" validate:"required,min=1"`
    StopOnError bool   `json:"stop_on_error"`
}

type ParallelInput[I TaskInput] struct {
    Tasks           []I    `json:"tasks" validate:"required,min=1"`
    MaxConcurrency  int    `json:"max_concurrency,omitempty"`
    FailureStrategy string `json:"failure_strategy" validate:"oneof='' continue fail_fast"`
}

type LoopInput[I TaskInput] struct {
    Items           []string `json:"items" validate:"required,min=1"`
    Template        I        `json:"template" validate:"required"`
    Parallel        bool     `json:"parallel"`
    MaxConcurrency  int      `json:"max_concurrency,omitempty"`
    FailureStrategy string   `json:"failure_strategy" validate:"oneof='' continue fail_fast"`
}

type PipelineOutput[O TaskOutput] struct {
    Results       []O           `json:"results"`
    TotalSuccess  int           `json:"total_success"`
    TotalFailed   int           `json:"total_failed"`
    TotalDuration time.Duration `json:"total_duration"`
}

// ParallelOutput, LoopOutput follow same pattern
```

## Generic Workflow Implementations

Workflows are generic functions in `workflow/`:

```go
func ExecuteTaskWorkflow[I TaskInput, O TaskOutput](ctx workflow.Context, input I) (*O, error)
func PipelineWorkflow[I TaskInput, O TaskOutput](ctx workflow.Context, input PipelineInput[I]) (*PipelineOutput[O], error)
func ParallelWorkflow[I TaskInput, O TaskOutput](ctx workflow.Context, input ParallelInput[I]) (*ParallelOutput[O], error)
func DAGWorkflow[I TaskInput, O TaskOutput](ctx workflow.Context, input DAGInput[I]) (*DAGOutput[O], error)
func LoopWorkflow[I TaskInput, O TaskOutput](ctx workflow.Context, input LoopInput[I]) (*LoopOutput[O], error)
```

Activities are called via `input.ActivityName()` instead of hardcoded function references.

Docker module provides instantiated wrappers for Temporal registration:

```go
// docker/workflow.go
func ExecuteContainerWorkflow(ctx workflow.Context, input ContainerExecutionInput) (*ContainerExecutionOutput, error) {
    return workflow.ExecuteTaskWorkflow[ContainerExecutionInput, ContainerExecutionOutput](ctx, input)
}
```

## Package Layout

```
go-wf/
├── workflow/                    # Generic core
│   ├── task.go                  # TaskInput, TaskOutput interfaces
│   ├── execute.go               # ExecuteTaskWorkflow[I,O]
│   ├── pipeline.go              # PipelineWorkflow[I,O]
│   ├── parallel.go              # ParallelWorkflow[I,O]
│   ├── dag.go                   # DAGWorkflow[I,O]
│   ├── loop.go                  # LoopWorkflow[I,O]
│   ├── helpers.go               # Template substitution, output extraction
│   ├── payload/                 # Generic composite types
│   ├── builder/                 # Generic fluent builder API
│   ├── errors/                  # Error types (moved from docker/errors/)
│   └── artifacts/               # ArtifactStore interface + implementations
├── docker/                      # Docker activity module
│   ├── worker.go                # RegisterAll (instantiated workflow wrappers)
│   ├── operations.go            # Temporal client operations
│   ├── activity/                # StartContainerActivity (only Docker import)
│   ├── payload/                 # ContainerExecutionInput/Output
│   ├── template/                # Container, Script, HTTP templates
│   └── patterns/                # CI/CD, fan-out, loop patterns
├── examples/docker/
├── docs/
├── go.mod                       # Single module
└── Taskfile.yml
```

Dependency flow (one-way, no cycles):

```
workflow/*        → workflow/payload (no docker dependency)
docker/payload    → workflow/task (implements TaskInput/TaskOutput)
docker/activity   → docker/payload + pkg/v2/docker
docker/worker     → docker/activity + workflow/
docker/template   → docker/payload
docker/patterns   → docker/payload + docker/template + workflow/builder
```

## Migration Map

| Current | Destination | Change |
|---------|-------------|--------|
| `docker/workflow/*.go` | `workflow/*.go` | Rewritten as generic `[I,O]` |
| `docker/payload/` (composite types) | `workflow/payload/` | Generic versions |
| `docker/payload/` (container types) | `docker/payload/` | Stays, adds interface methods |
| `docker/errors/` | `workflow/errors/` | Move as-is |
| `docker/artifacts/` | `workflow/artifacts/` | Move as-is |
| `docker/builder/` | `workflow/builder/` | Rewritten as generic `[I]` |
| `docker/activity/` | `docker/activity/` | Stays as-is |
| `docker/template/` | `docker/template/` | Stays, updated imports |
| `docker/patterns/` | `docker/patterns/` | Stays, updated imports |
| `docker/worker.go` | `docker/worker.go` | Adds instantiated wrappers |
| `docker/operations.go` | `docker/operations.go` | Stays as-is |

## Test Strategy

- `workflow/*_test.go`: Unit tests with mock `TaskInput`/`TaskOutput` implementations
- `docker/*_test.go`: Updated imports, same test logic
- Integration tests: Unchanged, test end-to-end with real containers
- No published releases exist, so no backwards compatibility concern
