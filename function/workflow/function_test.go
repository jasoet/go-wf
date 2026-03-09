package workflow

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"

	"github.com/jasoet/go-wf/function/payload"
)

func TestExecuteFunctionWorkflow_Success(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	input := payload.FunctionExecutionInput{Name: "my-func", Args: map[string]string{"key": "val"}}

	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.Anything).Return(&payload.FunctionExecutionOutput{
		Name:     "my-func",
		Success:  true,
		Result:   map[string]string{"out": "result"},
		Duration: 1 * time.Second,
	}, nil)

	env.ExecuteWorkflow(ExecuteFunctionWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.FunctionExecutionOutput
	require.NoError(t, env.GetWorkflowResult(&result))

	assert.Equal(t, "my-func", result.Name)
	assert.True(t, result.Success)
	assert.Equal(t, "result", result.Result["out"])
}

func TestExecuteFunctionWorkflow_InvalidInput(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := payload.FunctionExecutionInput{} // Missing required Name

	env.ExecuteWorkflow(ExecuteFunctionWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	assert.Error(t, env.GetWorkflowError())
}

func TestExecuteFunctionWorkflow_ActivityError(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	input := payload.FunctionExecutionInput{Name: "fail-func"}

	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.Anything).Return(
		nil, fmt.Errorf("activity failed"))

	env.ExecuteWorkflow(ExecuteFunctionWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	assert.Error(t, env.GetWorkflowError())
}
