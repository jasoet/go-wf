package workflow

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"

	"github.com/jasoet/go-wf/function/payload"
	generic "github.com/jasoet/go-wf/workflow"
)

func TestParallelFunctionsWorkflow_Success(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	input := generic.ParallelInput[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput]{
		Tasks: []*payload.FunctionExecutionInput{
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

	var result generic.ParallelOutput[payload.FunctionExecutionOutput]
	require.NoError(t, env.GetWorkflowResult(&result))

	assert.Equal(t, 2, result.TotalSuccess)
	assert.Equal(t, 0, result.TotalFailed)
	assert.Len(t, result.Results, 2)
}

func TestParallelFunctionsWorkflow_InvalidInput(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	input := generic.ParallelInput[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput]{
		Tasks: []*payload.FunctionExecutionInput{},
	}

	env.ExecuteWorkflow(ParallelFunctionsWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	assert.Error(t, env.GetWorkflowError(), "Expected validation error for empty tasks")
}

func TestParallelFunctionsWorkflow_FailFast(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	input := generic.ParallelInput[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput]{
		Tasks: []*payload.FunctionExecutionInput{
			{Name: "task1"},
			{Name: "task2"},
		},
		FailureStrategy: "fail_fast",
	}

	env.OnActivity("ExecuteFunctionActivity", mock.Anything, *input.Tasks[0]).Return(
		&payload.FunctionExecutionOutput{Success: false, Error: "task1 failed"}, nil).Once()

	env.OnActivity("ExecuteFunctionActivity", mock.Anything, *input.Tasks[1]).Return(
		&payload.FunctionExecutionOutput{Success: true}, nil).Once()

	env.ExecuteWorkflow(ParallelFunctionsWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	assert.Error(t, env.GetWorkflowError(), "Expected workflow error with fail_fast strategy")
}

func TestParallelFunctionsWorkflow_ContinueStrategy(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	input := generic.ParallelInput[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput]{
		Tasks: []*payload.FunctionExecutionInput{
			{Name: "task1"},
			{Name: "task2"},
		},
		FailureStrategy: "continue",
	}

	env.OnActivity("ExecuteFunctionActivity", mock.Anything, *input.Tasks[0]).Return(
		&payload.FunctionExecutionOutput{Success: false, Error: "task1 failed"}, nil).Once()

	env.OnActivity("ExecuteFunctionActivity", mock.Anything, *input.Tasks[1]).Return(
		&payload.FunctionExecutionOutput{Success: true}, nil).Once()

	env.ExecuteWorkflow(ParallelFunctionsWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError(), "Workflow should not error with continue strategy")

	var result generic.ParallelOutput[payload.FunctionExecutionOutput]
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 1, result.TotalFailed, "Expected 1 failed")
}

func TestParallelFunctionsWorkflow_MultipleFailuresContinue(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	input := generic.ParallelInput[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput]{
		Tasks: []*payload.FunctionExecutionInput{
			{Name: "fn-0"},
			{Name: "fn-1"},
			{Name: "fn-2"},
			{Name: "fn-3"},
		},
		FailureStrategy: "continue",
	}

	env.OnActivity("ExecuteFunctionActivity", mock.Anything, *input.Tasks[0]).Return(
		&payload.FunctionExecutionOutput{Success: true, Name: "fn-0"}, nil).Once()
	env.OnActivity("ExecuteFunctionActivity", mock.Anything, *input.Tasks[2]).Return(
		&payload.FunctionExecutionOutput{Success: true, Name: "fn-2"}, nil).Once()

	env.OnActivity("ExecuteFunctionActivity", mock.Anything, *input.Tasks[1]).Return(
		&payload.FunctionExecutionOutput{Success: false, Error: "failed", Name: "fn-1"}, nil).Once()
	env.OnActivity("ExecuteFunctionActivity", mock.Anything, *input.Tasks[3]).Return(
		&payload.FunctionExecutionOutput{Success: false, Error: "failed", Name: "fn-3"}, nil).Once()

	env.ExecuteWorkflow(ParallelFunctionsWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result generic.ParallelOutput[payload.FunctionExecutionOutput]
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 2, result.TotalSuccess, "Expected 2 successful")
	assert.Equal(t, 2, result.TotalFailed, "Expected 2 failed")
	assert.Len(t, result.Results, 4, "Expected 4 results")
}

func TestParallelFunctionsWorkflow_AllFailContinue(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	input := generic.ParallelInput[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput]{
		Tasks: []*payload.FunctionExecutionInput{
			{Name: "fail-1"},
			{Name: "fail-2"},
			{Name: "fail-3"},
		},
		FailureStrategy: "continue",
	}

	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.Anything).Return(
		&payload.FunctionExecutionOutput{Success: false, Error: "failed"}, nil)

	env.ExecuteWorkflow(ParallelFunctionsWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result generic.ParallelOutput[payload.FunctionExecutionOutput]
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 3, result.TotalFailed, "Expected 3 failed")
	assert.Equal(t, 0, result.TotalSuccess, "Expected 0 successful")
}

func TestParallelFunctionsWorkflow_ActivityErrorFailFast(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	input := generic.ParallelInput[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput]{
		Tasks: []*payload.FunctionExecutionInput{
			{Name: "error-task"},
			{Name: "ok-task"},
		},
		FailureStrategy: "fail_fast",
	}

	env.OnActivity("ExecuteFunctionActivity", mock.Anything, *input.Tasks[0]).Return(
		nil, fmt.Errorf("function execution error")).Once()

	env.OnActivity("ExecuteFunctionActivity", mock.Anything, *input.Tasks[1]).Return(
		&payload.FunctionExecutionOutput{Success: true}, nil).Once()

	env.ExecuteWorkflow(ParallelFunctionsWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	assert.Error(t, env.GetWorkflowError(), "Expected workflow error with fail_fast and activity error")
}

func TestParallelFunctionsWorkflow_SingleFunction(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	input := generic.ParallelInput[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput]{
		Tasks: []*payload.FunctionExecutionInput{
			{Name: "only-task"},
		},
		FailureStrategy: "continue",
	}

	env.OnActivity("ExecuteFunctionActivity", mock.Anything, *input.Tasks[0]).Return(
		&payload.FunctionExecutionOutput{Success: true, Name: "only-task"}, nil).Once()

	env.ExecuteWorkflow(ParallelFunctionsWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result generic.ParallelOutput[payload.FunctionExecutionOutput]
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 1, result.TotalSuccess, "Expected 1 successful")
	assert.Len(t, result.Results, 1, "Expected 1 result")
}

func TestParallelFunctionsWorkflow_ResultCountMatchesInput(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	input := generic.ParallelInput[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput]{
		Tasks: []*payload.FunctionExecutionInput{
			{Name: "count-1"},
			{Name: "count-2"},
			{Name: "count-3"},
		},
		FailureStrategy: "continue",
	}

	var callCount atomic.Int32
	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.Anything).Return(
		func(_ context.Context, _ payload.FunctionExecutionInput) (*payload.FunctionExecutionOutput, error) {
			callCount.Add(1)
			return &payload.FunctionExecutionOutput{Success: true, Duration: 1 * time.Second}, nil
		})

	env.ExecuteWorkflow(ParallelFunctionsWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result generic.ParallelOutput[payload.FunctionExecutionOutput]
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Len(t, result.Results, 3, "Expected 3 results matching input count")
	assert.Equal(t, 3, result.TotalSuccess, "Expected 3 successful")
}
