package docker

import (
	"github.com/nexus-rpc/sdk-go/nexus"
	sdkactivity "go.temporal.io/sdk/activity"
	sdkworkflow "go.temporal.io/sdk/workflow"
	"testing"
	"time"

	"github.com/jasoet/go-wf/docker/activity"
	"github.com/jasoet/go-wf/docker/payload"
	"github.com/jasoet/go-wf/docker/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
)

// integrationMockWorker implements the Worker interface for testing
type integrationMockWorker struct{}

func (m *integrationMockWorker) RegisterWorkflow(interface{}) {}
func (m *integrationMockWorker) RegisterWorkflowWithOptions(interface{}, sdkworkflow.RegisterOptions) {
}
func (m *integrationMockWorker) RegisterDynamicWorkflow(interface{}, sdkworkflow.DynamicRegisterOptions) {
}
func (m *integrationMockWorker) RegisterActivity(interface{}) {}
func (m *integrationMockWorker) RegisterActivityWithOptions(interface{}, sdkactivity.RegisterOptions) {
}
func (m *integrationMockWorker) RegisterDynamicActivity(interface{}, sdkactivity.DynamicRegisterOptions) {
}
func (m *integrationMockWorker) RegisterNexusService(*nexus.Service) {}
func (m *integrationMockWorker) Run(<-chan interface{}) error        { return nil }
func (m *integrationMockWorker) Start() error                        { return nil }
func (m *integrationMockWorker) Stop()                               {}

func TestIntegrationContainerExecution(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Register the activity
	env.RegisterActivity(activity.StartContainerActivity)

	input := payload.ContainerExecutionInput{
		Image:      "alpine:latest",
		Command:    []string{"echo", "Hello, World!"},
		AutoRemove: true,
	}

	env.ExecuteWorkflow(workflow.ExecuteContainerWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.ContainerExecutionOutput
	err := env.GetWorkflowResult(&result)
	require.NoError(t, err)
	assert.NotEmpty(t, result.ContainerID)
	assert.Equal(t, 0, result.ExitCode)
}

func TestIntegrationPipelineWorkflow(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	env.RegisterActivity(activity.StartContainerActivity)

	input := payload.PipelineInput{
		Containers: []payload.ContainerExecutionInput{
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

	input := payload.ParallelInput{
		Containers: []payload.ContainerExecutionInput{
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

	input := payload.ContainerExecutionInput{
		Image: "nginx:alpine",
		Ports: []string{"0:80"}, // Use random port
		WaitStrategy: payload.WaitStrategyConfig{
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

	input := payload.ContainerExecutionInput{
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

	var result payload.ContainerExecutionOutput
	err := env.GetWorkflowResult(&result)
	require.NoError(t, err)
	assert.Contains(t, result.Stdout, "test_value")
}

func TestIntegrationDAGWorkflow(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	env.RegisterActivity(activity.StartContainerActivity)

	input := payload.DAGWorkflowInput{
		Nodes: []payload.DAGNode{
			{
				Name: "first",
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{
						Image:      "alpine:latest",
						Command:    []string{"echo", "First task"},
						AutoRemove: true,
					},
				},
			},
			{
				Name: "second",
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{
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
	input := payload.ContainerExecutionInput{
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
	w := &integrationMockWorker{}

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
