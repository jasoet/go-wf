# Test Coverage Improvement Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Bring all packages to 85%+ test coverage by testing OTel instrumentation with-config code paths.

**Architecture:** Add unit tests using no-op OTel SDK providers (`sdktrace.NewTracerProvider()`, `sdkmetric.NewMeterProvider()`) to exercise the with-config branches. Tests use mock inner functions/stores to verify delegation and error propagation through OTel instrumentation layers.

**Tech Stack:** Go 1.26+, testify, OTel SDK (`go.opentelemetry.io/otel/sdk`), `github.com/jasoet/pkg/v2/otel`, Temporal test suite

---

### Task 1: function/activity — OTel with-config tests

**Files:**
- Modify: `function/activity/otel_test.go`

**Step 1: Write the failing tests**

Add these tests to `function/activity/otel_test.go`:

```go
import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pkgotel "github.com/jasoet/pkg/v2/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	fn "github.com/jasoet/go-wf/function"
	"github.com/jasoet/go-wf/function/payload"
)

// otelContext creates a context with a minimal OTel config using no-op providers.
func otelContext() context.Context {
	cfg := pkgotel.NewConfig("test-service").
		WithTracerProvider(sdktrace.NewTracerProvider()).
		WithMeterProvider(sdkmetric.NewMeterProvider()).
		WithoutLogging()
	return pkgotel.ContextWithConfig(context.Background(), cfg)
}

func TestInstrumentedExecuteFunctionActivity_WithOTelConfig_Success(t *testing.T) {
	registry := fn.NewRegistry()
	registry.Register("echo", func(_ context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		return &fn.FunctionOutput{
			Result: map[string]string{"echoed": "true"},
		}, nil
	})

	activityFn := NewExecuteFunctionActivity(registry)
	wrapped := InstrumentedExecuteFunctionActivity(activityFn)

	ctx := otelContext()
	input := payload.FunctionExecutionInput{
		Name:    "echo",
		WorkDir: "/tmp",
	}

	output, err := wrapped(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, output)
	assert.True(t, output.Success)
	assert.Equal(t, "echo", output.Name)
	assert.Equal(t, "true", output.Result["echoed"])
}

func TestInstrumentedExecuteFunctionActivity_WithOTelConfig_HandlerError(t *testing.T) {
	registry := fn.NewRegistry()
	registry.Register("fail", func(_ context.Context, _ fn.FunctionInput) (*fn.FunctionOutput, error) {
		return nil, fmt.Errorf("handler failed")
	})

	activityFn := NewExecuteFunctionActivity(registry)
	wrapped := InstrumentedExecuteFunctionActivity(activityFn)

	ctx := otelContext()
	input := payload.FunctionExecutionInput{
		Name:    "fail",
		WorkDir: "/tmp",
	}

	output, err := wrapped(ctx, input)

	// Handler errors are captured in output, not returned as activity errors
	require.NoError(t, err)
	require.NotNil(t, output)
	assert.False(t, output.Success)
	assert.Contains(t, output.Error, "handler failed")
}

func TestInstrumentedExecuteFunctionActivity_WithOTelConfig_ActivityError(t *testing.T) {
	// Activity-level errors (validation, registry lookup) ARE returned as errors
	registry := fn.NewRegistry()
	activityFn := NewExecuteFunctionActivity(registry)
	wrapped := InstrumentedExecuteFunctionActivity(activityFn)

	ctx := otelContext()
	input := payload.FunctionExecutionInput{
		Name:    "missing",
		WorkDir: "/tmp",
	}

	output, err := wrapped(ctx, input)

	require.Error(t, err)
	require.NotNil(t, output)
	assert.False(t, output.Success)
}

func TestInstrumentedExecuteFunctionActivity_WithOTelConfig_NilOutput(t *testing.T) {
	// Test with an inner function that returns nil output (edge case)
	inner := func(_ context.Context, _ payload.FunctionExecutionInput) (*payload.FunctionExecutionOutput, error) {
		return nil, nil
	}

	wrapped := InstrumentedExecuteFunctionActivity(inner)
	ctx := otelContext()
	input := payload.FunctionExecutionInput{
		Name:    "noop",
		WorkDir: "/tmp",
	}

	assert.NotPanics(t, func() {
		output, err := wrapped(ctx, input)
		assert.NoError(t, err)
		assert.Nil(t, output)
	})
}

func TestRecordFunctionMetrics_WithConfig(t *testing.T) {
	ctx := otelContext()
	assert.NotPanics(t, func() {
		recordFunctionMetrics(ctx, "test-fn", "success", 5*time.Second)
	})
	assert.NotPanics(t, func() {
		recordFunctionMetrics(ctx, "test-fn", "failure", time.Duration(0))
	})
}
```

**Step 2: Run tests to verify they fail (or pass if code path works)**

Run: `task test:pkg -- ./function/activity/...`
Expected: Tests compile and run. Some may pass immediately (the OTel code is already implemented), which is fine — the goal is coverage.

**Step 3: Check coverage improvement**

Run: `task test:coverage -- ./function/activity/...`
Expected: Coverage should jump from 53.8% to 85%+

**Step 4: Commit**

```
git add function/activity/otel_test.go
git commit -m "test(function/activity): add OTel with-config coverage tests"
```

---

### Task 2: docker/activity — OTel with-config tests

**Files:**
- Modify: `docker/activity/otel_test.go`

**Step 1: Write the failing tests**

Add these tests to `docker/activity/otel_test.go`:

```go
import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pkgotel "github.com/jasoet/pkg/v2/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/jasoet/go-wf/docker/payload"
)

// otelContext creates a context with a minimal OTel config using no-op providers.
func otelContext() context.Context {
	cfg := pkgotel.NewConfig("test-service").
		WithTracerProvider(sdktrace.NewTracerProvider()).
		WithMeterProvider(sdkmetric.NewMeterProvider()).
		WithoutLogging()
	return pkgotel.ContextWithConfig(context.Background(), cfg)
}

func TestInstrumentedStartContainerActivity_WithOTelConfig_Success(t *testing.T) {
	expectedOutput := &payload.ContainerExecutionOutput{
		ContainerID: "abc-123",
		Success:     true,
		ExitCode:    0,
		Duration:    5 * time.Second,
		Endpoint:    "localhost:8080",
	}

	inner := func(_ context.Context, _ payload.ContainerExecutionInput) (*payload.ContainerExecutionOutput, error) {
		return expectedOutput, nil
	}

	wrapped := InstrumentedStartContainerActivity(inner)
	ctx := otelContext()
	input := payload.ContainerExecutionInput{
		Image:   "alpine:latest",
		Command: []string{"echo", "hello"},
		WorkDir: "/app",
	}

	output, err := wrapped(ctx, input)

	require.NoError(t, err)
	assert.Equal(t, expectedOutput, output)
}

func TestInstrumentedStartContainerActivity_WithOTelConfig_SuccessNoEndpoint(t *testing.T) {
	expectedOutput := &payload.ContainerExecutionOutput{
		ContainerID: "abc-123",
		Success:     true,
		ExitCode:    0,
		Duration:    2 * time.Second,
	}

	inner := func(_ context.Context, _ payload.ContainerExecutionInput) (*payload.ContainerExecutionOutput, error) {
		return expectedOutput, nil
	}

	wrapped := InstrumentedStartContainerActivity(inner)
	ctx := otelContext()
	input := payload.ContainerExecutionInput{
		Image: "alpine:latest",
	}

	output, err := wrapped(ctx, input)

	require.NoError(t, err)
	assert.Equal(t, expectedOutput, output)
}

func TestInstrumentedStartContainerActivity_WithOTelConfig_Failure(t *testing.T) {
	expectedOutput := &payload.ContainerExecutionOutput{
		ContainerID: "abc-123",
		Success:     false,
		ExitCode:    1,
		Error:       "command failed",
		Duration:    3 * time.Second,
	}

	inner := func(_ context.Context, _ payload.ContainerExecutionInput) (*payload.ContainerExecutionOutput, error) {
		return expectedOutput, nil
	}

	wrapped := InstrumentedStartContainerActivity(inner)
	ctx := otelContext()
	input := payload.ContainerExecutionInput{
		Image: "alpine:latest",
	}

	output, err := wrapped(ctx, input)

	require.NoError(t, err)
	assert.Equal(t, expectedOutput, output)
}

func TestInstrumentedStartContainerActivity_WithOTelConfig_Error(t *testing.T) {
	inner := func(_ context.Context, _ payload.ContainerExecutionInput) (*payload.ContainerExecutionOutput, error) {
		return nil, fmt.Errorf("container start failed")
	}

	wrapped := InstrumentedStartContainerActivity(inner)
	ctx := otelContext()
	input := payload.ContainerExecutionInput{
		Image: "alpine:latest",
	}

	output, err := wrapped(ctx, input)

	require.Error(t, err)
	assert.Nil(t, output)
	assert.Contains(t, err.Error(), "container start failed")
}

func TestRecordDockerMetrics_WithConfig(t *testing.T) {
	ctx := otelContext()
	assert.NotPanics(t, func() {
		recordDockerMetrics(ctx, "alpine:3.18", "success", 0, 5*time.Second)
	})
	assert.NotPanics(t, func() {
		recordDockerMetrics(ctx, "nginx:latest", "failure", 1, time.Duration(0))
	})
}

func TestImageBaseName_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		image    string
		expected string
	}{
		{
			name:     "empty string",
			image:    "",
			expected: "",
		},
		{
			name:     "just a tag separator",
			image:    ":latest",
			expected: ":latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := imageBaseName(tt.image)
			assert.Equal(t, tt.expected, result)
		})
	}
}
```

**Step 2: Run tests**

Run: `task test:pkg -- ./docker/activity/...`
Expected: All tests pass.

**Step 3: Check coverage**

Run: `task test:coverage -- ./docker/activity/...`
Expected: Coverage should jump from 56.8% to 85%+

**Step 4: Commit**

```
git add docker/activity/otel_test.go
git commit -m "test(docker/activity): add OTel with-config coverage tests"
```

---

### Task 3: workflow/artifacts — OTel with-config tests

**Files:**
- Modify: `workflow/artifacts/otel_test.go`

**Step 1: Write the failing tests**

Add these tests to `workflow/artifacts/otel_test.go`. First, extend `mockStore` to support configurable errors:

```go
import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pkgotel "github.com/jasoet/pkg/v2/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// otelContext creates a context with a minimal OTel config using no-op providers.
func otelContext() context.Context {
	cfg := pkgotel.NewConfig("test-service").
		WithTracerProvider(sdktrace.NewTracerProvider()).
		WithMeterProvider(sdkmetric.NewMeterProvider()).
		WithoutLogging()
	return pkgotel.ContextWithConfig(context.Background(), cfg)
}

// errorStore is a mock store that returns errors for all operations.
type errorStore struct {
	err error
}

func (e *errorStore) Upload(_ context.Context, _ ArtifactMetadata, _ io.Reader) error   { return e.err }
func (e *errorStore) Download(_ context.Context, _ ArtifactMetadata) (io.ReadCloser, error) { return nil, e.err }
func (e *errorStore) Delete(_ context.Context, _ ArtifactMetadata) error                { return e.err }
func (e *errorStore) Exists(_ context.Context, _ ArtifactMetadata) (bool, error)        { return false, e.err }
func (e *errorStore) List(_ context.Context, _ string) ([]ArtifactMetadata, error)      { return nil, e.err }
func (e *errorStore) Close() error                                                       { return e.err }

// --- Success path tests with OTel config ---

func TestInstrumentedStore_Upload_WithOTelConfig(t *testing.T) {
	mock := &mockStore{}
	store := NewInstrumentedStore(mock)
	ctx := otelContext()

	err := store.Upload(ctx, testMetadata(), bytes.NewReader([]byte("data")))

	require.NoError(t, err)
	assert.True(t, mock.uploadCalled)
}

func TestInstrumentedStore_Download_WithOTelConfig(t *testing.T) {
	mock := &mockStore{}
	store := NewInstrumentedStore(mock)
	ctx := otelContext()

	reader, err := store.Download(ctx, testMetadata())

	require.NoError(t, err)
	assert.NotNil(t, reader)
	assert.True(t, mock.downloadCalled)
	reader.Close()
}

func TestInstrumentedStore_Delete_WithOTelConfig(t *testing.T) {
	mock := &mockStore{}
	store := NewInstrumentedStore(mock)
	ctx := otelContext()

	err := store.Delete(ctx, testMetadata())

	require.NoError(t, err)
	assert.True(t, mock.deleteCalled)
}

func TestInstrumentedStore_Exists_WithOTelConfig(t *testing.T) {
	mock := &mockStore{}
	store := NewInstrumentedStore(mock)
	ctx := otelContext()

	exists, err := store.Exists(ctx, testMetadata())

	require.NoError(t, err)
	assert.True(t, exists)
	assert.True(t, mock.existsCalled)
}

func TestInstrumentedStore_List_WithOTelConfig(t *testing.T) {
	mock := &mockStore{}
	store := NewInstrumentedStore(mock)
	ctx := otelContext()

	items, err := store.List(ctx, "prefix/")

	require.NoError(t, err)
	assert.Len(t, items, 1)
	assert.True(t, mock.listCalled)
}

// --- Error path tests with OTel config ---

func TestInstrumentedStore_Upload_WithOTelConfig_Error(t *testing.T) {
	store := NewInstrumentedStore(&errorStore{err: fmt.Errorf("upload failed")})
	ctx := otelContext()

	err := store.Upload(ctx, testMetadata(), bytes.NewReader([]byte("data")))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "upload failed")
}

func TestInstrumentedStore_Download_WithOTelConfig_Error(t *testing.T) {
	store := NewInstrumentedStore(&errorStore{err: fmt.Errorf("download failed")})
	ctx := otelContext()

	reader, err := store.Download(ctx, testMetadata())

	require.Error(t, err)
	assert.Nil(t, reader)
}

func TestInstrumentedStore_Delete_WithOTelConfig_Error(t *testing.T) {
	store := NewInstrumentedStore(&errorStore{err: fmt.Errorf("delete failed")})
	ctx := otelContext()

	err := store.Delete(ctx, testMetadata())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "delete failed")
}

func TestInstrumentedStore_Exists_WithOTelConfig_Error(t *testing.T) {
	store := NewInstrumentedStore(&errorStore{err: fmt.Errorf("exists failed")})
	ctx := otelContext()

	exists, err := store.Exists(ctx, testMetadata())

	require.Error(t, err)
	assert.False(t, exists)
}

func TestInstrumentedStore_List_WithOTelConfig_Error(t *testing.T) {
	store := NewInstrumentedStore(&errorStore{err: fmt.Errorf("list failed")})
	ctx := otelContext()

	items, err := store.List(ctx, "prefix/")

	require.Error(t, err)
	assert.Nil(t, items)
}

func TestRecordArtifactMetrics_WithConfig(t *testing.T) {
	ctx := otelContext()
	operations := []string{"Upload", "Download", "Delete", "Exists", "List"}
	for _, op := range operations {
		assert.NotPanics(t, func() {
			recordArtifactMetrics(ctx, op, "success", time.Second)
		})
		assert.NotPanics(t, func() {
			recordArtifactMetrics(ctx, op, "failure", time.Duration(0))
		})
	}
}
```

**Step 2: Run tests**

Run: `task test:pkg -- ./workflow/artifacts/...`
Expected: All tests pass.

**Step 3: Check coverage**

Run: `task test:coverage -- ./workflow/artifacts/...`
Expected: Coverage should jump from 65.7% to 85%+

**Step 4: Commit**

```
git add workflow/artifacts/otel_test.go
git commit -m "test(artifacts): add OTel with-config and error path coverage tests"
```

---

### Task 4: workflow — Instrumented loop and parameterized loop tests

**Files:**
- Modify: `workflow/otel_test.go`

**Step 1: Write the failing tests**

Add these tests to `workflow/otel_test.go`. Add two wrapper functions and four test methods to the existing `OtelWorkflowTestSuite`:

```go
// Add wrapper functions after the existing ones (after line 31):

// instrumentedLoopWrapper is a non-generic workflow wrapper for testing.
func instrumentedLoopWrapper(ctx wf.Context, input LoopInput[testInput]) (*LoopOutput[testOutput], error) {
	substitutor := func(template testInput, item string, index int, params map[string]string) testInput {
		return testInput{
			Name:     fmt.Sprintf("%s-%s", template.Name, item),
			Value:    item,
			Activity: template.Activity,
		}
	}
	return InstrumentedLoopWorkflow[testInput, testOutput](ctx, input, substitutor)
}

// instrumentedParameterizedLoopWrapper is a non-generic workflow wrapper for testing.
func instrumentedParameterizedLoopWrapper(ctx wf.Context, input ParameterizedLoopInput[testInput]) (*LoopOutput[testOutput], error) {
	substitutor := func(template testInput, _ string, _ int, params map[string]string) testInput {
		value := template.Value
		for k, v := range params {
			value += fmt.Sprintf("-%s=%s", k, v)
		}
		return testInput{
			Name:     template.Name,
			Value:    value,
			Activity: template.Activity,
		}
	}
	return InstrumentedParameterizedLoopWorkflow[testInput, testOutput](ctx, input, substitutor)
}

// Add test methods to OtelWorkflowTestSuite:

func (s *OtelWorkflowTestSuite) TestInstrumentedLoopWorkflow_Sequential_Success() {
	env := s.NewTestWorkflowEnvironment()
	registerTestActivity(env)

	input := LoopInput[testInput]{
		Items:    []string{"a", "b", "c"},
		Template: testInput{Name: "step", Value: "template", Activity: "TestActivity"},
		Parallel: false,
	}

	env.OnActivity("TestActivity", mock.Anything, mock.Anything).Return(
		&testOutput{Result: "ok", Success: true}, nil)

	env.ExecuteWorkflow(instrumentedLoopWrapper, input)

	require.True(s.T(), env.IsWorkflowCompleted())
	require.NoError(s.T(), env.GetWorkflowError())

	var result LoopOutput[testOutput]
	require.NoError(s.T(), env.GetWorkflowResult(&result))
	assert.Equal(s.T(), 3, result.TotalSuccess)
	assert.Equal(s.T(), 0, result.TotalFailed)
	assert.Len(s.T(), result.Results, 3)
}

func (s *OtelWorkflowTestSuite) TestInstrumentedLoopWorkflow_Parallel_Success() {
	env := s.NewTestWorkflowEnvironment()
	registerTestActivity(env)

	input := LoopInput[testInput]{
		Items:    []string{"x", "y"},
		Template: testInput{Name: "task", Value: "template", Activity: "TestActivity"},
		Parallel: true,
	}

	env.OnActivity("TestActivity", mock.Anything, mock.Anything).Return(
		&testOutput{Result: "ok", Success: true}, nil)

	env.ExecuteWorkflow(instrumentedLoopWrapper, input)

	require.True(s.T(), env.IsWorkflowCompleted())
	require.NoError(s.T(), env.GetWorkflowError())

	var result LoopOutput[testOutput]
	require.NoError(s.T(), env.GetWorkflowResult(&result))
	assert.Equal(s.T(), 2, result.TotalSuccess)
	assert.Equal(s.T(), 0, result.TotalFailed)
}

func (s *OtelWorkflowTestSuite) TestInstrumentedLoopWorkflow_FailFast() {
	env := s.NewTestWorkflowEnvironment()
	registerTestActivity(env)

	input := LoopInput[testInput]{
		Items:           []string{"a", "b"},
		Template:        testInput{Name: "step", Value: "template", Activity: "TestActivity"},
		Parallel:        false,
		FailureStrategy: "fail_fast",
	}

	env.OnActivity("TestActivity", mock.Anything, mock.Anything).Return(
		&testOutput{Result: "fail", Success: false}, nil)

	env.ExecuteWorkflow(instrumentedLoopWrapper, input)

	require.True(s.T(), env.IsWorkflowCompleted())
	assert.Error(s.T(), env.GetWorkflowError())
}

func (s *OtelWorkflowTestSuite) TestInstrumentedParameterizedLoopWorkflow_Success() {
	env := s.NewTestWorkflowEnvironment()
	registerTestActivity(env)

	input := ParameterizedLoopInput[testInput]{
		Parameters: map[string][]string{
			"env":  {"dev", "prod"},
			"arch": {"amd64"},
		},
		Template: testInput{Name: "build", Value: "base", Activity: "TestActivity"},
		Parallel: false,
	}

	env.OnActivity("TestActivity", mock.Anything, mock.Anything).Return(
		&testOutput{Result: "ok", Success: true}, nil)

	env.ExecuteWorkflow(instrumentedParameterizedLoopWrapper, input)

	require.True(s.T(), env.IsWorkflowCompleted())
	require.NoError(s.T(), env.GetWorkflowError())

	var result LoopOutput[testOutput]
	require.NoError(s.T(), env.GetWorkflowResult(&result))
	assert.Equal(s.T(), 2, result.TotalSuccess) // 2 env * 1 arch = 2 combinations
	assert.Equal(s.T(), 0, result.TotalFailed)
}

func (s *OtelWorkflowTestSuite) TestInstrumentedParameterizedLoopWorkflow_FailFast() {
	env := s.NewTestWorkflowEnvironment()
	registerTestActivity(env)

	input := ParameterizedLoopInput[testInput]{
		Parameters: map[string][]string{
			"env": {"dev", "prod"},
		},
		Template:        testInput{Name: "deploy", Value: "base", Activity: "TestActivity"},
		Parallel:        false,
		FailureStrategy: "fail_fast",
	}

	env.OnActivity("TestActivity", mock.Anything, mock.Anything).Return(
		&testOutput{Result: "fail", Success: false}, nil)

	env.ExecuteWorkflow(instrumentedParameterizedLoopWrapper, input)

	require.True(s.T(), env.IsWorkflowCompleted())
	assert.Error(s.T(), env.GetWorkflowError())
}
```

**Step 2: Run tests**

Run: `task test:pkg -- ./workflow/...`
Expected: All tests pass.

**Step 3: Check coverage**

Run: `task test:coverage -- ./workflow/...`
Expected: Coverage should reach 85%+

**Step 4: Commit**

```
git add workflow/otel_test.go
git commit -m "test(workflow): add instrumented loop and parameterized loop coverage tests"
```

---

### Task 5: Verify all coverage targets met

**Step 1: Run full test suite with coverage**

Run: `task test`
Expected: All packages at 85%+:
- `function/activity` ≥ 85%
- `docker/activity` ≥ 85%
- `workflow/artifacts` ≥ 85%
- `workflow` ≥ 85%

**Step 2: Run linter**

Run: `task lint`
Expected: No new lint issues.

**Step 3: If any package is below 85%, identify remaining gaps**

Run: `task test:coverage -- ./package/path/...`
Review the HTML report to find remaining uncovered lines and add targeted tests.

**Step 4: Final commit if needed**

```
git add -A
git commit -m "test: final coverage adjustments to meet 85% target"
```
