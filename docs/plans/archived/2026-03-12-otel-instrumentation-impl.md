# OTel Instrumentation Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add comprehensive OpenTelemetry instrumentation to go-wf using `jasoet/pkg/v2/otel`, with three-signal correlation (traces, logs, metrics) and zero overhead when disabled.

**Architecture:** Wrapper pattern — each package gets an `otel.go` file with instrumented wrappers. Config flows via context (`otel.ContextWithConfig`). Activities get full spans + metrics; workflow orchestration gets structured logging only (Temporal `workflow.Context` limitation). Artifact store uses a decorator pattern.

**Tech Stack:** `github.com/jasoet/pkg/v2/otel` (Layers API, LogHelper, Field), OTel SDK metrics (via `Config.GetMeter`), Temporal SDK

**Design doc:** `docs/plans/2026-03-12-otel-instrumentation-design.md`

---

### Task 1: Add jasoet/pkg/v2/otel dependency

**Files:**
- Modify: `go.mod`

**Step 1: Check current jasoet/pkg version and add otel import**

The module `github.com/jasoet/pkg/v2 v2.8.8` is already in `go.mod`. The otel package is at `github.com/jasoet/pkg/v2/otel`. We just need to ensure it's importable — it should be since it's part of the same module.

Run: `task test:unit` to verify the project builds and tests pass before changes.

**Step 2: Verify otel package is accessible**

Create a temporary test to confirm the import works:

```bash
cd /Users/jasoet/Documents/Go/go-wf && go list github.com/jasoet/pkg/v2/otel
```

Expected: `github.com/jasoet/pkg/v2/otel` (no error)

If it fails, run `go get github.com/jasoet/pkg/v2@latest` to update.

**Step 3: Run tests to confirm no breakage**

Run: `task test:unit`
Expected: All tests pass.

---

### Task 2: Docker Activity Instrumentation (`docker/activity/otel.go`)

**Files:**
- Create: `docker/activity/otel.go`
- Create: `docker/activity/otel_test.go`
- Modify: `docker/worker.go:23-27` (register instrumented activity)

**Step 1: Write the test for instrumented activity**

Create `docker/activity/otel_test.go`:

```go
package activity

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jasoet/go-wf/docker/payload"
)

func TestInstrumentedStartContainerActivity_NilConfig(t *testing.T) {
	// When no OTel config is in context, the instrumented wrapper should
	// still call the underlying activity and return the same result.
	// We can't test real container execution in unit tests, but we can
	// verify the wrapper function signature and nil-config behavior.
	wrapped := InstrumentedStartContainerActivity(StartContainerActivity)
	assert.NotNil(t, wrapped)
}

func TestRecordDockerMetrics_NilConfig(t *testing.T) {
	// Verify metrics recording doesn't panic with nil config
	ctx := context.Background()
	assert.NotPanics(t, func() {
		recordDockerMetrics(ctx, "alpine", "success", 0, time.Second)
	})
}
```

**Step 2: Run test to verify it fails**

Run: `task test:pkg -- ./docker/activity/...`
Expected: FAIL — `InstrumentedStartContainerActivity` and `recordDockerMetrics` undefined.

**Step 3: Implement `docker/activity/otel.go`**

Create `docker/activity/otel.go`:

```go
package activity

import (
	"context"
	"strings"
	"time"

	pkgotel "github.com/jasoet/pkg/v2/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/jasoet/go-wf/docker/payload"
)

const (
	dockerMeterScope     = "go-wf/docker/activity"
	dockerTaskTotal      = "go_wf.docker.task.total"
	dockerTaskDuration   = "go_wf.docker.task.duration"
)

// InstrumentedStartContainerActivity wraps a container activity with OTel spans and metrics.
// When OTel config is not in context, the wrapper is a transparent pass-through with zero overhead.
func InstrumentedStartContainerActivity(
	inner func(ctx context.Context, input payload.ContainerExecutionInput) (*payload.ContainerExecutionOutput, error),
) func(ctx context.Context, input payload.ContainerExecutionInput) (*payload.ContainerExecutionOutput, error) {
	return func(ctx context.Context, input payload.ContainerExecutionInput) (*payload.ContainerExecutionOutput, error) {
		cfg := pkgotel.ConfigFromContext(ctx)
		if cfg == nil {
			return inner(ctx, input)
		}

		lc := pkgotel.Layers.StartService(ctx, "docker", "StartContainer",
			pkgotel.F("container.image", input.Image),
			pkgotel.F("container.name", input.Name),
			pkgotel.F("container.auto_remove", input.AutoRemove),
		)
		defer lc.End()

		if len(input.Command) > 0 {
			lc.Span.AddAttribute("container.command", strings.Join(input.Command, " "))
		}
		if input.WorkDir != "" {
			lc.Span.AddAttribute("container.work_dir", input.WorkDir)
		}

		output, err := inner(lc.Context(), input)

		if err != nil {
			lc.Error(err, "container execution failed")
			recordDockerMetrics(lc.Context(), input.Image, "failure", 0, time.Duration(0))
			return output, err
		}

		if output != nil {
			lc.Span.AddAttributes(
				pkgotel.F("container.id", output.ContainerID),
				pkgotel.F("container.exit_code", output.ExitCode),
				pkgotel.F("container.duration", output.Duration.String()),
			)
			if output.Endpoint != "" {
				lc.Span.AddAttribute("container.endpoint", output.Endpoint)
			}

			status := "success"
			if !output.Success {
				status = "failure"
			}
			recordDockerMetrics(lc.Context(), input.Image, status, output.ExitCode, output.Duration)

			if output.Success {
				lc.Success("container completed")
			} else {
				lc.Error(nil, "container exited with error",
					pkgotel.F("container.exit_code", output.ExitCode),
					pkgotel.F("container.error", output.Error),
				)
			}
		}

		return output, nil
	}
}

// imageBaseName extracts the image name without tag for low-cardinality metrics.
func imageBaseName(image string) string {
	// Remove tag (after last colon, but not in registry port)
	if idx := strings.LastIndex(image, ":"); idx > 0 {
		// Check if this colon is a tag separator (not a port in registry URL)
		afterColon := image[idx+1:]
		if !strings.Contains(afterColon, "/") {
			return image[:idx]
		}
	}
	return image
}

// recordDockerMetrics records counter and histogram metrics for docker task execution.
func recordDockerMetrics(ctx context.Context, image, status string, exitCode int, duration time.Duration) {
	cfg := pkgotel.ConfigFromContext(ctx)
	if cfg == nil {
		return
	}

	meter := cfg.GetMeter(dockerMeterScope)

	counter, err := meter.Int64Counter(dockerTaskTotal,
		metric.WithDescription("Total number of docker task executions"),
	)
	if err == nil {
		counter.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("status", status),
				attribute.String("image", imageBaseName(image)),
			),
		)
	}

	histogram, err := meter.Float64Histogram(dockerTaskDuration,
		metric.WithDescription("Duration of docker task executions in seconds"),
		metric.WithUnit("s"),
	)
	if err == nil {
		histogram.Record(ctx, duration.Seconds(),
			metric.WithAttributes(
				attribute.String("image", imageBaseName(image)),
				attribute.Int("exit_code", exitCode),
			),
		)
	}
}
```

**Step 4: Run test to verify it passes**

Run: `task test:pkg -- ./docker/activity/...`
Expected: PASS

**Step 5: Update worker registration to use instrumented activity**

Modify `docker/worker.go` — change `RegisterActivities` to wrap the activity:

```go
package docker

import (
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/worker"

	containerActivity "github.com/jasoet/go-wf/docker/activity"
	wf "github.com/jasoet/go-wf/docker/workflow"
)

// RegisterWorkflows registers all docker workflows with a worker.
func RegisterWorkflows(w worker.Worker) {
	w.RegisterWorkflow(wf.ExecuteContainerWorkflow)
	w.RegisterWorkflow(wf.ContainerPipelineWorkflow)
	w.RegisterWorkflow(wf.ParallelContainersWorkflow)
	w.RegisterWorkflow(wf.LoopWorkflow)
	w.RegisterWorkflow(wf.ParameterizedLoopWorkflow)
	w.RegisterWorkflow(wf.DAGWorkflow)
	w.RegisterWorkflow(wf.WorkflowWithParameters)
}

// RegisterActivities registers all docker activities with a worker.
// The activity is wrapped with OTel instrumentation that activates
// when otel.Config is present in the activity context.
func RegisterActivities(w worker.Worker) {
	instrumented := containerActivity.InstrumentedStartContainerActivity(containerActivity.StartContainerActivity)
	w.RegisterActivityWithOptions(instrumented, activity.RegisterOptions{
		Name: "StartContainerActivity",
	})
}

// RegisterAll registers both workflows and activities.
func RegisterAll(w worker.Worker) {
	RegisterWorkflows(w)
	RegisterActivities(w)
}
```

**Step 6: Run all unit tests**

Run: `task test:unit`
Expected: All tests pass.

**Step 7: Commit**

```
feat(docker): add OTel instrumentation for container activity

Adds span creation, metrics recording, and correlated logging for
StartContainerActivity using jasoet/pkg/v2/otel Layers API.
```

---

### Task 3: Function Activity Instrumentation (`function/activity/otel.go`)

**Files:**
- Create: `function/activity/otel.go`
- Create: `function/activity/otel_test.go`
- Modify: `function/worker.go:26-29` (register instrumented activity)

**Step 1: Write the test**

Create `function/activity/otel_test.go`:

```go
package activity

import (
	"context"
	"testing"
	"time"

	fn "github.com/jasoet/go-wf/function"
	"github.com/jasoet/go-wf/function/payload"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstrumentedExecuteFunctionActivity_NilConfig(t *testing.T) {
	registry := fn.NewRegistry()
	err := registry.Register("test-fn", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		return &fn.FunctionOutput{Result: map[string]string{"key": "value"}}, nil
	})
	require.NoError(t, err)

	inner := NewExecuteFunctionActivity(registry)
	wrapped := InstrumentedExecuteFunctionActivity(inner)
	assert.NotNil(t, wrapped)

	// Call without OTel config — should still work
	output, err := wrapped(context.Background(), payload.FunctionExecutionInput{
		Name: "test-fn",
	})
	require.NoError(t, err)
	assert.True(t, output.Success)
	assert.Equal(t, "value", output.Result["key"])
}

func TestRecordFunctionMetrics_NilConfig(t *testing.T) {
	ctx := context.Background()
	assert.NotPanics(t, func() {
		recordFunctionMetrics(ctx, "my-func", "success", time.Second)
	})
}
```

**Step 2: Run test to verify it fails**

Run: `task test:pkg -- ./function/activity/...`
Expected: FAIL — `InstrumentedExecuteFunctionActivity` and `recordFunctionMetrics` undefined.

**Step 3: Implement `function/activity/otel.go`**

Create `function/activity/otel.go`:

```go
package activity

import (
	"context"
	"time"

	pkgotel "github.com/jasoet/pkg/v2/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/jasoet/go-wf/function/payload"
)

const (
	functionMeterScope     = "go-wf/function/activity"
	functionTaskTotal      = "go_wf.function.task.total"
	functionTaskDuration   = "go_wf.function.task.duration"
)

// InstrumentedExecuteFunctionActivity wraps a function activity with OTel spans and metrics.
// When OTel config is not in context, the wrapper is a transparent pass-through with zero overhead.
func InstrumentedExecuteFunctionActivity(
	inner func(ctx context.Context, input payload.FunctionExecutionInput) (*payload.FunctionExecutionOutput, error),
) func(ctx context.Context, input payload.FunctionExecutionInput) (*payload.FunctionExecutionOutput, error) {
	return func(ctx context.Context, input payload.FunctionExecutionInput) (*payload.FunctionExecutionOutput, error) {
		cfg := pkgotel.ConfigFromContext(ctx)
		if cfg == nil {
			return inner(ctx, input)
		}

		lc := pkgotel.Layers.StartService(ctx, "function", "Execute",
			pkgotel.F("function.name", input.Name),
			pkgotel.F("function.has_data", len(input.Data) > 0),
		)
		defer lc.End()

		if input.WorkDir != "" {
			lc.Span.AddAttribute("function.work_dir", input.WorkDir)
		}

		output, err := inner(lc.Context(), input)

		if err != nil {
			lc.Error(err, "function execution failed")
			recordFunctionMetrics(lc.Context(), input.Name, "failure", time.Duration(0))
			return output, err
		}

		if output != nil {
			lc.Span.AddAttributes(
				pkgotel.F("function.duration", output.Duration.String()),
				pkgotel.F("function.has_result", len(output.Result) > 0),
			)

			status := "success"
			if !output.Success {
				status = "failure"
			}
			recordFunctionMetrics(lc.Context(), input.Name, status, output.Duration)

			if output.Success {
				lc.Success("function completed")
			} else {
				lc.Error(nil, "function returned error",
					pkgotel.F("function.error", output.Error),
				)
			}
		}

		return output, nil
	}
}

// recordFunctionMetrics records counter and histogram metrics for function execution.
func recordFunctionMetrics(ctx context.Context, name, status string, duration time.Duration) {
	cfg := pkgotel.ConfigFromContext(ctx)
	if cfg == nil {
		return
	}

	meter := cfg.GetMeter(functionMeterScope)

	counter, err := meter.Int64Counter(functionTaskTotal,
		metric.WithDescription("Total number of function task executions"),
	)
	if err == nil {
		counter.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("status", status),
				attribute.String("function_name", name),
			),
		)
	}

	histogram, err := meter.Float64Histogram(functionTaskDuration,
		metric.WithDescription("Duration of function task executions in seconds"),
		metric.WithUnit("s"),
	)
	if err == nil {
		histogram.Record(ctx, duration.Seconds(),
			metric.WithAttributes(
				attribute.String("function_name", name),
			),
		)
	}
}
```

**Step 4: Run test to verify it passes**

Run: `task test:pkg -- ./function/activity/...`
Expected: PASS

**Step 5: Update worker registration**

Modify `function/worker.go` — update `RegisterActivity` to wrap with instrumentation:

```go
package function

import (
	"go.temporal.io/sdk/activity"

	fnActivity "github.com/jasoet/go-wf/function/activity"
	wf "github.com/jasoet/go-wf/function/workflow"
)

// WorkflowRegistrar is the interface for registering workflows and activities.
type WorkflowRegistrar interface {
	RegisterWorkflow(w interface{})
	RegisterActivityWithOptions(a interface{}, options activity.RegisterOptions)
}

// RegisterWorkflows registers all function workflows.
func RegisterWorkflows(w WorkflowRegistrar) {
	w.RegisterWorkflow(wf.ExecuteFunctionWorkflow)
	w.RegisterWorkflow(wf.FunctionPipelineWorkflow)
	w.RegisterWorkflow(wf.ParallelFunctionsWorkflow)
	w.RegisterWorkflow(wf.LoopWorkflow)
	w.RegisterWorkflow(wf.ParameterizedLoopWorkflow)
}

// RegisterActivity registers a function execution activity.
// The activity is wrapped with OTel instrumentation that activates
// when otel.Config is present in the activity context.
// Create the activity with activity.NewExecuteFunctionActivity(registry).
func RegisterActivity(w WorkflowRegistrar, activityFn interface{}) {
	// Type-assert to wrap with instrumentation if possible
	if typed, ok := activityFn.(func(context.Context, payload.FunctionExecutionInput) (*payload.FunctionExecutionOutput, error)); ok {
		activityFn = fnActivity.InstrumentedExecuteFunctionActivity(typed)
	}
	w.RegisterActivityWithOptions(activityFn, activity.RegisterOptions{
		Name: "ExecuteFunctionActivity",
	})
}

// RegisterAll registers all function workflows and the given activity.
// Create the activity with activity.NewExecuteFunctionActivity(registry).
func RegisterAll(w WorkflowRegistrar, activityFn interface{}) {
	RegisterWorkflows(w)
	RegisterActivity(w, activityFn)
}
```

**Note:** The type assertion approach preserves backward compatibility — if the activity function doesn't match the expected signature (e.g., user passes something custom), it falls through without wrapping.

**Step 6: Run all unit tests**

Run: `task test:unit`
Expected: All tests pass.

**Step 7: Commit**

```
feat(function): add OTel instrumentation for function activity

Adds span creation, metrics recording, and correlated logging for
ExecuteFunctionActivity using jasoet/pkg/v2/otel Layers API.
```

---

### Task 4: Artifact Store Instrumentation (`workflow/artifacts/otel.go`)

**Files:**
- Create: `workflow/artifacts/otel.go`
- Create: `workflow/artifacts/otel_test.go`

**Step 1: Write the test**

Create `workflow/artifacts/otel_test.go`:

```go
package artifacts

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockStore implements ArtifactStore for testing.
type mockStore struct {
	uploadCalled   bool
	downloadCalled bool
	deleteCalled   bool
	existsCalled   bool
	listCalled     bool
}

func (m *mockStore) Upload(_ context.Context, _ ArtifactMetadata, _ io.Reader) error {
	m.uploadCalled = true
	return nil
}

func (m *mockStore) Download(_ context.Context, _ ArtifactMetadata) (io.ReadCloser, error) {
	m.downloadCalled = true
	return io.NopCloser(bytes.NewReader([]byte("test"))), nil
}

func (m *mockStore) Delete(_ context.Context, _ ArtifactMetadata) error {
	m.deleteCalled = true
	return nil
}

func (m *mockStore) Exists(_ context.Context, _ ArtifactMetadata) (bool, error) {
	m.existsCalled = true
	return true, nil
}

func (m *mockStore) List(_ context.Context, _ string) ([]ArtifactMetadata, error) {
	m.listCalled = true
	return []ArtifactMetadata{{Name: "test"}}, nil
}

func (m *mockStore) Close() error { return nil }

func TestInstrumentedStore_NilConfig(t *testing.T) {
	mock := &mockStore{}
	store := NewInstrumentedStore(mock)
	ctx := context.Background()

	metadata := ArtifactMetadata{
		Name:       "test-artifact",
		WorkflowID: "wf-1",
		RunID:      "run-1",
		StepName:   "step-1",
	}

	t.Run("Upload delegates to inner store", func(t *testing.T) {
		err := store.Upload(ctx, metadata, bytes.NewReader([]byte("data")))
		require.NoError(t, err)
		assert.True(t, mock.uploadCalled)
	})

	t.Run("Download delegates to inner store", func(t *testing.T) {
		reader, err := store.Download(ctx, metadata)
		require.NoError(t, err)
		assert.True(t, mock.downloadCalled)
		reader.Close()
	})

	t.Run("Delete delegates to inner store", func(t *testing.T) {
		err := store.Delete(ctx, metadata)
		require.NoError(t, err)
		assert.True(t, mock.deleteCalled)
	})

	t.Run("Exists delegates to inner store", func(t *testing.T) {
		exists, err := store.Exists(ctx, metadata)
		require.NoError(t, err)
		assert.True(t, exists)
		assert.True(t, mock.existsCalled)
	})

	t.Run("List delegates to inner store", func(t *testing.T) {
		items, err := store.List(ctx, "prefix")
		require.NoError(t, err)
		assert.Len(t, items, 1)
		assert.True(t, mock.listCalled)
	})

	t.Run("Close delegates to inner store", func(t *testing.T) {
		err := store.Close()
		require.NoError(t, err)
	})
}
```

**Step 2: Run test to verify it fails**

Run: `task test:pkg -- ./workflow/artifacts/...`
Expected: FAIL — `NewInstrumentedStore` undefined.

**Step 3: Implement `workflow/artifacts/otel.go`**

Create `workflow/artifacts/otel.go`:

```go
package artifacts

import (
	"context"
	"io"
	"time"

	pkgotel "github.com/jasoet/pkg/v2/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	artifactMeterScope        = "go-wf/artifacts"
	artifactOperationTotal    = "go_wf.artifact.operation.total"
	artifactOperationDuration = "go_wf.artifact.operation.duration"
)

// InstrumentedStore wraps an ArtifactStore with OTel spans and metrics.
// When OTel config is not in context, operations are transparent pass-throughs.
type InstrumentedStore struct {
	inner ArtifactStore
}

// NewInstrumentedStore creates a new InstrumentedStore wrapping the given store.
func NewInstrumentedStore(inner ArtifactStore) *InstrumentedStore {
	return &InstrumentedStore{inner: inner}
}

func (s *InstrumentedStore) Upload(ctx context.Context, metadata ArtifactMetadata, data io.Reader) error {
	cfg := pkgotel.ConfigFromContext(ctx)
	if cfg == nil {
		return s.inner.Upload(ctx, metadata, data)
	}

	start := time.Now()
	lc := pkgotel.Layers.StartRepository(ctx, "artifacts", "Upload",
		pkgotel.F("artifact.name", metadata.Name),
		pkgotel.F("artifact.type", metadata.Type),
		pkgotel.F("artifact.workflow_id", metadata.WorkflowID),
		pkgotel.F("artifact.step_name", metadata.StepName),
	)
	defer lc.End()

	err := s.inner.Upload(lc.Context(), metadata, data)

	if err != nil {
		lc.Error(err, "artifact upload failed")
		recordArtifactMetrics(lc.Context(), "upload", "failure", time.Since(start))
		return err
	}

	lc.Span.AddAttribute("artifact.size", metadata.Size)
	lc.Success("artifact uploaded")
	recordArtifactMetrics(lc.Context(), "upload", "success", time.Since(start))
	return nil
}

func (s *InstrumentedStore) Download(ctx context.Context, metadata ArtifactMetadata) (io.ReadCloser, error) {
	cfg := pkgotel.ConfigFromContext(ctx)
	if cfg == nil {
		return s.inner.Download(ctx, metadata)
	}

	start := time.Now()
	lc := pkgotel.Layers.StartRepository(ctx, "artifacts", "Download",
		pkgotel.F("artifact.name", metadata.Name),
		pkgotel.F("artifact.workflow_id", metadata.WorkflowID),
		pkgotel.F("artifact.step_name", metadata.StepName),
	)
	defer lc.End()

	reader, err := s.inner.Download(lc.Context(), metadata)

	if err != nil {
		lc.Error(err, "artifact download failed")
		recordArtifactMetrics(lc.Context(), "download", "failure", time.Since(start))
		return nil, err
	}

	lc.Success("artifact downloaded")
	recordArtifactMetrics(lc.Context(), "download", "success", time.Since(start))
	return reader, nil
}

func (s *InstrumentedStore) Delete(ctx context.Context, metadata ArtifactMetadata) error {
	cfg := pkgotel.ConfigFromContext(ctx)
	if cfg == nil {
		return s.inner.Delete(ctx, metadata)
	}

	start := time.Now()
	lc := pkgotel.Layers.StartRepository(ctx, "artifacts", "Delete",
		pkgotel.F("artifact.name", metadata.Name),
		pkgotel.F("artifact.workflow_id", metadata.WorkflowID),
		pkgotel.F("artifact.step_name", metadata.StepName),
	)
	defer lc.End()

	err := s.inner.Delete(lc.Context(), metadata)

	if err != nil {
		lc.Error(err, "artifact delete failed")
		recordArtifactMetrics(lc.Context(), "delete", "failure", time.Since(start))
		return err
	}

	lc.Success("artifact deleted")
	recordArtifactMetrics(lc.Context(), "delete", "success", time.Since(start))
	return nil
}

func (s *InstrumentedStore) Exists(ctx context.Context, metadata ArtifactMetadata) (bool, error) {
	cfg := pkgotel.ConfigFromContext(ctx)
	if cfg == nil {
		return s.inner.Exists(ctx, metadata)
	}

	start := time.Now()
	lc := pkgotel.Layers.StartRepository(ctx, "artifacts", "Exists",
		pkgotel.F("artifact.name", metadata.Name),
		pkgotel.F("artifact.workflow_id", metadata.WorkflowID),
	)
	defer lc.End()

	exists, err := s.inner.Exists(lc.Context(), metadata)

	if err != nil {
		lc.Error(err, "artifact exists check failed")
		recordArtifactMetrics(lc.Context(), "exists", "failure", time.Since(start))
		return false, err
	}

	lc.Span.AddAttribute("artifact.exists", exists)
	lc.Success("artifact exists check completed")
	recordArtifactMetrics(lc.Context(), "exists", "success", time.Since(start))
	return exists, nil
}

func (s *InstrumentedStore) List(ctx context.Context, prefix string) ([]ArtifactMetadata, error) {
	cfg := pkgotel.ConfigFromContext(ctx)
	if cfg == nil {
		return s.inner.List(ctx, prefix)
	}

	start := time.Now()
	lc := pkgotel.Layers.StartRepository(ctx, "artifacts", "List",
		pkgotel.F("artifact.prefix", prefix),
	)
	defer lc.End()

	items, err := s.inner.List(lc.Context(), prefix)

	if err != nil {
		lc.Error(err, "artifact list failed")
		recordArtifactMetrics(lc.Context(), "list", "failure", time.Since(start))
		return nil, err
	}

	lc.Span.AddAttribute("artifact.count", len(items))
	lc.Success("artifact list completed")
	recordArtifactMetrics(lc.Context(), "list", "success", time.Since(start))
	return items, nil
}

func (s *InstrumentedStore) Close() error {
	return s.inner.Close()
}

// recordArtifactMetrics records counter and histogram metrics for artifact operations.
func recordArtifactMetrics(ctx context.Context, operation, status string, duration time.Duration) {
	cfg := pkgotel.ConfigFromContext(ctx)
	if cfg == nil {
		return
	}

	meter := cfg.GetMeter(artifactMeterScope)

	counter, err := meter.Int64Counter(artifactOperationTotal,
		metric.WithDescription("Total number of artifact operations"),
	)
	if err == nil {
		counter.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("operation", operation),
				attribute.String("status", status),
			),
		)
	}

	histogram, err := meter.Float64Histogram(artifactOperationDuration,
		metric.WithDescription("Duration of artifact operations in seconds"),
		metric.WithUnit("s"),
	)
	if err == nil {
		histogram.Record(ctx, duration.Seconds(),
			metric.WithAttributes(
				attribute.String("operation", operation),
			),
		)
	}
}
```

**Step 4: Run test to verify it passes**

Run: `task test:pkg -- ./workflow/artifacts/...`
Expected: PASS

**Step 5: Commit**

```
feat(artifacts): add OTel instrumented artifact store decorator

Adds InstrumentedStore that wraps any ArtifactStore with spans,
metrics, and correlated logging using jasoet/pkg/v2/otel Layers API.
```

---

### Task 5: Workflow Orchestration Structured Logging (`workflow/otel.go`)

**Files:**
- Create: `workflow/otel.go`
- Modify: `docker/workflow/pipeline.go:18` (use instrumented wrapper)
- Modify: `docker/workflow/parallel.go:18` (use instrumented wrapper)
- Modify: `docker/workflow/loop.go:43-44,57-58` (use instrumented wrapper)
- Modify: `function/workflow/pipeline.go:17` (use instrumented wrapper)
- Modify: `function/workflow/parallel.go:18` (use instrumented wrapper)
- Modify: `function/workflow/loop.go:77-78,92-93` (use instrumented wrapper)

**Step 1: Write the test**

Create `workflow/otel_test.go`:

```go
package workflow

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
)

type OtelWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
}

func TestOtelWorkflowSuite(t *testing.T) {
	suite.Run(t, new(OtelWorkflowTestSuite))
}

func (s *OtelWorkflowTestSuite) TestInstrumentedPipelineWorkflow() {
	// Verify the instrumented wrapper logs and delegates to PipelineWorkflow.
	// Since we can't check OTel in workflow context, we verify behavior is unchanged.
	env := s.NewTestWorkflowEnvironment()
	env.RegisterActivity(mockActivity)

	input := PipelineInput[*mockInput]{
		Tasks:       []*mockInput{{Name: "step1"}, {Name: "step2"}},
		StopOnError: false,
	}

	env.ExecuteWorkflow(InstrumentedPipelineWorkflow[*mockInput, mockOutput], input)

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())

	var result PipelineOutput[mockOutput]
	s.NoError(env.GetWorkflowResult(&result))
	s.Equal(2, result.TotalSuccess)
	s.Equal(0, result.TotalFailed)
}

func (s *OtelWorkflowTestSuite) TestInstrumentedParallelWorkflow() {
	env := s.NewTestWorkflowEnvironment()
	env.RegisterActivity(mockActivity)

	input := ParallelInput[*mockInput]{
		Tasks: []*mockInput{{Name: "task1"}, {Name: "task2"}},
	}

	env.ExecuteWorkflow(InstrumentedParallelWorkflow[*mockInput, mockOutput], input)

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())

	var result ParallelOutput[mockOutput]
	s.NoError(env.GetWorkflowResult(&result))
	s.Equal(2, result.TotalSuccess)
}

// Test helpers — minimal mock types implementing TaskInput/TaskOutput.
type mockInput struct {
	Name string `json:"name" validate:"required"`
}

func (m *mockInput) Validate() error      { return nil }
func (m *mockInput) ActivityName() string  { return "mockActivity" }

type mockOutput struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

func (m mockOutput) IsSuccess() bool  { return m.Success }
func (m mockOutput) GetError() string { return m.Error }

func mockActivity(_ string) (mockOutput, error) {
	return mockOutput{Success: true}, nil
}
```

**Step 2: Run test to verify it fails**

Run: `task test:pkg -- ./workflow/...`
Expected: FAIL — `InstrumentedPipelineWorkflow` and `InstrumentedParallelWorkflow` undefined.

**Step 3: Implement `workflow/otel.go`**

Create `workflow/otel.go`:

```go
package workflow

import (
	"fmt"
	"time"

	wf "go.temporal.io/sdk/workflow"
)

// InstrumentedPipelineWorkflow wraps PipelineWorkflow with structured logging at boundaries.
func InstrumentedPipelineWorkflow[I TaskInput, O TaskOutput](ctx wf.Context, input PipelineInput[I]) (*PipelineOutput[O], error) {
	logger := wf.GetLogger(ctx)
	startTime := wf.Now(ctx)

	logger.Info("pipeline.start",
		"step_count", len(input.Tasks),
		"stop_on_error", input.StopOnError,
	)

	output, err := PipelineWorkflow[I, O](ctx, input)

	duration := wf.Now(ctx).Sub(startTime)

	if output != nil {
		logger.Info("pipeline.complete",
			"total_steps", len(input.Tasks),
			"success_count", output.TotalSuccess,
			"failure_count", output.TotalFailed,
			"duration", duration.String(),
		)
	} else if err != nil {
		logger.Error("pipeline.failed",
			"error", err,
			"duration", duration.String(),
		)
	}

	return output, err
}

// InstrumentedParallelWorkflow wraps ParallelWorkflow with structured logging at boundaries.
func InstrumentedParallelWorkflow[I TaskInput, O TaskOutput](ctx wf.Context, input ParallelInput[I]) (*ParallelOutput[O], error) {
	logger := wf.GetLogger(ctx)
	startTime := wf.Now(ctx)

	logger.Info("parallel.start",
		"task_count", len(input.Tasks),
		"max_concurrency", input.MaxConcurrency,
		"failure_strategy", input.FailureStrategy,
	)

	output, err := ParallelWorkflow[I, O](ctx, input)

	duration := wf.Now(ctx).Sub(startTime)

	if output != nil {
		logger.Info("parallel.complete",
			"total_tasks", len(input.Tasks),
			"success_count", output.TotalSuccess,
			"failure_count", output.TotalFailed,
			"duration", duration.String(),
		)
	} else if err != nil {
		logger.Error("parallel.failed",
			"error", err,
			"duration", duration.String(),
		)
	}

	return output, err
}

// InstrumentedLoopWorkflow wraps LoopWorkflow with structured logging at boundaries.
func InstrumentedLoopWorkflow[I TaskInput, O TaskOutput](ctx wf.Context, input LoopInput[I], substitutor Substitutor[I]) (*LoopOutput[O], error) {
	logger := wf.GetLogger(ctx)
	startTime := wf.Now(ctx)

	logger.Info("loop.start",
		"item_count", len(input.Items),
		"parallel", input.Parallel,
		"failure_strategy", input.FailureStrategy,
	)

	output, err := LoopWorkflow[I, O](ctx, input, substitutor)

	duration := wf.Now(ctx).Sub(startTime)

	if output != nil {
		logger.Info("loop.complete",
			"iterations", output.ItemCount,
			"success_count", output.TotalSuccess,
			"failure_count", output.TotalFailed,
			"duration", duration.String(),
		)
	} else if err != nil {
		logger.Error("loop.failed",
			"error", err,
			"duration", duration.String(),
		)
	}

	return output, err
}

// InstrumentedParameterizedLoopWorkflow wraps ParameterizedLoopWorkflow with structured logging.
func InstrumentedParameterizedLoopWorkflow[I TaskInput, O TaskOutput](ctx wf.Context, input ParameterizedLoopInput[I], substitutor Substitutor[I]) (*LoopOutput[O], error) {
	logger := wf.GetLogger(ctx)
	startTime := wf.Now(ctx)

	combinations := GenerateParameterCombinations(input.Parameters)
	logger.Info("parameterized_loop.start",
		"combination_count", len(combinations),
		"parallel", input.Parallel,
		"failure_strategy", input.FailureStrategy,
	)

	output, err := ParameterizedLoopWorkflow[I, O](ctx, input, substitutor)

	duration := wf.Now(ctx).Sub(startTime)

	if output != nil {
		logger.Info("parameterized_loop.complete",
			"iterations", output.ItemCount,
			"success_count", output.TotalSuccess,
			"failure_count", output.TotalFailed,
			"duration", duration.String(),
		)
	} else if err != nil {
		logger.Error("parameterized_loop.failed",
			"error", err,
			"duration", duration.String(),
		)
	}

	return output, err
}
```

**Step 4: Run test to verify it passes**

Run: `task test:pkg -- ./workflow/...`
Expected: PASS

**Step 5: Update docker workflow call sites**

Modify `docker/workflow/pipeline.go:18` — change:
```go
genericOutput, err := generic.PipelineWorkflow[*payload.ContainerExecutionInput, payload.ContainerExecutionOutput](ctx, genericInput)
```
to:
```go
genericOutput, err := generic.InstrumentedPipelineWorkflow[*payload.ContainerExecutionInput, payload.ContainerExecutionOutput](ctx, genericInput)
```

Modify `docker/workflow/parallel.go:18` — change:
```go
genericOutput, err := generic.ParallelWorkflow[*payload.ContainerExecutionInput, payload.ContainerExecutionOutput](ctx, genericInput)
```
to:
```go
genericOutput, err := generic.InstrumentedParallelWorkflow[*payload.ContainerExecutionInput, payload.ContainerExecutionOutput](ctx, genericInput)
```

Modify `docker/workflow/loop.go:43` — change:
```go
generic.LoopWorkflow[*payload.ContainerExecutionInput, payload.ContainerExecutionOutput](ctx, genericInput, containerSubstitutor()),
```
to:
```go
generic.InstrumentedLoopWorkflow[*payload.ContainerExecutionInput, payload.ContainerExecutionOutput](ctx, genericInput, containerSubstitutor()),
```

Modify `docker/workflow/loop.go:58` — change:
```go
generic.ParameterizedLoopWorkflow[*payload.ContainerExecutionInput, payload.ContainerExecutionOutput](ctx, genericInput, containerSubstitutor()),
```
to:
```go
generic.InstrumentedParameterizedLoopWorkflow[*payload.ContainerExecutionInput, payload.ContainerExecutionOutput](ctx, genericInput, containerSubstitutor()),
```

**Step 6: Update function workflow call sites**

Modify `function/workflow/pipeline.go:17` — change:
```go
genericOutput, err := generic.PipelineWorkflow[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput](ctx, genericInput)
```
to:
```go
genericOutput, err := generic.InstrumentedPipelineWorkflow[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput](ctx, genericInput)
```

Modify `function/workflow/parallel.go:18` — change:
```go
genericOutput, err := generic.ParallelWorkflow[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput](ctx, genericInput)
```
to:
```go
genericOutput, err := generic.InstrumentedParallelWorkflow[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput](ctx, genericInput)
```

Modify `function/workflow/loop.go:78` — change:
```go
generic.LoopWorkflow[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput](ctx, genericInput, functionSubstitutor()),
```
to:
```go
generic.InstrumentedLoopWorkflow[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput](ctx, genericInput, functionSubstitutor()),
```

Modify `function/workflow/loop.go:93` — change:
```go
generic.ParameterizedLoopWorkflow[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput](ctx, genericInput, functionSubstitutor()),
```
to:
```go
generic.InstrumentedParameterizedLoopWorkflow[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput](ctx, genericInput, functionSubstitutor()),
```

**Step 7: Run all unit tests**

Run: `task test:unit`
Expected: All tests pass.

**Step 8: Commit**

```
feat(workflow): add instrumented orchestration wrappers with structured logging

Adds InstrumentedPipelineWorkflow, InstrumentedParallelWorkflow,
InstrumentedLoopWorkflow, and InstrumentedParameterizedLoopWorkflow
with structured log events at workflow boundaries. Updates docker
and function workflow call sites to use instrumented versions.
```

---

### Task 6: Lint and Format

**Files:**
- All new and modified files

**Step 1: Format code**

Run: `task fmt`

**Step 2: Run linter**

Run: `task lint`

Fix any issues found.

**Step 3: Run full test suite**

Run: `task test:unit`
Expected: All tests pass, no lint errors.

**Step 4: Commit (if any formatting changes)**

```
style: format OTel instrumentation files
```

---

### Task 7: Update Documentation

**Files:**
- Modify: `INSTRUCTION.md`
- Modify: `README.md`

**Step 1: Update INSTRUCTION.md**

Add to the Architecture section a note about OTel instrumentation:

> **Observability:** OTel instrumentation via `jasoet/pkg/v2/otel`. Activities get full spans + metrics (Layers.StartService). Workflow orchestration has structured logging wrappers. Artifact store uses an InstrumentedStore decorator (Layers.StartRepository). All instrumentation is opt-in via `otel.ContextWithConfig()` — zero overhead when disabled.

Add to the Key Paths table:
| `workflow/otel.go` | Instrumented workflow orchestration wrappers |
| `docker/activity/otel.go` | Docker activity OTel spans + metrics |
| `function/activity/otel.go` | Function activity OTel spans + metrics |
| `workflow/artifacts/otel.go` | Instrumented artifact store decorator |

**Step 2: Update README.md**

Add an "Observability" section under Features explaining:
- OTel integration via `jasoet/pkg/v2/otel`
- Three-signal correlation (traces, logs, metrics)
- Zero overhead when disabled
- Consumer integration snippet (the 4-line setup from design doc section 6)

**Step 3: Commit**

```
docs: add OTel instrumentation documentation
```

---

### Task 8: Archive design doc

**Files:**
- Move: `docs/plans/2026-03-12-otel-instrumentation-design.md` → `docs/plans/archived/`
- Move: `docs/plans/2026-03-12-otel-instrumentation-impl.md` → `docs/plans/archived/`

**Step 1: Archive**

```bash
mv docs/plans/2026-03-12-otel-instrumentation-design.md docs/plans/archived/
mv docs/plans/2026-03-12-otel-instrumentation-impl.md docs/plans/archived/
```

**Step 2: Commit**

```
chore: archive OTel instrumentation plans
```
