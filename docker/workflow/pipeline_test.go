package workflow

import (
	"fmt"
	"testing"
	"time"

	"github.com/jasoet/go-wf/docker/activity"
	"github.com/jasoet/go-wf/docker/payload"
	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/testsuite"
)

// TestContainerPipelineWorkflow_Success tests successful pipeline execution.
func TestContainerPipelineWorkflow_Success(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := payload.PipelineInput{
		Containers: []payload.ContainerExecutionInput{
			{Image: "alpine:latest", Command: []string{"echo", "step1"}},
			{Image: "alpine:latest", Command: []string{"echo", "step2"}},
		},
		StopOnError: true,
	}

	env.OnActivity(activity.StartContainerActivity, mock.Anything, mock.Anything).Return(
		&payload.ContainerExecutionOutput{Success: true, ExitCode: 0}, nil)

	env.ExecuteWorkflow(ContainerPipelineWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("Workflow did not complete")
	}

	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("Workflow failed: %v", err)
	}

	var result payload.PipelineOutput
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatalf("Failed to get result: %v", err)
	}

	if result.TotalSuccess != 2 {
		t.Errorf("Expected 2 successful containers, got %d", result.TotalSuccess)
	}
}

// TestContainerPipelineWorkflow_InvalidInput tests pipeline validation.
func TestContainerPipelineWorkflow_InvalidInput(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := payload.PipelineInput{
		Containers: []payload.ContainerExecutionInput{},
	}

	env.ExecuteWorkflow(ContainerPipelineWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("Workflow did not complete")
	}

	if err := env.GetWorkflowError(); err == nil {
		t.Error("Expected validation error for empty containers")
	}
}

// TestContainerPipelineWorkflow_WithNamedSteps tests pipeline with step names.
func TestContainerPipelineWorkflow_WithNamedSteps(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := payload.PipelineInput{
		Containers: []payload.ContainerExecutionInput{
			{Image: "alpine:latest", Name: "build"},
			{Image: "alpine:latest", Name: "test"},
			{Image: "alpine:latest", Name: "deploy"},
		},
		StopOnError: false,
	}

	env.OnActivity(activity.StartContainerActivity, mock.Anything, mock.Anything).Return(
		&payload.ContainerExecutionOutput{Success: true, ExitCode: 0, Duration: time.Second}, nil)

	env.ExecuteWorkflow(ContainerPipelineWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("Workflow did not complete")
	}

	var result payload.PipelineOutput
	env.GetWorkflowResult(&result)

	if len(result.Results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(result.Results))
	}
}

// TestContainerPipelineWorkflow_StopOnError tests pipeline stops on error.
func TestContainerPipelineWorkflow_StopOnError(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := payload.PipelineInput{
		Containers: []payload.ContainerExecutionInput{
			{Image: "alpine:latest", Name: "step1"},
			{Image: "alpine:latest", Name: "step2"},
		},
		StopOnError: true,
	}

	// First call succeeds
	env.OnActivity(activity.StartContainerActivity, mock.Anything, input.Containers[0]).Return(
		&payload.ContainerExecutionOutput{Success: true, ExitCode: 0}, nil).Once()

	// Second call fails
	env.OnActivity(activity.StartContainerActivity, mock.Anything, input.Containers[1]).Return(
		&payload.ContainerExecutionOutput{Success: false, ExitCode: 1}, fmt.Errorf("container failed")).Once()

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

	input := payload.PipelineInput{
		Containers: []payload.ContainerExecutionInput{
			{Image: "alpine:latest", Name: "step1"},
			{Image: "alpine:latest", Name: "step2"},
			{Image: "alpine:latest", Name: "step3"},
		},
		StopOnError: false,
	}

	// First succeeds
	env.OnActivity(activity.StartContainerActivity, mock.Anything, input.Containers[0]).Return(
		&payload.ContainerExecutionOutput{Success: true, ExitCode: 0}, nil).Once()

	// Second fails
	env.OnActivity(activity.StartContainerActivity, mock.Anything, input.Containers[1]).Return(
		&payload.ContainerExecutionOutput{Success: false, ExitCode: 1}, nil).Once()

	// Third succeeds
	env.OnActivity(activity.StartContainerActivity, mock.Anything, input.Containers[2]).Return(
		&payload.ContainerExecutionOutput{Success: true, ExitCode: 0}, nil).Once()

	env.ExecuteWorkflow(ContainerPipelineWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("Workflow did not complete")
	}

	if env.GetWorkflowError() != nil {
		t.Fatalf("Workflow should not error when StopOnError is false: %v", env.GetWorkflowError())
	}

	var result payload.PipelineOutput
	env.GetWorkflowResult(&result)

	if result.TotalSuccess != 2 {
		t.Errorf("Expected 2 successful, got %d", result.TotalSuccess)
	}
	if result.TotalFailed != 1 {
		t.Errorf("Expected 1 failed, got %d", result.TotalFailed)
	}
}

// TestContainerPipelineWorkflow_NoNamedSteps tests automatic step naming.
func TestContainerPipelineWorkflow_NoNamedSteps(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := payload.PipelineInput{
		Containers: []payload.ContainerExecutionInput{
			{Image: "alpine:latest"}, // No name
			{Image: "alpine:latest"}, // No name
		},
		StopOnError: true,
	}

	env.OnActivity(activity.StartContainerActivity, mock.Anything, mock.Anything).Return(
		&payload.ContainerExecutionOutput{Success: true, ExitCode: 0}, nil)

	env.ExecuteWorkflow(ContainerPipelineWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("Workflow did not complete")
	}

	var result payload.PipelineOutput
	env.GetWorkflowResult(&result)

	if result.TotalSuccess != 2 {
		t.Errorf("Expected 2 successful, got %d", result.TotalSuccess)
	}
}
