package function

import (
	"context"
	"testing"

	"github.com/nexus-rpc/sdk-go/nexus"
	"github.com/stretchr/testify/mock"
	sdkactivity "go.temporal.io/sdk/activity"
	sdkworkflow "go.temporal.io/sdk/workflow"

	"github.com/jasoet/go-wf/function/payload"
)

// mockWorker is a mock implementation of worker.Worker for testing registration.
type mockWorker struct {
	mock.Mock
}

func (m *mockWorker) RegisterWorkflow(w interface{}) {
	m.Called(w)
}

func (m *mockWorker) RegisterWorkflowWithOptions(w interface{}, options sdkworkflow.RegisterOptions) {
	m.Called(w, options)
}

func (m *mockWorker) RegisterDynamicWorkflow(w interface{}, options sdkworkflow.DynamicRegisterOptions) {
	m.Called(w, options)
}

func (m *mockWorker) RegisterActivity(a interface{}) {
	m.Called(a)
}

func (m *mockWorker) RegisterActivityWithOptions(a interface{}, options sdkactivity.RegisterOptions) {
	m.Called(a, options)
}

func (m *mockWorker) RegisterDynamicActivity(a interface{}, options sdkactivity.DynamicRegisterOptions) {
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

// TestRegisterWorkflows tests workflow registration.
func TestRegisterWorkflows(t *testing.T) {
	mw := new(mockWorker)

	// Expect RegisterWorkflowWithOptions to be called 6 times (one for each workflow).
	// RegisterWorkflows routes through job.RegisterWorkflowOnce which calls
	// RegisterWorkflowWithOptions with an explicit name.
	mw.On("RegisterWorkflowWithOptions", mock.Anything, mock.AnythingOfType("internal.RegisterWorkflowOptions")).Return().Times(6)

	RegisterWorkflows(mw)

	mw.AssertExpectations(t)
}

// TestRegisterActivity tests activity registration.
func TestRegisterActivity(t *testing.T) {
	mw := new(mockWorker)

	// Use a stub activity function since importing function/activity would create a cycle.
	stubActivity := func(_ context.Context, _ payload.FunctionExecutionInput) (*payload.FunctionExecutionOutput, error) {
		return nil, nil
	}

	// Expect RegisterActivityWithOptions to be called once (for ExecuteFunctionActivity).
	mw.On("RegisterActivityWithOptions", mock.Anything, sdkactivity.RegisterOptions{
		Name: "ExecuteFunctionActivity",
	}).Return().Once()

	RegisterActivity(mw, stubActivity)

	mw.AssertExpectations(t)
}

// TestRegisterAll tests combined registration.
func TestRegisterAll(t *testing.T) {
	mw := new(mockWorker)

	stubActivity := func(_ context.Context, _ payload.FunctionExecutionInput) (*payload.FunctionExecutionOutput, error) {
		return nil, nil
	}

	// Expect 6 workflows + 1 activity = 7 total registrations.
	// Workflows use RegisterWorkflowWithOptions via job.RegisterWorkflowOnce.
	mw.On("RegisterWorkflowWithOptions", mock.Anything, mock.AnythingOfType("internal.RegisterWorkflowOptions")).Return().Times(6)
	mw.On("RegisterActivityWithOptions", mock.Anything, sdkactivity.RegisterOptions{
		Name: "ExecuteFunctionActivity",
	}).Return().Once()

	RegisterAll(mw, stubActivity)

	mw.AssertExpectations(t)
}

// TestRegisterAll_Idempotent verifies that calling RegisterAll twice on the same
// worker is a no-op for the second call and does not panic or duplicate registrations.
func TestRegisterAll_Idempotent(t *testing.T) {
	mw := new(mockWorker)

	stubActivity := func(_ context.Context, _ payload.FunctionExecutionInput) (*payload.FunctionExecutionOutput, error) {
		return nil, nil
	}

	// First call: all 6 workflows + 1 activity should be registered.
	mw.On("RegisterWorkflowWithOptions", mock.Anything, mock.AnythingOfType("internal.RegisterWorkflowOptions")).Return().Times(6)
	mw.On("RegisterActivityWithOptions", mock.Anything, sdkactivity.RegisterOptions{
		Name: "ExecuteFunctionActivity",
	}).Return().Once()

	// First call registers everything.
	RegisterAll(mw, stubActivity)

	// Second call must be a no-op (job.RegisterWorkflowOnce / RegisterActivityOnce
	// deduplicate on (worker, typeName) — no new calls expected on the mock).
	RegisterAll(mw, stubActivity)

	mw.AssertExpectations(t)
}
