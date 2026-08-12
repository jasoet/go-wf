package workflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"

	"github.com/jasoet/go-wf/v2/container/payload"
	generic "github.com/jasoet/go-wf/v2/workflow"
)

// TestGenericContainerPipelineWorkflow tests the generic pipeline entry point.
func TestGenericContainerPipelineWorkflow(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerContainerActivity(env)

	input := generic.PipelineInput[*payload.ContainerExecutionInput, payload.ContainerExecutionOutput]{
		Tasks: []*payload.ContainerExecutionInput{
			{Image: "alpine:latest", Command: []string{"echo", "one"}},
			{Image: "alpine:latest", Command: []string{"echo", "two"}},
		},
		StopOnError: true,
	}

	env.OnActivity("StartContainerActivity", mock.Anything, mock.Anything).Return(
		&payload.ContainerExecutionOutput{Success: true, ExitCode: 0}, nil)

	env.ExecuteWorkflow(GenericContainerPipelineWorkflow, input)

	require.True(t, env.IsWorkflowCompleted(), "Workflow did not complete")
	require.NoError(t, env.GetWorkflowError(), "Workflow failed")

	var result generic.PipelineOutput[payload.ContainerExecutionOutput]
	require.NoError(t, env.GetWorkflowResult(&result), "Failed to get result")
	assert.Equal(t, 2, result.TotalSuccess)
	assert.Len(t, result.Results, 2)
}
