package docker

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/testsuite"
)

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

// TestRegisterAllWithWorker tests worker registration.
// This is a placeholder for integration tests.
func TestRegisterAllWithWorker(t *testing.T) {
	// This would require a real Temporal server connection
	// Skip for unit tests, will be covered in integration tests
	t.Skip("Requires real Temporal worker, tested in integration tests")
}
