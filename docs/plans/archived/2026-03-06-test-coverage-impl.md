# Test Coverage Improvement Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Increase overall test coverage from 81.4% to 85%+ by adding tests across four packages.

**Architecture:** Broad sweep approach — add unit tests for payload validation and operations error paths, integration tests for container activity, and filesystem-based error path tests for artifacts.

**Tech Stack:** Go 1.26+, testify (assert/require), Temporal SDK mocks, testcontainers-go, go-playground/validator

---

### Task 1: Payload — LoopInput and ParameterizedLoopInput Validation Tests

**Files:**
- Modify: `docker/payload/payloads_test.go`

**Step 1: Write the tests**

Add to `docker/payload/payloads_test.go`:

```go
func TestLoopInput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   LoopInput
		wantErr bool
	}{
		{
			name: "valid loop input",
			input: LoopInput{
				Items:    []string{"a", "b", "c"},
				Template: ContainerExecutionInput{Image: "alpine:latest"},
			},
			wantErr: false,
		},
		{
			name: "valid with parallel and fail_fast",
			input: LoopInput{
				Items:           []string{"a"},
				Template:        ContainerExecutionInput{Image: "alpine:latest"},
				Parallel:        true,
				MaxConcurrency:  2,
				FailureStrategy: "fail_fast",
			},
			wantErr: false,
		},
		{
			name: "invalid - empty items",
			input: LoopInput{
				Items:    []string{},
				Template: ContainerExecutionInput{Image: "alpine:latest"},
			},
			wantErr: true,
		},
		{
			name: "invalid - nil items",
			input: LoopInput{
				Template: ContainerExecutionInput{Image: "alpine:latest"},
			},
			wantErr: true,
		},
		{
			name: "invalid - missing template image",
			input: LoopInput{
				Items:    []string{"a"},
				Template: ContainerExecutionInput{},
			},
			wantErr: true,
		},
		{
			name: "invalid - bad failure strategy",
			input: LoopInput{
				Items:           []string{"a"},
				Template:        ContainerExecutionInput{Image: "alpine:latest"},
				FailureStrategy: "invalid",
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
		errMsg  string
	}{
		{
			name: "valid parameterized loop",
			input: ParameterizedLoopInput{
				Parameters: map[string][]string{
					"os":   {"linux", "darwin"},
					"arch": {"amd64", "arm64"},
				},
				Template: ContainerExecutionInput{Image: "alpine:latest"},
			},
			wantErr: false,
		},
		{
			name: "valid with single parameter",
			input: ParameterizedLoopInput{
				Parameters: map[string][]string{
					"version": {"1.0", "2.0"},
				},
				Template:        ContainerExecutionInput{Image: "alpine:latest"},
				Parallel:        true,
				FailureStrategy: "continue",
			},
			wantErr: false,
		},
		{
			name: "invalid - nil parameters",
			input: ParameterizedLoopInput{
				Template: ContainerExecutionInput{Image: "alpine:latest"},
			},
			wantErr: true,
		},
		{
			name: "invalid - empty parameter array",
			input: ParameterizedLoopInput{
				Parameters: map[string][]string{
					"os": {},
				},
				Template: ContainerExecutionInput{Image: "alpine:latest"},
			},
			wantErr: true,
			errMsg:  "cannot be empty",
		},
		{
			name: "invalid - missing template image",
			input: ParameterizedLoopInput{
				Parameters: map[string][]string{
					"os": {"linux"},
				},
				Template: ContainerExecutionInput{},
			},
			wantErr: true,
		},
		{
			name: "invalid - bad failure strategy",
			input: ParameterizedLoopInput{
				Parameters: map[string][]string{
					"os": {"linux"},
				},
				Template:        ContainerExecutionInput{Image: "alpine:latest"},
				FailureStrategy: "invalid",
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
			if tt.errMsg != "" && err != nil {
				assert.Contains(t, err.Error(), tt.errMsg)
			}
		})
	}
}
```

**Step 2: Run tests to verify they pass**

Run: `cd /Users/jasoet/Documents/Go/go-wf && go test ./docker/payload/ -run "TestLoopInput_Validate|TestParameterizedLoopInput_Validate" -v`
Expected: All PASS

**Step 3: Commit**

```bash
git add docker/payload/payloads_test.go
git commit -m "test(payload): add LoopInput and ParameterizedLoopInput validation tests"
```

---

### Task 2: Payload — DAGWorkflowInput Validation and Extended Struct Tests

**Files:**
- Create: `docker/payload/payloads_extended_test.go`

**Step 1: Write the tests**

Create `docker/payload/payloads_extended_test.go`:

```go
package payload

import (
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
)

func TestDAGWorkflowInput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   DAGWorkflowInput
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid DAG with single node",
			input: DAGWorkflowInput{
				Nodes: []DAGNode{
					{
						Name: "build",
						Container: ExtendedContainerInput{
							ContainerExecutionInput: ContainerExecutionInput{Image: "alpine:latest"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid DAG with dependencies",
			input: DAGWorkflowInput{
				Nodes: []DAGNode{
					{
						Name: "build",
						Container: ExtendedContainerInput{
							ContainerExecutionInput: ContainerExecutionInput{Image: "alpine:latest"},
						},
					},
					{
						Name: "test",
						Container: ExtendedContainerInput{
							ContainerExecutionInput: ContainerExecutionInput{Image: "alpine:latest"},
						},
						Dependencies: []string{"build"},
					},
				},
				FailFast: true,
			},
			wantErr: false,
		},
		{
			name:    "invalid - empty nodes",
			input:   DAGWorkflowInput{},
			wantErr: true,
			errMsg:  "at least one node is required",
		},
		{
			name: "invalid - dependency not found",
			input: DAGWorkflowInput{
				Nodes: []DAGNode{
					{
						Name: "test",
						Container: ExtendedContainerInput{
							ContainerExecutionInput: ContainerExecutionInput{Image: "alpine:latest"},
						},
						Dependencies: []string{"non-existent"},
					},
				},
			},
			wantErr: true,
			errMsg:  "dependency node not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("DAGWorkflowInput.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.errMsg != "" && err != nil {
				assert.Contains(t, err.Error(), tt.errMsg)
			}
		})
	}
}

func TestArtifact_Validation(t *testing.T) {
	validate := validator.New()

	tests := []struct {
		name    string
		input   Artifact
		wantErr bool
	}{
		{
			name:    "valid file artifact",
			input:   Artifact{Name: "output", Path: "/tmp/output.tar", Type: "file"},
			wantErr: false,
		},
		{
			name:    "valid directory artifact",
			input:   Artifact{Name: "logs", Path: "/var/log", Type: "directory"},
			wantErr: false,
		},
		{
			name:    "valid archive artifact",
			input:   Artifact{Name: "bundle", Path: "/tmp/bundle.tar.gz", Type: "archive"},
			wantErr: false,
		},
		{
			name:    "invalid - missing name",
			input:   Artifact{Path: "/tmp/file", Type: "file"},
			wantErr: true,
		},
		{
			name:    "invalid - missing path",
			input:   Artifact{Name: "test", Type: "file"},
			wantErr: true,
		},
		{
			name:    "invalid - bad type",
			input:   Artifact{Name: "test", Path: "/tmp", Type: "invalid"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.Struct(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Artifact validation error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSecretReference_Validation(t *testing.T) {
	validate := validator.New()

	tests := []struct {
		name    string
		input   SecretReference
		wantErr bool
	}{
		{
			name:    "valid secret reference",
			input:   SecretReference{Name: "db-secret", Key: "password", EnvVar: "DB_PASSWORD"},
			wantErr: false,
		},
		{
			name:    "invalid - missing name",
			input:   SecretReference{Key: "password", EnvVar: "DB_PASSWORD"},
			wantErr: true,
		},
		{
			name:    "invalid - missing key",
			input:   SecretReference{Name: "db-secret", EnvVar: "DB_PASSWORD"},
			wantErr: true,
		},
		{
			name:    "invalid - missing env_var",
			input:   SecretReference{Name: "db-secret", Key: "password"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.Struct(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("SecretReference validation error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOutputDefinition_Validation(t *testing.T) {
	validate := validator.New()

	tests := []struct {
		name    string
		input   OutputDefinition
		wantErr bool
	}{
		{
			name:    "valid stdout output",
			input:   OutputDefinition{Name: "build-id", ValueFrom: "stdout"},
			wantErr: false,
		},
		{
			name:    "valid stderr output",
			input:   OutputDefinition{Name: "errors", ValueFrom: "stderr"},
			wantErr: false,
		},
		{
			name:    "valid exitCode output",
			input:   OutputDefinition{Name: "code", ValueFrom: "exitCode"},
			wantErr: false,
		},
		{
			name:    "valid file output",
			input:   OutputDefinition{Name: "result", ValueFrom: "file", Path: "/tmp/result.json"},
			wantErr: false,
		},
		{
			name:    "invalid - missing name",
			input:   OutputDefinition{ValueFrom: "stdout"},
			wantErr: true,
		},
		{
			name:    "invalid - missing value_from",
			input:   OutputDefinition{Name: "test"},
			wantErr: true,
		},
		{
			name:    "invalid - bad value_from",
			input:   OutputDefinition{Name: "test", ValueFrom: "invalid"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.Struct(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("OutputDefinition validation error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestInputMapping_Validation(t *testing.T) {
	validate := validator.New()

	tests := []struct {
		name    string
		input   InputMapping
		wantErr bool
	}{
		{
			name:    "valid input mapping",
			input:   InputMapping{Name: "BUILD_ID", From: "build.build-id"},
			wantErr: false,
		},
		{
			name:    "valid with default",
			input:   InputMapping{Name: "VERSION", From: "build.version", Default: "latest"},
			wantErr: false,
		},
		{
			name:    "invalid - missing name",
			input:   InputMapping{From: "build.id"},
			wantErr: true,
		},
		{
			name:    "invalid - missing from",
			input:   InputMapping{Name: "BUILD_ID"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.Struct(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("InputMapping validation error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestWorkflowParameter_Validation(t *testing.T) {
	validate := validator.New()

	tests := []struct {
		name    string
		input   WorkflowParameter
		wantErr bool
	}{
		{
			name:    "valid parameter",
			input:   WorkflowParameter{Name: "env", Value: "production"},
			wantErr: false,
		},
		{
			name:    "valid with description",
			input:   WorkflowParameter{Name: "env", Value: "prod", Description: "Target environment", Required: true},
			wantErr: false,
		},
		{
			name:    "invalid - missing name",
			input:   WorkflowParameter{Value: "production"},
			wantErr: true,
		},
		{
			name:    "invalid - missing value",
			input:   WorkflowParameter{Name: "env"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.Struct(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("WorkflowParameter validation error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDAGNode_Validation(t *testing.T) {
	validate := validator.New()

	tests := []struct {
		name    string
		input   DAGNode
		wantErr bool
	}{
		{
			name: "valid node",
			input: DAGNode{
				Name: "build",
				Container: ExtendedContainerInput{
					ContainerExecutionInput: ContainerExecutionInput{Image: "alpine:latest"},
				},
			},
			wantErr: false,
		},
		{
			name: "valid with dependencies",
			input: DAGNode{
				Name: "test",
				Container: ExtendedContainerInput{
					ContainerExecutionInput: ContainerExecutionInput{Image: "alpine:latest"},
				},
				Dependencies: []string{"build"},
			},
			wantErr: false,
		},
		{
			name: "invalid - missing name",
			input: DAGNode{
				Container: ExtendedContainerInput{
					ContainerExecutionInput: ContainerExecutionInput{Image: "alpine:latest"},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.Struct(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("DAGNode validation error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
```

**Step 2: Run tests to verify they pass**

Run: `cd /Users/jasoet/Documents/Go/go-wf && go test ./docker/payload/ -v`
Expected: All PASS

**Step 3: Commit**

```bash
git add docker/payload/payloads_extended_test.go
git commit -m "test(payload): add validation tests for DAG, Artifact, Secret, Output, Input, and Parameter structs"
```

---

### Task 3: Operations — SubmitWorkflow Error Paths

**Files:**
- Modify: `docker/operations_test.go`

**Step 1: Write the tests**

Add to `docker/operations_test.go`:

```go
func TestSubmitWorkflow_PointerTypes(t *testing.T) {
	mockClient := new(mocks.Client)
	mockWorkflowRun := new(mocks.WorkflowRun)

	mockWorkflowRun.On("GetID").Return("workflow-123")
	mockWorkflowRun.On("GetRunID").Return("run-456")
	mockClient.On("ExecuteWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(mockWorkflowRun, nil)

	tests := []struct {
		name  string
		input interface{}
	}{
		{
			name:  "pointer ContainerExecutionInput",
			input: &payload.ContainerExecutionInput{Image: "alpine:latest"},
		},
		{
			name: "pointer PipelineInput",
			input: &payload.PipelineInput{
				Containers: []payload.ContainerExecutionInput{{Image: "alpine:latest"}},
			},
		},
		{
			name: "pointer ParallelInput",
			input: &payload.ParallelInput{
				Containers: []payload.ContainerExecutionInput{{Image: "alpine:latest"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, err := SubmitWorkflow(context.Background(), mockClient, tt.input, "docker-queue")
			assert.NoError(t, err)
			assert.NotNil(t, status)
			assert.Equal(t, "Running", status.Status)
		})
	}
}

func TestSubmitWorkflow_UnsupportedType(t *testing.T) {
	mockClient := new(mocks.Client)

	status, err := SubmitWorkflow(context.Background(), mockClient, "invalid-input", "docker-queue")
	assert.Error(t, err)
	assert.Nil(t, status)
	assert.Contains(t, err.Error(), "unsupported workflow input type")
}

func TestSubmitWorkflow_ExecuteError(t *testing.T) {
	mockClient := new(mocks.Client)

	mockClient.On("ExecuteWorkflow",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
	).Return(nil, fmt.Errorf("temporal unavailable"))

	input := payload.ContainerExecutionInput{Image: "alpine:latest"}
	status, err := SubmitWorkflow(context.Background(), mockClient, input, "docker-queue")
	assert.Error(t, err)
	assert.Nil(t, status)
	assert.Contains(t, err.Error(), "failed to start workflow")
}
```

Note: add `"fmt"` to the imports.

**Step 2: Run tests to verify they pass**

Run: `cd /Users/jasoet/Documents/Go/go-wf && go test ./docker/ -run "TestSubmitWorkflow_" -v`
Expected: All PASS

**Step 3: Commit**

```bash
git add docker/operations_test.go
git commit -m "test(operations): add SubmitWorkflow error path and pointer type tests"
```

---

### Task 4: Operations — SubmitAndWait and GetWorkflowStatus Error Paths

**Files:**
- Modify: `docker/operations_test.go`

**Step 1: Write the tests**

Add to `docker/operations_test.go`:

```go
func TestSubmitAndWait_SubmitError(t *testing.T) {
	mockClient := new(mocks.Client)

	// SubmitWorkflow will fail due to unsupported type
	status, err := SubmitAndWait(context.Background(), mockClient, "invalid", "queue", 1*time.Minute)
	assert.Error(t, err)
	assert.Nil(t, status)
}

func TestSubmitAndWait_GetError(t *testing.T) {
	mockClient := new(mocks.Client)
	mockWorkflowRun := new(mocks.WorkflowRun)

	mockWorkflowRun.On("GetID").Return("workflow-123")
	mockWorkflowRun.On("GetRunID").Return("run-456")
	mockWorkflowRun.On("Get", mock.Anything, mock.Anything).Return(fmt.Errorf("workflow failed"))

	mockClient.On("ExecuteWorkflow",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
	).Return(mockWorkflowRun, nil)
	mockClient.On("GetWorkflow", mock.Anything, "workflow-123", "run-456").Return(mockWorkflowRun)

	input := payload.ContainerExecutionInput{Image: "alpine:latest"}
	status, err := SubmitAndWait(context.Background(), mockClient, input, "queue", 1*time.Minute)
	assert.Error(t, err)
	assert.NotNil(t, status)
	assert.Equal(t, "Failed", status.Status)
	assert.NotNil(t, status.CloseTime)
}

func TestGetWorkflowStatus_Error(t *testing.T) {
	mockClient := new(mocks.Client)
	mockWorkflowRun := new(mocks.WorkflowRun)

	mockWorkflowRun.On("Get", mock.Anything, mock.Anything).Return(fmt.Errorf("workflow error"))
	mockClient.On("GetWorkflow", mock.Anything, "workflow-123", "run-456").Return(mockWorkflowRun)

	status, err := GetWorkflowStatus(context.Background(), mockClient, "workflow-123", "run-456")
	assert.NoError(t, err) // GetWorkflowStatus doesn't return error, it sets status
	assert.NotNil(t, status)
	assert.Equal(t, "Failed or Running", status.Status)
	assert.NotNil(t, status.Error)
}
```

**Step 2: Run tests to verify they pass**

Run: `cd /Users/jasoet/Documents/Go/go-wf && go test ./docker/ -run "TestSubmitAndWait_|TestGetWorkflowStatus_" -v`
Expected: All PASS

**Step 3: Commit**

```bash
git add docker/operations_test.go
git commit -m "test(operations): add SubmitAndWait and GetWorkflowStatus error path tests"
```

---

### Task 5: Operations — WatchWorkflow and QueryWorkflow

**Files:**
- Modify: `docker/operations_test.go`

**Step 1: Write the tests**

Replace the skipped `TestQueryWorkflow` and add `TestWatchWorkflow_ContextCancellation`:

```go
func TestWatchWorkflow_ContextCancellation(t *testing.T) {
	mockClient := new(mocks.Client)
	mockWorkflowRun := new(mocks.WorkflowRun)

	// Mock a still-running workflow (Get returns error meaning not completed)
	mockWorkflowRun.On("Get", mock.Anything, mock.Anything).Return(fmt.Errorf("workflow still running"))
	mockClient.On("GetWorkflow", mock.Anything, "workflow-123", "run-456").Return(mockWorkflowRun)

	ctx, cancel := context.WithCancel(context.Background())
	updates := make(chan *WorkflowStatus, 10)

	// Cancel context before first tick
	cancel()

	err := WatchWorkflow(ctx, mockClient, "workflow-123", "run-456", updates)
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}
```

Replace the skipped `TestQueryWorkflow` with a working version. The `client.QueryWorkflow` returns `converter.EncodedValue`. The Temporal mocks package provides a mock for this:

```go
func TestQueryWorkflow(t *testing.T) {
	mockClient := new(mocks.Client)

	t.Run("success", func(t *testing.T) {
		mockEncodedValue := &mocks.EncodedValue{}
		mockEncodedValue.On("Get", mock.Anything).Return(nil).Run(func(args mock.Arguments) {
			// Set the result pointer
			ptr := args.Get(0).(*string)
			*ptr = "running"
		})
		mockClient.On("QueryWorkflow", mock.Anything, "workflow-123", "run-456", "status").Return(mockEncodedValue, nil).Once()

		var result string
		err := QueryWorkflow(context.Background(), mockClient, "workflow-123", "run-456", "status", &result)
		assert.NoError(t, err)
		assert.Equal(t, "running", result)
	})

	t.Run("query error", func(t *testing.T) {
		mockClient.On("QueryWorkflow", mock.Anything, "wf-err", "run-err", "status").Return(nil, fmt.Errorf("query failed")).Once()

		var result string
		err := QueryWorkflow(context.Background(), mockClient, "wf-err", "run-err", "status", &result)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "query failed")
	})
}
```

Note: Check if `mocks.EncodedValue` exists in the Temporal SDK mocks. If not, we'll need to create a simple mock struct implementing `converter.EncodedValue`. The interface has two methods: `Get(valuePtr interface{}) error` and `HasValue() bool`. If the SDK mock doesn't exist, add this to the test file:

```go
// mockEncodedValue implements converter.EncodedValue for testing.
type mockEncodedValue struct {
	mock.Mock
}

func (m *mockEncodedValue) HasValue() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *mockEncodedValue) Get(valuePtr interface{}) error {
	args := m.Called(valuePtr)
	return args.Error(0)
}
```

**Step 2: Run tests to verify they pass**

Run: `cd /Users/jasoet/Documents/Go/go-wf && go test ./docker/ -run "TestQueryWorkflow|TestWatchWorkflow_Context" -v`
Expected: All PASS

**Step 3: Commit**

```bash
git add docker/operations_test.go
git commit -m "test(operations): add QueryWorkflow and WatchWorkflow context cancellation tests"
```

---

### Task 6: Activity — Integration Tests with Real Podman

**Files:**
- Create: `docker/activity/container_integration_test.go`

**Step 1: Write the integration tests**

Create `docker/activity/container_integration_test.go`:

```go
//go:build integration

package activity

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"

	"github.com/jasoet/go-wf/docker/payload"
)

func TestStartContainerActivity_HappyPath(t *testing.T) {
	var env testsuite.TestActivityEnvironment
	suite := &testsuite.WorkflowTestSuite{}
	env = *suite.NewTestActivityEnvironment()
	env.RegisterActivity(StartContainerActivity)

	input := payload.ContainerExecutionInput{
		Image:      "alpine:latest",
		Command:    []string{"echo", "hello world"},
		AutoRemove: true,
	}

	result, err := env.ExecuteActivity(StartContainerActivity, input)
	require.NoError(t, err)

	var output payload.ContainerExecutionOutput
	require.NoError(t, result.Get(&output))

	assert.Equal(t, 0, output.ExitCode)
	assert.True(t, output.Success)
	assert.Contains(t, output.Stdout, "hello world")
	assert.NotEmpty(t, output.ContainerID)
	assert.NotZero(t, output.Duration)
}

func TestStartContainerActivity_WithEnv(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestActivityEnvironment()
	env.RegisterActivity(StartContainerActivity)

	input := payload.ContainerExecutionInput{
		Image:   "alpine:latest",
		Command: []string{"sh", "-c", "echo $MY_VAR"},
		Env: map[string]string{
			"MY_VAR": "test-value",
		},
		AutoRemove: true,
	}

	result, err := env.ExecuteActivity(StartContainerActivity, input)
	require.NoError(t, err)

	var output payload.ContainerExecutionOutput
	require.NoError(t, result.Get(&output))

	assert.Equal(t, 0, output.ExitCode)
	assert.True(t, output.Success)
	assert.Contains(t, output.Stdout, "test-value")
}

func TestStartContainerActivity_WithEntrypoint(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestActivityEnvironment()
	env.RegisterActivity(StartContainerActivity)

	input := payload.ContainerExecutionInput{
		Image:      "alpine:latest",
		Entrypoint: []string{"sh", "-c"},
		Command:    []string{"echo entrypoint-test"},
		AutoRemove: true,
	}

	result, err := env.ExecuteActivity(StartContainerActivity, input)
	require.NoError(t, err)

	var output payload.ContainerExecutionOutput
	require.NoError(t, result.Get(&output))

	assert.Equal(t, 0, output.ExitCode)
	assert.True(t, output.Success)
	assert.Contains(t, output.Stdout, "entrypoint-test")
}

func TestStartContainerActivity_Failure(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestActivityEnvironment()
	env.RegisterActivity(StartContainerActivity)

	input := payload.ContainerExecutionInput{
		Image:      "alpine:latest",
		Command:    []string{"sh", "-c", "exit 42"},
		AutoRemove: true,
	}

	result, err := env.ExecuteActivity(StartContainerActivity, input)
	// The activity returns the output AND an error on non-zero exit
	// Check if we get an error or a result with non-zero exit code
	if err != nil {
		// Activity returned error - check if it wraps useful info
		assert.Error(t, err)
		return
	}

	var output payload.ContainerExecutionOutput
	require.NoError(t, result.Get(&output))

	assert.NotEqual(t, 0, output.ExitCode)
	assert.False(t, output.Success)
}

func TestStartContainerActivity_WithWorkDir(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestActivityEnvironment()
	env.RegisterActivity(StartContainerActivity)

	input := payload.ContainerExecutionInput{
		Image:      "alpine:latest",
		Command:    []string{"pwd"},
		WorkDir:    "/tmp",
		AutoRemove: true,
	}

	result, err := env.ExecuteActivity(StartContainerActivity, input)
	require.NoError(t, err)

	var output payload.ContainerExecutionOutput
	require.NoError(t, result.Get(&output))

	assert.Equal(t, 0, output.ExitCode)
	assert.Contains(t, output.Stdout, "/tmp")
}

func TestStartContainerActivity_WithLabels(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestActivityEnvironment()
	env.RegisterActivity(StartContainerActivity)

	input := payload.ContainerExecutionInput{
		Image:   "alpine:latest",
		Command: []string{"echo", "labeled"},
		Labels: map[string]string{
			"test-label": "test-value",
		},
		AutoRemove: true,
	}

	result, err := env.ExecuteActivity(StartContainerActivity, input)
	require.NoError(t, err)

	var output payload.ContainerExecutionOutput
	require.NoError(t, result.Get(&output))

	assert.Equal(t, 0, output.ExitCode)
	assert.True(t, output.Success)
}

func TestStartContainerActivity_WithName(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestActivityEnvironment()
	env.RegisterActivity(StartContainerActivity)

	ctx := context.Background()
	_ = ctx

	input := payload.ContainerExecutionInput{
		Image:      "alpine:latest",
		Command:    []string{"echo", "named"},
		Name:       "test-named-container",
		AutoRemove: true,
	}

	result, err := env.ExecuteActivity(StartContainerActivity, input)
	require.NoError(t, err)

	var output payload.ContainerExecutionOutput
	require.NoError(t, result.Get(&output))

	assert.Equal(t, 0, output.ExitCode)
	assert.True(t, output.Success)
}

func TestStartContainerActivity_WithWaitStrategy(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestActivityEnvironment()
	env.RegisterActivity(StartContainerActivity)

	input := payload.ContainerExecutionInput{
		Image:   "alpine:latest",
		Command: []string{"sh", "-c", "echo 'ready to serve' && sleep 0.1"},
		WaitStrategy: payload.WaitStrategyConfig{
			Type:       "log",
			LogMessage: "ready to serve",
		},
		AutoRemove: true,
	}

	result, err := env.ExecuteActivity(StartContainerActivity, input)
	require.NoError(t, err)

	var output payload.ContainerExecutionOutput
	require.NoError(t, result.Get(&output))

	assert.Equal(t, 0, output.ExitCode)
	assert.True(t, output.Success)
}

func TestStartContainerActivity_Stderr(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestActivityEnvironment()
	env.RegisterActivity(StartContainerActivity)

	input := payload.ContainerExecutionInput{
		Image:      "alpine:latest",
		Command:    []string{"sh", "-c", "echo error-output >&2"},
		AutoRemove: true,
	}

	result, err := env.ExecuteActivity(StartContainerActivity, input)
	require.NoError(t, err)

	var output payload.ContainerExecutionOutput
	require.NoError(t, result.Get(&output))

	assert.Equal(t, 0, output.ExitCode)
	assert.True(t, output.Success)
	assert.Contains(t, output.Stderr, "error-output")
}
```

**Step 2: Run integration tests to verify**

Run: `cd /Users/jasoet/Documents/Go/go-wf && go test ./docker/activity/ -tags integration -run "TestStartContainerActivity" -v -timeout 5m`
Expected: All PASS (requires Podman running)

**Step 3: Commit**

```bash
git add docker/activity/container_integration_test.go
git commit -m "test(activity): add integration tests for StartContainerActivity with real containers"
```

---

### Task 7: Artifacts — ExtractArchive Error Paths

**Files:**
- Modify: `workflow/artifacts/local_test.go`

**Step 1: Write the tests**

Add to `workflow/artifacts/local_test.go`:

```go
func TestExtractArchive_InvalidGzip(t *testing.T) {
	destDir := t.TempDir()

	// Pass invalid gzip data
	err := ExtractArchive(bytes.NewReader([]byte("not gzip data")), destDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create gzip reader")
}

func TestExtractArchive_DirectoryTraversal(t *testing.T) {
	// Create a malicious archive with path traversal
	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzWriter)

	// Add a file with directory traversal path
	header := &tar.Header{
		Name: "../../etc/passwd",
		Mode: 0o600,
		Size: 4,
	}
	err := tarWriter.WriteHeader(header)
	require.NoError(t, err)
	_, err = tarWriter.Write([]byte("evil"))
	require.NoError(t, err)

	require.NoError(t, tarWriter.Close())
	require.NoError(t, gzWriter.Close())

	destDir := t.TempDir()
	err = ExtractArchive(&buf, destDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "illegal file path")
}

func TestArchiveDirectory_NonExistentSource(t *testing.T) {
	var buf bytes.Buffer
	err := ArchiveDirectory("/non/existent/directory", &buf)
	assert.Error(t, err)
}

func TestLocalFileStore_DownloadNonNotExistError(t *testing.T) {
	// Use a path where the file name is too long to trigger non-NotExist error
	tmpDir := t.TempDir()
	store, err := NewLocalFileStore(tmpDir)
	require.NoError(t, err)

	ctx := context.Background()

	// Upload a file first
	metadata := ArtifactMetadata{
		Name:       "test",
		WorkflowID: "wf-1",
		RunID:      "run-1",
		StepName:   "step-1",
	}
	err = store.Upload(ctx, metadata, bytes.NewReader([]byte("test")))
	require.NoError(t, err)

	// Make the file unreadable
	fullPath := filepath.Join(tmpDir, metadata.StorageKey())
	err = os.Chmod(fullPath, 0o000)
	require.NoError(t, err)
	defer os.Chmod(fullPath, 0o644) //nolint:errcheck // cleanup

	// Try to download - should get permission error, not "not found"
	reader, err := store.Download(ctx, metadata)
	assert.Error(t, err)
	assert.Nil(t, reader)
	assert.Contains(t, err.Error(), "failed to open file")
}
```

Note: Add `"archive/tar"`, `"compress/gzip"` to imports.

**Step 2: Run tests to verify**

Run: `cd /Users/jasoet/Documents/Go/go-wf && go test ./workflow/artifacts/ -run "TestExtractArchive_|TestArchiveDirectory_NonExistent|TestLocalFileStore_DownloadNonNotExist" -v`
Expected: All PASS

**Step 3: Commit**

```bash
git add workflow/artifacts/local_test.go
git commit -m "test(artifacts): add archive error paths and filesystem error tests"
```

---

### Task 8: Artifacts — Upload/Download Activity Error Paths

**Files:**
- Modify: `workflow/artifacts/local_test.go`

**Step 1: Write the tests**

Add to `workflow/artifacts/local_test.go`:

```go
func TestUploadArtifactActivity_SourceStatError(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewLocalFileStore(tmpDir)
	require.NoError(t, err)

	ctx := context.Background()

	// Auto-detect type with non-existent source should fail at stat
	input := UploadArtifactInput{
		Metadata: ArtifactMetadata{
			Name:       "test",
			WorkflowID: "wf-1",
			RunID:      "run-1",
			StepName:   "step-1",
			// No type - will try to auto-detect via stat
		},
		SourcePath: "/non/existent/path",
	}

	err = UploadArtifactActivity(ctx, store, input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to stat source path")
}

func TestUploadFile_OpenError(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewLocalFileStore(tmpDir)
	require.NoError(t, err)

	ctx := context.Background()

	input := UploadArtifactInput{
		Metadata: ArtifactMetadata{
			Name:       "test",
			WorkflowID: "wf-1",
			RunID:      "run-1",
			StepName:   "step-1",
			Type:       "file",
		},
		SourcePath: "/non/existent/file.txt",
	}

	err = UploadArtifactActivity(ctx, store, input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open source file")
}

func TestUploadDirectory_ArchiveError(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewLocalFileStore(tmpDir)
	require.NoError(t, err)

	ctx := context.Background()

	input := UploadArtifactInput{
		Metadata: ArtifactMetadata{
			Name:       "test",
			WorkflowID: "wf-1",
			RunID:      "run-1",
			StepName:   "step-1",
			Type:       "directory",
		},
		SourcePath: "/non/existent/directory",
	}

	err = UploadArtifactActivity(ctx, store, input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to archive directory")
}

func TestDownloadArtifactActivity_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewLocalFileStore(tmpDir)
	require.NoError(t, err)

	ctx := context.Background()

	input := DownloadArtifactInput{
		Metadata: ArtifactMetadata{
			Name:       "non-existent",
			WorkflowID: "wf-1",
			RunID:      "run-1",
			StepName:   "step-1",
			Type:       "file",
		},
		DestPath: filepath.Join(t.TempDir(), "dest.txt"),
	}

	err = DownloadArtifactActivity(ctx, store, input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to download artifact")
}

func TestCleanupWorkflowArtifacts_EmptyWorkflow(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewLocalFileStore(tmpDir)
	require.NoError(t, err)

	ctx := context.Background()

	// Cleanup non-existent workflow should succeed (no artifacts to delete)
	err = CleanupWorkflowArtifacts(ctx, store, "non-existent-wf", "non-existent-run")
	assert.NoError(t, err)
}

func TestLocalFileStore_ExistsNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewLocalFileStore(tmpDir)
	require.NoError(t, err)

	ctx := context.Background()

	exists, err := store.Exists(ctx, ArtifactMetadata{
		Name:       "nothing",
		WorkflowID: "wf",
		RunID:      "run",
		StepName:   "step",
	})
	assert.NoError(t, err)
	assert.False(t, exists)
}

func TestLocalFileStore_ListWithFiles(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewLocalFileStore(tmpDir)
	require.NoError(t, err)

	ctx := context.Background()

	// Create files with insufficient path depth (less than 4 parts) to test skip logic
	shallowDir := filepath.Join(tmpDir, "wf-1", "run-1")
	require.NoError(t, os.MkdirAll(shallowDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(shallowDir, "shallow-file"), []byte("data"), 0o600))

	// Also create a proper 4-level file
	properDir := filepath.Join(tmpDir, "wf-1", "run-1", "step-1")
	require.NoError(t, os.MkdirAll(properDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(properDir, "proper-file"), []byte("data"), 0o600))

	listed, err := store.List(ctx, "wf-1/")
	require.NoError(t, err)

	// Should only contain the proper 4-level file, skipping the shallow one
	assert.Len(t, listed, 1)
	assert.Equal(t, "proper-file", listed[0].Name)
}
```

**Step 2: Run tests to verify**

Run: `cd /Users/jasoet/Documents/Go/go-wf && go test ./workflow/artifacts/ -run "TestUploadArtifactActivity_Source|TestUploadFile_Open|TestUploadDirectory_Archive|TestDownloadArtifactActivity_NotFound|TestCleanup.*Empty|TestLocalFileStore_Exists|TestLocalFileStore_ListWith" -v`
Expected: All PASS

**Step 3: Commit**

```bash
git add workflow/artifacts/local_test.go
git commit -m "test(artifacts): add upload/download error paths and list skip logic tests"
```

---

### Task 9: Run Full Test Suite and Verify Coverage

**Step 1: Run unit tests with coverage**

Run: `cd /Users/jasoet/Documents/Go/go-wf && task test:unit`
Expected: All PASS, coverage improved

**Step 2: Run full test suite (unit + integration)**

Run: `cd /Users/jasoet/Documents/Go/go-wf && task test`
Expected: All PASS, overall coverage >= 85%

**Step 3: Run lint**

Run: `cd /Users/jasoet/Documents/Go/go-wf && task lint`
Expected: No errors

**Step 4: Format code**

Run: `cd /Users/jasoet/Documents/Go/go-wf && task fmt`

**Step 5: Fix any issues found and commit**

If lint or formatting issues exist, fix them and commit:

```bash
git add -A
git commit -m "style: fix lint and formatting issues in new tests"
```

---

### Task 10: Final Commit and Branch Cleanup

**Step 1: Verify clean state**

Run: `git status`
Expected: Clean working tree

**Step 2: Push branch**

Run: `git push`

**Step 3: Create PR or update existing one**

Use `gh pr create` or push to existing branch for review.
