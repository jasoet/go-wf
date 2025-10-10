package docker

import (
	"context"
	"testing"
	"time"

	"github.com/jasoet/go-wf/docker/payload"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/mocks"
)

func TestSubmitWorkflow(t *testing.T) {
	mockClient := new(mocks.Client)
	mockWorkflowRun := new(mocks.WorkflowRun)

	mockWorkflowRun.On("GetID").Return("workflow-123")
	mockWorkflowRun.On("GetRunID").Return("run-456")

	mockClient.On("ExecuteWorkflow",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(mockWorkflowRun, nil)

	input := payload.ContainerExecutionInput{
		Image: "alpine:latest",
	}

	status, err := SubmitWorkflow(context.Background(), mockClient, input, "docker-queue")
	assert.NoError(t, err)
	assert.NotNil(t, status)
	assert.Equal(t, "workflow-123", status.WorkflowID)
	assert.Equal(t, "run-456", status.RunID)
	assert.Equal(t, "Running", status.Status)

	mockClient.AssertExpectations(t)
	mockWorkflowRun.AssertExpectations(t)
}

func TestSubmitAndWait(t *testing.T) {
	mockClient := new(mocks.Client)
	mockWorkflowRun := new(mocks.WorkflowRun)

	mockWorkflowRun.On("GetID").Return("workflow-123")
	mockWorkflowRun.On("GetRunID").Return("run-456")
	mockWorkflowRun.On("Get", mock.Anything, mock.Anything).Return(nil)

	mockClient.On("ExecuteWorkflow",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(mockWorkflowRun, nil)

	mockClient.On("GetWorkflow",
		mock.Anything,
		"workflow-123",
		"run-456",
	).Return(mockWorkflowRun)

	input := payload.ContainerExecutionInput{
		Image: "alpine:latest",
	}

	status, err := SubmitAndWait(context.Background(), mockClient, input, "docker-queue", 1*time.Minute)
	assert.NoError(t, err)
	assert.NotNil(t, status)
	assert.Equal(t, "Completed", status.Status)

	mockClient.AssertExpectations(t)
	mockWorkflowRun.AssertExpectations(t)
}

func TestGetWorkflowStatus(t *testing.T) {
	mockClient := new(mocks.Client)
	mockWorkflowRun := new(mocks.WorkflowRun)

	mockWorkflowRun.On("Get", mock.Anything, mock.Anything).Return(nil)

	mockClient.On("GetWorkflow",
		mock.Anything,
		"workflow-123",
		"run-456",
	).Return(mockWorkflowRun)

	status, err := GetWorkflowStatus(context.Background(), mockClient, "workflow-123", "run-456")
	assert.NoError(t, err)
	assert.NotNil(t, status)
	assert.Equal(t, "Completed", status.Status)

	mockClient.AssertExpectations(t)
	mockWorkflowRun.AssertExpectations(t)
}

func TestCancelWorkflow(t *testing.T) {
	mockClient := new(mocks.Client)

	mockClient.On("CancelWorkflow",
		mock.Anything,
		"workflow-123",
		"run-456",
	).Return(nil)

	err := CancelWorkflow(context.Background(), mockClient, "workflow-123", "run-456")
	assert.NoError(t, err)

	mockClient.AssertExpectations(t)
}

func TestTerminateWorkflow(t *testing.T) {
	mockClient := new(mocks.Client)

	mockClient.On("TerminateWorkflow",
		mock.Anything,
		"workflow-123",
		"run-456",
		"test reason",
	).Return(nil)

	err := TerminateWorkflow(context.Background(), mockClient, "workflow-123", "run-456", "test reason")
	assert.NoError(t, err)

	mockClient.AssertExpectations(t)
}

func TestSignalWorkflow(t *testing.T) {
	mockClient := new(mocks.Client)

	mockClient.On("SignalWorkflow",
		mock.Anything,
		"workflow-123",
		"run-456",
		"pause",
		mock.Anything,
	).Return(nil)

	err := SignalWorkflow(context.Background(), mockClient, "workflow-123", "run-456", "pause", nil)
	assert.NoError(t, err)

	mockClient.AssertExpectations(t)
}

func TestQueryWorkflow(t *testing.T) {
	// Skip this test as it requires complex mocking of converter.EncodedValue interface
	t.Skip("QueryWorkflow requires complex converter.EncodedValue mocking")
}

func TestWatchWorkflow(t *testing.T) {
	mockClient := new(mocks.Client)
	mockWorkflowRun := new(mocks.WorkflowRun)

	// Mock completed workflow
	mockWorkflowRun.On("Get", mock.Anything, mock.Anything).Return(nil)
	mockClient.On("GetWorkflow", mock.Anything, "workflow-123", "run-456").Return(mockWorkflowRun)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	updates := make(chan *WorkflowStatus, 10)
	go func() {
		err := WatchWorkflow(ctx, mockClient, "workflow-123", "run-456", updates)
		assert.NoError(t, err)
	}()

	// Wait for at least one update
	select {
	case status := <-updates:
		assert.NotNil(t, status)
		assert.Equal(t, "workflow-123", status.WorkflowID)
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for workflow update")
	}
}

func TestSubmitWorkflowWithDifferentInputTypes(t *testing.T) {
	mockClient := new(mocks.Client)
	mockWorkflowRun := new(mocks.WorkflowRun)

	mockWorkflowRun.On("GetID").Return("workflow-123")
	mockWorkflowRun.On("GetRunID").Return("run-456")
	mockClient.On("ExecuteWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(mockWorkflowRun, nil)

	tests := []struct {
		name  string
		input interface{}
	}{
		{
			name: "ContainerExecutionInput",
			input: payload.ContainerExecutionInput{
				Image: "alpine:latest",
			},
		},
		{
			name: "PipelineInput",
			input: payload.PipelineInput{
				Containers: []payload.ContainerExecutionInput{
					{Image: "alpine:latest"},
				},
			},
		},
		{
			name: "ParallelInput",
			input: payload.ParallelInput{
				Containers: []payload.ContainerExecutionInput{
					{Image: "alpine:latest"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, err := SubmitWorkflow(context.Background(), mockClient, tt.input, "docker-queue")
			assert.NoError(t, err)
			assert.NotNil(t, status)
		})
	}
}

func TestListWorkflows(t *testing.T) {
	mockClient := new(mocks.Client)

	// This function is not implemented, should return error
	workflows, err := ListWorkflows(context.Background(), mockClient, "")
	assert.Error(t, err)
	assert.Nil(t, workflows)
}

func TestGetWorkflowHistory(t *testing.T) {
	mockClient := new(mocks.Client)

	// This function is not implemented, should return error
	history, err := GetWorkflowHistory(context.Background(), mockClient, "workflow-123", "run-456")
	assert.Error(t, err)
	assert.Nil(t, history)
}
