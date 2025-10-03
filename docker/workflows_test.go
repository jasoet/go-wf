package docker

import (
	"fmt"
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

// TestExecuteContainerWorkflow_WithTimeout tests workflow with custom timeout.
func TestExecuteContainerWorkflow_WithTimeout(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := ContainerExecutionInput{
		Image:      "alpine:latest",
		RunTimeout: 5 * time.Minute,
	}

	env.OnActivity(StartContainerActivity, mock.Anything, mock.Anything).Return(
		&ContainerExecutionOutput{Success: true, ExitCode: 0}, nil)

	env.ExecuteWorkflow(ExecuteContainerWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("Workflow did not complete")
	}
}

// TestContainerPipelineWorkflow_InvalidInput tests pipeline validation.
func TestContainerPipelineWorkflow_InvalidInput(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := PipelineInput{
		Containers: []ContainerExecutionInput{},
	}

	env.ExecuteWorkflow(ContainerPipelineWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("Workflow did not complete")
	}

	if err := env.GetWorkflowError(); err == nil {
		t.Error("Expected validation error for empty containers")
	}
}

// TestParallelContainersWorkflow_Success tests parallel execution.
func TestParallelContainersWorkflow_Success(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := ParallelInput{
		Containers: []ContainerExecutionInput{
			{Image: "alpine:latest", Name: "task1"},
			{Image: "nginx:alpine", Name: "task2"},
		},
		FailureStrategy: "continue",
	}

	env.OnActivity(StartContainerActivity, mock.Anything, mock.Anything).Return(
		&ContainerExecutionOutput{Success: true, ExitCode: 0}, nil)

	env.ExecuteWorkflow(ParallelContainersWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("Workflow did not complete")
	}

	var result ParallelOutput
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatalf("Failed to get result: %v", err)
	}

	if result.TotalSuccess != 2 {
		t.Errorf("Expected 2 successful containers, got %d", result.TotalSuccess)
	}
}

// TestParallelContainersWorkflow_InvalidInput tests parallel validation.
func TestParallelContainersWorkflow_InvalidInput(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := ParallelInput{
		Containers: []ContainerExecutionInput{},
	}

	env.ExecuteWorkflow(ParallelContainersWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("Workflow did not complete")
	}

	if err := env.GetWorkflowError(); err == nil {
		t.Error("Expected validation error")
	}
}

// TestContainerPipelineWorkflow_WithNamedSteps tests pipeline with step names.
func TestContainerPipelineWorkflow_WithNamedSteps(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := PipelineInput{
		Containers: []ContainerExecutionInput{
			{Image: "alpine:latest", Name: "build"},
			{Image: "alpine:latest", Name: "test"},
			{Image: "alpine:latest", Name: "deploy"},
		},
		StopOnError: false,
	}

	env.OnActivity(StartContainerActivity, mock.Anything, mock.Anything).Return(
		&ContainerExecutionOutput{Success: true, ExitCode: 0, Duration: time.Second}, nil)

	env.ExecuteWorkflow(ContainerPipelineWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("Workflow did not complete")
	}

	var result PipelineOutput
	env.GetWorkflowResult(&result)

	if len(result.Results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(result.Results))
	}
}

// TestExecuteContainerWorkflow_ActivityError tests activity error handling.
func TestExecuteContainerWorkflow_ActivityError(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := ContainerExecutionInput{
		Image: "alpine:latest",
	}

	env.OnActivity(StartContainerActivity, mock.Anything, mock.Anything).Return(
		nil, fmt.Errorf("activity execution failed"))

	env.ExecuteWorkflow(ExecuteContainerWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("Workflow did not complete")
	}

	if env.GetWorkflowError() == nil {
		t.Error("Expected workflow error when activity fails")
	}
}

// TestContainerPipelineWorkflow_StopOnError tests pipeline stops on error.
func TestContainerPipelineWorkflow_StopOnError(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := PipelineInput{
		Containers: []ContainerExecutionInput{
			{Image: "alpine:latest", Name: "step1"},
			{Image: "alpine:latest", Name: "step2"},
		},
		StopOnError: true,
	}

	// First call succeeds
	env.OnActivity(StartContainerActivity, mock.Anything, input.Containers[0]).Return(
		&ContainerExecutionOutput{Success: true, ExitCode: 0}, nil).Once()

	// Second call fails
	env.OnActivity(StartContainerActivity, mock.Anything, input.Containers[1]).Return(
		&ContainerExecutionOutput{Success: false, ExitCode: 1}, fmt.Errorf("container failed")).Once()

	env.ExecuteWorkflow(ContainerPipelineWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("Workflow did not complete")
	}

	// Should error because StopOnError is true
	if env.GetWorkflowError() == nil {
		t.Error("Expected workflow error when StopOnError is true")
	}
}

// TestContainerPipelineWorkflow_ContinueOnError tests pipeline continues despite errors.
func TestContainerPipelineWorkflow_ContinueOnError(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := PipelineInput{
		Containers: []ContainerExecutionInput{
			{Image: "alpine:latest", Name: "step1"},
			{Image: "alpine:latest", Name: "step2"},
			{Image: "alpine:latest", Name: "step3"},
		},
		StopOnError: false,
	}

	// First succeeds
	env.OnActivity(StartContainerActivity, mock.Anything, input.Containers[0]).Return(
		&ContainerExecutionOutput{Success: true, ExitCode: 0}, nil).Once()

	// Second fails
	env.OnActivity(StartContainerActivity, mock.Anything, input.Containers[1]).Return(
		&ContainerExecutionOutput{Success: false, ExitCode: 1}, nil).Once()

	// Third succeeds
	env.OnActivity(StartContainerActivity, mock.Anything, input.Containers[2]).Return(
		&ContainerExecutionOutput{Success: true, ExitCode: 0}, nil).Once()

	env.ExecuteWorkflow(ContainerPipelineWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("Workflow did not complete")
	}

	if env.GetWorkflowError() != nil {
		t.Fatalf("Workflow should not error when StopOnError is false: %v", env.GetWorkflowError())
	}

	var result PipelineOutput
	env.GetWorkflowResult(&result)

	if result.TotalSuccess != 2 {
		t.Errorf("Expected 2 successful, got %d", result.TotalSuccess)
	}
	if result.TotalFailed != 1 {
		t.Errorf("Expected 1 failed, got %d", result.TotalFailed)
	}
}

// TestParallelContainersWorkflow_FailFast tests fail fast strategy.
func TestParallelContainersWorkflow_FailFast(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := ParallelInput{
		Containers: []ContainerExecutionInput{
			{Image: "alpine:latest", Name: "task1"},
			{Image: "alpine:latest", Name: "task2"},
		},
		FailureStrategy: "fail_fast",
	}

	// First fails
	env.OnActivity(StartContainerActivity, mock.Anything, input.Containers[0]).Return(
		&ContainerExecutionOutput{Success: false, ExitCode: 1}, fmt.Errorf("failed")).Once()

	// Second succeeds
	env.OnActivity(StartContainerActivity, mock.Anything, input.Containers[1]).Return(
		&ContainerExecutionOutput{Success: true, ExitCode: 0}, nil).Once()

	env.ExecuteWorkflow(ParallelContainersWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("Workflow did not complete")
	}

	if env.GetWorkflowError() == nil {
		t.Error("Expected workflow error with fail_fast strategy")
	}
}

// TestParallelContainersWorkflow_ContinueStrategy tests continue on error.
func TestParallelContainersWorkflow_ContinueStrategy(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := ParallelInput{
		Containers: []ContainerExecutionInput{
			{Image: "alpine:latest", Name: "task1"},
			{Image: "alpine:latest", Name: "task2"},
		},
		FailureStrategy: "continue",
	}

	// First fails
	env.OnActivity(StartContainerActivity, mock.Anything, input.Containers[0]).Return(
		&ContainerExecutionOutput{Success: false, ExitCode: 1}, nil).Once()

	// Second succeeds
	env.OnActivity(StartContainerActivity, mock.Anything, input.Containers[1]).Return(
		&ContainerExecutionOutput{Success: true, ExitCode: 0}, nil).Once()

	env.ExecuteWorkflow(ParallelContainersWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("Workflow did not complete")
	}

	if env.GetWorkflowError() != nil {
		t.Errorf("Workflow should not error with continue strategy: %v", env.GetWorkflowError())
	}

	var result ParallelOutput
	env.GetWorkflowResult(&result)

	if result.TotalFailed != 1 {
		t.Errorf("Expected 1 failed, got %d", result.TotalFailed)
	}
}

// TestContainerPipelineWorkflow_NoNamedSteps tests automatic step naming.
func TestContainerPipelineWorkflow_NoNamedSteps(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := PipelineInput{
		Containers: []ContainerExecutionInput{
			{Image: "alpine:latest"}, // No name
			{Image: "alpine:latest"}, // No name
		},
		StopOnError: true,
	}

	env.OnActivity(StartContainerActivity, mock.Anything, mock.Anything).Return(
		&ContainerExecutionOutput{Success: true, ExitCode: 0}, nil)

	env.ExecuteWorkflow(ContainerPipelineWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("Workflow did not complete")
	}

	var result PipelineOutput
	env.GetWorkflowResult(&result)

	if result.TotalSuccess != 2 {
		t.Errorf("Expected 2 successful, got %d", result.TotalSuccess)
	}
}
