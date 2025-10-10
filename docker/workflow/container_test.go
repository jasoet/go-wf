package workflow

import (
	"fmt"
	"testing"
	"time"

	"github.com/jasoet/go-wf/docker"
	"github.com/jasoet/go-wf/docker/activity"
	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/testsuite"
)

// TestExecuteContainerWorkflow_Success tests successful container execution.
func TestExecuteContainerWorkflow_Success(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := docker.ContainerExecutionInput{
		Image:   "alpine:latest",
		Command: []string{"echo", "hello"},
	}

	env.OnActivity(activity.StartContainerActivity, mock.Anything, mock.Anything).Return(&docker.ContainerExecutionOutput{
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

	var result docker.ContainerExecutionOutput
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
	input := docker.ContainerExecutionInput{}

	env.ExecuteWorkflow(ExecuteContainerWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("Workflow did not complete")
	}

	if err := env.GetWorkflowError(); err == nil {
		t.Error("Expected validation error, got nil")
	}
}

// TestExecuteContainerWorkflow_WithTimeout tests workflow with custom timeout.
func TestExecuteContainerWorkflow_WithTimeout(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := docker.ContainerExecutionInput{
		Image:      "alpine:latest",
		RunTimeout: 5 * time.Minute,
	}

	env.OnActivity(activity.StartContainerActivity, mock.Anything, mock.Anything).Return(
		&docker.ContainerExecutionOutput{Success: true, ExitCode: 0}, nil)

	env.ExecuteWorkflow(ExecuteContainerWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("Workflow did not complete")
	}
}

// TestExecuteContainerWorkflow_ActivityError tests activity error handling.
func TestExecuteContainerWorkflow_ActivityError(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := docker.ContainerExecutionInput{
		Image: "alpine:latest",
	}

	env.OnActivity(activity.StartContainerActivity, mock.Anything, mock.Anything).Return(
		nil, fmt.Errorf("activity execution failed"))

	env.ExecuteWorkflow(ExecuteContainerWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("Workflow did not complete")
	}

	if env.GetWorkflowError() == nil {
		t.Error("Expected workflow error when activity fails")
	}
}
