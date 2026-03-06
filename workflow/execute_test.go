package workflow

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
	wf "go.temporal.io/sdk/workflow"
)

// executeTaskWrapper is a non-generic workflow wrapper for testing.
func executeTaskWrapper(ctx wf.Context, input testInput) (*testOutput, error) {
	return ExecuteTaskWorkflow[testInput, testOutput](ctx, input)
}

// executeTaskWithTimeoutWrapper is a non-generic workflow wrapper for testing.
func executeTaskWithTimeoutWrapper(ctx wf.Context, input testInput) (*testOutput, error) {
	return ExecuteTaskWorkflowWithTimeout[testInput, testOutput](ctx, input, 5*time.Minute)
}

func TestExecuteTaskWorkflow_Success(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerTestActivity(env)

	input := testInput{Name: "test", Value: "hello", Activity: "TestActivity"}

	env.OnActivity("TestActivity", mock.Anything, mock.Anything).Return(
		&testOutput{Result: "done", Success: true}, nil)

	env.ExecuteWorkflow(executeTaskWrapper, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result testOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.True(t, result.Success)
	assert.Equal(t, "done", result.Result)
}

func TestExecuteTaskWorkflow_InvalidInput(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := testInput{Name: "test", Value: ""} // Value is required

	env.ExecuteWorkflow(executeTaskWrapper, input)

	require.True(t, env.IsWorkflowCompleted())
	assert.Error(t, env.GetWorkflowError())
	assert.Contains(t, env.GetWorkflowError().Error(), "invalid input")
}

func TestExecuteTaskWorkflow_ActivityFailure(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerTestActivity(env)

	input := testInput{Name: "test", Value: "hello", Activity: "TestActivity"}

	env.OnActivity("TestActivity", mock.Anything, mock.Anything).Return(
		nil, assert.AnError)

	env.ExecuteWorkflow(executeTaskWrapper, input)

	require.True(t, env.IsWorkflowCompleted())
	assert.Error(t, env.GetWorkflowError())
}

func TestExecuteTaskWorkflowWithTimeout_Success(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerTestActivity(env)

	input := testInput{Name: "test", Value: "hello", Activity: "TestActivity"}

	env.OnActivity("TestActivity", mock.Anything, mock.Anything).Return(
		&testOutput{Result: "done", Success: true}, nil)

	env.ExecuteWorkflow(executeTaskWithTimeoutWrapper, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result testOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.True(t, result.Success)
}
