//go:build integration
// +build integration

package docker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
)

func TestIntegrationContainerExecution(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Register the activity
	env.RegisterActivity(StartContainerActivity)

	input := ContainerExecutionInput{
		Image:      "alpine:latest",
		Command:    []string{"echo", "Hello, World!"},
		AutoRemove: true,
	}

	env.ExecuteWorkflow(ExecuteContainerWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result ContainerExecutionOutput
	err := env.GetWorkflowResult(&result)
	require.NoError(t, err)
	assert.NotEmpty(t, result.ContainerID)
	assert.Equal(t, 0, result.ExitCode)
}

func TestIntegrationPipelineWorkflow(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	env.RegisterActivity(StartContainerActivity)

	input := PipelineInput{
		Containers: []ContainerExecutionInput{
			{
				Image:      "alpine:latest",
				Command:    []string{"echo", "Step 1"},
				AutoRemove: true,
			},
			{
				Image:      "alpine:latest",
				Command:    []string{"echo", "Step 2"},
				AutoRemove: true,
			},
		},
		StopOnError: true,
	}

	env.ExecuteWorkflow(ContainerPipelineWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
}

func TestIntegrationParallelWorkflow(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	env.RegisterActivity(StartContainerActivity)

	input := ParallelInput{
		Containers: []ContainerExecutionInput{
			{
				Image:      "alpine:latest",
				Name:       "task1",
				Command:    []string{"echo", "Task 1"},
				AutoRemove: true,
			},
			{
				Image:      "alpine:latest",
				Name:       "task2",
				Command:    []string{"echo", "Task 2"},
				AutoRemove: true,
			},
		},
		FailureStrategy: "continue",
	}

	env.ExecuteWorkflow(ParallelContainersWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
}

func TestIntegrationContainerWithWaitStrategy(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	env.RegisterActivity(StartContainerActivity)

	input := ContainerExecutionInput{
		Image: "nginx:alpine",
		Ports: []string{"0:80"}, // Use random port
		WaitStrategy: WaitStrategyConfig{
			Type:       "log",
			LogMessage: "start worker processes",
		},
		AutoRemove:   true,
		StartTimeout: 30 * time.Second,
	}

	env.ExecuteWorkflow(ExecuteContainerWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
}

func TestIntegrationContainerWithEnvironment(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	env.RegisterActivity(StartContainerActivity)

	input := ContainerExecutionInput{
		Image:   "alpine:latest",
		Command: []string{"sh", "-c", "echo $TEST_VAR"},
		Env: map[string]string{
			"TEST_VAR": "test_value",
		},
		AutoRemove: true,
	}

	env.ExecuteWorkflow(ExecuteContainerWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result ContainerExecutionOutput
	err := env.GetWorkflowResult(&result)
	require.NoError(t, err)
	assert.Contains(t, result.Stdout, "test_value")
}

func TestIntegrationDAGWorkflow(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	env.RegisterActivity(StartContainerActivity)

	input := DAGWorkflowInput{
		Nodes: []DAGNode{
			{
				Name: "first",
				Container: ExtendedContainerInput{
					ContainerExecutionInput: ContainerExecutionInput{
						Image:      "alpine:latest",
						Command:    []string{"echo", "First task"},
						AutoRemove: true,
					},
				},
			},
			{
				Name: "second",
				Container: ExtendedContainerInput{
					ContainerExecutionInput: ContainerExecutionInput{
						Image:      "alpine:latest",
						Command:    []string{"echo", "Second task"},
						AutoRemove: true,
					},
				},
				Dependencies: []string{"first"},
			},
		},
		FailFast: false,
	}

	env.ExecuteWorkflow(DAGWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
}

func TestIntegrationContainerWithVolumes(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	env.RegisterActivity(StartContainerActivity)

	// Create a temporary file to mount
	input := ContainerExecutionInput{
		Image:      "alpine:latest",
		Command:    []string{"sh", "-c", "echo 'test content' > /data/test.txt && cat /data/test.txt"},
		AutoRemove: true,
	}

	env.ExecuteWorkflow(ExecuteContainerWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
}

func TestIntegrationWorkflowRegistration(t *testing.T) {
	// Test that workflows and activities can be registered without error
	w := &mockWorker{}

	// These should not panic
	assert.NotPanics(t, func() {
		RegisterWorkflows(w)
	})
	assert.NotPanics(t, func() {
		RegisterActivities(w)
	})
	assert.NotPanics(t, func() {
		RegisterAll(w)
	})
}
