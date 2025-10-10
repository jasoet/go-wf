//go:build integration
// +build integration

package docker

import (
	"testing"
	"time"

	"github.com/jasoet/go-wf/docker/activity"
	"github.com/jasoet/go-wf/docker/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
)

// mockWorker implements the Worker interface for testing
type mockWorker struct{}

func (m *mockWorker) RegisterWorkflow(interface{})                         {}
func (m *mockWorker) RegisterWorkflowWithOptions(interface{}, interface{}) {}
func (m *mockWorker) RegisterDynamicWorkflow(interface{}, interface{})     {}
func (m *mockWorker) RegisterActivity(interface{})                         {}
func (m *mockWorker) RegisterActivityWithOptions(interface{}, interface{}) {}
func (m *mockWorker) RegisterDynamicActivity(interface{}, interface{})     {}
func (m *mockWorker) RegisterNexusService(interface{})                     {}
func (m *mockWorker) Run(<-chan interface{}) error                         { return nil }
func (m *mockWorker) Start() error                                         { return nil }
func (m *mockWorker) Stop()                                                {}

func TestIntegrationContainerExecution(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Register the activity
	env.RegisterActivity(activity.StartContainerActivity)

	input := ContainerExecutionInput{
		Image:      "alpine:latest",
		Command:    []string{"echo", "Hello, World!"},
		AutoRemove: true,
	}

	env.ExecuteWorkflow(workflow.ExecuteContainerWorkflow, input)

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

	env.RegisterActivity(activity.StartContainerActivity)

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

	env.ExecuteWorkflow(workflow.ContainerPipelineWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
}

func TestIntegrationParallelWorkflow(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	env.RegisterActivity(activity.StartContainerActivity)

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

	env.ExecuteWorkflow(workflow.ParallelContainersWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
}

func TestIntegrationContainerWithWaitStrategy(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	env.RegisterActivity(activity.StartContainerActivity)

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

	env.ExecuteWorkflow(workflow.ExecuteContainerWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
}

func TestIntegrationContainerWithEnvironment(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	env.RegisterActivity(activity.StartContainerActivity)

	input := ContainerExecutionInput{
		Image:   "alpine:latest",
		Command: []string{"sh", "-c", "echo $TEST_VAR"},
		Env: map[string]string{
			"TEST_VAR": "test_value",
		},
		AutoRemove: true,
	}

	env.ExecuteWorkflow(workflow.ExecuteContainerWorkflow, input)

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

	env.RegisterActivity(activity.StartContainerActivity)

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

	env.ExecuteWorkflow(workflow.DAGWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
}

func TestIntegrationContainerWithVolumes(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	env.RegisterActivity(activity.StartContainerActivity)

	// Create a temporary file to mount
	input := ContainerExecutionInput{
		Image:      "alpine:latest",
		Command:    []string{"sh", "-c", "echo 'test content' > /data/test.txt && cat /data/test.txt"},
		AutoRemove: true,
	}

	env.ExecuteWorkflow(workflow.ExecuteContainerWorkflow, input)

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
