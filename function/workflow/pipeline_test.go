package workflow

import (
	"context"
	"fmt"
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

func TestFunctionPipelineWorkflow_InvalidInput(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	input := payload.PipelineInput{
		Functions: []payload.FunctionExecutionInput{},
	}

	env.ExecuteWorkflow(FunctionPipelineWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	assert.Error(t, env.GetWorkflowError(), "Expected validation error for empty functions")
}

func TestFunctionPipelineWorkflow_StopOnError(t *testing.T) {
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

	env.OnActivity("ExecuteFunctionActivity", mock.Anything, input.Functions[0]).Return(
		&payload.FunctionExecutionOutput{Success: true}, nil).Once()

	env.OnActivity("ExecuteFunctionActivity", mock.Anything, input.Functions[1]).Return(
		&payload.FunctionExecutionOutput{Success: false, Error: "step2 failed"}, fmt.Errorf("activity failed")).Once()

	env.ExecuteWorkflow(FunctionPipelineWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	assert.Error(t, env.GetWorkflowError(), "Expected workflow error when StopOnError is true")
}

func TestFunctionPipelineWorkflow_ContinueOnError(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	input := payload.PipelineInput{
		Functions: []payload.FunctionExecutionInput{
			{Name: "step1"},
			{Name: "step2"},
			{Name: "step3"},
		},
		StopOnError: false,
	}

	env.OnActivity("ExecuteFunctionActivity", mock.Anything, input.Functions[0]).Return(
		&payload.FunctionExecutionOutput{Success: true}, nil).Once()

	env.OnActivity("ExecuteFunctionActivity", mock.Anything, input.Functions[1]).Return(
		&payload.FunctionExecutionOutput{Success: false, Error: "step2 failed"}, nil).Once()

	env.OnActivity("ExecuteFunctionActivity", mock.Anything, input.Functions[2]).Return(
		&payload.FunctionExecutionOutput{Success: true}, nil).Once()

	env.ExecuteWorkflow(FunctionPipelineWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError(), "Workflow should not error when StopOnError is false")

	var result payload.PipelineOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 2, result.TotalSuccess, "Expected 2 successful")
	assert.Equal(t, 1, result.TotalFailed, "Expected 1 failed")
}

func TestFunctionPipelineWorkflow_AllFail(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	input := payload.PipelineInput{
		Functions: []payload.FunctionExecutionInput{
			{Name: "step1"},
			{Name: "step2"},
			{Name: "step3"},
		},
		StopOnError: false,
	}

	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.Anything).Return(
		&payload.FunctionExecutionOutput{Success: false, Error: "failed"}, nil)

	env.ExecuteWorkflow(FunctionPipelineWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError(), "Workflow should not error when StopOnError is false")

	var result payload.PipelineOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 3, result.TotalFailed, "Expected 3 failed")
	assert.Equal(t, 0, result.TotalSuccess, "Expected 0 successful")
	assert.Len(t, result.Results, 3, "Expected 3 results")
}

func TestFunctionPipelineWorkflow_StopOnErrorFirstStep(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	input := payload.PipelineInput{
		Functions: []payload.FunctionExecutionInput{
			{Name: "step1"},
			{Name: "step2"},
			{Name: "step3"},
		},
		StopOnError: true,
	}

	env.OnActivity("ExecuteFunctionActivity", mock.Anything, input.Functions[0]).Return(
		nil, fmt.Errorf("function crashed")).Once()

	env.ExecuteWorkflow(FunctionPipelineWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	assert.Error(t, env.GetWorkflowError(), "Expected workflow error when first step fails with StopOnError")
}

func TestFunctionPipelineWorkflow_SingleFunction(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	input := payload.PipelineInput{
		Functions: []payload.FunctionExecutionInput{
			{Name: "only-step"},
		},
		StopOnError: true,
	}

	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.Anything).Return(
		&payload.FunctionExecutionOutput{Success: true, Name: "only-step"}, nil)

	env.ExecuteWorkflow(FunctionPipelineWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.PipelineOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 1, result.TotalSuccess, "Expected 1 successful")
	assert.Len(t, result.Results, 1, "Expected 1 result")
}

func TestFunctionPipelineWorkflow_ResultTracking(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	input := payload.PipelineInput{
		Functions: []payload.FunctionExecutionInput{
			{Name: "build"},
			{Name: "test"},
			{Name: "deploy"},
		},
		StopOnError: false,
	}

	callCount := 0
	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.Anything).Return(
		func(_ context.Context, in payload.FunctionExecutionInput) (*payload.FunctionExecutionOutput, error) {
			callCount++
			return &payload.FunctionExecutionOutput{
				Success:  true,
				Name:     in.Name,
				Duration: time.Duration(callCount) * time.Second,
			}, nil
		})

	env.ExecuteWorkflow(FunctionPipelineWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.PipelineOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 3, result.TotalSuccess, "Expected 3 successful")
	require.Len(t, result.Results, 3, "Expected 3 results")
	assert.Equal(t, "build", result.Results[0].Name)
	assert.Equal(t, "test", result.Results[1].Name)
	assert.Equal(t, "deploy", result.Results[2].Name)
	assert.True(t, result.Results[0].Success)
	assert.True(t, result.Results[1].Success)
	assert.True(t, result.Results[2].Success)
}
