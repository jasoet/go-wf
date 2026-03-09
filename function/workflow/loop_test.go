package workflow

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"

	"github.com/jasoet/go-wf/function/payload"
)

func TestLoopWorkflow_Success(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	input := payload.LoopInput{
		Items:    []string{"a", "b", "c"},
		Template: payload.FunctionExecutionInput{Name: "process-{{item}}"},
	}

	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.Anything).Return(&payload.FunctionExecutionOutput{
		Success:  true,
		Duration: 1 * time.Second,
	}, nil)

	env.ExecuteWorkflow(LoopWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.LoopOutput
	require.NoError(t, env.GetWorkflowResult(&result))

	assert.Equal(t, 3, result.TotalSuccess)
	assert.Equal(t, 3, result.ItemCount)
}

func TestParameterizedLoopWorkflow_Success(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	input := payload.ParameterizedLoopInput{
		Parameters: map[string][]string{
			"env": {"dev", "prod"},
		},
		Template: payload.FunctionExecutionInput{
			Name: "deploy",
			Args: map[string]string{"target": "{{.env}}"},
		},
	}

	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.Anything).Return(&payload.FunctionExecutionOutput{
		Success:  true,
		Duration: 1 * time.Second,
	}, nil)

	env.ExecuteWorkflow(ParameterizedLoopWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.LoopOutput
	require.NoError(t, env.GetWorkflowResult(&result))

	assert.Equal(t, 2, result.TotalSuccess)
	assert.Equal(t, 2, result.ItemCount)
}
