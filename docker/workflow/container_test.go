package workflow

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"

	"github.com/jasoet/go-wf/docker/activity"
	"github.com/jasoet/go-wf/docker/payload"
)

// TestExecuteContainerWorkflow_Success tests successful container execution.
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

	require.True(t, env.IsWorkflowCompleted(), "Workflow did not complete")
	require.NoError(t, env.GetWorkflowError())

	var result payload.ContainerExecutionOutput
	require.NoError(t, env.GetWorkflowResult(&result))

	assert.Equal(t, "container-123", result.ContainerID)
}

// TestExecuteContainerWorkflow_InvalidInput tests validation failure.
func TestExecuteContainerWorkflow_InvalidInput(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Missing required image field
	input := payload.ContainerExecutionInput{}

	env.ExecuteWorkflow(ExecuteContainerWorkflow, input)

	require.True(t, env.IsWorkflowCompleted(), "Workflow did not complete")
	assert.Error(t, env.GetWorkflowError(), "Expected validation error")
}

// TestExecuteContainerWorkflow_WithTimeout tests workflow with custom timeout.
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

	require.True(t, env.IsWorkflowCompleted(), "Workflow did not complete")
}

// TestExecuteContainerWorkflow_ActivityError tests activity error handling.
func TestExecuteContainerWorkflow_ActivityError(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := payload.ContainerExecutionInput{
		Image: "alpine:latest",
	}

	env.OnActivity(activity.StartContainerActivity, mock.Anything, mock.Anything).Return(
		nil, fmt.Errorf("activity execution failed"))

	env.ExecuteWorkflow(ExecuteContainerWorkflow, input)

	require.True(t, env.IsWorkflowCompleted(), "Workflow did not complete")
	assert.Error(t, env.GetWorkflowError(), "Expected workflow error when activity fails")
}

// TestExecuteContainerWorkflow_ContainerFailure tests container returning non-zero exit code.
func TestExecuteContainerWorkflow_ContainerFailure(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := payload.ContainerExecutionInput{
		Image:   "alpine:latest",
		Command: []string{"false"},
	}

	env.OnActivity(activity.StartContainerActivity, mock.Anything, mock.Anything).Return(&payload.ContainerExecutionOutput{
		Success:  false,
		ExitCode: 1,
		Stderr:   "command failed",
	}, nil)

	env.ExecuteWorkflow(ExecuteContainerWorkflow, input)

	require.True(t, env.IsWorkflowCompleted(), "Workflow did not complete")
	require.NoError(t, env.GetWorkflowError())

	var result payload.ContainerExecutionOutput
	require.NoError(t, env.GetWorkflowResult(&result))

	assert.False(t, result.Success)
	assert.Equal(t, 1, result.ExitCode)
	assert.Equal(t, "command failed", result.Stderr)
}

// TestExecuteContainerWorkflow_DefaultTimeout tests the default 10-minute timeout path.
func TestExecuteContainerWorkflow_DefaultTimeout(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := payload.ContainerExecutionInput{
		Image:      "alpine:latest",
		Command:    []string{"echo", "hello"},
		RunTimeout: 0,
	}

	env.OnActivity(activity.StartContainerActivity, mock.Anything, mock.Anything).Return(&payload.ContainerExecutionOutput{
		Success:  true,
		ExitCode: 0,
	}, nil)

	env.ExecuteWorkflow(ExecuteContainerWorkflow, input)

	require.True(t, env.IsWorkflowCompleted(), "Workflow did not complete")
	require.NoError(t, env.GetWorkflowError())

	var result payload.ContainerExecutionOutput
	require.NoError(t, env.GetWorkflowResult(&result))

	assert.True(t, result.Success)
	assert.Equal(t, 0, result.ExitCode)
}

// TestExecuteContainerWorkflow_FullResultFields tests that all output fields pass through correctly.
func TestExecuteContainerWorkflow_FullResultFields(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := payload.ContainerExecutionInput{
		Image:   "ubuntu:22.04",
		Command: []string{"bash", "-c", "echo hello"},
	}

	startedAt := time.Date(2026, 3, 6, 10, 0, 0, 0, time.UTC)
	finishedAt := time.Date(2026, 3, 6, 10, 0, 5, 0, time.UTC)

	env.OnActivity(activity.StartContainerActivity, mock.Anything, mock.Anything).Return(&payload.ContainerExecutionOutput{
		ContainerID: "abc-def-123",
		Success:     true,
		ExitCode:    0,
		Stdout:      "hello\n",
		Stderr:      "",
		Duration:    5 * time.Second,
		StartedAt:   startedAt,
		FinishedAt:  finishedAt,
	}, nil)

	env.ExecuteWorkflow(ExecuteContainerWorkflow, input)

	require.True(t, env.IsWorkflowCompleted(), "Workflow did not complete")
	require.NoError(t, env.GetWorkflowError())

	var result payload.ContainerExecutionOutput
	require.NoError(t, env.GetWorkflowResult(&result))

	assert.Equal(t, "abc-def-123", result.ContainerID)
	assert.True(t, result.Success)
	assert.Equal(t, 0, result.ExitCode)
	assert.Equal(t, "hello\n", result.Stdout)
	assert.Equal(t, "", result.Stderr)
	assert.Equal(t, 5*time.Second, result.Duration)
	assert.Equal(t, startedAt, result.StartedAt)
	assert.Equal(t, finishedAt, result.FinishedAt)
}
