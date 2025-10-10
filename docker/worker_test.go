package docker

import (
	"testing"

	"github.com/nexus-rpc/sdk-go/nexus"
	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

// mockWorker is a mock implementation of worker.Worker for testing registration.
type mockWorker struct {
	mock.Mock
}

func (m *mockWorker) RegisterWorkflow(w interface{}) {
	m.Called(w)
}

func (m *mockWorker) RegisterWorkflowWithOptions(w interface{}, options workflow.RegisterOptions) {
	m.Called(w, options)
}

func (m *mockWorker) RegisterDynamicWorkflow(w interface{}, options workflow.DynamicRegisterOptions) {
	m.Called(w, options)
}

func (m *mockWorker) RegisterActivity(a interface{}) {
	m.Called(a)
}

func (m *mockWorker) RegisterActivityWithOptions(a interface{}, options activity.RegisterOptions) {
	m.Called(a, options)
}

func (m *mockWorker) RegisterDynamicActivity(a interface{}, options activity.DynamicRegisterOptions) {
	m.Called(a, options)
}

func (m *mockWorker) RegisterNexusService(service *nexus.Service) {
	m.Called(service)
}

func (m *mockWorker) Run(stopCh <-chan interface{}) error {
	args := m.Called(stopCh)
	return args.Error(0)
}

func (m *mockWorker) Start() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockWorker) Stop() {
	m.Called()
}

// TestExecuteContainerWorkflowExecution verifies the workflow executes correctly.
func TestExecuteContainerWorkflowExecution(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Mock the activity to avoid actual container execution
	// Use mock.Anything for context parameter
	env.OnActivity(StartContainerActivity, mock.Anything, mock.Anything).Return(&ContainerExecutionOutput{
		ContainerID: "test-container-id",
		Success:     true,
		ExitCode:    0,
	}, nil)

	// Execute the workflow
	env.ExecuteWorkflow(ExecuteContainerWorkflow, ContainerExecutionInput{
		Image: "alpine:latest",
	})

	if !env.IsWorkflowCompleted() {
		t.Error("Workflow did not complete")
	}

	if env.GetWorkflowError() != nil {
		t.Errorf("Workflow returned error: %v", env.GetWorkflowError())
	}
}

// TestRegisterWorkflows tests workflow registration.
func TestRegisterWorkflows(t *testing.T) {
	mw := new(mockWorker)

	// Expect RegisterWorkflow to be called 7 times (one for each workflow)
	// ExecuteContainerWorkflow, ContainerPipelineWorkflow, ParallelContainersWorkflow,
	// LoopWorkflow, ParameterizedLoopWorkflow, DAGWorkflow, WorkflowWithParameters
	mw.On("RegisterWorkflow", mock.Anything).Return().Times(7)

	RegisterWorkflows(mw)

	mw.AssertExpectations(t)
}

// TestRegisterActivities tests activity registration.
func TestRegisterActivities(t *testing.T) {
	mw := new(mockWorker)

	// Expect RegisterActivity to be called once (for StartContainerActivity)
	mw.On("RegisterActivity", mock.Anything).Return().Once()

	RegisterActivities(mw)

	mw.AssertExpectations(t)
}

// TestRegisterAll tests combined registration.
func TestRegisterAll(t *testing.T) {
	mw := new(mockWorker)

	// Expect 7 workflows + 1 activity = 8 total registrations
	mw.On("RegisterWorkflow", mock.Anything).Return().Times(7)
	mw.On("RegisterActivity", mock.Anything).Return().Once()

	RegisterAll(mw)

	mw.AssertExpectations(t)
}
