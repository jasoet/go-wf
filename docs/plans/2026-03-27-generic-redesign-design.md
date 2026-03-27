# Generic Redesign — go-wf

**Date:** 2026-03-27
**Status:** Approved
**Branch:** `feat/generic-redesign`

## Goal

Make the entire go-wf library fully generic with compile-time type safety. Breaking changes are acceptable for all packages except `datasync/` (already generic, minimal changes only). The result should be clean, type-safe, verbose (AI-friendly), and well tested.

## Principles

- Full datasync-style generics across container, function, and workflow core
- Unified package structure across all domains
- Single generic orchestration implementation in `workflow/` — domain packages are thin wrappers
- `Store[T]` replaces `artifacts/` with typed storage and codec-based serialization
- Verbose is better than clever — compiler catches mistakes, not humans

## Core Generic Types (`workflow/`)

### Constraints

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

### Orchestration Types (two type parameters: I, O)

```go
type PipelineInput[I TaskInput, O TaskOutput] struct {
    Tasks       []I
    StopOnError bool
    Cleanup     bool
}

type PipelineOutput[O TaskOutput] struct {
    Results       []O
    TotalSuccess  int
    TotalFailed   int
    TotalDuration time.Duration
}

type ParallelInput[I TaskInput, O TaskOutput] struct {
    Tasks           []I
    MaxConcurrency  int
    FailureStrategy string
}

type ParallelOutput[O TaskOutput] struct {
    Results       []O
    TotalSuccess  int
    TotalFailed   int
    TotalDuration time.Duration
}

type LoopInput[I TaskInput, O TaskOutput] struct {
    Items           []string
    Template        I
    Parallel        bool
    MaxConcurrency  int
    FailureStrategy string
}

type ParameterizedLoopInput[I TaskInput, O TaskOutput] struct {
    Parameters      map[string][]string
    Template        I
    Parallel        bool
    MaxConcurrency  int
    FailureStrategy string
}

type LoopOutput[O TaskOutput] struct {
    Results       []O
    TotalSuccess  int
    TotalFailed   int
    TotalDuration time.Duration
    ItemCount     int
}
```

### Generic Workflow Functions

Shared orchestration — domain packages pass callbacks:

```go
func ExecutePipeline[I TaskInput, O TaskOutput](
    ctx wf.Context, input PipelineInput[I, O],
    execute func(wf.Context, I) (*O, error),
) (*PipelineOutput[O], error)

func ExecuteParallel[I TaskInput, O TaskOutput](
    ctx wf.Context, input ParallelInput[I, O],
    execute func(wf.Context, I) (*O, error),
) (*ParallelOutput[O], error)

func ExecuteLoop[I TaskInput, O TaskOutput](
    ctx wf.Context, input LoopInput[I, O],
    substitute func(template I, item string, index int) I,
    execute func(wf.Context, I) (*O, error),
) (*LoopOutput[O], error)

func ExecuteParameterizedLoop[I TaskInput, O TaskOutput](
    ctx wf.Context, input ParameterizedLoopInput[I, O],
    substitute func(template I, params map[string]string) I,
    execute func(wf.Context, I) (*O, error),
) (*LoopOutput[O], error)
```

### DAG

```go
type DAGNode[I TaskInput, O TaskOutput] struct {
    Name         string
    Input        I
    Dependencies []string
}

type DAGInput[I TaskInput, O TaskOutput] struct {
    Nodes       []DAGNode[I, O]
    FailFast    bool
    MaxParallel int
    Store       store.RawStore
}

type DAGOutput[O TaskOutput] struct {
    Results       map[string]*O
    NodeResults   []NodeResult[O]
    TotalSuccess  int
    TotalFailed   int
    TotalDuration time.Duration
}

func ExecuteDAG[I TaskInput, O TaskOutput](
    ctx wf.Context, input DAGInput[I, O],
    execute func(wf.Context, I) (*O, error),
) (*DAGOutput[O], error)
```

## Store (`workflow/store/`)

Replaces `workflow/artifacts/`. Two layers: byte-level implementations and typed wrappers.

### Interfaces

```go
type RawStore interface {
    Upload(ctx context.Context, key string, data io.Reader) error
    Download(ctx context.Context, key string) (io.ReadCloser, error)
    Delete(ctx context.Context, key string) error
    Exists(ctx context.Context, key string) (bool, error)
    List(ctx context.Context, prefix string) ([]string, error)
    Close() error
}

type Store[T any] interface {
    Save(ctx context.Context, key string, value T) error
    Load(ctx context.Context, key string) (T, error)
    Delete(ctx context.Context, key string) error
    Exists(ctx context.Context, key string) (bool, error)
    List(ctx context.Context, prefix string) ([]string, error)
    Close() error
}

type Codec[T any] interface {
    Encode(value T) (io.Reader, error)
    Decode(reader io.ReadCloser) (T, error)
}
```

### Implementations

- `TypedStore[T]` — adapts `RawStore` + `Codec[T]` into `Store[T]`
- `JSONCodec[T]` — JSON serialization (default)
- `BytesCodec` — pass-through for `[]byte`
- `LocalStore` — filesystem (implements `RawStore`)
- `S3Store` — S3-compatible (implements `RawStore`)
- `InstrumentedStore` — OTel wrapper (implements `RawStore`)
- `KeyBuilder` — composable key generation (replaces `ArtifactMetadata.StorageKey()`)

### Constructors

```go
func NewTypedStore[T any](raw RawStore, codec Codec[T]) Store[T]
func NewJSONStore[T any](raw RawStore) Store[T]
func NewBytesStore(raw RawStore) Store[[]byte]
```

### Store Activities

```go
func UploadActivity[T any](ctx context.Context, store Store[T], key string, value T) error
func DownloadActivity[T any](ctx context.Context, store Store[T], key string) (T, error)
func DeleteActivity(ctx context.Context, store RawStore, key string) error
func CleanupActivity(ctx context.Context, store RawStore, prefix string) error
```

## Container Package (`container/`)

### Types (moved from `container/payload/`)

```go
type ContainerInput struct {
    Image        string
    Command      []string
    Entrypoint   []string
    Env          map[string]string
    Ports        []string
    Volumes      []string
    WorkDir      string
    User         string
    Name         string
    Labels       map[string]string
    AutoRemove   bool
    WaitStrategy WaitStrategyConfig
    StartTimeout time.Duration
    RunTimeout   time.Duration
}

func (i *ContainerInput) Validate() error     { ... }
func (i *ContainerInput) ActivityName() string { return "StartContainerActivity" }

type ContainerOutput struct {
    ContainerID string
    ExitCode    int
    Stdout      string
    Stderr      string
    Endpoint    string
    Ports       map[string]string
    StartedAt   time.Time
    FinishedAt  time.Time
    Duration    time.Duration
    Success     bool
    Error       string
}

func (o *ContainerOutput) IsSuccess() bool  { return o.Success }
func (o *ContainerOutput) GetError() string { return o.Error }
```

### Activity

```go
type Activities[I workflow.TaskInput, O workflow.TaskOutput] struct {
    executor func(ctx context.Context, input I) (*O, error)
}
```

### Builder

```go
type WorkflowBuilder[I workflow.TaskInput, O workflow.TaskOutput] struct { ... }

func NewWorkflowBuilder[I workflow.TaskInput, O workflow.TaskOutput]() *WorkflowBuilder[I, O]
func NewContainerBuilder() *WorkflowBuilder[ContainerInput, ContainerOutput]

func (b *WorkflowBuilder[I, O]) Add(input I) *WorkflowBuilder[I, O]
func (b *WorkflowBuilder[I, O]) BuildPipeline() (*workflow.PipelineInput[I, O], error)
func (b *WorkflowBuilder[I, O]) BuildParallel() (*workflow.ParallelInput[I, O], error)
func (b *WorkflowBuilder[I, O]) BuildSingle() (*I, error)
```

### Operations (generic)

```go
func SubmitWorkflow[I workflow.TaskInput](
    ctx context.Context, client client.Client,
    input I, taskQueue string, opts ...Option,
) (*WorkflowStatus, error)

func SubmitAndWait[I workflow.TaskInput, O workflow.TaskOutput](
    ctx context.Context, client client.Client,
    input I, taskQueue string, timeout time.Duration,
) (*O, error)
```

### Workflow (thin wrappers)

```go
func PipelineWorkflow(ctx wf.Context, input workflow.PipelineInput[ContainerInput, ContainerOutput]) (*workflow.PipelineOutput[ContainerOutput], error) {
    return workflow.ExecutePipeline(ctx, input, executeContainer)
}
```

### Template

```go
type WorkflowSource[I workflow.TaskInput] interface {
    ToInput() I
}
```

## Function Package (`function/`)

Identical structure to container with domain-specific types.

### Types (moved from `function/payload/`)

```go
type FunctionInput struct {
    Name    string
    Args    map[string]string
    Data    []byte
    Env     map[string]string
    WorkDir string
    Timeout time.Duration
    Labels  map[string]string
}

func (i *FunctionInput) Validate() error      { ... }
func (i *FunctionInput) ActivityName() string  { return "ExecuteFunctionActivity" }

type FunctionOutput struct {
    Name       string
    Success    bool
    Error      string
    Result     map[string]string
    Data       []byte
    Duration   time.Duration
    StartedAt  time.Time
    FinishedAt time.Time
}

func (o *FunctionOutput) IsSuccess() bool  { return o.Success }
func (o *FunctionOutput) GetError() string { return o.Error }
```

### Workflow (thin wrappers)

```go
func PipelineWorkflow(ctx wf.Context, input workflow.PipelineInput[FunctionInput, FunctionOutput]) (*workflow.PipelineOutput[FunctionOutput], error) {
    return workflow.ExecutePipeline(ctx, input, executeFunction)
}
```

## Datasync Impact (minimal)

```go
// Job field change only:
type Job[T, U any] struct {
    // ... existing fields ...
    Store store.RawStore  // was: ArtifactConfig *artifacts.ArtifactConfig
}
```

Import path change: `workflow/artifacts` → `workflow/store`.

## Complete Package Layout

```
go-wf/
  workflow/
    task.go                    # TaskInput, TaskOutput constraints
    types.go                   # PipelineInput[I,O], ParallelInput[I,O], LoopInput[I,O], etc.
    pipeline.go                # ExecutePipeline[I,O]
    parallel.go                # ExecuteParallel[I,O]
    loop.go                    # ExecuteLoop[I,O], ExecuteParameterizedLoop[I,O]
    dag.go                     # ExecuteDAG[I,O], DAGNode[I,O], DAGInput[I,O]
    helpers.go                 # SubstituteTemplate, GenerateParameterCombinations
    otel.go                    # Instrumented wrappers
    errors/
      errors.go
    store/
      store.go                 # RawStore, Store[T], Codec[T], TypedStore[T]
      codec.go                 # JSONCodec[T], BytesCodec
      key.go                   # KeyBuilder
      local.go                 # LocalStore
      s3.go                    # S3Store
      otel.go                  # InstrumentedStore
      activities.go            # Generic store activities
    testutil/
      temporal.go

  container/
    types.go                   # ContainerInput, ContainerOutput, WaitStrategyConfig
    types_extended.go          # DAG types, ResourceLimits, ConditionalBehavior
    operations.go              # SubmitWorkflow[I], SubmitAndWait[I,O]
    activity/
      container.go             # Activities[ContainerInput, ContainerOutput]
      otel.go
    builder/
      builder.go               # WorkflowBuilder[I,O], LoopBuilder[I,O]
      options.go
    workflow/
      container.go             # Single execution
      pipeline.go              # → workflow.ExecutePipeline
      parallel.go              # → workflow.ExecuteParallel
      loop.go                  # → workflow.ExecuteLoop
      dag.go                   # → workflow.ExecuteDAG
    template/
      container.go
      script.go
      http.go
    worker.go

  function/
    types.go                   # FunctionInput, FunctionOutput
    types_extended.go          # DAG types, OutputMapping, InputMapping
    registry.go                # Function registry
    activity/
      function.go              # Activities[FunctionInput, FunctionOutput]
      otel.go
    builder/
      builder.go               # WorkflowBuilder[I,O], LoopBuilder[I,O]
      options.go
    workflow/
      function.go              # Single execution
      pipeline.go              # → workflow.ExecutePipeline
      parallel.go              # → workflow.ExecuteParallel
      loop.go                  # → workflow.ExecuteLoop
      dag.go                   # → workflow.ExecuteDAG
    worker.go

  datasync/
    source.go                  # Source[T], ParamSource[T,P]
    mapper.go                  # Mapper[T,U], MapperFunc[T,U], IdentityMapper[T]
    sink.go                    # Sink[U], WriteResult
    job.go                     # Job[T,U] (Store field updated)
    map_result.go              # MapResult[U], DetailedMapper[T,U], RecordMapper[T,U]
    insert_sink.go             # InsertIfAbsentSink[U, ID comparable]
    registration.go            # JobRegistration, BuildRegistration[T,U]
    runner.go                  # Runner[T,U]
    activity/
      sync.go                  # Activities[T,U]
      otel.go
    builder/
      builder.go               # SyncJobBuilder[T,U]
    workflow/
      sync.go                  # Workflow functions, registration
    worker.go                  # RegisterJob[T,U], RegisterAll
```

## Breaking Changes Summary

| Area | Before | After |
|------|--------|-------|
| Orchestration types | `PipelineInput[I]` | `PipelineInput[I, O]` |
| Orchestration logic | Duplicated in container/ and function/ | Single impl in workflow/, domains wrap |
| Storage | `artifacts.ArtifactStore` + `ArtifactMetadata` | `store.RawStore` + `Store[T]` + `Codec[T]` |
| Payload location | `container/payload/`, `function/payload/` | `container/types.go`, `function/types.go` |
| Builder return | `Build() (interface{}, error)` | `BuildPipeline() (*PipelineInput[I,O], error)` |
| Operations | `SubmitWorkflow(input interface{})` | `SubmitWorkflow[I TaskInput](input I)` |
| Activity struct | Concrete per domain | `Activities[I, O]` per domain |
| Template source | `WorkflowSource` (non-generic) | `WorkflowSource[I TaskInput]` |
| Datasync store | `ArtifactConfig *artifacts.ArtifactConfig` | `Store store.RawStore` |

## What Gets Eliminated

- `container/workflow/helpers.go` — absorbed into generic `workflow/` core
- `function/workflow/helpers.go` — same
- Duplicated pipeline/parallel/loop/DAG logic across container and function
- `workflow/artifacts/` package entirely (replaced by `workflow/store/`)
- `container/payload/` subpackage (types move to `container/`)
- `function/payload/` subpackage (types move to `function/`)
- All `interface{}` in public APIs
