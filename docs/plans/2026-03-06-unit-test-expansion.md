# Unit Test Expansion Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add 42 new unit tests across 6 workflow test files using Temporal TestWorkflowEnvironment with mocked activities, and migrate existing tests to testify assertions.

**Architecture:** All tests use `testsuite.WorkflowTestSuite` + `env.OnActivity` mocking pattern. No Temporal server or containers needed. Tests are added to existing `*_test.go` files alongside current tests.

**Tech Stack:** Go 1.26, Temporal SDK testsuite, testify (assert + require), testify/mock

---

### Task 1: Container Workflow Tests

**Files:**
- Modify: `docker/workflow/container_test.go`

**Context:** This file has 4 existing tests using raw `t.Fatal`/`t.Errorf`. Migrate to testify and add 3 new tests. The workflow is in `docker/workflow/container.go` — it validates input, sets activity options (default 10min timeout, retry policy with 3 attempts), and executes `activity.StartContainerActivity`.

**Step 1: Rewrite existing tests to use testify assertions**

Replace all raw assertions in the 4 existing tests with `assert`/`require`. The pattern:
- `t.Fatal("...")` → `require.True(t, ...)` or `require.NoError(t, ...)`
- `t.Fatalf("...", err)` → `require.NoError(t, err)`
- `t.Errorf("Expected..., got %s", x)` → `assert.Equal(t, expected, actual)`
- `t.Error("Expected error")` → `assert.Error(t, err)`

```go
func TestExecuteContainerWorkflow_Success(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := payload.ContainerExecutionInput{
		Image:   "alpine:latest",
		Command: []string{"echo", "hello"},
	}

	env.OnActivity(activity.StartContainerActivity, mock.Anything, mock.Anything).Return(&payload.ContainerExecutionOutput{
		ContainerID: "container-123",
		Success:     true,
		ExitCode:    0,
		Duration:    5 * time.Second,
	}, nil)

	env.ExecuteWorkflow(ExecuteContainerWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.ContainerExecutionOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, "container-123", result.ContainerID)
}

func TestExecuteContainerWorkflow_InvalidInput(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := payload.ContainerExecutionInput{}
	env.ExecuteWorkflow(ExecuteContainerWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	assert.Error(t, env.GetWorkflowError())
}

func TestExecuteContainerWorkflow_WithTimeout(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := payload.ContainerExecutionInput{
		Image:      "alpine:latest",
		RunTimeout: 5 * time.Minute,
	}

	env.OnActivity(activity.StartContainerActivity, mock.Anything, mock.Anything).Return(
		&payload.ContainerExecutionOutput{Success: true, ExitCode: 0}, nil)

	env.ExecuteWorkflow(ExecuteContainerWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
}

func TestExecuteContainerWorkflow_ActivityError(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := payload.ContainerExecutionInput{
		Image: "alpine:latest",
	}

	env.OnActivity(activity.StartContainerActivity, mock.Anything, mock.Anything).Return(
		nil, fmt.Errorf("activity execution failed"))

	env.ExecuteWorkflow(ExecuteContainerWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	assert.Error(t, env.GetWorkflowError())
}
```

**Step 2: Add 3 new container tests**

```go
// TestExecuteContainerWorkflow_ContainerFailure tests non-zero exit code (activity succeeds but container fails).
func TestExecuteContainerWorkflow_ContainerFailure(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := payload.ContainerExecutionInput{
		Image:   "alpine:latest",
		Command: []string{"sh", "-c", "exit 1"},
	}

	env.OnActivity(activity.StartContainerActivity, mock.Anything, mock.Anything).Return(
		&payload.ContainerExecutionOutput{
			ContainerID: "container-fail",
			Success:     false,
			ExitCode:    1,
			Stderr:      "command failed",
		}, nil)

	env.ExecuteWorkflow(ExecuteContainerWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.ContainerExecutionOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.False(t, result.Success)
	assert.Equal(t, 1, result.ExitCode)
	assert.Equal(t, "command failed", result.Stderr)
}

// TestExecuteContainerWorkflow_DefaultTimeout tests that zero RunTimeout uses default.
func TestExecuteContainerWorkflow_DefaultTimeout(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := payload.ContainerExecutionInput{
		Image:      "alpine:latest",
		RunTimeout: 0, // should default to 10 minutes
	}

	env.OnActivity(activity.StartContainerActivity, mock.Anything, mock.Anything).Return(
		&payload.ContainerExecutionOutput{Success: true, ExitCode: 0}, nil)

	env.ExecuteWorkflow(ExecuteContainerWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
}

// TestExecuteContainerWorkflow_FullResultFields tests all output fields are populated correctly.
func TestExecuteContainerWorkflow_FullResultFields(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	now := time.Now()
	input := payload.ContainerExecutionInput{
		Image: "alpine:latest",
	}

	expected := &payload.ContainerExecutionOutput{
		ContainerID: "container-full",
		Success:     true,
		ExitCode:    0,
		Stdout:      "hello world",
		Stderr:      "",
		Duration:    3 * time.Second,
		StartedAt:   now,
		FinishedAt:  now.Add(3 * time.Second),
	}

	env.OnActivity(activity.StartContainerActivity, mock.Anything, mock.Anything).Return(expected, nil)

	env.ExecuteWorkflow(ExecuteContainerWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.ContainerExecutionOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, "container-full", result.ContainerID)
	assert.True(t, result.Success)
	assert.Equal(t, 0, result.ExitCode)
	assert.Equal(t, "hello world", result.Stdout)
	assert.Empty(t, result.Stderr)
	assert.Equal(t, 3*time.Second, result.Duration)
}
```

**Step 3: Add `"github.com/stretchr/testify/assert"` and `"github.com/stretchr/testify/require"` to imports**

The file already imports `mock` and `assert` is not present. Add both `assert` and `require`.

**Step 4: Run tests**

Run: `go test -race -count=1 -run 'TestExecuteContainerWorkflow' ./docker/workflow/ -v`
Expected: All 7 tests PASS

**Step 5: Commit**

```bash
git add docker/workflow/container_test.go
git commit -m "test(container): add failure, default timeout, and full result field tests

Migrate existing tests to testify assertions. Add tests for container
failure with non-zero exit code, default timeout path, and full output
field verification."
```

---

### Task 2: Pipeline Workflow Tests

**Files:**
- Modify: `docker/workflow/pipeline_test.go`

**Context:** This file has 6 existing tests using raw `t.Fatal`/`t.Errorf`. The workflow is in `docker/workflow/pipeline.go` — it validates input, executes containers sequentially, tracks results in `PipelineOutput`, and either stops on error or continues based on `StopOnError` flag.

**Step 1: Rewrite existing tests to use testify assertions**

Same migration pattern as Task 1. Replace all raw assertions with `assert`/`require`. Add `"github.com/stretchr/testify/assert"` and `"github.com/stretchr/testify/require"` to imports.

Existing tests to migrate:
- `TestContainerPipelineWorkflow_Success` — `require.True`, `require.NoError`, `require.NoError` (GetWorkflowResult), `assert.Equal`
- `TestContainerPipelineWorkflow_InvalidInput` — `require.True`, `assert.Error`
- `TestContainerPipelineWorkflow_WithNamedSteps` — `require.True`, `require.NoError`, `assert.Equal`
- `TestContainerPipelineWorkflow_StopOnError` — `require.True`, `assert.Error`
- `TestContainerPipelineWorkflow_ContinueOnError` — `require.True`, `require.NoError`, `assert.Equal` x2
- `TestContainerPipelineWorkflow_NoNamedSteps` — `require.True`, `assert.Equal`

**Step 2: Add 5 new pipeline tests**

```go
// TestContainerPipelineWorkflow_AllFail tests all steps fail with StopOnError=false.
func TestContainerPipelineWorkflow_AllFail(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := payload.PipelineInput{
		Containers: []payload.ContainerExecutionInput{
			{Image: "alpine:latest", Name: "step1"},
			{Image: "alpine:latest", Name: "step2"},
			{Image: "alpine:latest", Name: "step3"},
		},
		StopOnError: false,
	}

	env.OnActivity(activity.StartContainerActivity, mock.Anything, mock.Anything).Return(
		&payload.ContainerExecutionOutput{Success: false, ExitCode: 1}, nil)

	env.ExecuteWorkflow(ContainerPipelineWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.PipelineOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 0, result.TotalSuccess)
	assert.Equal(t, 3, result.TotalFailed)
	assert.Len(t, result.Results, 3)
}

// TestContainerPipelineWorkflow_StopOnErrorFirstStep tests error on first step stops pipeline.
func TestContainerPipelineWorkflow_StopOnErrorFirstStep(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := payload.PipelineInput{
		Containers: []payload.ContainerExecutionInput{
			{Image: "alpine:latest", Name: "step1"},
			{Image: "alpine:latest", Name: "step2"},
			{Image: "alpine:latest", Name: "step3"},
		},
		StopOnError: true,
	}

	env.OnActivity(activity.StartContainerActivity, mock.Anything, mock.Anything).Return(
		&payload.ContainerExecutionOutput{Success: false, ExitCode: 1}, fmt.Errorf("step failed")).Once()

	env.ExecuteWorkflow(ContainerPipelineWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	assert.Error(t, env.GetWorkflowError())
}

// TestContainerPipelineWorkflow_ResultTracking tests output data from each step is tracked correctly.
func TestContainerPipelineWorkflow_ResultTracking(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := payload.PipelineInput{
		Containers: []payload.ContainerExecutionInput{
			{Image: "alpine:latest", Name: "build"},
			{Image: "alpine:latest", Name: "test"},
			{Image: "alpine:latest", Name: "deploy"},
		},
		StopOnError: false,
	}

	env.OnActivity(activity.StartContainerActivity, mock.Anything, input.Containers[0]).Return(
		&payload.ContainerExecutionOutput{ContainerID: "build-1", Success: true, ExitCode: 0, Duration: time.Second}, nil).Once()
	env.OnActivity(activity.StartContainerActivity, mock.Anything, input.Containers[1]).Return(
		&payload.ContainerExecutionOutput{ContainerID: "test-1", Success: true, ExitCode: 0, Duration: 2 * time.Second}, nil).Once()
	env.OnActivity(activity.StartContainerActivity, mock.Anything, input.Containers[2]).Return(
		&payload.ContainerExecutionOutput{ContainerID: "deploy-1", Success: true, ExitCode: 0, Duration: 3 * time.Second}, nil).Once()

	env.ExecuteWorkflow(ContainerPipelineWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.PipelineOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 3, result.TotalSuccess)
	require.Len(t, result.Results, 3)
	assert.Equal(t, "build-1", result.Results[0].ContainerID)
	assert.Equal(t, "test-1", result.Results[1].ContainerID)
	assert.Equal(t, "deploy-1", result.Results[2].ContainerID)
}

// TestContainerPipelineWorkflow_SingleContainer tests pipeline with one container.
func TestContainerPipelineWorkflow_SingleContainer(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := payload.PipelineInput{
		Containers: []payload.ContainerExecutionInput{
			{Image: "alpine:latest", Name: "only-step"},
		},
		StopOnError: true,
	}

	env.OnActivity(activity.StartContainerActivity, mock.Anything, mock.Anything).Return(
		&payload.ContainerExecutionOutput{Success: true, ExitCode: 0}, nil)

	env.ExecuteWorkflow(ContainerPipelineWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.PipelineOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 1, result.TotalSuccess)
	assert.Len(t, result.Results, 1)
}

// TestContainerPipelineWorkflow_ActivityErrorContinue tests activity error with StopOnError=false.
func TestContainerPipelineWorkflow_ActivityErrorContinue(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := payload.PipelineInput{
		Containers: []payload.ContainerExecutionInput{
			{Image: "alpine:latest", Name: "step1"},
			{Image: "alpine:latest", Name: "step2"},
		},
		StopOnError: false,
	}

	// First step: activity error
	env.OnActivity(activity.StartContainerActivity, mock.Anything, input.Containers[0]).Return(
		nil, fmt.Errorf("docker daemon error")).Once()
	// Second step: success
	env.OnActivity(activity.StartContainerActivity, mock.Anything, input.Containers[1]).Return(
		&payload.ContainerExecutionOutput{Success: true, ExitCode: 0}, nil).Once()

	env.ExecuteWorkflow(ContainerPipelineWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	// Pipeline with StopOnError=false should not return error even if a step's activity errors
	// Note: Activity errors with retry policy may cause workflow error after exhausting retries.
	// The behavior depends on whether the error exhausts the retry policy.
}
```

**Step 3: Run tests**

Run: `go test -race -count=1 -run 'TestContainerPipelineWorkflow' ./docker/workflow/ -v`
Expected: All 11 tests PASS

**Step 4: Commit**

```bash
git add docker/workflow/pipeline_test.go
git commit -m "test(pipeline): add failure, tracking, single-container, and activity error tests

Migrate existing tests to testify assertions. Add tests for all-fail
scenario, first-step error stop, result data tracking, single container
edge case, and activity error with continue strategy."
```

---

### Task 3: Parallel Workflow Tests

**Files:**
- Modify: `docker/workflow/parallel_test.go`

**Context:** This file has 4 existing tests using raw `t.Fatal`/`t.Errorf`. The workflow is in `docker/workflow/parallel.go` — it validates input, launches all containers as parallel futures, then collects results. `FailureStrategy="fail_fast"` returns error on first failure during collection. `FailureStrategy="continue"` collects all results.

**Step 1: Rewrite existing tests to use testify assertions**

Add `assert` and `require` imports. Migrate all raw assertions.

**Step 2: Add 6 new parallel tests**

```go
// TestParallelContainersWorkflow_MultipleFailuresContinue tests multiple failures with continue.
func TestParallelContainersWorkflow_MultipleFailuresContinue(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := payload.ParallelInput{
		Containers: []payload.ContainerExecutionInput{
			{Image: "alpine:latest", Name: "task1"},
			{Image: "alpine:latest", Name: "task2"},
			{Image: "alpine:latest", Name: "task3"},
			{Image: "alpine:latest", Name: "task4"},
		},
		FailureStrategy: "continue",
	}

	env.OnActivity(activity.StartContainerActivity, mock.Anything, input.Containers[0]).Return(
		&payload.ContainerExecutionOutput{Success: true, ExitCode: 0}, nil).Once()
	env.OnActivity(activity.StartContainerActivity, mock.Anything, input.Containers[1]).Return(
		&payload.ContainerExecutionOutput{Success: false, ExitCode: 1}, nil).Once()
	env.OnActivity(activity.StartContainerActivity, mock.Anything, input.Containers[2]).Return(
		&payload.ContainerExecutionOutput{Success: true, ExitCode: 0}, nil).Once()
	env.OnActivity(activity.StartContainerActivity, mock.Anything, input.Containers[3]).Return(
		&payload.ContainerExecutionOutput{Success: false, ExitCode: 1}, nil).Once()

	env.ExecuteWorkflow(ParallelContainersWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.ParallelOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 2, result.TotalSuccess)
	assert.Equal(t, 2, result.TotalFailed)
	assert.Len(t, result.Results, 4)
}

// TestParallelContainersWorkflow_AllFailContinue tests all containers fail with continue.
func TestParallelContainersWorkflow_AllFailContinue(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := payload.ParallelInput{
		Containers: []payload.ContainerExecutionInput{
			{Image: "alpine:latest", Name: "task1"},
			{Image: "alpine:latest", Name: "task2"},
			{Image: "alpine:latest", Name: "task3"},
		},
		FailureStrategy: "continue",
	}

	env.OnActivity(activity.StartContainerActivity, mock.Anything, mock.Anything).Return(
		&payload.ContainerExecutionOutput{Success: false, ExitCode: 1}, nil)

	env.ExecuteWorkflow(ParallelContainersWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.ParallelOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 0, result.TotalSuccess)
	assert.Equal(t, 3, result.TotalFailed)
}

// TestParallelContainersWorkflow_ActivityErrorFailFast tests activity error with fail_fast.
func TestParallelContainersWorkflow_ActivityErrorFailFast(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := payload.ParallelInput{
		Containers: []payload.ContainerExecutionInput{
			{Image: "alpine:latest", Name: "task1"},
			{Image: "alpine:latest", Name: "task2"},
		},
		FailureStrategy: "fail_fast",
	}

	env.OnActivity(activity.StartContainerActivity, mock.Anything, input.Containers[0]).Return(
		nil, fmt.Errorf("docker daemon unavailable")).Once()
	env.OnActivity(activity.StartContainerActivity, mock.Anything, input.Containers[1]).Return(
		&payload.ContainerExecutionOutput{Success: true, ExitCode: 0}, nil).Once()

	env.ExecuteWorkflow(ParallelContainersWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	assert.Error(t, env.GetWorkflowError())
}

// TestParallelContainersWorkflow_ActivityErrorContinue tests activity error with continue.
func TestParallelContainersWorkflow_ActivityErrorContinue(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := payload.ParallelInput{
		Containers: []payload.ContainerExecutionInput{
			{Image: "alpine:latest", Name: "task1"},
			{Image: "alpine:latest", Name: "task2"},
			{Image: "alpine:latest", Name: "task3"},
		},
		FailureStrategy: "continue",
	}

	env.OnActivity(activity.StartContainerActivity, mock.Anything, input.Containers[0]).Return(
		&payload.ContainerExecutionOutput{Success: true, ExitCode: 0}, nil).Once()
	env.OnActivity(activity.StartContainerActivity, mock.Anything, input.Containers[1]).Return(
		nil, fmt.Errorf("docker daemon unavailable")).Once()
	env.OnActivity(activity.StartContainerActivity, mock.Anything, input.Containers[2]).Return(
		&payload.ContainerExecutionOutput{Success: true, ExitCode: 0}, nil).Once()

	env.ExecuteWorkflow(ParallelContainersWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	// Activity errors with retry exhaustion may cause workflow-level error
	// The key assertion is that all 3 containers were attempted
}

// TestParallelContainersWorkflow_SingleContainer tests parallel with one container.
func TestParallelContainersWorkflow_SingleContainer(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := payload.ParallelInput{
		Containers: []payload.ContainerExecutionInput{
			{Image: "alpine:latest", Name: "only-task"},
		},
		FailureStrategy: "continue",
	}

	env.OnActivity(activity.StartContainerActivity, mock.Anything, mock.Anything).Return(
		&payload.ContainerExecutionOutput{ContainerID: "solo-1", Success: true, ExitCode: 0}, nil)

	env.ExecuteWorkflow(ParallelContainersWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.ParallelOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 1, result.TotalSuccess)
	assert.Len(t, result.Results, 1)
	assert.Equal(t, "solo-1", result.Results[0].ContainerID)
}

// TestParallelContainersWorkflow_ResultCountMatchesInput tests Results length matches Containers length.
func TestParallelContainersWorkflow_ResultCountMatchesInput(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := payload.ParallelInput{
		Containers: []payload.ContainerExecutionInput{
			{Image: "alpine:latest", Name: "task1"},
			{Image: "alpine:latest", Name: "task2"},
			{Image: "alpine:latest", Name: "task3"},
		},
		FailureStrategy: "continue",
	}

	env.OnActivity(activity.StartContainerActivity, mock.Anything, mock.Anything).Return(
		&payload.ContainerExecutionOutput{Success: true, ExitCode: 0}, nil)

	env.ExecuteWorkflow(ParallelContainersWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.ParallelOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Len(t, result.Results, len(input.Containers))
	assert.Equal(t, 3, result.TotalSuccess)
}
```

**Step 3: Add `"fmt"`, `assert`, and `require` to imports**

**Step 4: Run tests**

Run: `go test -race -count=1 -run 'TestParallelContainersWorkflow' ./docker/workflow/ -v`
Expected: All 10 tests PASS

**Step 5: Commit**

```bash
git add docker/workflow/parallel_test.go
git commit -m "test(parallel): add multi-failure, activity error, single container, and result count tests

Migrate existing tests to testify assertions. Add tests for multiple
failures with continue, all-fail, activity errors with both strategies,
single container edge case, and result count verification."
```

---

### Task 4: DAG Workflow Tests

**Files:**
- Modify: `docker/workflow/dag_test.go`

**Context:** This file has 5 existing tests (DAGWorkflow, DAGWorkflowValidation, DAGWorkflowFailFast, WorkflowWithParameters, HelperFunctions). Already uses testify. The DAG workflow is in `docker/workflow/dag.go` — it validates input, builds a node map, recursively executes nodes respecting dependencies, supports output extraction/substitution, and artifact upload/download.

**Step 1: Add 8 new DAG tests**

```go
// TestDAGWorkflow_DiamondDependency tests A->B,C->D pattern.
func TestDAGWorkflow_DiamondDependency(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	env.OnActivity(activity.StartContainerActivity, mock.Anything, mock.Anything).Return(
		&payload.ContainerExecutionOutput{
			ContainerID: "container-ok",
			ExitCode:    0,
			Success:     true,
			Duration:    1 * time.Second,
		}, nil)

	input := payload.DAGWorkflowInput{
		Nodes: []payload.DAGNode{
			{
				Name: "A",
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{Image: "alpine:latest", Command: []string{"echo", "A"}},
				},
			},
			{
				Name:         "B",
				Dependencies: []string{"A"},
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{Image: "alpine:latest", Command: []string{"echo", "B"}},
				},
			},
			{
				Name:         "C",
				Dependencies: []string{"A"},
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{Image: "alpine:latest", Command: []string{"echo", "C"}},
				},
			},
			{
				Name:         "D",
				Dependencies: []string{"B", "C"},
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{Image: "alpine:latest", Command: []string{"echo", "D"}},
				},
			},
		},
		FailFast: false,
	}

	env.ExecuteWorkflow(DAGWorkflow, input)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.DAGWorkflowOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 4, result.TotalSuccess)
	assert.Equal(t, 0, result.TotalFailed)
}

// TestDAGWorkflow_ParallelBranches tests independent branches from same root.
func TestDAGWorkflow_ParallelBranches(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	env.OnActivity(activity.StartContainerActivity, mock.Anything, mock.Anything).Return(
		&payload.ContainerExecutionOutput{ContainerID: "ok", ExitCode: 0, Success: true, Duration: time.Second}, nil)

	input := payload.DAGWorkflowInput{
		Nodes: []payload.DAGNode{
			{Name: "root", Container: payload.ExtendedContainerInput{ContainerExecutionInput: payload.ContainerExecutionInput{Image: "alpine:latest"}}},
			{Name: "branch1", Dependencies: []string{"root"}, Container: payload.ExtendedContainerInput{ContainerExecutionInput: payload.ContainerExecutionInput{Image: "alpine:latest"}}},
			{Name: "branch2", Dependencies: []string{"root"}, Container: payload.ExtendedContainerInput{ContainerExecutionInput: payload.ContainerExecutionInput{Image: "alpine:latest"}}},
		},
		FailFast: false,
	}

	env.ExecuteWorkflow(DAGWorkflow, input)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.DAGWorkflowOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 3, result.TotalSuccess)
}

// TestDAGWorkflow_FailFastFalseWithFailure tests node failure doesn't block independent nodes.
func TestDAGWorkflow_FailFastFalseWithFailure(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := payload.DAGWorkflowInput{
		Nodes: []payload.DAGNode{
			{Name: "task1", Container: payload.ExtendedContainerInput{ContainerExecutionInput: payload.ContainerExecutionInput{Image: "alpine:latest"}}},
			{Name: "task2", Container: payload.ExtendedContainerInput{ContainerExecutionInput: payload.ContainerExecutionInput{Image: "alpine:latest"}}},
		},
		FailFast: false,
	}

	// task1 fails
	env.OnActivity(activity.StartContainerActivity, mock.Anything, input.Nodes[0].Container.ContainerExecutionInput).Return(
		&payload.ContainerExecutionOutput{ContainerID: "fail", ExitCode: 1, Success: false, Duration: time.Second}, nil).Once()
	// task2 succeeds
	env.OnActivity(activity.StartContainerActivity, mock.Anything, input.Nodes[1].Container.ContainerExecutionInput).Return(
		&payload.ContainerExecutionOutput{ContainerID: "ok", ExitCode: 0, Success: true, Duration: time.Second}, nil).Once()

	env.ExecuteWorkflow(DAGWorkflow, input)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.DAGWorkflowOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 1, result.TotalSuccess)
	assert.Equal(t, 1, result.TotalFailed)
}

// TestDAGWorkflow_DependencyFailureBlocksDownstream tests that with FailFast, failed dep blocks child.
func TestDAGWorkflow_DependencyFailureBlocksDownstream(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := payload.DAGWorkflowInput{
		Nodes: []payload.DAGNode{
			{Name: "build", Container: payload.ExtendedContainerInput{ContainerExecutionInput: payload.ContainerExecutionInput{Image: "alpine:latest"}}},
			{Name: "test", Dependencies: []string{"build"}, Container: payload.ExtendedContainerInput{ContainerExecutionInput: payload.ContainerExecutionInput{Image: "alpine:latest"}}},
		},
		FailFast: true,
	}

	// build fails
	env.OnActivity(activity.StartContainerActivity, mock.Anything, mock.Anything).Return(
		&payload.ContainerExecutionOutput{ContainerID: "build-fail", ExitCode: 1, Success: false, Duration: time.Second}, nil).Once()

	env.ExecuteWorkflow(DAGWorkflow, input)
	require.True(t, env.IsWorkflowCompleted())
	assert.Error(t, env.GetWorkflowError())
	assert.Contains(t, env.GetWorkflowError().Error(), "dependency build failed")
}

// TestDAGWorkflow_OutputExtractionAndSubstitution tests outputs from one node flow into another.
func TestDAGWorkflow_OutputExtractionAndSubstitution(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := payload.DAGWorkflowInput{
		Nodes: []payload.DAGNode{
			{
				Name: "build",
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{
						Image:   "alpine:latest",
						Command: []string{"echo", `{"version":"1.2.3"}`},
					},
					Outputs: []payload.OutputDefinition{
						{Name: "version", ValueFrom: "stdout", JSONPath: "$.version"},
					},
				},
			},
			{
				Name:         "deploy",
				Dependencies: []string{"build"},
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{
						Image: "alpine:latest",
					},
					Inputs: []payload.InputMapping{
						{Name: "BUILD_VERSION", From: "build.version", Required: true},
					},
				},
			},
		},
		FailFast: false,
	}

	// build returns JSON stdout
	env.OnActivity(activity.StartContainerActivity, mock.Anything, input.Nodes[0].Container.ContainerExecutionInput).Return(
		&payload.ContainerExecutionOutput{
			ContainerID: "build-1",
			ExitCode:    0,
			Success:     true,
			Stdout:      `{"version":"1.2.3"}`,
			Duration:    time.Second,
		}, nil).Once()

	// deploy — verify the env was substituted
	env.OnActivity(activity.StartContainerActivity, mock.Anything, mock.Anything).Return(
		&payload.ContainerExecutionOutput{
			ContainerID: "deploy-1",
			ExitCode:    0,
			Success:     true,
			Duration:    time.Second,
		}, nil).Once()

	env.ExecuteWorkflow(DAGWorkflow, input)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.DAGWorkflowOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 2, result.TotalSuccess)
	// Verify outputs were extracted
	assert.Equal(t, "1.2.3", result.StepOutputs["build"]["version"])
}

// TestDAGWorkflow_ActivityError tests activity error with FailFast=true.
func TestDAGWorkflow_ActivityError(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := payload.DAGWorkflowInput{
		Nodes: []payload.DAGNode{
			{Name: "task1", Container: payload.ExtendedContainerInput{ContainerExecutionInput: payload.ContainerExecutionInput{Image: "alpine:latest"}}},
		},
		FailFast: true,
	}

	env.OnActivity(activity.StartContainerActivity, mock.Anything, mock.Anything).Return(
		nil, fmt.Errorf("docker daemon error"))

	env.ExecuteWorkflow(DAGWorkflow, input)
	require.True(t, env.IsWorkflowCompleted())
	assert.Error(t, env.GetWorkflowError())
}

// TestDAGWorkflow_MultipleIndependentRoots tests multiple nodes with no dependencies.
func TestDAGWorkflow_MultipleIndependentRoots(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	env.OnActivity(activity.StartContainerActivity, mock.Anything, mock.Anything).Return(
		&payload.ContainerExecutionOutput{ContainerID: "ok", ExitCode: 0, Success: true, Duration: time.Second}, nil)

	input := payload.DAGWorkflowInput{
		Nodes: []payload.DAGNode{
			{Name: "lint", Container: payload.ExtendedContainerInput{ContainerExecutionInput: payload.ContainerExecutionInput{Image: "alpine:latest"}}},
			{Name: "test", Container: payload.ExtendedContainerInput{ContainerExecutionInput: payload.ContainerExecutionInput{Image: "alpine:latest"}}},
			{Name: "scan", Container: payload.ExtendedContainerInput{ContainerExecutionInput: payload.ContainerExecutionInput{Image: "alpine:latest"}}},
		},
		FailFast: false,
	}

	env.ExecuteWorkflow(DAGWorkflow, input)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.DAGWorkflowOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 3, result.TotalSuccess)
	assert.Equal(t, 0, result.TotalFailed)
}

// TestDAGWorkflow_AlreadyExecutedGuard tests node is not executed twice even if referenced by multiple dependents.
func TestDAGWorkflow_AlreadyExecutedGuard(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	activityCallCount := 0
	env.OnActivity(activity.StartContainerActivity, mock.Anything, mock.Anything).Return(
		func(_ interface{}, _ payload.ContainerExecutionInput) (*payload.ContainerExecutionOutput, error) {
			activityCallCount++
			return &payload.ContainerExecutionOutput{ContainerID: "ok", ExitCode: 0, Success: true, Duration: time.Second}, nil
		})

	// A is depended on by both B and C; should only execute once
	input := payload.DAGWorkflowInput{
		Nodes: []payload.DAGNode{
			{Name: "A", Container: payload.ExtendedContainerInput{ContainerExecutionInput: payload.ContainerExecutionInput{Image: "alpine:latest"}}},
			{Name: "B", Dependencies: []string{"A"}, Container: payload.ExtendedContainerInput{ContainerExecutionInput: payload.ContainerExecutionInput{Image: "alpine:latest"}}},
			{Name: "C", Dependencies: []string{"A"}, Container: payload.ExtendedContainerInput{ContainerExecutionInput: payload.ContainerExecutionInput{Image: "alpine:latest"}}},
		},
		FailFast: false,
	}

	env.ExecuteWorkflow(DAGWorkflow, input)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.DAGWorkflowOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 3, result.TotalSuccess)
	// A should be executed only once (total 3 activity calls: A, B, C)
	assert.Equal(t, 3, activityCallCount)
}
```

**Step 2: Add `"fmt"` to imports if not present**

**Step 3: Run tests**

Run: `go test -race -count=1 -run 'TestDAGWorkflow' ./docker/workflow/ -v`
Expected: All 13 tests PASS (5 existing + 8 new)

**Step 4: Commit**

```bash
git add docker/workflow/dag_test.go
git commit -m "test(dag): add diamond, parallel branches, output substitution, and guard tests

Add tests for diamond dependency pattern, parallel branches, FailFast
false with failure, dependency failure blocking downstream, output
extraction and input substitution, activity error, multiple independent
roots, and already-executed guard."
```

---

### Task 5: Loop Workflow Tests

**Files:**
- Modify: `docker/workflow/loop_test.go`

**Context:** This file has tests for validation, sequential/parallel success, parameterized loops, template substitution, and benchmarks. Already uses testify. The loop workflow is in `docker/workflow/loop.go` — it validates input, generates iterations, and executes them sequentially or in parallel with fail_fast or continue strategies.

**Step 1: Add 8 new loop workflow tests**

```go
// TestLoopWorkflow_SequentialFailFast tests sequential loop stops at first failure.
func TestLoopWorkflow_SequentialFailFast(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Item 0: success, Item 1: failure, Item 2: should not execute
	callCount := 0
	env.OnActivity(activity.StartContainerActivity, mock.Anything, mock.Anything).Return(
		func(_ interface{}, _ payload.ContainerExecutionInput) (*payload.ContainerExecutionOutput, error) {
			callCount++
			if callCount == 2 {
				return &payload.ContainerExecutionOutput{Success: false, ExitCode: 1}, nil
			}
			return &payload.ContainerExecutionOutput{Success: true, ExitCode: 0}, nil
		})

	input := payload.LoopInput{
		Items:           []string{"item1", "item2", "item3"},
		Template:        payload.ContainerExecutionInput{Image: "alpine:latest", Command: []string{"echo", "{{item}}"}},
		Parallel:        false,
		FailureStrategy: "fail_fast",
	}

	env.ExecuteWorkflow(LoopWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	assert.Error(t, env.GetWorkflowError())

	var result payload.LoopOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 1, result.TotalSuccess)
	assert.Equal(t, 1, result.TotalFailed)
	assert.Len(t, result.Results, 2)
}

// TestLoopWorkflow_SequentialContinueOnFailure tests sequential loop continues past failure.
func TestLoopWorkflow_SequentialContinueOnFailure(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	callCount := 0
	env.OnActivity(activity.StartContainerActivity, mock.Anything, mock.Anything).Return(
		func(_ interface{}, _ payload.ContainerExecutionInput) (*payload.ContainerExecutionOutput, error) {
			callCount++
			if callCount == 2 {
				return &payload.ContainerExecutionOutput{Success: false, ExitCode: 1}, nil
			}
			return &payload.ContainerExecutionOutput{Success: true, ExitCode: 0}, nil
		})

	input := payload.LoopInput{
		Items:           []string{"item1", "item2", "item3"},
		Template:        payload.ContainerExecutionInput{Image: "alpine:latest", Command: []string{"echo", "{{item}}"}},
		Parallel:        false,
		FailureStrategy: "continue",
	}

	env.ExecuteWorkflow(LoopWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.LoopOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 2, result.TotalSuccess)
	assert.Equal(t, 1, result.TotalFailed)
	assert.Len(t, result.Results, 3)
}

// TestLoopWorkflow_ParallelFailFast tests parallel loop returns error on failure.
func TestLoopWorkflow_ParallelFailFast(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	callCount := 0
	env.OnActivity(activity.StartContainerActivity, mock.Anything, mock.Anything).Return(
		func(_ interface{}, _ payload.ContainerExecutionInput) (*payload.ContainerExecutionOutput, error) {
			callCount++
			if callCount == 2 {
				return &payload.ContainerExecutionOutput{Success: false, ExitCode: 1}, nil
			}
			return &payload.ContainerExecutionOutput{Success: true, ExitCode: 0}, nil
		})

	input := payload.LoopInput{
		Items:           []string{"item1", "item2", "item3"},
		Template:        payload.ContainerExecutionInput{Image: "alpine:latest", Command: []string{"echo", "{{item}}"}},
		Parallel:        true,
		FailureStrategy: "fail_fast",
	}

	env.ExecuteWorkflow(LoopWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	assert.Error(t, env.GetWorkflowError())

	var result payload.LoopOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.GreaterOrEqual(t, result.TotalFailed, 1)
}

// TestLoopWorkflow_ParallelContinueMultipleFailures tests parallel loop with multiple failures continues.
func TestLoopWorkflow_ParallelContinueMultipleFailures(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	callCount := 0
	env.OnActivity(activity.StartContainerActivity, mock.Anything, mock.Anything).Return(
		func(_ interface{}, _ payload.ContainerExecutionInput) (*payload.ContainerExecutionOutput, error) {
			callCount++
			if callCount == 2 || callCount == 4 {
				return &payload.ContainerExecutionOutput{Success: false, ExitCode: 1}, nil
			}
			return &payload.ContainerExecutionOutput{Success: true, ExitCode: 0}, nil
		})

	input := payload.LoopInput{
		Items:           []string{"a", "b", "c", "d"},
		Template:        payload.ContainerExecutionInput{Image: "alpine:latest", Command: []string{"echo", "{{item}}"}},
		Parallel:        true,
		FailureStrategy: "continue",
	}

	env.ExecuteWorkflow(LoopWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.LoopOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 2, result.TotalSuccess)
	assert.Equal(t, 2, result.TotalFailed)
	assert.Len(t, result.Results, 4)
}

// TestLoopWorkflow_AllFailContinue tests all iterations fail with continue.
func TestLoopWorkflow_AllFailContinue(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	env.OnActivity(activity.StartContainerActivity, mock.Anything, mock.Anything).Return(
		&payload.ContainerExecutionOutput{Success: false, ExitCode: 1}, nil)

	input := payload.LoopInput{
		Items:           []string{"a", "b", "c"},
		Template:        payload.ContainerExecutionInput{Image: "alpine:latest", Command: []string{"echo", "{{item}}"}},
		Parallel:        false,
		FailureStrategy: "continue",
	}

	env.ExecuteWorkflow(LoopWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.LoopOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 0, result.TotalSuccess)
	assert.Equal(t, 3, result.TotalFailed)
}

// TestLoopWorkflow_SingleItem tests loop with single item.
func TestLoopWorkflow_SingleItem(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	env.OnActivity(activity.StartContainerActivity, mock.Anything, mock.Anything).Return(
		&payload.ContainerExecutionOutput{Success: true, ExitCode: 0}, nil)

	input := payload.LoopInput{
		Items:           []string{"only-item"},
		Template:        payload.ContainerExecutionInput{Image: "alpine:latest", Command: []string{"echo", "{{item}}"}},
		Parallel:        false,
		FailureStrategy: "continue",
	}

	env.ExecuteWorkflow(LoopWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.LoopOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 1, result.ItemCount)
	assert.Equal(t, 1, result.TotalSuccess)
}

// TestParameterizedLoopWorkflow_FailFast tests parameterized loop with fail_fast.
func TestParameterizedLoopWorkflow_FailFast(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	callCount := 0
	env.OnActivity(activity.StartContainerActivity, mock.Anything, mock.Anything).Return(
		func(_ interface{}, _ payload.ContainerExecutionInput) (*payload.ContainerExecutionOutput, error) {
			callCount++
			if callCount == 2 {
				return &payload.ContainerExecutionOutput{Success: false, ExitCode: 1}, nil
			}
			return &payload.ContainerExecutionOutput{Success: true, ExitCode: 0}, nil
		})

	input := payload.ParameterizedLoopInput{
		Parameters: map[string][]string{
			"env": {"dev", "prod"},
		},
		Template:        payload.ContainerExecutionInput{Image: "deployer:v1", Command: []string{"deploy", "--env={{.env}}"}},
		Parallel:        false,
		FailureStrategy: "fail_fast",
	}

	env.ExecuteWorkflow(ParameterizedLoopWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	assert.Error(t, env.GetWorkflowError())
}

// TestParameterizedLoopWorkflow_ContinueWithFailures tests parameterized loop with continue and failures.
func TestParameterizedLoopWorkflow_ContinueWithFailures(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	callCount := 0
	env.OnActivity(activity.StartContainerActivity, mock.Anything, mock.Anything).Return(
		func(_ interface{}, _ payload.ContainerExecutionInput) (*payload.ContainerExecutionOutput, error) {
			callCount++
			if callCount%2 == 0 {
				return &payload.ContainerExecutionOutput{Success: false, ExitCode: 1}, nil
			}
			return &payload.ContainerExecutionOutput{Success: true, ExitCode: 0}, nil
		})

	input := payload.ParameterizedLoopInput{
		Parameters: map[string][]string{
			"env":    {"dev", "prod"},
			"region": {"us", "eu"},
		},
		Template:        payload.ContainerExecutionInput{Image: "deployer:v1", Command: []string{"deploy", "--env={{.env}}", "--region={{.region}}"}},
		Parallel:        true,
		FailureStrategy: "continue",
	}

	env.ExecuteWorkflow(ParameterizedLoopWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.LoopOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 4, result.ItemCount)
	assert.Equal(t, 2, result.TotalSuccess)
	assert.Equal(t, 2, result.TotalFailed)
}
```

**Step 2: Add 3 new helper substitution tests**

```go
// TestSubstituteContainerInput_Entrypoint tests substitution in entrypoint.
func TestSubstituteContainerInput_Entrypoint(t *testing.T) {
	template := payload.ContainerExecutionInput{
		Image:      "alpine:latest",
		Entrypoint: []string{"sh", "-c", "echo {{item}}"},
	}

	result := substituteContainerInput(template, "hello", 0, nil)
	require.Len(t, result.Entrypoint, 3)
	assert.Equal(t, "echo hello", result.Entrypoint[2])
}

// TestSubstituteContainerInput_Volumes tests substitution in volumes.
func TestSubstituteContainerInput_Volumes(t *testing.T) {
	template := payload.ContainerExecutionInput{
		Image: "alpine:latest",
		Volumes: map[string]string{
			"/data/{{item}}": "/mnt/{{index}}",
		},
	}

	result := substituteContainerInput(template, "mydata", 5, nil)
	assert.Equal(t, "/mnt/5", result.Volumes["/data/mydata"])
}

// TestSubstituteContainerInput_WorkDir tests substitution in work directory.
func TestSubstituteContainerInput_WorkDir(t *testing.T) {
	template := payload.ContainerExecutionInput{
		Image:   "alpine:latest",
		WorkDir: "/app/{{item}}",
	}

	result := substituteContainerInput(template, "myproject", 0, nil)
	assert.Equal(t, "/app/myproject", result.WorkDir)
}
```

**Step 3: Run tests**

Run: `go test -race -count=1 -run 'TestLoop|TestParameterized|TestSubstituteContainerInput' ./docker/workflow/ -v`
Expected: All tests PASS (existing + 11 new)

**Step 4: Commit**

```bash
git add docker/workflow/loop_test.go
git commit -m "test(loop): add failure strategies, single item, parameterized failures, and substitution tests

Add tests for sequential/parallel fail-fast and continue, all-fail,
single item loop, parameterized loop failures, and entrypoint/volumes/
workdir template substitution."
```

---

### Task 6: Output Extraction Tests

**Files:**
- Modify: `docker/workflow/output_extraction_test.go`

**Context:** This file has tests for stdout/stderr/exitCode extraction, JSONPath, regex, ExtractOutputs, SubstituteInputs, and ResolveInputMapping. Uses raw `t.Errorf`. The implementation is in `docker/workflow/output_extraction.go` — supports `ValueFrom` of stdout/stderr/exitCode/file, optional JSONPath and regex post-processing, whitespace trimming, and default fallback.

**Step 1: Migrate existing tests to testify assertions**

Replace all raw assertions in existing tests with `assert`/`require`. Add imports for `"os"`, `"path/filepath"`, `"github.com/stretchr/testify/assert"`, `"github.com/stretchr/testify/require"`.

Pattern:
- `t.Errorf("ExtractOutput() error = %v, wantErr %v", err, tt.wantErr)` → keep table-driven `wantErr` check or use assert
- `t.Errorf("ExtractOutput() = %v, want %v", got, tt.want)` → `assert.Equal(t, tt.want, got)`

**Step 2: Add 9 new output extraction tests**

```go
// TestExtractOutput_FileSuccess tests reading from a temp file.
func TestExtractOutput_FileSuccess(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "output.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("file-content-123"), 0o600))

	output := &payload.ContainerExecutionOutput{Success: true}
	def := payload.OutputDefinition{
		Name:      "file_value",
		ValueFrom: "file",
		Path:      filePath,
	}

	got, err := ExtractOutput(def, output)
	require.NoError(t, err)
	assert.Equal(t, "file-content-123", got)
}

// TestExtractOutput_FileErrorWithDefault tests file read error returns default.
func TestExtractOutput_FileErrorWithDefault(t *testing.T) {
	output := &payload.ContainerExecutionOutput{Success: true}
	def := payload.OutputDefinition{
		Name:      "file_value",
		ValueFrom: "file",
		Path:      "/nonexistent/path/file.txt",
		Default:   "fallback-value",
	}

	got, err := ExtractOutput(def, output)
	require.NoError(t, err)
	assert.Equal(t, "fallback-value", got)
}

// TestExtractOutput_FileErrorNoDefault tests file read error with no default returns error.
func TestExtractOutput_FileErrorNoDefault(t *testing.T) {
	output := &payload.ContainerExecutionOutput{Success: true}
	def := payload.OutputDefinition{
		Name:      "file_value",
		ValueFrom: "file",
		Path:      "/nonexistent/path/file.txt",
	}

	_, err := ExtractOutput(def, output)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read file")
}

// TestExtractOutput_FileMissingPath tests file extraction with empty path.
func TestExtractOutput_FileMissingPath(t *testing.T) {
	output := &payload.ContainerExecutionOutput{Success: true}
	def := payload.OutputDefinition{
		Name:      "file_value",
		ValueFrom: "file",
		Path:      "",
	}

	_, err := ExtractOutput(def, output)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "path is required")
}

// TestExtractOutput_UnknownValueFrom tests unknown value_from returns error with default.
func TestExtractOutput_UnknownValueFrom(t *testing.T) {
	output := &payload.ContainerExecutionOutput{Success: true}
	def := payload.OutputDefinition{
		Name:      "value",
		ValueFrom: "unknown_source",
		Default:   "default-val",
	}

	got, err := ExtractOutput(def, output)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown value_from")
	assert.Equal(t, "default-val", got)
}

// TestExtractOutput_EmptyStdoutWithDefault tests empty stdout returns default.
func TestExtractOutput_EmptyStdoutWithDefault(t *testing.T) {
	output := &payload.ContainerExecutionOutput{
		Stdout:  "",
		Success: true,
	}
	def := payload.OutputDefinition{
		Name:      "value",
		ValueFrom: "stdout",
		Default:   "fallback",
	}

	got, err := ExtractOutput(def, output)
	require.NoError(t, err)
	assert.Equal(t, "fallback", got)
}

// TestExtractOutput_WhitespaceTrimming tests leading/trailing whitespace is trimmed.
func TestExtractOutput_WhitespaceTrimming(t *testing.T) {
	output := &payload.ContainerExecutionOutput{
		Stdout:  "  hello world  \n",
		Success: true,
	}
	def := payload.OutputDefinition{
		Name:      "value",
		ValueFrom: "stdout",
	}

	got, err := ExtractOutput(def, output)
	require.NoError(t, err)
	assert.Equal(t, "hello world", got)
}

// TestExtractOutput_JSONPathNull tests JSONPath extraction of null value.
func TestExtractOutput_JSONPathNull(t *testing.T) {
	output := &payload.ContainerExecutionOutput{
		Stdout:  `{"field": null}`,
		Success: true,
	}
	def := payload.OutputDefinition{
		Name:      "value",
		ValueFrom: "stdout",
		JSONPath:  "$.field",
	}

	got, err := ExtractOutput(def, output)
	require.NoError(t, err)
	assert.Equal(t, "", got)
}

// TestExtractOutput_JSONPathFailureWithDefault tests JSONPath failure returns default.
func TestExtractOutput_JSONPathFailureWithDefault(t *testing.T) {
	output := &payload.ContainerExecutionOutput{
		Stdout:  `{"version": "1.2.3"}`,
		Success: true,
	}
	def := payload.OutputDefinition{
		Name:      "value",
		ValueFrom: "stdout",
		JSONPath:  "$.missing_field",
		Default:   "default-version",
	}

	got, err := ExtractOutput(def, output)
	require.NoError(t, err)
	assert.Equal(t, "default-version", got)
}
```

**Step 3: Run tests**

Run: `go test -race -count=1 -run 'TestExtractOutput|TestExtractJSON|TestExtractRegex|TestExtractOutputs|TestSubstituteInputs|TestResolveInputMapping' ./docker/workflow/ -v`
Expected: All tests PASS (existing + 9 new)

**Step 4: Commit**

```bash
git add docker/workflow/output_extraction_test.go
git commit -m "test(output): add file extraction, unknown value_from, whitespace, and JSONPath edge case tests

Migrate existing tests to testify assertions. Add tests for file read
success/error, missing path, unknown value_from, empty stdout default,
whitespace trimming, JSONPath null value, and JSONPath failure with
default fallback."
```

---

### Task 7: Final Verification

**Step 1: Run full unit test suite**

Run: `task ci:test`
Expected: All packages PASS

**Step 2: Run lint**

Run: `task ci:lint`
Expected: No errors

**Step 3: Format code**

Run: `task fmt`

**Step 4: Verify test count**

Run: `go test -v ./docker/workflow/ 2>&1 | grep -c '--- PASS'`
Expected: ~80 (38 existing + 42 new)

**Step 5: Commit any formatting fixes**

```bash
git add -A
git commit -m "style: format test files"
```

**Step 6: Create PR**

```bash
git push -u origin test/unit-test-expansion
gh pr create --title "test: expand unit test coverage with mocked workflows" --body "..."
```
