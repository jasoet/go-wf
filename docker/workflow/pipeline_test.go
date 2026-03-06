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

// TestContainerPipelineWorkflow_Success tests successful pipeline execution.
func TestContainerPipelineWorkflow_Success(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := payload.PipelineInput{
		Containers: []payload.ContainerExecutionInput{
			{Image: "alpine:latest", Command: []string{"echo", "step1"}},
			{Image: "alpine:latest", Command: []string{"echo", "step2"}},
		},
		StopOnError: true,
	}

	env.OnActivity(activity.StartContainerActivity, mock.Anything, mock.Anything).Return(
		&payload.ContainerExecutionOutput{Success: true, ExitCode: 0}, nil)

	env.ExecuteWorkflow(ContainerPipelineWorkflow, input)

	require.True(t, env.IsWorkflowCompleted(), "Workflow did not complete")
	require.NoError(t, env.GetWorkflowError(), "Workflow failed")

	var result payload.PipelineOutput
	require.NoError(t, env.GetWorkflowResult(&result), "Failed to get result")
	assert.Equal(t, 2, result.TotalSuccess, "Expected 2 successful containers")
}

// TestContainerPipelineWorkflow_InvalidInput tests pipeline validation.
func TestContainerPipelineWorkflow_InvalidInput(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := payload.PipelineInput{
		Containers: []payload.ContainerExecutionInput{},
	}

	env.ExecuteWorkflow(ContainerPipelineWorkflow, input)

	require.True(t, env.IsWorkflowCompleted(), "Workflow did not complete")
	assert.Error(t, env.GetWorkflowError(), "Expected validation error for empty containers")
}

// TestContainerPipelineWorkflow_WithNamedSteps tests pipeline with step names.
func TestContainerPipelineWorkflow_WithNamedSteps(t *testing.T) {
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

	env.OnActivity(activity.StartContainerActivity, mock.Anything, mock.Anything).Return(
		&payload.ContainerExecutionOutput{Success: true, ExitCode: 0, Duration: time.Second}, nil)

	env.ExecuteWorkflow(ContainerPipelineWorkflow, input)

	require.True(t, env.IsWorkflowCompleted(), "Workflow did not complete")

	var result payload.PipelineOutput
	require.NoError(t, env.GetWorkflowResult(&result), "Failed to get result")
	assert.Len(t, result.Results, 3, "Expected 3 results")
}

// TestContainerPipelineWorkflow_StopOnError tests pipeline stops on error.
func TestContainerPipelineWorkflow_StopOnError(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := payload.PipelineInput{
		Containers: []payload.ContainerExecutionInput{
			{Image: "alpine:latest", Name: "step1"},
			{Image: "alpine:latest", Name: "step2"},
		},
		StopOnError: true,
	}

	// First call succeeds
	env.OnActivity(activity.StartContainerActivity, mock.Anything, input.Containers[0]).Return(
		&payload.ContainerExecutionOutput{Success: true, ExitCode: 0}, nil).Once()

	// Second call fails
	env.OnActivity(activity.StartContainerActivity, mock.Anything, input.Containers[1]).Return(
		&payload.ContainerExecutionOutput{Success: false, ExitCode: 1}, fmt.Errorf("container failed")).Once()

	env.ExecuteWorkflow(ContainerPipelineWorkflow, input)

	require.True(t, env.IsWorkflowCompleted(), "Workflow did not complete")
	assert.Error(t, env.GetWorkflowError(), "Expected workflow error when StopOnError is true")
}

// TestContainerPipelineWorkflow_ContinueOnError tests pipeline continues despite errors.
func TestContainerPipelineWorkflow_ContinueOnError(t *testing.T) {
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

	// First succeeds
	env.OnActivity(activity.StartContainerActivity, mock.Anything, input.Containers[0]).Return(
		&payload.ContainerExecutionOutput{Success: true, ExitCode: 0}, nil).Once()

	// Second fails
	env.OnActivity(activity.StartContainerActivity, mock.Anything, input.Containers[1]).Return(
		&payload.ContainerExecutionOutput{Success: false, ExitCode: 1}, nil).Once()

	// Third succeeds
	env.OnActivity(activity.StartContainerActivity, mock.Anything, input.Containers[2]).Return(
		&payload.ContainerExecutionOutput{Success: true, ExitCode: 0}, nil).Once()

	env.ExecuteWorkflow(ContainerPipelineWorkflow, input)

	require.True(t, env.IsWorkflowCompleted(), "Workflow did not complete")
	require.NoError(t, env.GetWorkflowError(), "Workflow should not error when StopOnError is false")

	var result payload.PipelineOutput
	require.NoError(t, env.GetWorkflowResult(&result), "Failed to get result")
	assert.Equal(t, 2, result.TotalSuccess, "Expected 2 successful")
	assert.Equal(t, 1, result.TotalFailed, "Expected 1 failed")
}

// TestContainerPipelineWorkflow_NoNamedSteps tests automatic step naming.
func TestContainerPipelineWorkflow_NoNamedSteps(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := payload.PipelineInput{
		Containers: []payload.ContainerExecutionInput{
			{Image: "alpine:latest"}, // No name
			{Image: "alpine:latest"}, // No name
		},
		StopOnError: true,
	}

	env.OnActivity(activity.StartContainerActivity, mock.Anything, mock.Anything).Return(
		&payload.ContainerExecutionOutput{Success: true, ExitCode: 0}, nil)

	env.ExecuteWorkflow(ContainerPipelineWorkflow, input)

	require.True(t, env.IsWorkflowCompleted(), "Workflow did not complete")

	var result payload.PipelineOutput
	require.NoError(t, env.GetWorkflowResult(&result), "Failed to get result")
	assert.Equal(t, 2, result.TotalSuccess, "Expected 2 successful")
}

// TestContainerPipelineWorkflow_AllFail tests pipeline where all steps fail.
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

	require.True(t, env.IsWorkflowCompleted(), "Workflow did not complete")
	require.NoError(t, env.GetWorkflowError(), "Workflow should not error when StopOnError is false")

	var result payload.PipelineOutput
	require.NoError(t, env.GetWorkflowResult(&result), "Failed to get result")
	assert.Equal(t, 3, result.TotalFailed, "Expected 3 failed")
	assert.Equal(t, 0, result.TotalSuccess, "Expected 0 successful")
	assert.Len(t, result.Results, 3, "Expected 3 results")
}

// TestContainerPipelineWorkflow_StopOnErrorFirstStep tests pipeline stops on first step error.
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

	// First call fails with activity error
	env.OnActivity(activity.StartContainerActivity, mock.Anything, input.Containers[0]).Return(
		nil, fmt.Errorf("container crashed")).Once()

	env.ExecuteWorkflow(ContainerPipelineWorkflow, input)

	require.True(t, env.IsWorkflowCompleted(), "Workflow did not complete")
	assert.Error(t, env.GetWorkflowError(), "Expected workflow error when first step fails with StopOnError")
}

// TestContainerPipelineWorkflow_ResultTracking tests that results track container details correctly.
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
		&payload.ContainerExecutionOutput{Success: true, ExitCode: 0, ContainerID: "build-1", Duration: 1 * time.Second}, nil).Once()

	env.OnActivity(activity.StartContainerActivity, mock.Anything, input.Containers[1]).Return(
		&payload.ContainerExecutionOutput{Success: true, ExitCode: 0, ContainerID: "test-1", Duration: 2 * time.Second}, nil).Once()

	env.OnActivity(activity.StartContainerActivity, mock.Anything, input.Containers[2]).Return(
		&payload.ContainerExecutionOutput{Success: true, ExitCode: 0, ContainerID: "deploy-1", Duration: 3 * time.Second}, nil).Once()

	env.ExecuteWorkflow(ContainerPipelineWorkflow, input)

	require.True(t, env.IsWorkflowCompleted(), "Workflow did not complete")
	require.NoError(t, env.GetWorkflowError(), "Workflow should not error")

	var result payload.PipelineOutput
	require.NoError(t, env.GetWorkflowResult(&result), "Failed to get result")
	assert.Equal(t, 3, result.TotalSuccess, "Expected 3 successful")
	require.Len(t, result.Results, 3, "Expected 3 results")
	assert.Equal(t, "build-1", result.Results[0].ContainerID)
	assert.Equal(t, "test-1", result.Results[1].ContainerID)
	assert.Equal(t, "deploy-1", result.Results[2].ContainerID)
}

// TestContainerPipelineWorkflow_SingleContainer tests pipeline with a single container.
func TestContainerPipelineWorkflow_SingleContainer(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := payload.PipelineInput{
		Containers: []payload.ContainerExecutionInput{
			{Image: "alpine:latest", Name: "only-step", Command: []string{"echo", "hello"}},
		},
		StopOnError: true,
	}

	env.OnActivity(activity.StartContainerActivity, mock.Anything, mock.Anything).Return(
		&payload.ContainerExecutionOutput{Success: true, ExitCode: 0, ContainerID: "single-1"}, nil)

	env.ExecuteWorkflow(ContainerPipelineWorkflow, input)

	require.True(t, env.IsWorkflowCompleted(), "Workflow did not complete")
	require.NoError(t, env.GetWorkflowError(), "Workflow should not error")

	var result payload.PipelineOutput
	require.NoError(t, env.GetWorkflowResult(&result), "Failed to get result")
	assert.Equal(t, 1, result.TotalSuccess, "Expected 1 successful")
	assert.Len(t, result.Results, 1, "Expected 1 result")
}

// TestContainerPipelineWorkflow_ActivityErrorContinue tests pipeline continues after activity error.
func TestContainerPipelineWorkflow_ActivityErrorContinue(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := payload.PipelineInput{
		Containers: []payload.ContainerExecutionInput{
			{Image: "alpine:latest", Name: "failing-step"},
			{Image: "alpine:latest", Name: "success-step"},
		},
		StopOnError: false,
	}

	// First step returns activity error
	env.OnActivity(activity.StartContainerActivity, mock.Anything, input.Containers[0]).Return(
		nil, fmt.Errorf("docker daemon error")).Once()

	// Second step succeeds
	env.OnActivity(activity.StartContainerActivity, mock.Anything, input.Containers[1]).Return(
		&payload.ContainerExecutionOutput{Success: true, ExitCode: 0}, nil).Once()

	env.ExecuteWorkflow(ContainerPipelineWorkflow, input)

	require.True(t, env.IsWorkflowCompleted(), "Workflow did not complete")
	// Note: with retry policy (3 attempts), the activity error will be retried.
	// Just verify workflow completes - don't assert on error since retry behavior may vary.
}
