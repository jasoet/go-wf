# Generic Redesign Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make go-wf fully generic with compile-time type safety across all packages.

**Architecture:** Six-phase bottom-up refactor — store first (no deps), then core types, then container/function/datasync, then cleanup. Each phase compiles and tests independently.

**Tech Stack:** Go 1.26+ generics, Temporal SDK, testcontainers-go, testify, go-playground/validator

**Design Doc:** `docs/plans/2026-03-27-generic-redesign-design.md`

---

## Phase 1: Store Redesign (`workflow/store/`)

Replace `workflow/artifacts/` with generic `Store[T]` + `RawStore` + `Codec[T]`. Build from scratch, keep artifacts until consumers migrate.

### Task 1: RawStore Interface and Codec

**Files:**
- Create: `workflow/store/store.go`
- Create: `workflow/store/codec.go`
- Create: `workflow/store/store_test.go`

**Step 1: Write the failing test**

```go
// workflow/store/store_test.go
package store_test

import (
    "bytes"
    "io"
    "testing"

    "github.com/jasoet/go-wf/workflow/store"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestJSONCodec_EncodeDecode(t *testing.T) {
    type Sample struct {
        Name  string `json:"name"`
        Value int    `json:"value"`
    }

    codec := store.NewJSONCodec[Sample]()

    original := Sample{Name: "test", Value: 42}
    reader, err := codec.Encode(original)
    require.NoError(t, err)

    decoded, err := codec.Decode(io.NopCloser(reader))
    require.NoError(t, err)
    assert.Equal(t, original, decoded)
}

func TestBytesCodec_EncodeDecode(t *testing.T) {
    codec := store.NewBytesCodec()

    original := []byte("hello world")
    reader, err := codec.Encode(original)
    require.NoError(t, err)

    decoded, err := codec.Decode(io.NopCloser(reader))
    require.NoError(t, err)
    assert.Equal(t, original, decoded)
}
```

**Step 2: Run test to verify it fails**

Run: `task test:pkg -- ./workflow/store/...`
Expected: FAIL — package doesn't exist yet

**Step 3: Write minimal implementation**

```go
// workflow/store/store.go
package store

import (
    "context"
    "io"
)

// RawStore is the byte-level storage interface. Implementations provide this.
type RawStore interface {
    Upload(ctx context.Context, key string, data io.Reader) error
    Download(ctx context.Context, key string) (io.ReadCloser, error)
    Delete(ctx context.Context, key string) error
    Exists(ctx context.Context, key string) (bool, error)
    List(ctx context.Context, prefix string) ([]string, error)
    Close() error
}

// Store is a typed storage interface with automatic serialization.
type Store[T any] interface {
    Save(ctx context.Context, key string, value T) error
    Load(ctx context.Context, key string) (T, error)
    Delete(ctx context.Context, key string) error
    Exists(ctx context.Context, key string) (bool, error)
    List(ctx context.Context, prefix string) ([]string, error)
    Close() error
}

// Codec defines serialization strategy for Store[T].
type Codec[T any] interface {
    Encode(value T) (io.Reader, error)
    Decode(reader io.ReadCloser) (T, error)
}

// TypedStore adapts a RawStore + Codec into a Store[T].
type TypedStore[T any] struct {
    raw   RawStore
    codec Codec[T]
}

// NewTypedStore creates a new TypedStore.
func NewTypedStore[T any](raw RawStore, codec Codec[T]) Store[T] {
    return &TypedStore[T]{raw: raw, codec: codec}
}

// NewJSONStore creates a Store[T] that serializes as JSON.
func NewJSONStore[T any](raw RawStore) Store[T] {
    return NewTypedStore[T](raw, NewJSONCodec[T]())
}

// NewBytesStore creates a Store[[]byte] with pass-through serialization.
func NewBytesStore(raw RawStore) Store[[]byte] {
    return NewTypedStore[[]byte](raw, NewBytesCodec())
}

func (s *TypedStore[T]) Save(ctx context.Context, key string, value T) error {
    reader, err := s.codec.Encode(value)
    if err != nil {
        return err
    }
    return s.raw.Upload(ctx, key, reader)
}

func (s *TypedStore[T]) Load(ctx context.Context, key string) (T, error) {
    var zero T
    reader, err := s.raw.Download(ctx, key)
    if err != nil {
        return zero, err
    }
    defer reader.Close()
    return s.codec.Decode(reader)
}

func (s *TypedStore[T]) Delete(ctx context.Context, key string) error {
    return s.raw.Delete(ctx, key)
}

func (s *TypedStore[T]) Exists(ctx context.Context, key string) (bool, error) {
    return s.raw.Exists(ctx, key)
}

func (s *TypedStore[T]) List(ctx context.Context, prefix string) ([]string, error) {
    return s.raw.List(ctx, prefix)
}

func (s *TypedStore[T]) Close() error {
    return s.raw.Close()
}
```

```go
// workflow/store/codec.go
package store

import (
    "bytes"
    "encoding/json"
    "io"
)

// JSONCodec serializes/deserializes values as JSON.
type JSONCodec[T any] struct{}

// NewJSONCodec creates a new JSONCodec.
func NewJSONCodec[T any]() Codec[T] {
    return &JSONCodec[T]{}
}

func (c *JSONCodec[T]) Encode(value T) (io.Reader, error) {
    data, err := json.Marshal(value)
    if err != nil {
        return nil, err
    }
    return bytes.NewReader(data), nil
}

func (c *JSONCodec[T]) Decode(reader io.ReadCloser) (T, error) {
    var value T
    if err := json.NewDecoder(reader).Decode(&value); err != nil {
        return value, err
    }
    return value, nil
}

// BytesCodec is a pass-through codec for raw bytes.
type BytesCodec struct{}

// NewBytesCodec creates a new BytesCodec.
func NewBytesCodec() Codec[[]byte] {
    return &BytesCodec{}
}

func (c *BytesCodec) Encode(value []byte) (io.Reader, error) {
    return bytes.NewReader(value), nil
}

func (c *BytesCodec) Decode(reader io.ReadCloser) ([]byte, error) {
    return io.ReadAll(reader)
}
```

**Step 4: Run test to verify it passes**

Run: `task test:pkg -- ./workflow/store/...`
Expected: PASS

**Step 5: Commit**

```
feat(store): add Store[T], RawStore, Codec[T] interfaces and TypedStore adapter
```

---

### Task 2: KeyBuilder

**Files:**
- Create: `workflow/store/key.go`
- Modify: `workflow/store/store_test.go`

**Step 1: Write the failing test**

```go
// append to workflow/store/store_test.go
func TestKeyBuilder(t *testing.T) {
    key := store.NewKeyBuilder().
        WithWorkflow("wf-123").
        WithRun("run-456").
        WithStep("step-1").
        WithName("output.json").
        Build()

    assert.Equal(t, "wf-123/run-456/step-1/output.json", key)
}

func TestKeyBuilder_Partial(t *testing.T) {
    key := store.NewKeyBuilder().
        WithWorkflow("wf-123").
        WithName("data.bin").
        Build()

    assert.Equal(t, "wf-123/data.bin", key)
}
```

**Step 2: Run test to verify it fails**

Run: `task test:pkg -- ./workflow/store/...`
Expected: FAIL — KeyBuilder not defined

**Step 3: Write minimal implementation**

```go
// workflow/store/key.go
package store

import "strings"

// KeyBuilder constructs storage keys from composable parts.
type KeyBuilder struct {
    parts []string
}

// NewKeyBuilder creates a new KeyBuilder.
func NewKeyBuilder() *KeyBuilder {
    return &KeyBuilder{}
}

func (kb *KeyBuilder) WithWorkflow(id string) *KeyBuilder {
    kb.parts = append(kb.parts, id)
    return kb
}

func (kb *KeyBuilder) WithRun(id string) *KeyBuilder {
    kb.parts = append(kb.parts, id)
    return kb
}

func (kb *KeyBuilder) WithStep(name string) *KeyBuilder {
    kb.parts = append(kb.parts, name)
    return kb
}

func (kb *KeyBuilder) WithName(name string) *KeyBuilder {
    kb.parts = append(kb.parts, name)
    return kb
}

// Build returns the composed key as a slash-separated path.
func (kb *KeyBuilder) Build() string {
    return strings.Join(kb.parts, "/")
}
```

**Step 4: Run test to verify it passes**

Run: `task test:pkg -- ./workflow/store/...`
Expected: PASS

**Step 5: Commit**

```
feat(store): add KeyBuilder for composable storage key generation
```

---

### Task 3: LocalStore (implements RawStore)

**Files:**
- Create: `workflow/store/local.go`
- Create: `workflow/store/local_test.go`

**Step 1: Write the failing test**

```go
// workflow/store/local_test.go
package store_test

import (
    "context"
    "testing"

    "github.com/jasoet/go-wf/workflow/store"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestLocalStore_RoundTrip(t *testing.T) {
    dir := t.TempDir()
    raw, err := store.NewLocalStore(dir)
    require.NoError(t, err)
    defer raw.Close()

    ctx := context.Background()
    s := store.NewJSONStore[map[string]string](raw)

    data := map[string]string{"key": "value"}
    err = s.Save(ctx, "test/data.json", data)
    require.NoError(t, err)

    exists, err := s.Exists(ctx, "test/data.json")
    require.NoError(t, err)
    assert.True(t, exists)

    loaded, err := s.Load(ctx, "test/data.json")
    require.NoError(t, err)
    assert.Equal(t, data, loaded)

    keys, err := s.List(ctx, "test/")
    require.NoError(t, err)
    assert.Len(t, keys, 1)

    err = s.Delete(ctx, "test/data.json")
    require.NoError(t, err)

    exists, err = s.Exists(ctx, "test/data.json")
    require.NoError(t, err)
    assert.False(t, exists)
}
```

**Step 2: Run test to verify it fails**

Run: `task test:pkg -- ./workflow/store/...`
Expected: FAIL — NewLocalStore not defined

**Step 3: Write minimal implementation**

Port logic from `workflow/artifacts/local.go` but with simplified `RawStore` interface (string keys instead of ArtifactMetadata). Key differences:
- `Upload(ctx, key string, data io.Reader)` instead of `Upload(ctx, ArtifactMetadata, io.Reader)`
- `List` returns `[]string` (keys) instead of `[]ArtifactMetadata`
- No archive/directory logic — store deals with raw bytes only
- Path traversal validation on keys

```go
// workflow/store/local.go
package store

import (
    "context"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "strings"
)

const maxUploadSize = 1 << 30 // 1 GB

// LocalStore implements RawStore using the local filesystem.
type LocalStore struct {
    basePath string
}

// NewLocalStore creates a LocalStore at the given base path.
func NewLocalStore(basePath string) (RawStore, error) {
    absPath, err := filepath.Abs(basePath)
    if err != nil {
        return nil, fmt.Errorf("invalid base path: %w", err)
    }
    if err := os.MkdirAll(absPath, 0o750); err != nil {
        return nil, fmt.Errorf("failed to create base directory: %w", err)
    }
    return &LocalStore{basePath: absPath}, nil
}

func (s *LocalStore) resolvePath(key string) (string, error) {
    if strings.Contains(key, "..") {
        return "", fmt.Errorf("invalid key: path traversal detected")
    }
    return filepath.Join(s.basePath, key), nil
}

func (s *LocalStore) Upload(_ context.Context, key string, data io.Reader) error {
    path, err := s.resolvePath(key)
    if err != nil {
        return err
    }
    if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
        return fmt.Errorf("failed to create directory: %w", err)
    }
    f, err := os.Create(path) //#nosec G304
    if err != nil {
        return fmt.Errorf("failed to create file: %w", err)
    }
    defer f.Close()
    if _, err := io.Copy(f, io.LimitReader(data, maxUploadSize)); err != nil {
        return fmt.Errorf("failed to write file: %w", err)
    }
    return nil
}

func (s *LocalStore) Download(_ context.Context, key string) (io.ReadCloser, error) {
    path, err := s.resolvePath(key)
    if err != nil {
        return nil, err
    }
    f, err := os.Open(path) //#nosec G304
    if err != nil {
        return nil, fmt.Errorf("failed to open file: %w", err)
    }
    return f, nil
}

func (s *LocalStore) Delete(_ context.Context, key string) error {
    path, err := s.resolvePath(key)
    if err != nil {
        return err
    }
    return os.Remove(path)
}

func (s *LocalStore) Exists(_ context.Context, key string) (bool, error) {
    path, err := s.resolvePath(key)
    if err != nil {
        return false, err
    }
    _, err = os.Stat(path)
    if os.IsNotExist(err) {
        return false, nil
    }
    return err == nil, err
}

func (s *LocalStore) List(_ context.Context, prefix string) ([]string, error) {
    if strings.Contains(prefix, "..") {
        return nil, fmt.Errorf("invalid prefix: path traversal detected")
    }
    searchPath := filepath.Join(s.basePath, prefix)
    var keys []string
    err := filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }
        if !info.IsDir() {
            rel, err := filepath.Rel(s.basePath, path)
            if err != nil {
                return err
            }
            keys = append(keys, rel)
        }
        return nil
    })
    if os.IsNotExist(err) {
        return nil, nil
    }
    return keys, err
}

func (s *LocalStore) Close() error {
    return nil
}
```

**Step 4: Run test to verify it passes**

Run: `task test:pkg -- ./workflow/store/...`
Expected: PASS

**Step 5: Commit**

```
feat(store): add LocalStore implementing RawStore for filesystem storage
```

---

### Task 4: S3Store (implements RawStore)

**Files:**
- Create: `workflow/store/s3.go`
- Create: `workflow/store/s3_integration_test.go`

Port from `workflow/artifacts/s3.go`. Adapt to `RawStore` interface (string keys, `[]string` list). Integration test only (requires S3/MinIO).

**Step 1: Write the integration test**

```go
// workflow/store/s3_integration_test.go
//go:build integration

package store_test

import (
    "context"
    "testing"

    "github.com/jasoet/go-wf/workflow/store"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestS3Store_RoundTrip(t *testing.T) {
    cfg := store.S3Config{
        Endpoint:  "http://localhost:9000",
        AccessKey: "minioadmin",
        SecretKey: "minioadmin",
        Bucket:    "test-store",
        Region:    "us-east-1",
    }

    ctx := context.Background()
    raw, err := store.NewS3Store(ctx, cfg)
    require.NoError(t, err)
    defer raw.Close()

    s := store.NewJSONStore[map[string]string](raw)

    data := map[string]string{"key": "value"}
    err = s.Save(ctx, "test/data.json", data)
    require.NoError(t, err)

    loaded, err := s.Load(ctx, "test/data.json")
    require.NoError(t, err)
    assert.Equal(t, data, loaded)

    err = s.Delete(ctx, "test/data.json")
    require.NoError(t, err)
}
```

**Step 2: Write implementation**

Port from `workflow/artifacts/s3.go`, adapting to RawStore interface. Key changes:
- `S3Config` same as current but no `ArtifactMetadata` dependency
- `Upload(ctx, key string, data io.Reader)` — key is used directly as object key (with optional prefix)
- `List` returns `[]string` keys
- Remove metadata-as-tags logic (not needed for raw store)

**Step 3: Run unit tests (S3 integration test requires infra)**

Run: `task test:pkg -- ./workflow/store/...`
Expected: PASS (unit tests only, integration skipped)

**Step 4: Commit**

```
feat(store): add S3Store implementing RawStore for S3-compatible storage
```

---

### Task 5: InstrumentedStore (OTel wrapper)

**Files:**
- Create: `workflow/store/otel.go`
- Create: `workflow/store/otel_test.go`

Port from `workflow/artifacts/otel.go`. Wraps `RawStore` with OTel spans and metrics.

**Step 1: Write test**

Test that InstrumentedStore delegates to inner store correctly (unit test with mock or real LocalStore).

**Step 2: Write implementation**

Port from `workflow/artifacts/otel.go`, adapting to `RawStore` interface. Same metrics: `go_wf.store.operation.total`, `go_wf.store.operation.duration`.

**Step 3: Run tests**

Run: `task test:pkg -- ./workflow/store/...`
Expected: PASS

**Step 4: Commit**

```
feat(store): add InstrumentedStore with OTel spans and metrics
```

---

## Phase 2: Core Workflow Types Update

Update `workflow/` types to carry both `I` and `O` type parameters. Update existing generic workflow functions accordingly.

### Task 6: Update Orchestration Input Types

**Files:**
- Modify: `workflow/types.go`
- Modify: `workflow/types_test.go` (if exists)

**Step 1: Update types to add O parameter**

Change all input types from `[I TaskInput]` to `[I TaskInput, O TaskOutput]`:

```go
type PipelineInput[I TaskInput, O TaskOutput] struct { ... }
type ParallelInput[I TaskInput, O TaskOutput] struct { ... }
type LoopInput[I TaskInput, O TaskOutput] struct { ... }
type ParameterizedLoopInput[I TaskInput, O TaskOutput] struct { ... }
```

Note: `O` is not used in the struct fields — it's a phantom type parameter that enables the compiler to enforce I/O type pairing at call sites.

**Step 2: Update workflow functions to use new types**

Modify: `workflow/execute.go`, `workflow/pipeline.go`, `workflow/parallel.go`, `workflow/loop.go`

Change signatures, e.g.:
```go
// Before:
func PipelineWorkflow[I TaskInput, O TaskOutput](ctx wf.Context, input PipelineInput[I]) (*PipelineOutput[O], error)

// After:
func PipelineWorkflow[I TaskInput, O TaskOutput](ctx wf.Context, input PipelineInput[I, O]) (*PipelineOutput[O], error)
```

Same for `ParallelWorkflow`, `LoopWorkflow`, `ParameterizedLoopWorkflow`.

**Step 3: Update OTel wrappers**

Modify: `workflow/otel.go` — same signature changes.

**Step 4: Update Substitutor type**

Already fine — `Substitutor[I TaskInput]` doesn't reference output type.

**Step 5: Fix all workflow tests**

Modify: `workflow/execute_test.go`, `workflow/pipeline_test.go`, `workflow/parallel_test.go`, `workflow/loop_test.go`, `workflow/otel_test.go`, `workflow/testutil_test.go`

Update all instantiations to provide both type params:
```go
// Before:
PipelineInput[*MockInput]{...}

// After:
PipelineInput[*MockInput, MockOutput]{...}
```

**Step 6: Run tests**

Run: `task test:pkg -- ./workflow/...`
Expected: PASS

**Step 7: Commit**

```
refactor(workflow): add output type parameter to orchestration input types
```

---

### Task 7: Add DAG Types to Core

**Files:**
- Create: `workflow/dag.go`
- Create: `workflow/dag_test.go`

Move shared DAG types to workflow core. The actual DAG execution logic stays in container/function for now (they have different artifact/data-passing needs), but the types are shared.

**Step 1: Define core DAG types**

```go
// workflow/dag.go
package workflow

import "time"

// DAGNode defines a single node in a DAG workflow.
type DAGNode[I TaskInput, O TaskOutput] struct {
    Name         string   `json:"name" validate:"required"`
    Input        I        `json:"input" validate:"required"`
    Dependencies []string `json:"dependencies,omitempty"`
}

// DAGInput defines a DAG workflow execution.
type DAGInput[I TaskInput, O TaskOutput] struct {
    Nodes       []DAGNode[I, O] `json:"nodes" validate:"required,min=1"`
    FailFast    bool            `json:"fail_fast"`
    MaxParallel int             `json:"max_parallel,omitempty"`
}

// Validate validates DAG input including cycle detection.
func (d *DAGInput[I, O]) Validate() error {
    // Validate nodes, check for cycles, validate dependencies exist
    // Port cycle detection from function/payload/payload_extended.go
}

// NodeResult holds the result of a single DAG node execution.
type NodeResult[O TaskOutput] struct {
    Name     string        `json:"name"`
    Result   *O            `json:"result,omitempty"`
    Error    string        `json:"error,omitempty"`
    Duration time.Duration `json:"duration"`
    Success  bool          `json:"success"`
}

// DAGOutput holds the results of a DAG workflow execution.
type DAGOutput[O TaskOutput] struct {
    Results       map[string]*O       `json:"results"`
    NodeResults   []NodeResult[O]     `json:"node_results"`
    TotalSuccess  int                 `json:"total_success"`
    TotalFailed   int                 `json:"total_failed"`
    TotalDuration time.Duration       `json:"total_duration"`
}
```

**Step 2: Write tests for Validate (cycle detection)**

**Step 3: Run tests**

Run: `task test:pkg -- ./workflow/...`
Expected: PASS

**Step 4: Commit**

```
feat(workflow): add shared DAG types with cycle detection
```

---

## Phase 3: Container Package Refactor

### Task 8: Move Container Types to Package Root

**Files:**
- Create: `container/types.go` (move from `container/payload/payloads.go`)
- Create: `container/types_extended.go` (move from `container/payload/payloads_extended.go`)
- Modify: All files importing `container/payload`

**Step 1: Create new type files**

Move `ContainerExecutionInput` → `ContainerInput`, `ContainerExecutionOutput` → `ContainerOutput` in `container/types.go`. Keep all validation logic.

Move extended types (DAG, conditional, resources, artifacts, secrets) to `container/types_extended.go`.

**Step 2: Update all internal imports**

Update `container/activity/`, `container/builder/`, `container/workflow/`, `container/template/`, `container/operations.go`, `container/worker.go` to import from `container` instead of `container/payload`.

**Step 3: Run tests**

Run: `task test:pkg -- ./container/...`
Expected: PASS

**Step 4: Commit**

```
refactor(container): move payload types to package root
```

---

### Task 9: Generic Container Builder

**Files:**
- Modify: `container/builder/builder.go`
- Modify: `container/builder/builder_test.go`

**Step 1: Make builder generic**

```go
type WorkflowBuilder[I workflow.TaskInput, O workflow.TaskOutput] struct {
    inputs      []I
    stopOnError bool
    cleanup     bool
    parallel    bool
    failFast    bool
    maxConcurrency int
    errors      []error
}

func NewWorkflowBuilder[I workflow.TaskInput, O workflow.TaskOutput]() *WorkflowBuilder[I, O]

func NewContainerBuilder() *WorkflowBuilder[container.ContainerInput, container.ContainerOutput]

func (b *WorkflowBuilder[I, O]) Add(input I) *WorkflowBuilder[I, O]
func (b *WorkflowBuilder[I, O]) BuildPipeline() (*workflow.PipelineInput[I, O], error)
func (b *WorkflowBuilder[I, O]) BuildParallel() (*workflow.ParallelInput[I, O], error)
func (b *WorkflowBuilder[I, O]) BuildSingle() (*I, error)
```

Remove `Build() (interface{}, error)` — replaced by typed BuildPipeline/BuildParallel/BuildSingle.

**Step 2: Make LoopBuilder generic**

```go
type LoopBuilder[I workflow.TaskInput, O workflow.TaskOutput] struct { ... }

func NewLoopBuilder[I workflow.TaskInput, O workflow.TaskOutput](items []string) *LoopBuilder[I, O]
func (b *LoopBuilder[I, O]) BuildLoop() (*workflow.LoopInput[I, O], error)
```

**Step 3: Update tests**

**Step 4: Run tests**

Run: `task test:pkg -- ./container/builder/...`
Expected: PASS

**Step 5: Commit**

```
refactor(container): make WorkflowBuilder and LoopBuilder generic
```

---

### Task 10: Container Workflow Thin Wrappers

**Files:**
- Modify: `container/workflow/container.go`
- Modify: `container/workflow/pipeline.go`
- Modify: `container/workflow/parallel.go`
- Modify: `container/workflow/loop.go`
- Delete: `container/workflow/helpers.go` (toTaskPtrs, toPipelineOutput, toParallelOutput no longer needed)
- Modify: `container/workflow/*_test.go`

**Step 1: Simplify workflow wrappers**

Container workflows now directly use generic core types — no more type conversion:

```go
// container/workflow/pipeline.go
func ContainerPipelineWorkflow(
    ctx wf.Context,
    input workflow.PipelineInput[*container.ContainerInput, container.ContainerOutput],
) (*workflow.PipelineOutput[container.ContainerOutput], error) {
    return generic.InstrumentedPipelineWorkflow[*container.ContainerInput, container.ContainerOutput](ctx, input)
}
```

No more `toTaskPtrs()`, `toPipelineOutput()`, `toParallelOutput()` — types flow through directly.

**Step 2: Update loop workflows**

Loop workflows pass the substitutor directly:

```go
func LoopWorkflow(
    ctx wf.Context,
    input workflow.LoopInput[*container.ContainerInput, container.ContainerOutput],
) (*workflow.LoopOutput[container.ContainerOutput], error) {
    return generic.InstrumentedLoopWorkflow[*container.ContainerInput, container.ContainerOutput](
        ctx, input, containerSubstitutor(),
    )
}
```

**Step 3: Delete helpers.go**

Remove `container/workflow/helpers.go` — all conversion functions are eliminated.

**Step 4: Update all container workflow tests**

Update test inputs to use `workflow.PipelineInput[*container.ContainerInput, container.ContainerOutput]` directly instead of `payload.PipelineInput`.

**Step 5: Run tests**

Run: `task test:pkg -- ./container/...`
Expected: PASS

**Step 6: Commit**

```
refactor(container): simplify workflows to thin generic wrappers, remove type conversion helpers
```

---

### Task 11: Generic Container Operations

**Files:**
- Modify: `container/operations.go`
- Modify: `container/operations_test.go` (if exists)

**Step 1: Make SubmitWorkflow generic**

```go
func SubmitWorkflow[I workflow.TaskInput](
    ctx context.Context, c client.Client,
    input I, taskQueue string, opts ...client.StartWorkflowOptions,
) (*WorkflowStatus, error)
```

Remove the `interface{}` type switch — the generic type enforces correctness.

**Step 2: Make SubmitAndWait generic**

```go
func SubmitAndWait[I workflow.TaskInput, O workflow.TaskOutput](
    ctx context.Context, c client.Client,
    input I, taskQueue string, timeout time.Duration,
) (*O, error)
```

**Step 3: Update WorkflowStatus**

```go
type WorkflowStatus struct {
    WorkflowID string
    RunID      string
    Status     string
    StartTime  time.Time
    CloseTime  *time.Time
    Error      error
}
```

Remove `Result interface{}` field — callers use `SubmitAndWait[I,O]` for typed results.

**Step 4: Run tests**

Run: `task test:pkg -- ./container/...`
Expected: PASS

**Step 5: Commit**

```
refactor(container): make operations generic, remove interface{} dispatch
```

---

### Task 12: Container Template Generic

**Files:**
- Modify: `container/template/container.go`
- Modify: `container/template/script.go`
- Modify: `container/template/http.go`

**Step 1: Make WorkflowSource generic**

```go
type WorkflowSource[I workflow.TaskInput] interface {
    ToInput() I
}
```

Container implements `WorkflowSource[container.ContainerInput]`.

**Step 2: Update worker registration**

Modify: `container/worker.go` — ensure all registration uses new types.

**Step 3: Run all container tests**

Run: `task test:pkg -- ./container/...`
Expected: PASS

**Step 4: Commit**

```
refactor(container): make templates generic with WorkflowSource[I]
```

---

### Task 13: Delete container/payload/ Package

**Files:**
- Delete: `container/payload/payloads.go`
- Delete: `container/payload/payloads_extended.go`
- Delete: `container/payload/` directory

**Step 1: Verify no remaining imports**

Search for `container/payload` in entire codebase. All should be gone after previous tasks.

**Step 2: Delete the package**

**Step 3: Run full test suite**

Run: `task test:unit`
Expected: PASS

**Step 4: Commit**

```
refactor(container): remove deprecated payload subpackage
```

---

## Phase 4: Function Package Refactor

Same pattern as container. Tasks mirror Phase 3.

### Task 14: Move Function Types to Package Root

**Files:**
- Create: `function/types.go` (from `function/payload/payload.go`)
- Create: `function/types_extended.go` (from `function/payload/payload_extended.go`)
- Modify: All files importing `function/payload`

Same approach as Task 8. Rename `FunctionExecutionInput` → `FunctionInput`, `FunctionExecutionOutput` → `FunctionOutput`.

**Commit:**

```
refactor(function): move payload types to package root
```

---

### Task 15: Generic Function Builder

**Files:**
- Modify: `function/builder/builder.go`
- Modify: `function/builder/builder_test.go`

Same approach as Task 9. `NewFunctionBuilder()` convenience constructor.

**Commit:**

```
refactor(function): make WorkflowBuilder and LoopBuilder generic
```

---

### Task 16: Function Workflow Thin Wrappers

**Files:**
- Modify: `function/workflow/function.go`
- Modify: `function/workflow/pipeline.go`
- Modify: `function/workflow/parallel.go`
- Modify: `function/workflow/loop.go`
- Delete: `function/workflow/helpers.go`
- Modify: `function/workflow/*_test.go`

Same approach as Task 10.

**Commit:**

```
refactor(function): simplify workflows to thin generic wrappers, remove type conversion helpers
```

---

### Task 17: Function Worker and Registry Update

**Files:**
- Modify: `function/worker.go`
- Move/keep: `function/registry.go` (the fn.Registry)

Update worker registration to use new type names. Ensure `RegisterActivity` uses generic activity type.

**Commit:**

```
refactor(function): update worker registration for generic types
```

---

### Task 18: Delete function/payload/ Package

Same approach as Task 13.

**Commit:**

```
refactor(function): remove deprecated payload subpackage
```

---

## Phase 5: Datasync Alignment

Minimal changes — datasync is already the gold standard.

### Task 19: Migrate Datasync to New Store

**Files:**
- Modify: `datasync/job.go` — change `ArtifactConfig *artifacts.ArtifactConfig` to `Store store.RawStore`
- Modify: `datasync/activity/sync.go` — update store usage
- Modify: `datasync/workflow/sync.go` — update store references
- Modify: All datasync tests referencing artifacts

**Step 1: Update Job struct**

```go
type Job[T, U any] struct {
    Name   string
    Source Source[T]
    Mapper Mapper[T, U]
    Sink   Sink[U]
    Schedule time.Duration
    // ... timeout fields ...
    Metadata any
    Store    store.RawStore // was: ArtifactConfig *artifacts.ArtifactConfig
}
```

**Step 2: Update activity and workflow code**

Anywhere that references `ArtifactConfig` or `artifacts.` — update to `store.`.

**Step 3: Run tests**

Run: `task test:pkg -- ./datasync/...`
Expected: PASS

**Step 4: Commit**

```
refactor(datasync): migrate from artifacts to store package
```

---

## Phase 6: Cleanup

### Task 20: Delete workflow/artifacts/ Package

**Files:**
- Delete: `workflow/artifacts/store.go`
- Delete: `workflow/artifacts/local.go`
- Delete: `workflow/artifacts/s3.go`
- Delete: `workflow/artifacts/otel.go`
- Delete: `workflow/artifacts/activities.go`
- Delete: `workflow/artifacts/helpers.go`
- Delete: `workflow/artifacts/` directory and all test files

**Step 1: Verify no remaining imports**

Search for `workflow/artifacts` in entire codebase.

**Step 2: Delete the package**

**Step 3: Run full test suite**

Run: `task test:unit`
Expected: PASS

**Step 4: Commit**

```
refactor(workflow): remove deprecated artifacts package, replaced by store
```

---

### Task 21: Update Examples

**Files:**
- Modify: `examples/container/*.go`
- Modify: `examples/function/*.go`
- Modify: `examples/trigger/*.go`

Update all examples to use new type names and import paths.

**Commit:**

```
docs(examples): update examples for generic redesign
```

---

### Task 22: Update Documentation

**Files:**
- Modify: `INSTRUCTION.md` — update key paths, architecture section
- Modify: `README.md` — update usage examples

**Commit:**

```
docs: update INSTRUCTION.md and README.md for generic redesign
```

---

### Task 23: Final Lint and Test Pass

**Step 1: Format all code**

Run: `task fmt`

**Step 2: Run linter**

Run: `task lint`
Fix any issues.

**Step 3: Run full test suite**

Run: `task test:unit`
Expected: PASS

**Step 4: Commit any fixes**

```
chore: fix lint issues from generic redesign
```

---

## Task Dependency Graph

```
Phase 1 (Store):       T1 → T2 → T3 → T4 → T5
Phase 2 (Core):        T6 → T7
Phase 3 (Container):   T8 → T9 → T10 → T11 → T12 → T13
Phase 4 (Function):    T14 → T15 → T16 → T17 → T18
Phase 5 (Datasync):    T19
Phase 6 (Cleanup):     T20 → T21 → T22 → T23

Dependencies across phases:
- Phase 2 depends on Phase 1 (types reference store)
- Phase 3 depends on Phase 2 (container uses updated core types)
- Phase 4 depends on Phase 2 (function uses updated core types)
- Phase 5 depends on Phase 1 (datasync uses new store)
- Phase 6 depends on Phase 3, 4, 5 (all consumers migrated)

Parallel opportunities:
- Phase 3 and Phase 4 can run in parallel (independent domains)
- Tasks 1-5 (store) can be batched
- Tasks 4-5 (S3, OTel) can be deferred if not blocking
```

## Verification Checklist

After all tasks complete:

- [ ] `task test:unit` — all unit tests pass
- [ ] `task lint` — no lint errors
- [ ] `task fmt` — no formatting changes
- [ ] No `interface{}` in public APIs (except `Job.Metadata: any` which is intentional)
- [ ] No `container/payload` or `function/payload` imports remain
- [ ] No `workflow/artifacts` imports remain
- [ ] All builder methods return typed results (no `interface{}`)
- [ ] All operations are generic (no type switches)
- [ ] `datasync/` still compiles and passes tests
- [ ] Examples compile and run
