package workflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
	wf "go.temporal.io/sdk/workflow"
)

// pipelineWrapper is a non-generic workflow wrapper for testing.
func pipelineWrapper(ctx wf.Context, input PipelineInput[testInput, testOutput]) (*PipelineOutput[testOutput], error) {
	return PipelineWorkflow[testInput, testOutput](ctx, input)
}

func TestPipelineWorkflow_AllSuccess(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerTestActivity(env)

	input := PipelineInput[testInput, testOutput]{
		Tasks: []testInput{
			{Name: "step1", Value: "a", Activity: "TestActivity"},
			{Name: "step2", Value: "b", Activity: "TestActivity"},
			{Name: "step3", Value: "c", Activity: "TestActivity"},
		},
		StopOnError: true,
	}

	env.OnActivity("TestActivity", mock.Anything, mock.Anything).Return(
		&testOutput{Result: "ok", Success: true}, nil)

	env.ExecuteWorkflow(pipelineWrapper, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result PipelineOutput[testOutput]
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 3, result.TotalSuccess)
	assert.Equal(t, 0, result.TotalFailed)
	assert.Len(t, result.Results, 3)
}

func TestPipelineWorkflow_StopOnError(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerTestActivity(env)

	input := PipelineInput[testInput, testOutput]{
		Tasks: []testInput{
			{Name: "step1", Value: "a", Activity: "TestActivity"},
			{Name: "step2", Value: "b", Activity: "TestActivity"},
			{Name: "step3", Value: "c", Activity: "TestActivity"},
		},
		StopOnError: true,
	}

	// First succeeds
	env.OnActivity("TestActivity", mock.Anything, input.Tasks[0]).Return(
		&testOutput{Result: "ok", Success: true}, nil).Once()

	// Second fails
	env.OnActivity("TestActivity", mock.Anything, input.Tasks[1]).Return(
		&testOutput{Result: "fail", Success: false}, nil).Once()

	env.ExecuteWorkflow(pipelineWrapper, input)

	require.True(t, env.IsWorkflowCompleted())
	assert.Error(t, env.GetWorkflowError())
	assert.Contains(t, env.GetWorkflowError().Error(), "pipeline stopped at step 2")
}

func TestPipelineWorkflow_ContinueOnError(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerTestActivity(env)

	input := PipelineInput[testInput, testOutput]{
		Tasks: []testInput{
			{Name: "step1", Value: "a", Activity: "TestActivity"},
			{Name: "step2", Value: "b", Activity: "TestActivity"},
			{Name: "step3", Value: "c", Activity: "TestActivity"},
		},
		StopOnError: false,
	}

	// First succeeds
	env.OnActivity("TestActivity", mock.Anything, input.Tasks[0]).Return(
		&testOutput{Result: "ok", Success: true}, nil).Once()

	// Second fails
	env.OnActivity("TestActivity", mock.Anything, input.Tasks[1]).Return(
		&testOutput{Result: "fail", Success: false}, nil).Once()

	// Third succeeds
	env.OnActivity("TestActivity", mock.Anything, input.Tasks[2]).Return(
		&testOutput{Result: "ok", Success: true}, nil).Once()

	env.ExecuteWorkflow(pipelineWrapper, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result PipelineOutput[testOutput]
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 2, result.TotalSuccess)
	assert.Equal(t, 1, result.TotalFailed)
	assert.Len(t, result.Results, 3)
}

func TestPipelineWorkflow_InvalidInput(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := PipelineInput[testInput, testOutput]{
		Tasks: []testInput{},
	}

	env.ExecuteWorkflow(pipelineWrapper, input)

	require.True(t, env.IsWorkflowCompleted())
	assert.Error(t, env.GetWorkflowError())
}
