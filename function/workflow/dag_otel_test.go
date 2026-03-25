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

func TestInstrumentedDAGWorkflow_Success(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.Anything).Return(
		&payload.FunctionExecutionOutput{
			Name:     "test-func",
			Success:  true,
			Result:   map[string]string{"status": "ok"},
			Duration: 1 * time.Second,
		}, nil)

	input := payload.DAGWorkflowInput{
		Nodes: []payload.FunctionDAGNode{
			{
				Name:     "build",
				Function: payload.FunctionExecutionInput{Name: "build-func"},
			},
			{
				Name:         "test",
				Function:     payload.FunctionExecutionInput{Name: "test-func"},
				Dependencies: []string{"build"},
			},
		},
		FailFast:    false,
		MaxParallel: 2,
	}

	env.ExecuteWorkflow(InstrumentedDAGWorkflow, input)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.FunctionDAGWorkflowOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 2, result.TotalSuccess)
	assert.Equal(t, 0, result.TotalFailed)
}

func TestInstrumentedDAGWorkflow_ValidationError(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := payload.DAGWorkflowInput{
		Nodes: []payload.FunctionDAGNode{},
	}

	env.ExecuteWorkflow(InstrumentedDAGWorkflow, input)
	require.True(t, env.IsWorkflowCompleted())
	assert.Error(t, env.GetWorkflowError())
}
