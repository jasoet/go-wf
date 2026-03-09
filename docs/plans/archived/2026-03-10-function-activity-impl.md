# Function Activity Module Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a `function/` module parallel to `docker/` that lets users register named Go functions and orchestrate them via the generic workflow core.

**Architecture:** Function registry maps string names to Go handler functions. A single Temporal activity (`ExecuteFunctionActivity`) dispatches to the registered handler. Thin workflow wrappers delegate to the generic core. Builder API provides fluent composition.

**Tech Stack:** Go 1.26+, Temporal SDK, validator/v10, testify

---

### Task 1: Registry — Types and Core

**Files:**
- Create: `function/registry.go`
- Test: `function/registry_test.go`

**Step 1: Write the failing test**

```go
// function/registry_test.go
package function

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	require.NotNil(t, r)
	assert.False(t, r.Has("nonexistent"))
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()

	handler := func(_ context.Context, input FunctionInput) (*FunctionOutput, error) {
		return &FunctionOutput{Result: map[string]string{"key": "value"}}, nil
	}

	r.Register("my-func", handler)
	assert.True(t, r.Has("my-func"))

	got, err := r.Get("my-func")
	require.NoError(t, err)
	require.NotNil(t, got)

	// Call the handler to verify it works
	out, err := got(context.Background(), FunctionInput{})
	require.NoError(t, err)
	assert.Equal(t, "value", out.Result["key"])
}

func TestRegistry_GetNotFound(t *testing.T) {
	r := NewRegistry()

	_, err := r.Get("missing")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing")
}

func TestRegistry_Has(t *testing.T) {
	r := NewRegistry()

	r.Register("exists", func(_ context.Context, _ FunctionInput) (*FunctionOutput, error) {
		return &FunctionOutput{}, nil
	})

	assert.True(t, r.Has("exists"))
	assert.False(t, r.Has("nope"))
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/jasoet/Documents/Go/go-wf && go test ./function/... -v -run TestNewRegistry`
Expected: FAIL — package/files don't exist

**Step 3: Write minimal implementation**

```go
// function/registry.go
package function

import (
	"context"
	"fmt"
	"sync"
)

// FunctionInput is the input passed to registered handler functions.
type FunctionInput struct {
	Args    map[string]string `json:"args,omitempty"`
	Data    []byte            `json:"data,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	WorkDir string            `json:"work_dir,omitempty"`
}

// FunctionOutput is the output returned by registered handler functions.
type FunctionOutput struct {
	Result map[string]string `json:"result,omitempty"`
	Data   []byte            `json:"data,omitempty"`
}

// Handler is the function signature for registered handlers.
type Handler func(ctx context.Context, input FunctionInput) (*FunctionOutput, error)

// Registry maps function names to handlers.
type Registry struct {
	mu       sync.RWMutex
	handlers map[string]Handler
}

// NewRegistry creates a new empty function registry.
func NewRegistry() *Registry {
	return &Registry{
		handlers: make(map[string]Handler),
	}
}

// Register adds a named handler to the registry.
func (r *Registry) Register(name string, handler Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[name] = handler
}

// Get retrieves a handler by name.
func (r *Registry) Get(name string) (Handler, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.handlers[name]
	if !ok {
		return nil, fmt.Errorf("function %q not found in registry", name)
	}
	return h, nil
}

// Has returns true if a handler with the given name is registered.
func (r *Registry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.handlers[name]
	return ok
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/jasoet/Documents/Go/go-wf && go test ./function/... -v`
Expected: PASS

**Step 5: Format and commit**

```bash
task fmt
git add function/registry.go function/registry_test.go
git commit -m "feat(function): add function registry with handler types"
```

---

### Task 2: Payloads — FunctionExecutionInput/Output

**Files:**
- Create: `function/payload/payload.go`
- Test: `function/payload/payload_test.go`

**Step 1: Write the failing test**

```go
// function/payload/payload_test.go
package payload

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFunctionExecutionInput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   FunctionExecutionInput
		wantErr bool
	}{
		{
			name:    "valid input with name only",
			input:   FunctionExecutionInput{Name: "my-func"},
			wantErr: false,
		},
		{
			name: "valid input with all fields",
			input: FunctionExecutionInput{
				Name:    "my-func",
				Args:    map[string]string{"key": "value"},
				Data:    []byte("hello"),
				Env:     map[string]string{"FOO": "bar"},
				WorkDir: "/tmp",
				Labels:  map[string]string{"env": "test"},
			},
			wantErr: false,
		},
		{
			name:    "invalid - missing name",
			input:   FunctionExecutionInput{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFunctionExecutionInput_ActivityName(t *testing.T) {
	input := &FunctionExecutionInput{Name: "my-func"}
	assert.Equal(t, "ExecuteFunctionActivity", input.ActivityName())
}

func TestFunctionExecutionOutput_IsSuccess(t *testing.T) {
	assert.True(t, FunctionExecutionOutput{Success: true}.IsSuccess())
	assert.False(t, FunctionExecutionOutput{Success: false}.IsSuccess())
}

func TestFunctionExecutionOutput_GetError(t *testing.T) {
	assert.Equal(t, "fail", FunctionExecutionOutput{Error: "fail"}.GetError())
	assert.Equal(t, "", FunctionExecutionOutput{}.GetError())
}

func TestPipelineInput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   PipelineInput
		wantErr bool
	}{
		{
			name: "valid pipeline",
			input: PipelineInput{
				Functions: []FunctionExecutionInput{
					{Name: "step1"},
					{Name: "step2"},
				},
				StopOnError: true,
			},
			wantErr: false,
		},
		{
			name: "invalid - empty functions",
			input: PipelineInput{
				Functions: []FunctionExecutionInput{},
			},
			wantErr: true,
		},
		{
			name: "invalid - nil functions",
			input: PipelineInput{
				Functions: nil,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("PipelineInput.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParallelInput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   ParallelInput
		wantErr bool
	}{
		{
			name: "valid parallel with continue",
			input: ParallelInput{
				Functions:       []FunctionExecutionInput{{Name: "a"}, {Name: "b"}},
				FailureStrategy: "continue",
			},
			wantErr: false,
		},
		{
			name: "valid parallel with fail_fast",
			input: ParallelInput{
				Functions:       []FunctionExecutionInput{{Name: "a"}},
				FailureStrategy: "fail_fast",
			},
			wantErr: false,
		},
		{
			name: "valid parallel with empty strategy",
			input: ParallelInput{
				Functions:       []FunctionExecutionInput{{Name: "a"}},
				FailureStrategy: "",
			},
			wantErr: false,
		},
		{
			name: "invalid - empty functions",
			input: ParallelInput{
				Functions: []FunctionExecutionInput{},
			},
			wantErr: true,
		},
		{
			name: "invalid - bad failure strategy",
			input: ParallelInput{
				Functions:       []FunctionExecutionInput{{Name: "a"}},
				FailureStrategy: "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("ParallelInput.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoopInput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   LoopInput
		wantErr bool
	}{
		{
			name: "valid loop",
			input: LoopInput{
				Items:    []string{"a", "b"},
				Template: FunctionExecutionInput{Name: "process"},
			},
			wantErr: false,
		},
		{
			name: "invalid - empty items",
			input: LoopInput{
				Items:    []string{},
				Template: FunctionExecutionInput{Name: "process"},
			},
			wantErr: true,
		},
		{
			name: "invalid - missing template name",
			input: LoopInput{
				Items:    []string{"a"},
				Template: FunctionExecutionInput{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("LoopInput.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParameterizedLoopInput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   ParameterizedLoopInput
		wantErr bool
	}{
		{
			name: "valid parameterized loop",
			input: ParameterizedLoopInput{
				Parameters: map[string][]string{"os": {"linux", "darwin"}},
				Template:   FunctionExecutionInput{Name: "build"},
			},
			wantErr: false,
		},
		{
			name: "invalid - nil parameters",
			input: ParameterizedLoopInput{
				Template: FunctionExecutionInput{Name: "build"},
			},
			wantErr: true,
		},
		{
			name: "invalid - empty parameter array",
			input: ParameterizedLoopInput{
				Parameters: map[string][]string{"os": {}},
				Template:   FunctionExecutionInput{Name: "build"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("ParameterizedLoopInput.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/jasoet/Documents/Go/go-wf && go test ./function/payload/... -v -run TestFunctionExecutionInput_Validate`
Expected: FAIL — package doesn't exist

**Step 3: Write minimal implementation**

```go
// function/payload/payload.go
package payload

import (
	"fmt"
	"time"

	"github.com/go-playground/validator/v10"

	"github.com/jasoet/go-wf/workflow"
)

// Compile-time interface checks.
var (
	_ workflow.TaskInput  = (*FunctionExecutionInput)(nil)
	_ workflow.TaskOutput = FunctionExecutionOutput{}
)

const functionActivityName = "ExecuteFunctionActivity"

// FunctionExecutionInput defines input for single function execution.
type FunctionExecutionInput struct {
	// Required fields
	Name string `json:"name" validate:"required"`

	// Optional configuration
	Args    map[string]string `json:"args,omitempty"`
	Data    []byte            `json:"data,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	WorkDir string            `json:"work_dir,omitempty"`
	Timeout time.Duration     `json:"timeout,omitempty"`

	// Metadata
	Labels map[string]string `json:"labels,omitempty"`
}

// FunctionExecutionOutput defines output from function execution.
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

// PipelineInput defines sequential function execution.
type PipelineInput struct {
	Functions   []FunctionExecutionInput `json:"functions" validate:"required,min=1"`
	StopOnError bool                     `json:"stop_on_error"`
}

// PipelineOutput defines pipeline execution results.
type PipelineOutput struct {
	Results       []FunctionExecutionOutput `json:"results"`
	TotalSuccess  int                       `json:"total_success"`
	TotalFailed   int                       `json:"total_failed"`
	TotalDuration time.Duration             `json:"total_duration"`
}

// ParallelInput defines parallel function execution.
type ParallelInput struct {
	Functions       []FunctionExecutionInput `json:"functions" validate:"required,min=1"`
	MaxConcurrency  int                      `json:"max_concurrency,omitempty"`
	FailureStrategy string                   `json:"failure_strategy" validate:"oneof='' continue fail_fast"`
}

// ParallelOutput defines parallel execution results.
type ParallelOutput struct {
	Results       []FunctionExecutionOutput `json:"results"`
	TotalSuccess  int                       `json:"total_success"`
	TotalFailed   int                       `json:"total_failed"`
	TotalDuration time.Duration             `json:"total_duration"`
}

// LoopInput defines loop iteration over items.
type LoopInput struct {
	Items           []string               `json:"items" validate:"required,min=1"`
	Template        FunctionExecutionInput `json:"template" validate:"required"`
	Parallel        bool                   `json:"parallel"`
	MaxConcurrency  int                    `json:"max_concurrency,omitempty"`
	FailureStrategy string                 `json:"failure_strategy" validate:"oneof='' continue fail_fast"`
}

// ParameterizedLoopInput defines loop iteration with multiple parameters.
type ParameterizedLoopInput struct {
	Parameters      map[string][]string    `json:"parameters" validate:"required,min=1"`
	Template        FunctionExecutionInput `json:"template" validate:"required"`
	Parallel        bool                   `json:"parallel"`
	MaxConcurrency  int                    `json:"max_concurrency,omitempty"`
	FailureStrategy string                 `json:"failure_strategy" validate:"oneof='' continue fail_fast"`
}

// LoopOutput defines loop execution results.
type LoopOutput struct {
	Results       []FunctionExecutionOutput `json:"results"`
	TotalSuccess  int                       `json:"total_success"`
	TotalFailed   int                       `json:"total_failed"`
	TotalDuration time.Duration             `json:"total_duration"`
	ItemCount     int                       `json:"item_count"`
}

// Validate validates input using struct tags.
func (i *FunctionExecutionInput) Validate() error {
	validate := validator.New()
	return validate.Struct(i)
}

// ActivityName returns the Temporal activity name for function execution.
func (i *FunctionExecutionInput) ActivityName() string {
	return functionActivityName
}

// IsSuccess returns whether the function executed successfully.
func (o FunctionExecutionOutput) IsSuccess() bool {
	return o.Success
}

// GetError returns the error message from function execution.
func (o FunctionExecutionOutput) GetError() string {
	return o.Error
}

// Validate validates pipeline input using struct tags.
func (i *PipelineInput) Validate() error {
	validate := validator.New()
	return validate.Struct(i)
}

// Validate validates parallel input using struct tags.
func (i *ParallelInput) Validate() error {
	validate := validator.New()
	return validate.Struct(i)
}

// Validate validates loop input using struct tags.
func (i *LoopInput) Validate() error {
	validate := validator.New()
	if err := validate.Struct(i); err != nil {
		return err
	}
	return i.Template.Validate()
}

// Validate validates parameterized loop input using struct tags.
func (i *ParameterizedLoopInput) Validate() error {
	validate := validator.New()
	if err := validate.Struct(i); err != nil {
		return err
	}

	if len(i.Parameters) == 0 {
		return fmt.Errorf("at least one parameter array is required")
	}

	for key, values := range i.Parameters {
		if len(values) == 0 {
			return fmt.Errorf("parameter array '%s' cannot be empty", key)
		}
	}

	return i.Template.Validate()
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/jasoet/Documents/Go/go-wf && go test ./function/payload/... -v`
Expected: PASS

**Step 5: Format and commit**

```bash
task fmt
git add function/payload/payload.go function/payload/payload_test.go
git commit -m "feat(function): add function execution payload types"
```

---

### Task 3: Activity — ExecuteFunctionActivity

**Files:**
- Create: `function/activity/function.go`
- Test: `function/activity/function_test.go`

**Step 1: Write the failing test**

```go
// function/activity/function_test.go
package activity

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fn "github.com/jasoet/go-wf/function"
	"github.com/jasoet/go-wf/function/payload"
)

func TestExecuteFunctionActivity_Success(t *testing.T) {
	registry := fn.NewRegistry()
	registry.Register("greet", func(_ context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		name := input.Args["name"]
		return &fn.FunctionOutput{
			Result: map[string]string{"greeting": "hello " + name},
		}, nil
	})

	activity := NewExecuteFunctionActivity(registry)

	input := payload.FunctionExecutionInput{
		Name: "greet",
		Args: map[string]string{"name": "world"},
	}

	output, err := activity(context.Background(), input)
	require.NoError(t, err)
	require.NotNil(t, output)

	assert.True(t, output.Success)
	assert.Equal(t, "greet", output.Name)
	assert.Equal(t, "hello world", output.Result["greeting"])
	assert.NotZero(t, output.Duration)
	assert.NotZero(t, output.StartedAt)
	assert.NotZero(t, output.FinishedAt)
	assert.Empty(t, output.Error)
}

func TestExecuteFunctionActivity_HandlerError(t *testing.T) {
	registry := fn.NewRegistry()
	registry.Register("fail", func(_ context.Context, _ fn.FunctionInput) (*fn.FunctionOutput, error) {
		return nil, fmt.Errorf("something went wrong")
	})

	activity := NewExecuteFunctionActivity(registry)

	input := payload.FunctionExecutionInput{Name: "fail"}

	output, err := activity(context.Background(), input)
	require.NoError(t, err) // Activity itself succeeds, but output captures the error
	require.NotNil(t, output)

	assert.False(t, output.Success)
	assert.Equal(t, "fail", output.Name)
	assert.Contains(t, output.Error, "something went wrong")
}

func TestExecuteFunctionActivity_NotFound(t *testing.T) {
	registry := fn.NewRegistry()

	activity := NewExecuteFunctionActivity(registry)

	input := payload.FunctionExecutionInput{Name: "missing"}

	output, err := activity(context.Background(), input)
	require.Error(t, err)
	require.NotNil(t, output)

	assert.False(t, output.Success)
	assert.Contains(t, output.Error, "missing")
}

func TestExecuteFunctionActivity_ValidationError(t *testing.T) {
	registry := fn.NewRegistry()

	activity := NewExecuteFunctionActivity(registry)

	// Missing required Name field
	input := payload.FunctionExecutionInput{}

	output, err := activity(context.Background(), input)
	require.Error(t, err)
	require.NotNil(t, output)

	assert.False(t, output.Success)
}

func TestExecuteFunctionActivity_WithData(t *testing.T) {
	registry := fn.NewRegistry()
	registry.Register("echo-data", func(_ context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		return &fn.FunctionOutput{
			Data: input.Data,
		}, nil
	})

	activity := NewExecuteFunctionActivity(registry)

	input := payload.FunctionExecutionInput{
		Name: "echo-data",
		Data: []byte("raw bytes"),
	}

	output, err := activity(context.Background(), input)
	require.NoError(t, err)

	assert.True(t, output.Success)
	assert.Equal(t, []byte("raw bytes"), output.Data)
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/jasoet/Documents/Go/go-wf && go test ./function/activity/... -v`
Expected: FAIL — package doesn't exist

**Step 3: Write minimal implementation**

```go
// function/activity/function.go
package activity

import (
	"context"
	"time"

	fn "github.com/jasoet/go-wf/function"
	"github.com/jasoet/go-wf/function/payload"
)

// NewExecuteFunctionActivity creates a Temporal activity that dispatches to registered handlers.
func NewExecuteFunctionActivity(registry *fn.Registry) func(ctx context.Context, input payload.FunctionExecutionInput) (*payload.FunctionExecutionOutput, error) {
	return func(ctx context.Context, input payload.FunctionExecutionInput) (*payload.FunctionExecutionOutput, error) {
		startTime := time.Now()

		// Validate input
		if err := input.Validate(); err != nil {
			return &payload.FunctionExecutionOutput{
				Name:       input.Name,
				StartedAt:  startTime,
				FinishedAt: time.Now(),
				Success:    false,
				Error:      err.Error(),
			}, err
		}

		// Look up handler
		handler, err := registry.Get(input.Name)
		if err != nil {
			return &payload.FunctionExecutionOutput{
				Name:       input.Name,
				StartedAt:  startTime,
				FinishedAt: time.Now(),
				Success:    false,
				Error:      err.Error(),
			}, err
		}

		// Build function input from payload
		fnInput := fn.FunctionInput{
			Args:    input.Args,
			Data:    input.Data,
			Env:     input.Env,
			WorkDir: input.WorkDir,
		}

		// Call handler
		fnOutput, handlerErr := handler(ctx, fnInput)
		finishTime := time.Now()

		output := &payload.FunctionExecutionOutput{
			Name:       input.Name,
			StartedAt:  startTime,
			FinishedAt: finishTime,
			Duration:   finishTime.Sub(startTime),
		}

		if handlerErr != nil {
			output.Success = false
			output.Error = handlerErr.Error()
			return output, nil
		}

		output.Success = true
		if fnOutput != nil {
			output.Result = fnOutput.Result
			output.Data = fnOutput.Data
		}

		return output, nil
	}
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/jasoet/Documents/Go/go-wf && go test ./function/activity/... -v`
Expected: PASS

**Step 5: Format and commit**

```bash
task fmt
git add function/activity/function.go function/activity/function_test.go
git commit -m "feat(function): add function dispatcher activity"
```

---

### Task 4: Workflows — Single, Pipeline, Parallel

**Files:**
- Create: `function/workflow/function.go`
- Create: `function/workflow/helpers.go`
- Create: `function/workflow/pipeline.go`
- Create: `function/workflow/parallel.go`
- Create: `function/workflow/testutil_test.go`
- Test: `function/workflow/function_test.go`
- Test: `function/workflow/pipeline_test.go`
- Test: `function/workflow/parallel_test.go`

**Step 1: Write the failing tests**

```go
// function/workflow/testutil_test.go
package workflow

import (
	"context"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"

	"github.com/jasoet/go-wf/function/payload"
)

// stubExecuteFunctionActivity is a stub for test registration.
func stubExecuteFunctionActivity(_ context.Context, _ payload.FunctionExecutionInput) (*payload.FunctionExecutionOutput, error) {
	return nil, nil
}

// registerFunctionActivity registers the stub activity with the test environment.
func registerFunctionActivity(env *testsuite.TestWorkflowEnvironment) {
	env.RegisterActivityWithOptions(stubExecuteFunctionActivity, activity.RegisterOptions{Name: "ExecuteFunctionActivity"})
}
```

```go
// function/workflow/function_test.go
package workflow

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"

	"github.com/jasoet/go-wf/function/payload"
)

func TestExecuteFunctionWorkflow_Success(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	input := payload.FunctionExecutionInput{Name: "my-func", Args: map[string]string{"key": "val"}}

	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.Anything).Return(&payload.FunctionExecutionOutput{
		Name:     "my-func",
		Success:  true,
		Result:   map[string]string{"out": "result"},
		Duration: 1 * time.Second,
	}, nil)

	env.ExecuteWorkflow(ExecuteFunctionWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.FunctionExecutionOutput
	require.NoError(t, env.GetWorkflowResult(&result))

	assert.Equal(t, "my-func", result.Name)
	assert.True(t, result.Success)
	assert.Equal(t, "result", result.Result["out"])
}

func TestExecuteFunctionWorkflow_InvalidInput(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := payload.FunctionExecutionInput{} // Missing required Name

	env.ExecuteWorkflow(ExecuteFunctionWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	assert.Error(t, env.GetWorkflowError())
}

func TestExecuteFunctionWorkflow_ActivityError(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	input := payload.FunctionExecutionInput{Name: "fail-func"}

	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.Anything).Return(
		nil, fmt.Errorf("activity failed"))

	env.ExecuteWorkflow(ExecuteFunctionWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	assert.Error(t, env.GetWorkflowError())
}
```

```go
// function/workflow/pipeline_test.go
package workflow

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"

	"github.com/jasoet/go-wf/function/payload"
)

func TestFunctionPipelineWorkflow_Success(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	input := payload.PipelineInput{
		Functions: []payload.FunctionExecutionInput{
			{Name: "step1"},
			{Name: "step2"},
		},
		StopOnError: true,
	}

	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.Anything).Return(&payload.FunctionExecutionOutput{
		Success:  true,
		Duration: 1 * time.Second,
	}, nil)

	env.ExecuteWorkflow(FunctionPipelineWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.PipelineOutput
	require.NoError(t, env.GetWorkflowResult(&result))

	assert.Equal(t, 2, result.TotalSuccess)
	assert.Equal(t, 0, result.TotalFailed)
	assert.Len(t, result.Results, 2)
}
```

```go
// function/workflow/parallel_test.go
package workflow

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"

	"github.com/jasoet/go-wf/function/payload"
)

func TestParallelFunctionsWorkflow_Success(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	input := payload.ParallelInput{
		Functions: []payload.FunctionExecutionInput{
			{Name: "task-a"},
			{Name: "task-b"},
		},
		FailureStrategy: "continue",
	}

	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.Anything).Return(&payload.FunctionExecutionOutput{
		Success:  true,
		Duration: 1 * time.Second,
	}, nil)

	env.ExecuteWorkflow(ParallelFunctionsWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.ParallelOutput
	require.NoError(t, env.GetWorkflowResult(&result))

	assert.Equal(t, 2, result.TotalSuccess)
	assert.Equal(t, 0, result.TotalFailed)
	assert.Len(t, result.Results, 2)
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/jasoet/Documents/Go/go-wf && go test ./function/workflow/... -v`
Expected: FAIL — package doesn't exist

**Step 3: Write minimal implementation**

```go
// function/workflow/helpers.go
package workflow

import (
	"github.com/jasoet/go-wf/function/payload"
	generic "github.com/jasoet/go-wf/workflow"
)

// toTaskPtrs converts a slice of FunctionExecutionInput values to a slice of pointers.
func toTaskPtrs(functions []payload.FunctionExecutionInput) []*payload.FunctionExecutionInput {
	ptrs := make([]*payload.FunctionExecutionInput, len(functions))
	for i := range functions {
		ptrs[i] = &functions[i]
	}
	return ptrs
}

// toPipelineOutput converts a generic pipeline output to a function-specific output.
func toPipelineOutput(g *generic.PipelineOutput[payload.FunctionExecutionOutput], err error) (*payload.PipelineOutput, error) {
	if g == nil {
		return nil, err
	}
	return &payload.PipelineOutput{
		Results: g.Results, TotalSuccess: g.TotalSuccess, TotalFailed: g.TotalFailed, TotalDuration: g.TotalDuration,
	}, err
}

// toParallelOutput converts a generic parallel output to a function-specific output.
func toParallelOutput(g *generic.ParallelOutput[payload.FunctionExecutionOutput], err error) (*payload.ParallelOutput, error) {
	if g == nil {
		return nil, err
	}
	return &payload.ParallelOutput{
		Results: g.Results, TotalSuccess: g.TotalSuccess, TotalFailed: g.TotalFailed, TotalDuration: g.TotalDuration,
	}, err
}
```

```go
// function/workflow/function.go
package workflow

import (
	wf "go.temporal.io/sdk/workflow"

	"github.com/jasoet/go-wf/function/payload"
	generic "github.com/jasoet/go-wf/workflow"
)

// ExecuteFunctionWorkflow runs a single function and returns results.
func ExecuteFunctionWorkflow(ctx wf.Context, input payload.FunctionExecutionInput) (*payload.FunctionExecutionOutput, error) {
	return generic.ExecuteTaskWorkflow[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput](ctx, &input)
}
```

```go
// function/workflow/pipeline.go
package workflow

import (
	wf "go.temporal.io/sdk/workflow"

	"github.com/jasoet/go-wf/function/payload"
	generic "github.com/jasoet/go-wf/workflow"
)

// FunctionPipelineWorkflow executes functions sequentially.
func FunctionPipelineWorkflow(ctx wf.Context, input payload.PipelineInput) (*payload.PipelineOutput, error) {
	genericInput := generic.PipelineInput[*payload.FunctionExecutionInput]{
		Tasks:       toTaskPtrs(input.Functions),
		StopOnError: input.StopOnError,
	}

	genericOutput, err := generic.PipelineWorkflow[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput](ctx, genericInput)

	return toPipelineOutput(genericOutput, err)
}
```

```go
// function/workflow/parallel.go
package workflow

import (
	wf "go.temporal.io/sdk/workflow"

	"github.com/jasoet/go-wf/function/payload"
	generic "github.com/jasoet/go-wf/workflow"
)

// ParallelFunctionsWorkflow executes multiple functions in parallel.
func ParallelFunctionsWorkflow(ctx wf.Context, input payload.ParallelInput) (*payload.ParallelOutput, error) {
	genericInput := generic.ParallelInput[*payload.FunctionExecutionInput]{
		Tasks:           toTaskPtrs(input.Functions),
		MaxConcurrency:  input.MaxConcurrency,
		FailureStrategy: input.FailureStrategy,
	}

	genericOutput, err := generic.ParallelWorkflow[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput](ctx, genericInput)

	return toParallelOutput(genericOutput, err)
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /Users/jasoet/Documents/Go/go-wf && go test ./function/workflow/... -v`
Expected: PASS

**Step 5: Format and commit**

```bash
task fmt
git add function/workflow/
git commit -m "feat(function): add single, pipeline, and parallel workflow wrappers"
```

---

### Task 5: Workflows — Loop and Parameterized Loop

**Files:**
- Create: `function/workflow/loop.go`
- Test: `function/workflow/loop_test.go`

**Step 1: Write the failing test**

```go
// function/workflow/loop_test.go
package workflow

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"

	"github.com/jasoet/go-wf/function/payload"
)

func TestLoopWorkflow_Success(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	input := payload.LoopInput{
		Items:    []string{"a", "b", "c"},
		Template: payload.FunctionExecutionInput{Name: "process-{{item}}"},
	}

	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.Anything).Return(&payload.FunctionExecutionOutput{
		Success:  true,
		Duration: 1 * time.Second,
	}, nil)

	env.ExecuteWorkflow(LoopWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.LoopOutput
	require.NoError(t, env.GetWorkflowResult(&result))

	assert.Equal(t, 3, result.TotalSuccess)
	assert.Equal(t, 3, result.ItemCount)
}

func TestParameterizedLoopWorkflow_Success(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	input := payload.ParameterizedLoopInput{
		Parameters: map[string][]string{
			"env": {"dev", "prod"},
		},
		Template: payload.FunctionExecutionInput{
			Name: "deploy",
			Args: map[string]string{"target": "{{.env}}"},
		},
	}

	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.Anything).Return(&payload.FunctionExecutionOutput{
		Success:  true,
		Duration: 1 * time.Second,
	}, nil)

	env.ExecuteWorkflow(ParameterizedLoopWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.LoopOutput
	require.NoError(t, env.GetWorkflowResult(&result))

	assert.Equal(t, 2, result.TotalSuccess)
	assert.Equal(t, 2, result.ItemCount)
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/jasoet/Documents/Go/go-wf && go test ./function/workflow/... -v -run TestLoopWorkflow`
Expected: FAIL — functions don't exist

**Step 3: Write minimal implementation**

```go
// function/workflow/loop.go
package workflow

import (
	wf "go.temporal.io/sdk/workflow"

	"github.com/jasoet/go-wf/function/payload"
	generic "github.com/jasoet/go-wf/workflow"
)

// functionSubstitutor returns a substitutor function for function inputs.
func functionSubstitutor() func(*payload.FunctionExecutionInput, string, int, map[string]string) *payload.FunctionExecutionInput {
	return func(tmpl *payload.FunctionExecutionInput, item string, index int, params map[string]string) *payload.FunctionExecutionInput {
		result := substituteFunctionInput(*tmpl, item, index, params)
		return &result
	}
}

// substituteFunctionInput creates a new function input with substituted values.
func substituteFunctionInput(template payload.FunctionExecutionInput, item string, index int, params map[string]string) payload.FunctionExecutionInput {
	result := template

	// Substitute in name
	result.Name = generic.SubstituteTemplate(template.Name, item, index, params)

	// Substitute in args
	if len(template.Args) > 0 {
		result.Args = make(map[string]string, len(template.Args))
		for key, value := range template.Args {
			newKey := generic.SubstituteTemplate(key, item, index, params)
			newValue := generic.SubstituteTemplate(value, item, index, params)
			result.Args[newKey] = newValue
		}
	}

	// Substitute in env
	if len(template.Env) > 0 {
		result.Env = make(map[string]string, len(template.Env))
		for key, value := range template.Env {
			newKey := generic.SubstituteTemplate(key, item, index, params)
			newValue := generic.SubstituteTemplate(value, item, index, params)
			result.Env[newKey] = newValue
		}
	}

	// Substitute in work directory
	if template.WorkDir != "" {
		result.WorkDir = generic.SubstituteTemplate(template.WorkDir, item, index, params)
	}

	return result
}

// toLoopOutput converts a generic loop output to a function-specific loop output.
func toLoopOutput(g *generic.LoopOutput[payload.FunctionExecutionOutput], err error) (*payload.LoopOutput, error) {
	if g == nil {
		return nil, err
	}
	return &payload.LoopOutput{
		Results:       g.Results,
		TotalSuccess:  g.TotalSuccess,
		TotalFailed:   g.TotalFailed,
		TotalDuration: g.TotalDuration,
		ItemCount:     g.ItemCount,
	}, err
}

// LoopWorkflow executes functions in a loop over items.
func LoopWorkflow(ctx wf.Context, input payload.LoopInput) (*payload.LoopOutput, error) {
	genericInput := generic.LoopInput[*payload.FunctionExecutionInput]{
		Items:           input.Items,
		Template:        &input.Template,
		Parallel:        input.Parallel,
		MaxConcurrency:  input.MaxConcurrency,
		FailureStrategy: input.FailureStrategy,
	}

	return toLoopOutput(
		generic.LoopWorkflow[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput](ctx, genericInput, functionSubstitutor()),
	)
}

// ParameterizedLoopWorkflow executes functions with parameterized loops.
func ParameterizedLoopWorkflow(ctx wf.Context, input payload.ParameterizedLoopInput) (*payload.LoopOutput, error) {
	genericInput := generic.ParameterizedLoopInput[*payload.FunctionExecutionInput]{
		Parameters:      input.Parameters,
		Template:        &input.Template,
		Parallel:        input.Parallel,
		MaxConcurrency:  input.MaxConcurrency,
		FailureStrategy: input.FailureStrategy,
	}

	return toLoopOutput(
		generic.ParameterizedLoopWorkflow[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput](ctx, genericInput, functionSubstitutor()),
	)
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /Users/jasoet/Documents/Go/go-wf && go test ./function/workflow/... -v`
Expected: PASS

**Step 5: Format and commit**

```bash
task fmt
git add function/workflow/loop.go function/workflow/loop_test.go
git commit -m "feat(function): add loop and parameterized loop workflows"
```

---

### Task 6: Builder — WorkflowBuilder and LoopBuilder

**Files:**
- Create: `function/builder/builder.go`
- Create: `function/builder/options.go`
- Create: `function/builder/source.go`
- Test: `function/builder/builder_test.go`
- Test: `function/builder/loop_builder_test.go`

**Step 1: Write the failing tests**

```go
// function/builder/builder_test.go
package builder

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jasoet/go-wf/function/payload"
)

func TestWorkflowBuilder_BuildPipeline(t *testing.T) {
	input, err := NewWorkflowBuilder("test-pipeline").
		AddInput(payload.FunctionExecutionInput{Name: "step1"}).
		AddInput(payload.FunctionExecutionInput{Name: "step2"}).
		StopOnError(true).
		BuildPipeline()

	require.NoError(t, err)
	require.NotNil(t, input)

	assert.Len(t, input.Functions, 2)
	assert.True(t, input.StopOnError)
}

func TestWorkflowBuilder_BuildParallel(t *testing.T) {
	input, err := NewWorkflowBuilder("test-parallel").
		AddInput(payload.FunctionExecutionInput{Name: "task-a"}).
		AddInput(payload.FunctionExecutionInput{Name: "task-b"}).
		Parallel(true).
		FailFast(true).
		MaxConcurrency(5).
		BuildParallel()

	require.NoError(t, err)
	require.NotNil(t, input)

	assert.Len(t, input.Functions, 2)
	assert.Equal(t, "fail_fast", input.FailureStrategy)
	assert.Equal(t, 5, input.MaxConcurrency)
}

func TestWorkflowBuilder_BuildSingle(t *testing.T) {
	input, err := NewWorkflowBuilder("single").
		AddInput(payload.FunctionExecutionInput{Name: "only-one"}).
		BuildSingle()

	require.NoError(t, err)
	require.NotNil(t, input)

	assert.Equal(t, "only-one", input.Name)
}

func TestWorkflowBuilder_Build_AutoSelectsPipeline(t *testing.T) {
	result, err := NewWorkflowBuilder("auto").
		AddInput(payload.FunctionExecutionInput{Name: "step1"}).
		Build()

	require.NoError(t, err)

	_, ok := result.(*payload.PipelineInput)
	assert.True(t, ok, "Expected PipelineInput for non-parallel mode")
}

func TestWorkflowBuilder_Build_AutoSelectsParallel(t *testing.T) {
	result, err := NewWorkflowBuilder("auto").
		AddInput(payload.FunctionExecutionInput{Name: "step1"}).
		Parallel(true).
		Build()

	require.NoError(t, err)

	_, ok := result.(*payload.ParallelInput)
	assert.True(t, ok, "Expected ParallelInput for parallel mode")
}

func TestWorkflowBuilder_EmptyError(t *testing.T) {
	_, err := NewWorkflowBuilder("empty").BuildPipeline()
	assert.Error(t, err)
}

func TestWorkflowBuilder_NilSourceError(t *testing.T) {
	_, err := NewWorkflowBuilder("nil-source").
		Add(nil).
		AddInput(payload.FunctionExecutionInput{Name: "ok"}).
		BuildPipeline()
	assert.Error(t, err)
}

func TestWorkflowBuilder_WithSource(t *testing.T) {
	source := NewFunctionSource(payload.FunctionExecutionInput{Name: "from-source"})

	input, err := NewWorkflowBuilder("with-source").
		Add(source).
		BuildSingle()

	require.NoError(t, err)
	assert.Equal(t, "from-source", input.Name)
}

func TestWorkflowBuilder_WithOptions(t *testing.T) {
	b := NewWorkflowBuilder("opts",
		WithStopOnError(false),
		WithParallelMode(true),
		WithFailFast(true),
		WithMaxConcurrency(10),
	)

	b.AddInput(payload.FunctionExecutionInput{Name: "a"})

	input, err := b.BuildParallel()
	require.NoError(t, err)

	assert.Equal(t, "fail_fast", input.FailureStrategy)
	assert.Equal(t, 10, input.MaxConcurrency)
}

func TestWorkflowBuilder_Count(t *testing.T) {
	b := NewWorkflowBuilder("count")
	assert.Equal(t, 0, b.Count())

	b.AddInput(payload.FunctionExecutionInput{Name: "a"})
	assert.Equal(t, 1, b.Count())
}
```

```go
// function/builder/loop_builder_test.go
package builder

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jasoet/go-wf/function/payload"
)

func TestLoopBuilder_BuildLoop(t *testing.T) {
	input, err := NewLoopBuilder([]string{"a", "b", "c"}).
		WithTemplate(payload.FunctionExecutionInput{Name: "process-{{item}}"}).
		Parallel(true).
		FailFast(true).
		MaxConcurrency(2).
		BuildLoop()

	require.NoError(t, err)
	require.NotNil(t, input)

	assert.Len(t, input.Items, 3)
	assert.Equal(t, "process-{{item}}", input.Template.Name)
	assert.True(t, input.Parallel)
	assert.Equal(t, "fail_fast", input.FailureStrategy)
}

func TestLoopBuilder_BuildParameterizedLoop(t *testing.T) {
	input, err := NewParameterizedLoopBuilder(map[string][]string{
		"env":    {"dev", "prod"},
		"region": {"us", "eu"},
	}).
		WithTemplate(payload.FunctionExecutionInput{
			Name: "deploy",
			Args: map[string]string{"target": "{{.env}}-{{.region}}"},
		}).
		BuildParameterizedLoop()

	require.NoError(t, err)
	require.NotNil(t, input)

	assert.Len(t, input.Parameters, 2)
	assert.Equal(t, "deploy", input.Template.Name)
}

func TestLoopBuilder_EmptyItemsError(t *testing.T) {
	_, err := NewLoopBuilder([]string{}).
		WithTemplate(payload.FunctionExecutionInput{Name: "test"}).
		BuildLoop()
	assert.Error(t, err)
}

func TestLoopBuilder_EmptyParametersError(t *testing.T) {
	_, err := NewParameterizedLoopBuilder(map[string][]string{}).
		WithTemplate(payload.FunctionExecutionInput{Name: "test"}).
		BuildParameterizedLoop()
	assert.Error(t, err)
}

func TestForEach(t *testing.T) {
	lb := ForEach([]string{"x", "y"}, payload.FunctionExecutionInput{Name: "fn"})
	input, err := lb.BuildLoop()
	require.NoError(t, err)
	assert.Len(t, input.Items, 2)
}

func TestForEachParam(t *testing.T) {
	lb := ForEachParam(
		map[string][]string{"v": {"1", "2"}},
		payload.FunctionExecutionInput{Name: "fn"},
	)
	input, err := lb.BuildParameterizedLoop()
	require.NoError(t, err)
	assert.Len(t, input.Parameters, 1)
}

func TestLoopBuilder_NilSourceError(t *testing.T) {
	_, err := NewLoopBuilder([]string{"a"}).
		WithSource(nil).
		BuildLoop()
	assert.Error(t, err)
}

func TestLoopBuilder_WithSource(t *testing.T) {
	source := NewFunctionSource(payload.FunctionExecutionInput{Name: "from-source"})
	input, err := NewLoopBuilder([]string{"a"}).
		WithSource(source).
		BuildLoop()

	require.NoError(t, err)
	assert.Equal(t, "from-source", input.Template.Name)
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/jasoet/Documents/Go/go-wf && go test ./function/builder/... -v`
Expected: FAIL — package doesn't exist

**Step 3: Write minimal implementation**

```go
// function/builder/source.go
package builder

import "github.com/jasoet/go-wf/function/payload"

// WorkflowSource represents a composable workflow component that can generate function execution inputs.
type WorkflowSource interface {
	// ToInput converts the source into a FunctionExecutionInput.
	ToInput() payload.FunctionExecutionInput
}

// WorkflowSourceFunc is a function adapter for WorkflowSource interface.
type WorkflowSourceFunc func() payload.FunctionExecutionInput

// ToInput implements WorkflowSource interface.
func (f WorkflowSourceFunc) ToInput() payload.FunctionExecutionInput {
	return f()
}

// FunctionSource creates a WorkflowSource from a FunctionExecutionInput.
type FunctionSource struct {
	input payload.FunctionExecutionInput
}

// NewFunctionSource creates a new function source.
func NewFunctionSource(input payload.FunctionExecutionInput) *FunctionSource {
	return &FunctionSource{input: input}
}

// ToInput implements WorkflowSource interface.
func (f *FunctionSource) ToInput() payload.FunctionExecutionInput {
	return f.input
}
```

```go
// function/builder/options.go
package builder

// BuilderOption is a functional option for configuring WorkflowBuilder.
type BuilderOption func(*WorkflowBuilder)

// WithStopOnError configures whether the workflow should stop on first error.
func WithStopOnError(stop bool) BuilderOption {
	return func(b *WorkflowBuilder) {
		b.stopOnError = stop
	}
}

// WithParallelMode enables parallel execution mode.
func WithParallelMode(parallel bool) BuilderOption {
	return func(b *WorkflowBuilder) {
		b.parallelMode = parallel
	}
}

// WithFailFast enables fail-fast behavior for parallel workflows.
func WithFailFast(failFast bool) BuilderOption {
	return func(b *WorkflowBuilder) {
		b.failFast = failFast
	}
}

// WithMaxConcurrency sets maximum concurrent functions for parallel workflows.
func WithMaxConcurrency(max int) BuilderOption {
	return func(b *WorkflowBuilder) {
		b.maxConcurrency = max
	}
}
```

```go
// function/builder/builder.go
package builder

import (
	"fmt"

	"github.com/jasoet/go-wf/function/payload"
)

const (
	// FailureStrategyContinue indicates that workflow should continue after failures.
	FailureStrategyContinue = "continue"
	// FailureStrategyFailFast indicates that workflow should stop on first failure.
	FailureStrategyFailFast = "fail_fast"
)

// WorkflowBuilder provides a fluent API for constructing function workflow inputs.
type WorkflowBuilder struct {
	name           string
	functions      []payload.FunctionExecutionInput
	stopOnError    bool
	parallelMode   bool
	failFast       bool
	maxConcurrency int
	errors         []error
}

// NewWorkflowBuilder creates a new workflow builder with the specified name.
func NewWorkflowBuilder(name string, opts ...BuilderOption) *WorkflowBuilder {
	b := &WorkflowBuilder{
		name:        name,
		functions:   make([]payload.FunctionExecutionInput, 0),
		stopOnError: true,
	}

	for _, opt := range opts {
		opt(b)
	}

	return b
}

// Add adds a workflow source to the builder.
func (b *WorkflowBuilder) Add(source WorkflowSource) *WorkflowBuilder {
	if source == nil {
		b.errors = append(b.errors, fmt.Errorf("cannot add nil source"))
		return b
	}

	input := source.ToInput()
	b.functions = append(b.functions, input)
	return b
}

// AddInput adds a function execution input directly to the builder.
func (b *WorkflowBuilder) AddInput(input payload.FunctionExecutionInput) *WorkflowBuilder {
	b.functions = append(b.functions, input)
	return b
}

// StopOnError configures whether the workflow should stop on first error.
func (b *WorkflowBuilder) StopOnError(stop bool) *WorkflowBuilder {
	b.stopOnError = stop
	return b
}

// Parallel configures the builder to create a parallel execution workflow.
func (b *WorkflowBuilder) Parallel(parallel bool) *WorkflowBuilder {
	b.parallelMode = parallel
	return b
}

// FailFast configures fail-fast behavior for parallel workflows.
func (b *WorkflowBuilder) FailFast(failFast bool) *WorkflowBuilder {
	b.failFast = failFast
	return b
}

// MaxConcurrency sets the maximum number of concurrent functions for parallel workflows.
func (b *WorkflowBuilder) MaxConcurrency(max int) *WorkflowBuilder {
	b.maxConcurrency = max
	return b
}

// BuildPipeline creates a pipeline workflow configuration.
func (b *WorkflowBuilder) BuildPipeline() (*payload.PipelineInput, error) {
	if len(b.errors) > 0 {
		return nil, b.errors[0]
	}

	if len(b.functions) == 0 {
		return nil, fmt.Errorf("pipeline workflow requires at least one function")
	}

	input := &payload.PipelineInput{
		Functions:   b.functions,
		StopOnError: b.stopOnError,
	}

	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("pipeline validation failed: %w", err)
	}

	return input, nil
}

// BuildParallel creates a parallel workflow configuration.
func (b *WorkflowBuilder) BuildParallel() (*payload.ParallelInput, error) {
	if len(b.errors) > 0 {
		return nil, b.errors[0]
	}

	if len(b.functions) == 0 {
		return nil, fmt.Errorf("parallel workflow requires at least one function")
	}

	failureStrategy := FailureStrategyContinue
	if b.failFast {
		failureStrategy = FailureStrategyFailFast
	}

	input := &payload.ParallelInput{
		Functions:       b.functions,
		MaxConcurrency:  b.maxConcurrency,
		FailureStrategy: failureStrategy,
	}

	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("parallel validation failed: %w", err)
	}

	return input, nil
}

// Build creates the appropriate workflow configuration based on the builder's mode.
func (b *WorkflowBuilder) Build() (interface{}, error) {
	if b.parallelMode {
		return b.BuildParallel()
	}
	return b.BuildPipeline()
}

// BuildSingle creates a single function execution workflow.
func (b *WorkflowBuilder) BuildSingle() (*payload.FunctionExecutionInput, error) {
	if len(b.errors) > 0 {
		return nil, b.errors[0]
	}

	if len(b.functions) == 0 {
		return nil, fmt.Errorf("single workflow requires at least one function")
	}

	input := &b.functions[0]

	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("single function validation failed: %w", err)
	}

	return input, nil
}

// Count returns the number of functions added to the builder.
func (b *WorkflowBuilder) Count() int {
	return len(b.functions)
}

// Errors returns all errors accumulated during building.
func (b *WorkflowBuilder) Errors() []error {
	return b.errors
}

// LoopBuilder provides a fluent API for constructing loop workflow inputs.
type LoopBuilder struct {
	items          []string
	parameters     map[string][]string
	template       payload.FunctionExecutionInput
	parallel       bool
	maxConcurrency int
	failFast       bool
	errors         []error
}

// NewLoopBuilder creates a new loop builder with the specified items.
func NewLoopBuilder(items []string) *LoopBuilder {
	return &LoopBuilder{
		items: items,
	}
}

// NewParameterizedLoopBuilder creates a new parameterized loop builder.
func NewParameterizedLoopBuilder(parameters map[string][]string) *LoopBuilder {
	return &LoopBuilder{
		parameters: parameters,
	}
}

// WithTemplate sets the function template for the loop.
func (lb *LoopBuilder) WithTemplate(template payload.FunctionExecutionInput) *LoopBuilder {
	lb.template = template
	return lb
}

// WithSource sets the function template from a workflow source.
func (lb *LoopBuilder) WithSource(source WorkflowSource) *LoopBuilder {
	if source == nil {
		lb.errors = append(lb.errors, fmt.Errorf("cannot use nil source"))
		return lb
	}
	lb.template = source.ToInput()
	return lb
}

// Parallel configures the loop to execute in parallel.
func (lb *LoopBuilder) Parallel(parallel bool) *LoopBuilder {
	lb.parallel = parallel
	return lb
}

// MaxConcurrency sets the maximum number of concurrent iterations.
func (lb *LoopBuilder) MaxConcurrency(max int) *LoopBuilder {
	lb.maxConcurrency = max
	return lb
}

// FailFast configures fail-fast behavior.
func (lb *LoopBuilder) FailFast(failFast bool) *LoopBuilder {
	lb.failFast = failFast
	return lb
}

// checkAndStrategy validates the builder state and returns the resolved failure strategy.
func (lb *LoopBuilder) checkAndStrategy() (string, error) {
	if len(lb.errors) > 0 {
		return "", lb.errors[0]
	}

	failureStrategy := FailureStrategyContinue
	if lb.failFast {
		failureStrategy = FailureStrategyFailFast
	}
	return failureStrategy, nil
}

// BuildLoop creates a loop workflow configuration for simple item iteration.
//
//nolint:dupl // BuildLoop and BuildParameterizedLoop construct different types with the same pattern
func (lb *LoopBuilder) BuildLoop() (*payload.LoopInput, error) {
	failureStrategy, err := lb.checkAndStrategy()
	if err != nil {
		return nil, err
	}

	if len(lb.items) == 0 {
		return nil, fmt.Errorf("loop requires at least one item")
	}

	input := &payload.LoopInput{
		Items:           lb.items,
		Template:        lb.template,
		Parallel:        lb.parallel,
		MaxConcurrency:  lb.maxConcurrency,
		FailureStrategy: failureStrategy,
	}

	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("loop validation failed: %w", err)
	}

	return input, nil
}

// BuildParameterizedLoop creates a parameterized loop workflow configuration.
//
//nolint:dupl // BuildParameterizedLoop and BuildLoop construct different types with the same pattern
func (lb *LoopBuilder) BuildParameterizedLoop() (*payload.ParameterizedLoopInput, error) {
	failureStrategy, err := lb.checkAndStrategy()
	if err != nil {
		return nil, err
	}

	if len(lb.parameters) == 0 {
		return nil, fmt.Errorf("parameterized loop requires at least one parameter")
	}

	input := &payload.ParameterizedLoopInput{
		Parameters:      lb.parameters,
		Template:        lb.template,
		Parallel:        lb.parallel,
		MaxConcurrency:  lb.maxConcurrency,
		FailureStrategy: failureStrategy,
	}

	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("parameterized loop validation failed: %w", err)
	}

	return input, nil
}

// ForEach creates a loop builder for iterating over items.
func ForEach(items []string, template payload.FunctionExecutionInput) *LoopBuilder {
	return NewLoopBuilder(items).WithTemplate(template)
}

// ForEachParam creates a parameterized loop builder.
func ForEachParam(parameters map[string][]string, template payload.FunctionExecutionInput) *LoopBuilder {
	return NewParameterizedLoopBuilder(parameters).WithTemplate(template)
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /Users/jasoet/Documents/Go/go-wf && go test ./function/builder/... -v`
Expected: PASS

**Step 5: Format and commit**

```bash
task fmt
git add function/builder/
git commit -m "feat(function): add workflow and loop builder with fluent API"
```

---

### Task 7: Registration — worker.go

**Files:**
- Create: `function/worker.go`
- Test: `function/worker_test.go`

**Step 1: Write the failing test**

```go
// function/worker_test.go
package function

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.temporal.io/sdk/testsuite"
)

func TestRegisterWorkflows(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Should not panic
	assert.NotPanics(t, func() {
		RegisterWorkflows(env)
	})
}

func TestRegisterActivities(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registry := NewRegistry()

	// Should not panic
	assert.NotPanics(t, func() {
		RegisterActivities(env, registry)
	})
}

func TestRegisterAll(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registry := NewRegistry()

	// Should not panic
	assert.NotPanics(t, func() {
		RegisterAll(env, registry)
	})
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/jasoet/Documents/Go/go-wf && go test ./function/ -v -run TestRegisterWorkflows`
Expected: FAIL — functions don't exist

**Step 3: Write minimal implementation**

Note: The `worker.go` registration uses `*testsuite.TestWorkflowEnvironment` in tests, but in production uses `worker.Worker`. Both implement the same registration methods. We need a common interface or use `worker.Worker` directly. Looking at docker's pattern, it uses `worker.Worker`. We should use the same but test with the test suite.

Looking at the docker `worker_test.go` more carefully to understand the test pattern:

```go
// function/worker.go
package function

import (
	"go.temporal.io/sdk/activity"

	fnActivity "github.com/jasoet/go-wf/function/activity"
	wf "github.com/jasoet/go-wf/function/workflow"
)

// WorkflowRegistrar is the interface for registering workflows (satisfied by both worker.Worker and TestWorkflowEnvironment).
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

// RegisterActivities registers the function execution activity.
func RegisterActivities(w WorkflowRegistrar, registry *Registry) {
	w.RegisterActivityWithOptions(fnActivity.NewExecuteFunctionActivity(registry), activity.RegisterOptions{
		Name: "ExecuteFunctionActivity",
	})
}

// RegisterAll registers both workflows and activities.
func RegisterAll(w WorkflowRegistrar, registry *Registry) {
	RegisterWorkflows(w)
	RegisterActivities(w, registry)
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/jasoet/Documents/Go/go-wf && go test ./function/ -v`
Expected: PASS

**Step 5: Format and commit**

```bash
task fmt
git add function/worker.go function/worker_test.go
git commit -m "feat(function): add workflow and activity registration"
```

---

### Task 8: Final Verification and Lint

**Step 1: Run all function module tests**

Run: `cd /Users/jasoet/Documents/Go/go-wf && go test ./function/... -v -count=1`
Expected: All PASS

**Step 2: Run linter**

Run: `task lint`
Expected: No errors (or fix any that appear)

**Step 3: Run full test suite to ensure no regressions**

Run: `task test:unit`
Expected: All PASS, no regressions in docker/ or workflow/

**Step 4: Format**

Run: `task fmt`

**Step 5: Commit any fixes**

```bash
git add -A
git commit -m "chore(function): fix lint and formatting issues"
```

---

### Task 9: Update Documentation

**Files:**
- Modify: `INSTRUCTION.md` — add function/ paths to Key Paths table, update Architecture section
- Modify: `README.md` — add function module documentation

**Step 1: Update INSTRUCTION.md**

Add to Key Paths table:
```
| `function/` | Go function activities (concrete implementation) |
| `function/activity/` | Temporal activity for function dispatch |
| `function/builder/` | Fluent builder API for function workflows |
| `function/payload/` | Type-safe payload structs |
| `function/workflow/` | Workflow implementations (function, pipeline, parallel, loop) |
```

Add to Architecture section:
```
**Function Module (`function/`)** — concrete implementation
- **Registry** maps named Go handler functions for dispatch
- **Activity** dispatches to registered handlers via closure
- **Payloads** implement `TaskInput`/`TaskOutput` interfaces
- **Workflows** register with Temporal workers via `function.RegisterAll(w, registry)`
- **Builder** provides fluent API to compose function workflows
```

**Step 2: Update README.md**

Add function module section with usage example.

**Step 3: Commit**

```bash
git add INSTRUCTION.md README.md
git commit -m "docs: add function activity module documentation"
```
