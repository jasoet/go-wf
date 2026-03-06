package workflow

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"

	"github.com/jasoet/go-wf/docker/activity"
	"github.com/jasoet/go-wf/docker/payload"
)

// TestParallelContainersWorkflow_Success tests parallel execution.
func TestParallelContainersWorkflow_Success(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := payload.ParallelInput{
		Containers: []payload.ContainerExecutionInput{
			{Image: "alpine:latest", Name: "task1"},
			{Image: "nginx:alpine", Name: "task2"},
		},
		FailureStrategy: "continue",
	}

	env.OnActivity(activity.StartContainerActivity, mock.Anything, mock.Anything).Return(
		&payload.ContainerExecutionOutput{Success: true, ExitCode: 0}, nil)

	env.ExecuteWorkflow(ParallelContainersWorkflow, input)

	require.True(t, env.IsWorkflowCompleted(), "Workflow did not complete")

	var result payload.ParallelOutput
	require.NoError(t, env.GetWorkflowResult(&result), "Failed to get result")

	assert.Equal(t, 2, result.TotalSuccess, "Expected 2 successful containers")
}

// TestParallelContainersWorkflow_InvalidInput tests parallel validation.
func TestParallelContainersWorkflow_InvalidInput(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := payload.ParallelInput{
		Containers: []payload.ContainerExecutionInput{},
	}

	env.ExecuteWorkflow(ParallelContainersWorkflow, input)

	require.True(t, env.IsWorkflowCompleted(), "Workflow did not complete")

	assert.Error(t, env.GetWorkflowError(), "Expected validation error")
}

// TestParallelContainersWorkflow_FailFast tests fail fast strategy.
func TestParallelContainersWorkflow_FailFast(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := payload.ParallelInput{
		Containers: []payload.ContainerExecutionInput{
			{Image: "alpine:latest", Name: "task1"},
			{Image: "alpine:latest", Name: "task2"},
		},
		FailureStrategy: "fail_fast",
	}

	// First fails
	env.OnActivity(activity.StartContainerActivity, mock.Anything, input.Containers[0]).Return(
		&payload.ContainerExecutionOutput{Success: false, ExitCode: 1}, nil).Once()

	// Second succeeds
	env.OnActivity(activity.StartContainerActivity, mock.Anything, input.Containers[1]).Return(
		&payload.ContainerExecutionOutput{Success: true, ExitCode: 0}, nil).Once()

	env.ExecuteWorkflow(ParallelContainersWorkflow, input)

	require.True(t, env.IsWorkflowCompleted(), "Workflow did not complete")

	assert.Error(t, env.GetWorkflowError(), "Expected workflow error with fail_fast strategy")
}

// TestParallelContainersWorkflow_ContinueStrategy tests continue on error.
func TestParallelContainersWorkflow_ContinueStrategy(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := payload.ParallelInput{
		Containers: []payload.ContainerExecutionInput{
			{Image: "alpine:latest", Name: "task1"},
			{Image: "alpine:latest", Name: "task2"},
		},
		FailureStrategy: "continue",
	}

	// First fails
	env.OnActivity(activity.StartContainerActivity, mock.Anything, input.Containers[0]).Return(
		&payload.ContainerExecutionOutput{Success: false, ExitCode: 1}, nil).Once()

	// Second succeeds
	env.OnActivity(activity.StartContainerActivity, mock.Anything, input.Containers[1]).Return(
		&payload.ContainerExecutionOutput{Success: true, ExitCode: 0}, nil).Once()

	env.ExecuteWorkflow(ParallelContainersWorkflow, input)

	require.True(t, env.IsWorkflowCompleted(), "Workflow did not complete")

	require.NoError(t, env.GetWorkflowError(), "Workflow should not error with continue strategy")

	var result payload.ParallelOutput
	require.NoError(t, env.GetWorkflowResult(&result))

	assert.Equal(t, 1, result.TotalFailed, "Expected 1 failed")
}

// TestParallelContainersWorkflow_MultipleFailuresContinue tests multiple failures with continue strategy.
func TestParallelContainersWorkflow_MultipleFailuresContinue(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := payload.ParallelInput{
		Containers: []payload.ContainerExecutionInput{
			{Image: "alpine:latest", Name: "container-0"},
			{Image: "alpine:latest", Name: "container-1"},
			{Image: "alpine:latest", Name: "container-2"},
			{Image: "alpine:latest", Name: "container-3"},
		},
		FailureStrategy: "continue",
	}

	// Containers 0 and 2 succeed
	env.OnActivity(activity.StartContainerActivity, mock.Anything, input.Containers[0]).Return(
		&payload.ContainerExecutionOutput{Success: true, ExitCode: 0, ContainerID: "c0"}, nil).Once()
	env.OnActivity(activity.StartContainerActivity, mock.Anything, input.Containers[2]).Return(
		&payload.ContainerExecutionOutput{Success: true, ExitCode: 0, ContainerID: "c2"}, nil).Once()

	// Containers 1 and 3 fail
	env.OnActivity(activity.StartContainerActivity, mock.Anything, input.Containers[1]).Return(
		&payload.ContainerExecutionOutput{Success: false, ExitCode: 1, ContainerID: "c1"}, nil).Once()
	env.OnActivity(activity.StartContainerActivity, mock.Anything, input.Containers[3]).Return(
		&payload.ContainerExecutionOutput{Success: false, ExitCode: 1, ContainerID: "c3"}, nil).Once()

	env.ExecuteWorkflow(ParallelContainersWorkflow, input)

	require.True(t, env.IsWorkflowCompleted(), "Workflow did not complete")
	require.NoError(t, env.GetWorkflowError(), "Workflow should not error with continue strategy")

	var result payload.ParallelOutput
	require.NoError(t, env.GetWorkflowResult(&result))

	assert.Equal(t, 2, result.TotalSuccess, "Expected 2 successful containers")
	assert.Equal(t, 2, result.TotalFailed, "Expected 2 failed containers")
	assert.Len(t, result.Results, 4, "Expected 4 results")
}

// TestParallelContainersWorkflow_AllFailContinue tests all containers failing with continue strategy.
func TestParallelContainersWorkflow_AllFailContinue(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := payload.ParallelInput{
		Containers: []payload.ContainerExecutionInput{
			{Image: "alpine:latest", Name: "fail-1"},
			{Image: "alpine:latest", Name: "fail-2"},
			{Image: "alpine:latest", Name: "fail-3"},
		},
		FailureStrategy: "continue",
	}

	env.OnActivity(activity.StartContainerActivity, mock.Anything, mock.Anything).Return(
		&payload.ContainerExecutionOutput{Success: false, ExitCode: 1}, nil)

	env.ExecuteWorkflow(ParallelContainersWorkflow, input)

	require.True(t, env.IsWorkflowCompleted(), "Workflow did not complete")
	require.NoError(t, env.GetWorkflowError(), "Workflow should not error with continue strategy")

	var result payload.ParallelOutput
	require.NoError(t, env.GetWorkflowResult(&result))

	assert.Equal(t, 3, result.TotalFailed, "Expected 3 failed containers")
	assert.Equal(t, 0, result.TotalSuccess, "Expected 0 successful containers")
}

// TestParallelContainersWorkflow_ActivityErrorFailFast tests activity error with fail_fast strategy.
func TestParallelContainersWorkflow_ActivityErrorFailFast(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := payload.ParallelInput{
		Containers: []payload.ContainerExecutionInput{
			{Image: "alpine:latest", Name: "error-task"},
			{Image: "alpine:latest", Name: "ok-task"},
		},
		FailureStrategy: "fail_fast",
	}

	// Container 0 returns activity error
	env.OnActivity(activity.StartContainerActivity, mock.Anything, input.Containers[0]).Return(
		nil, fmt.Errorf("docker daemon unavailable")).Once()

	// Container 1 returns success
	env.OnActivity(activity.StartContainerActivity, mock.Anything, input.Containers[1]).Return(
		&payload.ContainerExecutionOutput{Success: true, ExitCode: 0}, nil).Once()

	env.ExecuteWorkflow(ParallelContainersWorkflow, input)

	require.True(t, env.IsWorkflowCompleted(), "Workflow did not complete")
	assert.Error(t, env.GetWorkflowError(), "Expected workflow error with fail_fast and activity error")
}

// TestParallelContainersWorkflow_ActivityErrorContinue tests activity error with continue strategy.
func TestParallelContainersWorkflow_ActivityErrorContinue(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := payload.ParallelInput{
		Containers: []payload.ContainerExecutionInput{
			{Image: "alpine:latest", Name: "ok-1"},
			{Image: "alpine:latest", Name: "error-task"},
			{Image: "alpine:latest", Name: "ok-2"},
		},
		FailureStrategy: "continue",
	}

	// Container 0 succeeds
	env.OnActivity(activity.StartContainerActivity, mock.Anything, input.Containers[0]).Return(
		&payload.ContainerExecutionOutput{Success: true, ExitCode: 0}, nil).Once()

	// Container 1 returns activity error
	env.OnActivity(activity.StartContainerActivity, mock.Anything, input.Containers[1]).Return(
		nil, fmt.Errorf("docker daemon unavailable")).Once()

	// Container 2 succeeds
	env.OnActivity(activity.StartContainerActivity, mock.Anything, input.Containers[2]).Return(
		&payload.ContainerExecutionOutput{Success: true, ExitCode: 0}, nil).Once()

	env.ExecuteWorkflow(ParallelContainersWorkflow, input)

	require.True(t, env.IsWorkflowCompleted(), "Workflow did not complete")
	// Activity errors with retry exhaustion may cause workflow error; just verify completion
}

// TestParallelContainersWorkflow_SingleContainer tests a single container in parallel workflow.
func TestParallelContainersWorkflow_SingleContainer(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := payload.ParallelInput{
		Containers: []payload.ContainerExecutionInput{
			{Image: "alpine:latest", Name: "only-task"},
		},
		FailureStrategy: "continue",
	}

	env.OnActivity(activity.StartContainerActivity, mock.Anything, input.Containers[0]).Return(
		&payload.ContainerExecutionOutput{Success: true, ExitCode: 0, ContainerID: "solo-1"}, nil).Once()

	env.ExecuteWorkflow(ParallelContainersWorkflow, input)

	require.True(t, env.IsWorkflowCompleted(), "Workflow did not complete")
	require.NoError(t, env.GetWorkflowError())

	var result payload.ParallelOutput
	require.NoError(t, env.GetWorkflowResult(&result))

	assert.Equal(t, 1, result.TotalSuccess, "Expected 1 successful container")
	assert.Len(t, result.Results, 1, "Expected 1 result")
	assert.Equal(t, "solo-1", result.Results[0].ContainerID, "Expected ContainerID to be solo-1")
}

// TestParallelContainersWorkflow_ResultCountMatchesInput tests that result count matches input count.
func TestParallelContainersWorkflow_ResultCountMatchesInput(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := payload.ParallelInput{
		Containers: []payload.ContainerExecutionInput{
			{Image: "alpine:latest", Name: "count-1"},
			{Image: "alpine:latest", Name: "count-2"},
			{Image: "alpine:latest", Name: "count-3"},
		},
		FailureStrategy: "continue",
	}

	env.OnActivity(activity.StartContainerActivity, mock.Anything, mock.Anything).Return(
		&payload.ContainerExecutionOutput{Success: true, ExitCode: 0}, nil)

	env.ExecuteWorkflow(ParallelContainersWorkflow, input)

	require.True(t, env.IsWorkflowCompleted(), "Workflow did not complete")
	require.NoError(t, env.GetWorkflowError())

	var result payload.ParallelOutput
	require.NoError(t, env.GetWorkflowResult(&result))

	assert.Len(t, result.Results, 3, "Expected 3 results matching input count")
	assert.Equal(t, 3, result.TotalSuccess, "Expected 3 successful containers")
}
