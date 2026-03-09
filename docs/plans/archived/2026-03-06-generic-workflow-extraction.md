# Generic Workflow Extraction Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Extract generic workflow orchestration core from `docker/` into `workflow/` using Go generics, making docker a pluggable activity module.

**Architecture:** Bottom-up extraction. Create generic `workflow/` package with `TaskInput`/`TaskOutput` type constraints. Generic workflow functions (`Pipeline[I,O]`, `Parallel[I,O]`, `DAG[I,O]`, `Loop[I,O]`) use `ActivityName()` to dispatch activities. Docker module provides concrete types implementing these constraints and thin wrapper functions for Temporal registration.

**Tech Stack:** Go 1.26+ generics, Temporal SDK, go-playground/validator, testify

---

### Task 1: Create Generic Core Interfaces and Error Types

**Files:**
- Create: `workflow/task.go`
- Create: `workflow/errors/errors.go`
- Create: `workflow/errors/errors_test.go`

**Step 1: Create `workflow/task.go` with generic interfaces**

```go
package workflow

// TaskInput is the constraint for all workflow task inputs.
// Implementations must provide validation and specify which activity to invoke.
type TaskInput interface {
	// Validate validates the task input.
	Validate() error
	// ActivityName returns the Temporal activity name to execute.
	ActivityName() string
}

// TaskOutput is the constraint for all workflow task outputs.
// Implementations must report success status and error information.
type TaskOutput interface {
	// IsSuccess returns whether the task executed successfully.
	IsSuccess() bool
	// GetError returns the error message, or empty string if successful.
	GetError() string
}
```

**Step 2: Move errors package from `docker/errors/` to `workflow/errors/`**

Copy `docker/errors/errors.go` to `workflow/errors/errors.go`. Change package doc comment from "Docker workflow error" to "workflow error". Keep all types and predefined errors identical.

```go
package errors

import "fmt"

// ErrorType represents different types of errors.
type ErrorType string

const (
	ErrorTypeValidation    ErrorType = "validation"
	ErrorTypeExecution     ErrorType = "execution"
	ErrorTypeTimeout       ErrorType = "timeout"
	ErrorTypeConfiguration ErrorType = "configuration"
)

// WorkflowError represents a workflow error.
type WorkflowError struct {
	Type    ErrorType
	Message string
	Err     error
}

func (e *WorkflowError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Type, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Type, e.Message)
}

func (e *WorkflowError) Unwrap() error {
	return e.Err
}

func (e *WorkflowError) Wrap(msg string) *WorkflowError {
	return &WorkflowError{
		Type:    e.Type,
		Message: msg,
		Err:     e,
	}
}

var (
	ErrInvalidInput = &WorkflowError{
		Type:    ErrorTypeValidation,
		Message: "invalid input",
	}
	ErrExecutionFailed = &WorkflowError{
		Type:    ErrorTypeExecution,
		Message: "execution failed",
	}
	ErrTimeout = &WorkflowError{
		Type:    ErrorTypeTimeout,
		Message: "operation timed out",
	}
	ErrInvalidConfiguration = &WorkflowError{
		Type:    ErrorTypeConfiguration,
		Message: "invalid configuration",
	}
)

func NewValidationError(msg string, err error) *WorkflowError {
	return &WorkflowError{Type: ErrorTypeValidation, Message: msg, Err: err}
}

func NewExecutionError(msg string, err error) *WorkflowError {
	return &WorkflowError{Type: ErrorTypeExecution, Message: msg, Err: err}
}
```

**Step 3: Copy `docker/errors/errors_test.go` to `workflow/errors/errors_test.go`**

Update import path from `github.com/jasoet/go-wf/docker/errors` to `github.com/jasoet/go-wf/workflow/errors`.

**Step 4: Run tests**

Run: `task test:unit`
Expected: All tests pass including new `workflow/errors/` tests.

**Step 5: Commit**

```
feat(workflow): add generic TaskInput/TaskOutput interfaces and error types
```

---

### Task 2: Create Generic Payload Types

**Files:**
- Create: `workflow/payload/payload.go`
- Create: `workflow/payload/payload_test.go`

**Step 1: Create `workflow/payload/payload.go` with generic composite types**

```go
package payload

import (
	"time"

	"github.com/go-playground/validator/v10"

	"github.com/jasoet/go-wf/workflow"
)

// PipelineInput defines sequential task execution.
type PipelineInput[I workflow.TaskInput] struct {
	Tasks       []I  `json:"tasks" validate:"required,min=1"`
	StopOnError bool `json:"stop_on_error"`
	Cleanup     bool `json:"cleanup"`
}

// Validate validates pipeline input.
func (i *PipelineInput[I]) Validate() error {
	validate := validator.New()
	if err := validate.Struct(i); err != nil {
		return err
	}
	for idx := range i.Tasks {
		if err := i.Tasks[idx].Validate(); err != nil {
			return err
		}
	}
	return nil
}

// PipelineOutput defines pipeline execution results.
type PipelineOutput[O workflow.TaskOutput] struct {
	Results       []O           `json:"results"`
	TotalSuccess  int           `json:"total_success"`
	TotalFailed   int           `json:"total_failed"`
	TotalDuration time.Duration `json:"total_duration"`
}

// ParallelInput defines parallel task execution.
type ParallelInput[I workflow.TaskInput] struct {
	Tasks           []I    `json:"tasks" validate:"required,min=1"`
	MaxConcurrency  int    `json:"max_concurrency,omitempty"`
	FailureStrategy string `json:"failure_strategy" validate:"oneof='' continue fail_fast"`
}

// Validate validates parallel input.
func (i *ParallelInput[I]) Validate() error {
	validate := validator.New()
	if err := validate.Struct(i); err != nil {
		return err
	}
	for idx := range i.Tasks {
		if err := i.Tasks[idx].Validate(); err != nil {
			return err
		}
	}
	return nil
}

// ParallelOutput defines parallel execution results.
type ParallelOutput[O workflow.TaskOutput] struct {
	Results       []O           `json:"results"`
	TotalSuccess  int           `json:"total_success"`
	TotalFailed   int           `json:"total_failed"`
	TotalDuration time.Duration `json:"total_duration"`
}

// LoopInput defines loop iteration over items.
type LoopInput[I workflow.TaskInput] struct {
	Items           []string `json:"items" validate:"required,min=1"`
	Template        I        `json:"template" validate:"required"`
	Parallel        bool     `json:"parallel"`
	MaxConcurrency  int      `json:"max_concurrency,omitempty"`
	FailureStrategy string   `json:"failure_strategy" validate:"oneof='' continue fail_fast"`
}

// Validate validates loop input.
func (i *LoopInput[I]) Validate() error {
	validate := validator.New()
	if err := validate.Struct(i); err != nil {
		return err
	}
	return i.Template.Validate()
}

// ParameterizedLoopInput defines loop with multiple parameters.
type ParameterizedLoopInput[I workflow.TaskInput] struct {
	Parameters      map[string][]string `json:"parameters" validate:"required,min=1"`
	Template        I                   `json:"template" validate:"required"`
	Parallel        bool                `json:"parallel"`
	MaxConcurrency  int                 `json:"max_concurrency,omitempty"`
	FailureStrategy string              `json:"failure_strategy" validate:"oneof='' continue fail_fast"`
}

// Validate validates parameterized loop input.
func (i *ParameterizedLoopInput[I]) Validate() error {
	validate := validator.New()
	if err := validate.Struct(i); err != nil {
		return err
	}
	for key, values := range i.Parameters {
		if len(values) == 0 {
			return fmt.Errorf("parameter array '%s' cannot be empty", key)
		}
	}
	return i.Template.Validate()
}

// LoopOutput defines loop execution results.
type LoopOutput[O workflow.TaskOutput] struct {
	Results       []O           `json:"results"`
	TotalSuccess  int           `json:"total_success"`
	TotalFailed   int           `json:"total_failed"`
	TotalDuration time.Duration `json:"total_duration"`
	ItemCount     int           `json:"item_count"`
}
```

**Step 2: Create test file with a mock TaskInput/TaskOutput for testing**

```go
package payload

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockInput implements workflow.TaskInput for testing.
type mockInput struct {
	Name         string `json:"name" validate:"required"`
	Image        string `json:"image" validate:"required"`
	activityName string
}

func (m *mockInput) Validate() error {
	if m.Image == "" {
		return fmt.Errorf("image is required")
	}
	return nil
}

func (m *mockInput) ActivityName() string {
	return m.activityName
}

// mockOutput implements workflow.TaskOutput for testing.
type mockOutput struct {
	Success  bool   `json:"success"`
	ErrorMsg string `json:"error"`
}

func (m *mockOutput) IsSuccess() bool  { return m.Success }
func (m *mockOutput) GetError() string { return m.ErrorMsg }

func TestPipelineInput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   PipelineInput[*mockInput]
		wantErr bool
	}{
		{
			name: "valid pipeline",
			input: PipelineInput[*mockInput]{
				Tasks: []*mockInput{
					{Name: "step1", Image: "alpine", activityName: "test"},
				},
			},
			wantErr: false,
		},
		{
			name:    "empty tasks",
			input:   PipelineInput[*mockInput]{Tasks: []*mockInput{}},
			wantErr: true,
		},
		{
			name: "invalid task",
			input: PipelineInput[*mockInput]{
				Tasks: []*mockInput{{Name: "step1", Image: ""}},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestParallelInput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   ParallelInput[*mockInput]
		wantErr bool
	}{
		{
			name: "valid parallel",
			input: ParallelInput[*mockInput]{
				Tasks:           []*mockInput{{Name: "t1", Image: "alpine", activityName: "test"}},
				FailureStrategy: "continue",
			},
			wantErr: false,
		},
		{
			name:    "empty tasks",
			input:   ParallelInput[*mockInput]{Tasks: []*mockInput{}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestLoopInput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   LoopInput[*mockInput]
		wantErr bool
	}{
		{
			name: "valid loop",
			input: LoopInput[*mockInput]{
				Items:    []string{"a", "b"},
				Template: &mockInput{Name: "t", Image: "alpine", activityName: "test"},
			},
			wantErr: false,
		},
		{
			name: "empty items",
			input: LoopInput[*mockInput]{
				Items:    []string{},
				Template: &mockInput{Name: "t", Image: "alpine", activityName: "test"},
			},
			wantErr: true,
		},
		{
			name: "invalid template",
			input: LoopInput[*mockInput]{
				Items:    []string{"a"},
				Template: &mockInput{Name: "t", Image: ""},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
```

**Step 3: Run tests**

Run: `task test:unit`
Expected: All tests pass.

**Step 4: Commit**

```
feat(workflow): add generic payload types for pipeline, parallel, and loop
```

---

### Task 3: Create Generic Workflow Helpers

**Files:**
- Create: `workflow/helpers.go`
- Create: `workflow/helpers_test.go`

**Step 1: Create `workflow/helpers.go`**

Move `substituteTemplate`, `generateParameterCombinations`, `extractJSONPath`, `extractRegex`, `readFile`, `replaceAll`, `indexOf` from `docker/workflow/helpers.go` and `docker/workflow/output_extraction.go`. These are pure functions with no Docker dependency. Remove all references to `payload.ContainerExecutionInput` — these helpers only operate on strings and maps.

```go
package workflow

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.temporal.io/sdk/temporal"
	wf "go.temporal.io/sdk/workflow"
)

const (
	// FailureStrategyFailFast indicates workflow should stop on first failure.
	FailureStrategyFailFast = "fail_fast"
	// FailureStrategyContinue indicates workflow should continue after failures.
	FailureStrategyContinue = "continue"
)

// DefaultActivityOptions returns sensible default activity options.
func DefaultActivityOptions() wf.ActivityOptions {
	return wf.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
}

// SubstituteTemplate replaces template variables in a string.
// Supports: {{item}}, {{index}}, and {{.paramName}} syntax.
func SubstituteTemplate(tmpl, item string, index int, params map[string]string) string {
	result := tmpl
	result = strings.ReplaceAll(result, "{{item}}", item)
	result = strings.ReplaceAll(result, "{{index}}", fmt.Sprintf("%d", index))
	for key, value := range params {
		result = strings.ReplaceAll(result, fmt.Sprintf("{{.%s}}", key), value)
		result = strings.ReplaceAll(result, fmt.Sprintf("{{%s}}", key), value)
	}
	return result
}

// GenerateParameterCombinations generates all combinations of parameter values (cartesian product).
func GenerateParameterCombinations(params map[string][]string) []map[string]string {
	if len(params) == 0 {
		return nil
	}
	keys := make([]string, 0, len(params))
	values := make([][]string, 0, len(params))
	for key, vals := range params {
		keys = append(keys, key)
		values = append(values, vals)
	}
	var result []map[string]string
	var generate func(int, map[string]string)
	generate = func(depth int, current map[string]string) {
		if depth == len(keys) {
			combo := make(map[string]string, len(current))
			for k, v := range current {
				combo[k] = v
			}
			result = append(result, combo)
			return
		}
		key := keys[depth]
		for _, value := range values[depth] {
			current[key] = value
			generate(depth+1, current)
		}
	}
	generate(0, make(map[string]string))
	return result
}

// ExtractJSONPath extracts a value from JSON using a simple JSONPath expression.
func ExtractJSONPath(jsonStr, path string) (string, error) {
	// (exact same implementation as docker/workflow/output_extraction.go extractJSONPath)
	var data interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return "", fmt.Errorf("invalid JSON: %w", err)
	}
	path = strings.TrimPrefix(path, "$.")
	path = strings.TrimPrefix(path, "$")
	parts := strings.Split(path, ".")
	current := data
	for _, part := range parts {
		if part == "" {
			continue
		}
		if strings.Contains(part, "[") {
			re := regexp.MustCompile(`^(\w+)\[(\d+)\]$`)
			matches := re.FindStringSubmatch(part)
			if len(matches) != 3 {
				return "", fmt.Errorf("invalid array syntax: %s", part)
			}
			fieldName := matches[1]
			index, err := strconv.Atoi(matches[2])
			if err != nil {
				return "", fmt.Errorf("invalid array index %s: %w", matches[2], err)
			}
			if m, ok := current.(map[string]interface{}); ok {
				current = m[fieldName]
			} else {
				return "", fmt.Errorf("expected object at %s", fieldName)
			}
			if arr, ok := current.([]interface{}); ok {
				if index < 0 || index >= len(arr) {
					return "", fmt.Errorf("array index out of bounds: %d", index)
				}
				current = arr[index]
			} else {
				return "", fmt.Errorf("expected array at %s", fieldName)
			}
		} else {
			if m, ok := current.(map[string]interface{}); ok {
				var exists bool
				current, exists = m[part]
				if !exists {
					return "", fmt.Errorf("field %s not found", part)
				}
			} else {
				return "", fmt.Errorf("cannot navigate to %s", part)
			}
		}
	}
	switch v := current.(type) {
	case string:
		return v, nil
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	case int:
		return strconv.Itoa(v), nil
	case bool:
		return strconv.FormatBool(v), nil
	case nil:
		return "", nil
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("failed to marshal result: %w", err)
		}
		return string(b), nil
	}
}

// ExtractRegex extracts a value from text using a regular expression.
func ExtractRegex(text, pattern string) (string, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("invalid regex pattern: %w", err)
	}
	matches := re.FindStringSubmatch(text)
	if len(matches) == 0 {
		return "", fmt.Errorf("no match found for pattern: %s", pattern)
	}
	if len(matches) > 1 {
		return matches[1], nil
	}
	return matches[0], nil
}

// ReadFile reads a file and returns its contents as a string.
func ReadFile(path string) (string, error) {
	data, err := os.ReadFile(path) //#nosec G304 -- path comes from workflow config
	if err != nil {
		return "", err
	}
	return string(data), nil
}
```

**Step 2: Create `workflow/helpers_test.go`**

Port the existing tests from `docker/workflow/output_extraction_test.go` for the helper functions, updating imports. Add tests for `SubstituteTemplate` and `GenerateParameterCombinations`.

```go
package workflow

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubstituteTemplate(t *testing.T) {
	tests := []struct {
		name     string
		tmpl     string
		item     string
		index    int
		params   map[string]string
		expected string
	}{
		{
			name:     "replace item",
			tmpl:     "process {{item}}",
			item:     "file.csv",
			expected: "process file.csv",
		},
		{
			name:     "replace index",
			tmpl:     "step-{{index}}",
			index:    3,
			expected: "step-3",
		},
		{
			name:     "replace params",
			tmpl:     "deploy --env={{.env}} --region={{.region}}",
			params:   map[string]string{"env": "prod", "region": "us-east"},
			expected: "deploy --env=prod --region=us-east",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SubstituteTemplate(tt.tmpl, tt.item, tt.index, tt.params)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateParameterCombinations(t *testing.T) {
	tests := []struct {
		name   string
		params map[string][]string
		count  int
	}{
		{
			name:  "nil params",
			count: 0,
		},
		{
			name:   "single param",
			params: map[string][]string{"env": {"dev", "prod"}},
			count:  2,
		},
		{
			name: "two params",
			params: map[string][]string{
				"env":    {"dev", "prod"},
				"region": {"us", "eu"},
			},
			count: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateParameterCombinations(tt.params)
			assert.Len(t, result, tt.count)
		})
	}
}

func TestExtractJSONPath(t *testing.T) {
	jsonStr := `{"name":"test","version":"1.0","items":[{"id":1},{"id":2}]}`

	tests := []struct {
		name     string
		path     string
		expected string
		wantErr  bool
	}{
		{"simple field", "$.name", "test", false},
		{"nested via dot", "$.version", "1.0", false},
		{"array access", "$.items[0].id", "1", false},
		{"missing field", "$.missing", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ExtractJSONPath(jsonStr, tt.path)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestExtractRegex(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		pattern  string
		expected string
		wantErr  bool
	}{
		{"full match", "version: 1.2.3", `\d+\.\d+\.\d+`, "1.2.3", false},
		{"group capture", "build-id: abc123", `build-id: (\w+)`, "abc123", false},
		{"no match", "hello", `\d+`, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ExtractRegex(tt.text, tt.pattern)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestReadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(path, []byte("hello world"), 0o600))

	content, err := ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "hello world", content)

	_, err = ReadFile(filepath.Join(dir, "nonexistent"))
	assert.Error(t, err)
}
```

**Step 3: Run tests**

Run: `task test:unit`
Expected: All tests pass.

**Step 4: Commit**

```
feat(workflow): add generic helper functions for template substitution and output extraction
```

---

### Task 4: Create Generic Workflow Implementations

**Files:**
- Create: `workflow/execute.go`
- Create: `workflow/pipeline.go`
- Create: `workflow/parallel.go`
- Create: `workflow/loop.go`
- Create: `workflow/execute_test.go`
- Create: `workflow/pipeline_test.go`
- Create: `workflow/parallel_test.go`
- Create: `workflow/loop_test.go`

**Step 1: Create `workflow/execute.go` — single task workflow**

```go
package workflow

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/workflow"
)

// ExecuteTaskWorkflow runs a single task and returns results.
func ExecuteTaskWorkflow[I TaskInput, O TaskOutput](ctx workflow.Context, input I) (*O, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting task execution workflow", "activity", input.ActivityName())

	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	ao := DefaultActivityOptions()
	ctx = workflow.WithActivityOptions(ctx, ao)

	var output O
	err := workflow.ExecuteActivity(ctx, input.ActivityName(), input).Get(ctx, &output)
	if err != nil {
		logger.Error("Task execution failed", "error", err)
		return nil, err
	}

	logger.Info("Task execution completed", "success", output.IsSuccess())
	return &output, nil
}

// ExecuteTaskWorkflowWithTimeout runs a single task with a custom timeout.
func ExecuteTaskWorkflowWithTimeout[I TaskInput, O TaskOutput](ctx workflow.Context, input I, timeout time.Duration) (*O, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting task execution workflow", "activity", input.ActivityName(), "timeout", timeout)

	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	ao := DefaultActivityOptions()
	ao.StartToCloseTimeout = timeout
	ctx = workflow.WithActivityOptions(ctx, ao)

	var output O
	err := workflow.ExecuteActivity(ctx, input.ActivityName(), input).Get(ctx, &output)
	if err != nil {
		logger.Error("Task execution failed", "error", err)
		return nil, err
	}

	return &output, nil
}
```

**Step 2: Create `workflow/pipeline.go`**

```go
package workflow

import (
	"fmt"

	"go.temporal.io/sdk/workflow"

	"github.com/jasoet/go-wf/workflow/payload"
)

// PipelineWorkflow executes tasks sequentially.
func PipelineWorkflow[I TaskInput, O TaskOutput](ctx workflow.Context, input payload.PipelineInput[I]) (*payload.PipelineOutput[O], error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting pipeline workflow", "steps", len(input.Tasks))

	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	startTime := workflow.Now(ctx)
	output := &payload.PipelineOutput[O]{
		Results: make([]O, 0, len(input.Tasks)),
	}

	ctx = workflow.WithActivityOptions(ctx, DefaultActivityOptions())

	for i, task := range input.Tasks {
		taskName := fmt.Sprintf("step-%d", i+1)
		logger.Info("Executing pipeline step", "step", i+1, "name", taskName)

		var result O
		err := workflow.ExecuteActivity(ctx, task.ActivityName(), task).Get(ctx, &result)
		output.Results = append(output.Results, result)

		if err != nil || !result.IsSuccess() {
			output.TotalFailed++
			logger.Error("Pipeline step failed", "step", i+1, "error", err)
			if input.StopOnError {
				output.TotalDuration = workflow.Now(ctx).Sub(startTime)
				return output, fmt.Errorf("pipeline stopped at step %d: %w", i+1, err)
			}
			continue
		}

		output.TotalSuccess++
		logger.Info("Pipeline step completed", "step", i+1)
	}

	output.TotalDuration = workflow.Now(ctx).Sub(startTime)
	logger.Info("Pipeline workflow completed",
		"success", output.TotalSuccess,
		"failed", output.TotalFailed)

	return output, nil
}
```

**Step 3: Create `workflow/parallel.go`**

```go
package workflow

import (
	"fmt"

	"go.temporal.io/sdk/workflow"

	"github.com/jasoet/go-wf/workflow/payload"
)

// ParallelWorkflow executes tasks in parallel.
func ParallelWorkflow[I TaskInput, O TaskOutput](ctx workflow.Context, input payload.ParallelInput[I]) (*payload.ParallelOutput[O], error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting parallel workflow",
		"tasks", len(input.Tasks),
		"maxConcurrency", input.MaxConcurrency)

	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	startTime := workflow.Now(ctx)
	ctx = workflow.WithActivityOptions(ctx, DefaultActivityOptions())

	futures := make([]workflow.Future, len(input.Tasks))
	for i, task := range input.Tasks {
		futures[i] = workflow.ExecuteActivity(ctx, task.ActivityName(), task)
	}

	output := &payload.ParallelOutput[O]{
		Results: make([]O, 0, len(input.Tasks)),
	}

	for i, future := range futures {
		var result O
		err := future.Get(ctx, &result)
		output.Results = append(output.Results, result)

		if err != nil || !result.IsSuccess() {
			output.TotalFailed++
			if input.FailureStrategy == FailureStrategyFailFast {
				output.TotalDuration = workflow.Now(ctx).Sub(startTime)
				return output, fmt.Errorf("parallel execution failed at task %d: %w", i, err)
			}
		} else {
			output.TotalSuccess++
		}
	}

	output.TotalDuration = workflow.Now(ctx).Sub(startTime)
	logger.Info("Parallel workflow completed",
		"success", output.TotalSuccess,
		"failed", output.TotalFailed)

	return output, nil
}
```

**Step 4: Create `workflow/loop.go`**

```go
package workflow

import (
	"fmt"

	"go.temporal.io/sdk/workflow"

	"github.com/jasoet/go-wf/workflow/payload"
)

// LoopWorkflow executes a task template for each item.
func LoopWorkflow[I TaskInput, O TaskOutput](ctx workflow.Context, input payload.LoopInput[I], substitutor func(template I, item string, index int, params map[string]string) I) (*payload.LoopOutput[O], error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting loop workflow",
		"items", len(input.Items),
		"parallel", input.Parallel)

	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	ctx = workflow.WithActivityOptions(ctx, DefaultActivityOptions())
	startTime := workflow.Now(ctx)

	output := &payload.LoopOutput[O]{
		Results:   make([]O, 0, len(input.Items)),
		ItemCount: len(input.Items),
	}

	if input.Parallel {
		executeParallelLoop(ctx, logger, input, substitutor, output)
	} else {
		executeSequentialLoop(ctx, logger, input, substitutor, output)
	}

	output.TotalDuration = workflow.Now(ctx).Sub(startTime)

	if output.TotalFailed > 0 && input.FailureStrategy == FailureStrategyFailFast {
		return output, fmt.Errorf("loop failed: %d iterations failed", output.TotalFailed)
	}

	return output, nil
}

func executeParallelLoop[I TaskInput, O TaskOutput](ctx workflow.Context, logger interface {
	Info(string, ...interface{})
	Error(string, ...interface{})
}, input payload.LoopInput[I], substitutor func(I, string, int, map[string]string) I, output *payload.LoopOutput[O],
) {
	futures := make([]workflow.Future, len(input.Items))
	for i, item := range input.Items {
		taskInput := substitutor(input.Template, item, i, nil)
		futures[i] = workflow.ExecuteActivity(ctx, taskInput.ActivityName(), taskInput)
	}

	for i, future := range futures {
		var result O
		err := future.Get(ctx, &result)
		output.Results = append(output.Results, result)
		if err != nil || !result.IsSuccess() {
			output.TotalFailed++
			if input.FailureStrategy == FailureStrategyFailFast {
				return
			}
		} else {
			output.TotalSuccess++
		}
		_ = i
	}
}

func executeSequentialLoop[I TaskInput, O TaskOutput](ctx workflow.Context, logger interface {
	Info(string, ...interface{})
	Error(string, ...interface{})
}, input payload.LoopInput[I], substitutor func(I, string, int, map[string]string) I, output *payload.LoopOutput[O],
) {
	for i, item := range input.Items {
		taskInput := substitutor(input.Template, item, i, nil)
		var result O
		err := workflow.ExecuteActivity(ctx, taskInput.ActivityName(), taskInput).Get(ctx, &result)
		output.Results = append(output.Results, result)
		if err != nil || !result.IsSuccess() {
			output.TotalFailed++
			if input.FailureStrategy == FailureStrategyFailFast {
				return
			}
		} else {
			output.TotalSuccess++
		}
	}
}

// ParameterizedLoopWorkflow executes a task template for each parameter combination.
func ParameterizedLoopWorkflow[I TaskInput, O TaskOutput](ctx workflow.Context, input payload.ParameterizedLoopInput[I], substitutor func(template I, item string, index int, params map[string]string) I) (*payload.LoopOutput[O], error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting parameterized loop workflow", "parameters", len(input.Parameters))

	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	combinations := GenerateParameterCombinations(input.Parameters)

	// Convert to a LoopInput and delegate
	loopInput := payload.LoopInput[I]{
		Items:           make([]string, len(combinations)),
		Template:        input.Template,
		Parallel:        input.Parallel,
		MaxConcurrency:  input.MaxConcurrency,
		FailureStrategy: input.FailureStrategy,
	}

	// Use index-based iteration with params substitutor
	ctx = workflow.WithActivityOptions(ctx, DefaultActivityOptions())
	startTime := workflow.Now(ctx)

	output := &payload.LoopOutput[O]{
		Results:   make([]O, 0, len(combinations)),
		ItemCount: len(combinations),
	}

	if input.Parallel {
		futures := make([]workflow.Future, len(combinations))
		for i, params := range combinations {
			taskInput := substitutor(input.Template, "", i, params)
			futures[i] = workflow.ExecuteActivity(ctx, taskInput.ActivityName(), taskInput)
		}
		for _, future := range futures {
			var result O
			err := future.Get(ctx, &result)
			output.Results = append(output.Results, result)
			if err != nil || !result.IsSuccess() {
				output.TotalFailed++
				if input.FailureStrategy == FailureStrategyFailFast {
					break
				}
			} else {
				output.TotalSuccess++
			}
		}
	} else {
		for i, params := range combinations {
			taskInput := substitutor(input.Template, "", i, params)
			var result O
			err := workflow.ExecuteActivity(ctx, taskInput.ActivityName(), taskInput).Get(ctx, &result)
			output.Results = append(output.Results, result)
			if err != nil || !result.IsSuccess() {
				output.TotalFailed++
				if input.FailureStrategy == FailureStrategyFailFast {
					break
				}
			} else {
				output.TotalSuccess++
			}
		}
	}

	output.TotalDuration = workflow.Now(ctx).Sub(startTime)
	_ = loopInput
	_ = logger

	if output.TotalFailed > 0 && input.FailureStrategy == FailureStrategyFailFast {
		return output, fmt.Errorf("parameterized loop failed: %d iterations failed", output.TotalFailed)
	}

	return output, nil
}
```

**Step 5: Create test files using Temporal `TestWorkflowEnvironment` with mock TaskInput/TaskOutput**

Each test file should define a package-level mock type:

```go
// workflow/execute_test.go
package workflow

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
)

type testInput struct {
	Name         string `json:"name"`
	Value        string `json:"value" validate:"required"`
	activityName string
}

func (t *testInput) Validate() error {
	if t.Value == "" {
		return fmt.Errorf("value is required")
	}
	return nil
}

func (t *testInput) ActivityName() string { return t.activityName }

type testOutput struct {
	Result  string `json:"result"`
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

func (t *testOutput) IsSuccess() bool  { return t.Success }
func (t *testOutput) GetError() string { return t.Error }

func TestExecuteTaskWorkflow_Success(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()

	input := &testInput{Name: "test", Value: "hello", activityName: "TestActivity"}

	env.OnActivity("TestActivity", mock.Anything, mock.Anything).Return(
		&testOutput{Result: "done", Success: true}, nil,
	)

	env.ExecuteWorkflow(ExecuteTaskWorkflow[*testInput, testOutput], input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result testOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, "done", result.Result)
	assert.True(t, result.Success)
}

func TestExecuteTaskWorkflow_InvalidInput(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()

	input := &testInput{Name: "test", Value: "", activityName: "TestActivity"}
	env.ExecuteWorkflow(ExecuteTaskWorkflow[*testInput, testOutput], input)

	require.True(t, env.IsWorkflowCompleted())
	assert.Error(t, env.GetWorkflowError())
}
```

Follow the same pattern for `pipeline_test.go`, `parallel_test.go`, and `loop_test.go` — port the existing docker workflow tests but using the generic mock types and calling `PipelineWorkflow[*testInput, testOutput]` etc.

**Step 6: Run tests**

Run: `task test:unit`
Expected: All tests pass.

**Step 7: Commit**

```
feat(workflow): add generic workflow implementations (execute, pipeline, parallel, loop)
```

---

### Task 5: Move Artifacts Package

**Files:**
- Create: `workflow/artifacts/` (copy all files from `docker/artifacts/`)
- Modify: `workflow/artifacts/*.go` — update package references

**Step 1: Copy all files from `docker/artifacts/` to `workflow/artifacts/`**

Files to copy: `store.go`, `local.go`, `minio.go`, `activities.go`, `local_test.go`, `minio_integration_test.go`.

No content changes needed — the artifacts package has zero Docker imports. It's already fully generic.

**Step 2: Run tests**

Run: `task test:unit`
Expected: All tests pass including `workflow/artifacts/` tests.

**Step 3: Commit**

```
refactor(workflow): move artifacts package to workflow/artifacts
```

---

### Task 6: Add Interface Methods to Docker Payload Types

**Files:**
- Modify: `docker/payload/payloads.go`
- Modify: `docker/payload/payloads_test.go`

**Step 1: Add `TaskInput`/`TaskOutput` interface methods to docker payloads**

Add to `docker/payload/payloads.go`:

```go
import "github.com/jasoet/go-wf/workflow"

// Compile-time interface checks.
var (
	_ workflow.TaskInput  = (*ContainerExecutionInput)(nil)
	_ workflow.TaskOutput = (*ContainerExecutionOutput)(nil)
)

const containerActivityName = "StartContainerActivity"

// ActivityName returns the Temporal activity name for container execution.
func (i *ContainerExecutionInput) ActivityName() string {
	return containerActivityName
}

// IsSuccess returns whether the container executed successfully.
func (o *ContainerExecutionOutput) IsSuccess() bool {
	return o.Success
}

// GetError returns the error message from container execution.
func (o *ContainerExecutionOutput) GetError() string {
	return o.Error
}
```

**Step 2: Add tests for new methods**

Add to `docker/payload/payloads_test.go`:

```go
func TestContainerExecutionInput_ActivityName(t *testing.T) {
	input := &ContainerExecutionInput{Image: "alpine"}
	assert.Equal(t, "StartContainerActivity", input.ActivityName())
}

func TestContainerExecutionOutput_IsSuccess(t *testing.T) {
	assert.True(t, (&ContainerExecutionOutput{Success: true}).IsSuccess())
	assert.False(t, (&ContainerExecutionOutput{Success: false}).IsSuccess())
}

func TestContainerExecutionOutput_GetError(t *testing.T) {
	assert.Equal(t, "fail", (&ContainerExecutionOutput{Error: "fail"}).GetError())
	assert.Equal(t, "", (&ContainerExecutionOutput{}).GetError())
}
```

**Step 3: Run tests**

Run: `task test:unit`
Expected: All tests pass.

**Step 4: Commit**

```
feat(docker): implement TaskInput/TaskOutput interfaces on container payload types
```

---

### Task 7: Rewire Docker Workflows to Use Generic Core

**Files:**
- Modify: `docker/workflow/container.go`
- Modify: `docker/workflow/pipeline.go`
- Modify: `docker/workflow/parallel.go`
- Modify: `docker/workflow/loop.go`
- Modify: `docker/workflow/helpers.go`
- Modify: `docker/workflow/output_extraction.go`

**Step 1: Rewire `container.go` to delegate to generic `ExecuteTaskWorkflow`**

```go
package workflow

import (
	"github.com/jasoet/go-wf/docker/payload"
	generic "github.com/jasoet/go-wf/workflow"
)

// ExecuteContainerWorkflow runs a single container and returns results.
func ExecuteContainerWorkflow(ctx workflow.Context, input payload.ContainerExecutionInput) (*payload.ContainerExecutionOutput, error) {
	return generic.ExecuteTaskWorkflow[payload.ContainerExecutionInput, payload.ContainerExecutionOutput](ctx, input)
}
```

**Step 2: Rewire `pipeline.go`**

```go
func ContainerPipelineWorkflow(ctx workflow.Context, input payload.PipelineInput) (*payload.PipelineOutput, error) {
	// Convert to generic PipelineInput
	genericInput := genericPayload.PipelineInput[payload.ContainerExecutionInput]{
		Tasks:       input.Containers,
		StopOnError: input.StopOnError,
		Cleanup:     input.Cleanup,
	}
	genericOutput, err := generic.PipelineWorkflow[payload.ContainerExecutionInput, payload.ContainerExecutionOutput](ctx, genericInput)
	if err != nil {
		return convertPipelineOutput(genericOutput), err
	}
	return convertPipelineOutput(genericOutput), nil
}
```

Note: The docker-specific `PipelineInput` type has `Containers` field while the generic has `Tasks`. The wrapper converts between them. Similarly for output types.

**Step 3: Rewire `parallel.go` and `loop.go` following the same pattern**

Each delegates to the corresponding generic function, converting between docker-specific and generic payload types.

**Step 4: Update `helpers.go` — `substituteContainerInput` stays in docker, calls generic `SubstituteTemplate`**

```go
package workflow

import (
	"github.com/jasoet/go-wf/docker/payload"
	generic "github.com/jasoet/go-wf/workflow"
)

func substituteContainerInput(template payload.ContainerExecutionInput, item string, index int, params map[string]string) payload.ContainerExecutionInput {
	result := template
	result.Image = generic.SubstituteTemplate(template.Image, item, index, params)
	// ... same for Command, Entrypoint, Env, Name, WorkDir, Volumes
	return result
}
```

**Step 5: Update `output_extraction.go` — delegate to generic helpers**

```go
func ExtractOutput(def payload.OutputDefinition, containerOutput *payload.ContainerExecutionOutput) (string, error) {
	// Keep the container-specific ValueFrom mapping (stdout, stderr, exitCode)
	// Use generic.ExtractJSONPath, generic.ExtractRegex, generic.ReadFile
}
```

This file stays in docker because it maps container-specific fields (Stdout, Stderr, ExitCode) but uses generic extraction helpers.

**Step 6: Run all tests**

Run: `task test:unit`
Expected: All existing docker tests pass — behavior is identical, just delegating to generic core.

**Step 7: Commit**

```
refactor(docker): rewire workflows to delegate to generic workflow core
```

---

### Task 8: Update Docker Errors Import and Clean Up Old Packages

**Files:**
- Modify: `docker/payload/payloads_extended.go` — change import from `docker/errors` to `workflow/errors`
- Delete: `docker/errors/` (after updating all imports)
- Delete: `docker/artifacts/` (after verifying `workflow/artifacts/` works)
- Modify: `docker/workflow/dag.go` — update artifacts import

**Step 1: Update all imports from `docker/errors` to `workflow/errors`**

Search and replace across all files that import `github.com/jasoet/go-wf/docker/errors` to use `github.com/jasoet/go-wf/workflow/errors`.

**Step 2: Update all imports from `docker/artifacts` to `workflow/artifacts`**

Search and replace across all files that import `github.com/jasoet/go-wf/docker/artifacts` to use `github.com/jasoet/go-wf/workflow/artifacts`.

**Step 3: Remove old `docker/errors/` and `docker/artifacts/` directories**

Only after all imports are updated and tests pass.

**Step 4: Run all tests**

Run: `task test:unit`
Expected: All tests pass.

**Step 5: Commit**

```
refactor: migrate errors and artifacts imports to workflow package
```

---

### Task 9: Update Worker Registration and Remove Docker Activity Import from Workflows

**Files:**
- Modify: `docker/worker.go`
- Verify: `docker/workflow/*.go` no longer imports `docker/activity`

**Step 1: Verify workflow files no longer import `docker/activity`**

After Task 7, workflow files should call activities by name (via `ActivityName()`) not by function reference. Verify that `docker/workflow/*.go` source files (not test files) have no import of `github.com/jasoet/go-wf/docker/activity`.

**Step 2: Update `docker/worker.go`**

```go
package docker

import (
	"go.temporal.io/sdk/worker"

	"github.com/jasoet/go-wf/docker/activity"
	wf "github.com/jasoet/go-wf/docker/workflow"
)

func RegisterWorkflows(w worker.Worker) {
	w.RegisterWorkflow(wf.ExecuteContainerWorkflow)
	w.RegisterWorkflow(wf.ContainerPipelineWorkflow)
	w.RegisterWorkflow(wf.ParallelContainersWorkflow)
	w.RegisterWorkflow(wf.LoopWorkflow)
	w.RegisterWorkflow(wf.ParameterizedLoopWorkflow)
	w.RegisterWorkflow(wf.DAGWorkflow)
	w.RegisterWorkflow(wf.WorkflowWithParameters)
}

func RegisterActivities(w worker.Worker) {
	w.RegisterActivityWithOptions(activity.StartContainerActivity, worker.RegisterActivityOptions{
		Name: "StartContainerActivity",
	})
}

func RegisterAll(w worker.Worker) {
	RegisterWorkflows(w)
	RegisterActivities(w)
}
```

The key change is `RegisterActivityWithOptions` with explicit `Name` matching what `ActivityName()` returns.

**Step 3: Run all tests**

Run: `task test:unit`
Expected: All tests pass.

**Step 4: Commit**

```
refactor(docker): register activity by name to match generic ActivityName()
```

---

### Task 10: Update Examples, Docs, and Run Full Check

**Files:**
- Modify: `examples/docker/*.go` — update imports if needed
- Modify: `INSTRUCTION.md` — update key paths and architecture
- Modify: `README.md` — update package descriptions

**Step 1: Update examples to use new import paths**

Review all files in `examples/docker/` and update any imports that reference moved packages (errors, artifacts).

**Step 2: Update `INSTRUCTION.md`**

Add `workflow/` package paths to Key Paths table. Update Architecture section to describe the generic core + docker module split.

**Step 3: Update `README.md`**

Update project structure, package descriptions, and quick start to reflect the new architecture.

**Step 4: Run full check**

Run: `task test:unit`
Expected: All tests pass.

Run: `task lint`
Expected: Zero lint errors.

Run: `task fmt`
Expected: No formatting changes needed (or apply them).

**Step 5: Commit**

```
docs: update documentation for generic workflow architecture
```

---

### Task 11: Final Verification

**Step 1: Run full test suite**

Run: `task test:unit`
Expected: All tests pass, coverage >= 85% on workflow/ packages.

**Step 2: Verify no circular imports**

Run: `go build ./...`
Expected: Clean build with no import cycles.

**Step 3: Verify dependency direction**

Run: `grep -r "go-wf/docker" workflow/` (should return nothing)
Expected: No files in `workflow/` import from `docker/`.

**Step 4: Commit any final fixes**

```
chore: final cleanup for generic workflow extraction
```
