package workflow

import (
	"testing"

	"github.com/jasoet/go-wf/docker/activity"
	"github.com/jasoet/go-wf/docker/payload"
	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/testsuite"
)

// TestParallelContainersWorkflow_Success tests parallel execution.
func TestParallelContainersWorkflow_Success(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := payload.ParallelInput{
		Containers: []payload.ContainerExecutionInput{
			{Image: "alpine:latest", Name: "task1"},
			{Image: "nginx:alpine", Name: "task2"},
		},
		FailureStrategy: "continue",
	}

	env.OnActivity(activity.StartContainerActivity, mock.Anything, mock.Anything).Return(
		&payload.ContainerExecutionOutput{Success: true, ExitCode: 0}, nil)

	env.ExecuteWorkflow(ParallelContainersWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("Workflow did not complete")
	}

	var result payload.ParallelOutput
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

	input := payload.ParallelInput{
		Containers: []payload.ContainerExecutionInput{},
	}

	env.ExecuteWorkflow(ParallelContainersWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("Workflow did not complete")
	}

	if err := env.GetWorkflowError(); err == nil {
		t.Error("Expected validation error")
	}
}

// TestParallelContainersWorkflow_FailFast tests fail fast strategy.
func TestParallelContainersWorkflow_FailFast(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := payload.ParallelInput{
		Containers: []payload.ContainerExecutionInput{
			{Image: "alpine:latest", Name: "task1"},
			{Image: "alpine:latest", Name: "task2"},
		},
		FailureStrategy: "fail_fast",
	}

	// First fails
	env.OnActivity(activity.StartContainerActivity, mock.Anything, input.Containers[0]).Return(
		&payload.ContainerExecutionOutput{Success: false, ExitCode: 1}, nil).Once()

	// Second succeeds
	env.OnActivity(activity.StartContainerActivity, mock.Anything, input.Containers[1]).Return(
		&payload.ContainerExecutionOutput{Success: true, ExitCode: 0}, nil).Once()

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

	input := payload.ParallelInput{
		Containers: []payload.ContainerExecutionInput{
			{Image: "alpine:latest", Name: "task1"},
			{Image: "alpine:latest", Name: "task2"},
		},
		FailureStrategy: "continue",
	}

	// First fails
	env.OnActivity(activity.StartContainerActivity, mock.Anything, input.Containers[0]).Return(
		&payload.ContainerExecutionOutput{Success: false, ExitCode: 1}, nil).Once()

	// Second succeeds
	env.OnActivity(activity.StartContainerActivity, mock.Anything, input.Containers[1]).Return(
		&payload.ContainerExecutionOutput{Success: true, ExitCode: 0}, nil).Once()

	env.ExecuteWorkflow(ParallelContainersWorkflow, input)

	if !env.IsWorkflowCompleted() {
		t.Fatal("Workflow did not complete")
	}

	if env.GetWorkflowError() != nil {
		t.Errorf("Workflow should not error with continue strategy: %v", env.GetWorkflowError())
	}

	var result payload.ParallelOutput
	env.GetWorkflowResult(&result)

	if result.TotalFailed != 1 {
		t.Errorf("Expected 1 failed, got %d", result.TotalFailed)
	}
}
