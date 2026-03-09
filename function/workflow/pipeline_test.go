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
