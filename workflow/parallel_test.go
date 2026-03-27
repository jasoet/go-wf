package workflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
	wf "go.temporal.io/sdk/workflow"
)

// parallelWrapper is a non-generic workflow wrapper for testing.
func parallelWrapper(ctx wf.Context, input ParallelInput[testInput, testOutput]) (*ParallelOutput[testOutput], error) {
	return ParallelWorkflow[testInput, testOutput](ctx, input)
}

func TestParallelWorkflow_AllSuccess(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerTestActivity(env)

	input := ParallelInput[testInput, testOutput]{
		Tasks: []testInput{
			{Name: "task1", Value: "a", Activity: "TestActivity"},
			{Name: "task2", Value: "b", Activity: "TestActivity"},
			{Name: "task3", Value: "c", Activity: "TestActivity"},
		},
		FailureStrategy: "continue",
	}

	env.OnActivity("TestActivity", mock.Anything, mock.Anything).Return(
		&testOutput{Result: "ok", Success: true}, nil)

	env.ExecuteWorkflow(parallelWrapper, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result ParallelOutput[testOutput]
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 3, result.TotalSuccess)
	assert.Equal(t, 0, result.TotalFailed)
	assert.Len(t, result.Results, 3)
}

func TestParallelWorkflow_FailFast(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerTestActivity(env)

	input := ParallelInput[testInput, testOutput]{
		Tasks: []testInput{
			{Name: "task1", Value: "a", Activity: "TestActivity"},
			{Name: "task2", Value: "b", Activity: "TestActivity"},
			{Name: "task3", Value: "c", Activity: "TestActivity"},
		},
		FailureStrategy: FailureStrategyFailFast,
	}

	// First succeeds
	env.OnActivity("TestActivity", mock.Anything, input.Tasks[0]).Return(
		&testOutput{Result: "ok", Success: true}, nil).Once()

	// Second fails
	env.OnActivity("TestActivity", mock.Anything, input.Tasks[1]).Return(
		&testOutput{Result: "fail", Success: false}, nil).Once()

	// Third would succeed but should not matter
	env.OnActivity("TestActivity", mock.Anything, input.Tasks[2]).Return(
		&testOutput{Result: "ok", Success: true}, nil).Once()

	env.ExecuteWorkflow(parallelWrapper, input)

	require.True(t, env.IsWorkflowCompleted())
	assert.Error(t, env.GetWorkflowError())
	assert.Contains(t, env.GetWorkflowError().Error(), "parallel execution failed")
}

func TestParallelWorkflow_ContinueOnFailure(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerTestActivity(env)

	input := ParallelInput[testInput, testOutput]{
		Tasks: []testInput{
			{Name: "task1", Value: "a", Activity: "TestActivity"},
			{Name: "task2", Value: "b", Activity: "TestActivity"},
			{Name: "task3", Value: "c", Activity: "TestActivity"},
		},
		FailureStrategy: FailureStrategyContinue,
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

	env.ExecuteWorkflow(parallelWrapper, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result ParallelOutput[testOutput]
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 2, result.TotalSuccess)
	assert.Equal(t, 1, result.TotalFailed)
	assert.Len(t, result.Results, 3)
}

func TestParallelWorkflow_InvalidInput(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := ParallelInput[testInput, testOutput]{
		Tasks: []testInput{},
	}

	env.ExecuteWorkflow(parallelWrapper, input)

	require.True(t, env.IsWorkflowCompleted())
	assert.Error(t, env.GetWorkflowError())
}
