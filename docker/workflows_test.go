package docker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/testsuite"
)

// TestExecuteContainerWorkflow_Success tests successful container execution.
func TestExecuteContainerWorkflow_Success(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := ContainerExecutionInput{
		Image:   "alpine:latest",
		Command: []string{"echo", "hello"},
	}

	env.OnActivity(StartContainerActivity, mock.Anything, mock.Anything).Return(&ContainerExecutionOutput{
		ContainerID: "container-123",
		Success:     true,
		ExitCode:    0,
		Duration:    5 * time.Second,
	}, nil)

	env.ExecuteWorkflow(ExecuteContainerWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("Workflow did not complete")
	}

	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("Workflow failed: %v", err)
	}

	var result ContainerExecutionOutput
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatalf("Failed to get result: %v", err)
	}

	if result.ContainerID != "container-123" {
		t.Errorf("Expected container ID container-123, got %s", result.ContainerID)
	}
}

// TestExecuteContainerWorkflow_InvalidInput tests validation failure.
func TestExecuteContainerWorkflow_InvalidInput(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Missing required image field
	input := ContainerExecutionInput{}

	env.ExecuteWorkflow(ExecuteContainerWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("Workflow did not complete")
	}

	if err := env.GetWorkflowError(); err == nil {
		t.Error("Expected validation error, got nil")
	}
}

// TestContainerPipelineWorkflow_Success tests successful pipeline execution.
func TestContainerPipelineWorkflow_Success(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := PipelineInput{
		Containers: []ContainerExecutionInput{
			{Image: "alpine:latest", Command: []string{"echo", "step1"}},
			{Image: "alpine:latest", Command: []string{"echo", "step2"}},
		},
		StopOnError: true,
	}

	env.OnActivity(StartContainerActivity, mock.Anything, mock.Anything).Return(
		&ContainerExecutionOutput{Success: true, ExitCode: 0}, nil)

	env.ExecuteWorkflow(ContainerPipelineWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("Workflow did not complete")
	}

	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("Workflow failed: %v", err)
	}

	var result PipelineOutput
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatalf("Failed to get result: %v", err)
	}

	if result.TotalSuccess != 2 {
		t.Errorf("Expected 2 successful containers, got %d", result.TotalSuccess)
	}
}
