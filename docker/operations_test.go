package docker

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/mocks"

	"github.com/jasoet/go-wf/docker/payload"
)

// mockEncodedValue implements converter.EncodedValue for testing.
type mockEncodedValue struct {
	mock.Mock
}

func (m *mockEncodedValue) HasValue() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *mockEncodedValue) Get(valuePtr interface{}) error {
	args := m.Called(valuePtr)
	return args.Error(0)
}

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
	t.Run("success", func(t *testing.T) {
		mockClient := new(mocks.Client)
		mockValue := new(mockEncodedValue)
		mockValue.On("Get", mock.Anything).Return(nil).Run(func(args mock.Arguments) {
			ptr := args.Get(0).(*string)
			*ptr = "running"
		})
		mockClient.On("QueryWorkflow", mock.Anything, "workflow-123", "run-456", "status").Return(mockValue, nil)

		var result string
		err := QueryWorkflow(context.Background(), mockClient, "workflow-123", "run-456", "status", &result)
		assert.NoError(t, err)
		assert.Equal(t, "running", result)
	})

	t.Run("query error", func(t *testing.T) {
		mockClient := new(mocks.Client)
		mockClient.On("QueryWorkflow", mock.Anything, "wf-err", "run-err", "status").Return(nil, fmt.Errorf("query failed"))

		var result string
		err := QueryWorkflow(context.Background(), mockClient, "wf-err", "run-err", "status", &result)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "query failed")
	})
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

func TestSubmitWorkflow_PointerTypes(t *testing.T) {
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
			name:  "pointer ContainerExecutionInput",
			input: &payload.ContainerExecutionInput{Image: "alpine:latest"},
		},
		{
			name: "pointer PipelineInput",
			input: &payload.PipelineInput{
				Containers: []payload.ContainerExecutionInput{{Image: "alpine:latest"}},
			},
		},
		{
			name: "pointer ParallelInput",
			input: &payload.ParallelInput{
				Containers: []payload.ContainerExecutionInput{{Image: "alpine:latest"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, err := SubmitWorkflow(context.Background(), mockClient, tt.input, "docker-queue")
			assert.NoError(t, err)
			assert.NotNil(t, status)
			assert.Equal(t, "Running", status.Status)
		})
	}
}

func TestSubmitWorkflow_UnsupportedType(t *testing.T) {
	mockClient := new(mocks.Client)

	status, err := SubmitWorkflow(context.Background(), mockClient, "invalid-input", "docker-queue")
	assert.Error(t, err)
	assert.Nil(t, status)
	assert.Contains(t, err.Error(), "unsupported workflow input type")
}

func TestSubmitWorkflow_ExecuteError(t *testing.T) {
	mockClient := new(mocks.Client)

	mockClient.On("ExecuteWorkflow",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
	).Return(nil, fmt.Errorf("temporal unavailable"))

	input := payload.ContainerExecutionInput{Image: "alpine:latest"}
	status, err := SubmitWorkflow(context.Background(), mockClient, input, "docker-queue")
	assert.Error(t, err)
	assert.Nil(t, status)
	assert.Contains(t, err.Error(), "failed to start workflow")
}

func TestSubmitAndWait_SubmitError(t *testing.T) {
	mockClient := new(mocks.Client)

	status, err := SubmitAndWait(context.Background(), mockClient, "invalid", "queue", 1*time.Minute)
	assert.Error(t, err)
	assert.Nil(t, status)
}

func TestSubmitAndWait_GetError(t *testing.T) {
	mockClient := new(mocks.Client)
	mockWorkflowRun := new(mocks.WorkflowRun)

	mockWorkflowRun.On("GetID").Return("workflow-123")
	mockWorkflowRun.On("GetRunID").Return("run-456")
	mockWorkflowRun.On("Get", mock.Anything, mock.Anything).Return(fmt.Errorf("workflow failed"))

	mockClient.On("ExecuteWorkflow",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
	).Return(mockWorkflowRun, nil)
	mockClient.On("GetWorkflow", mock.Anything, "workflow-123", "run-456").Return(mockWorkflowRun)

	input := payload.ContainerExecutionInput{Image: "alpine:latest"}
	status, err := SubmitAndWait(context.Background(), mockClient, input, "queue", 1*time.Minute)
	assert.Error(t, err)
	assert.NotNil(t, status)
	assert.Equal(t, "Failed", status.Status)
	assert.NotNil(t, status.CloseTime)
}

func TestGetWorkflowStatus_Error(t *testing.T) {
	mockClient := new(mocks.Client)
	mockWorkflowRun := new(mocks.WorkflowRun)

	mockWorkflowRun.On("Get", mock.Anything, mock.Anything).Return(fmt.Errorf("workflow error"))
	mockClient.On("GetWorkflow", mock.Anything, "workflow-123", "run-456").Return(mockWorkflowRun)

	status, err := GetWorkflowStatus(context.Background(), mockClient, "workflow-123", "run-456")
	assert.NoError(t, err)
	assert.NotNil(t, status)
	assert.Equal(t, "Failed or Running", status.Status)
	assert.NotNil(t, status.Error)
}

func TestWatchWorkflow_ContextCancellation(t *testing.T) {
	mockClient := new(mocks.Client)
	mockWorkflowRun := new(mocks.WorkflowRun)

	mockWorkflowRun.On("Get", mock.Anything, mock.Anything).Return(fmt.Errorf("workflow still running"))
	mockClient.On("GetWorkflow", mock.Anything, "workflow-123", "run-456").Return(mockWorkflowRun)

	ctx, cancel := context.WithCancel(context.Background())
	updates := make(chan *WorkflowStatus, 10)

	cancel()

	err := WatchWorkflow(ctx, mockClient, "workflow-123", "run-456", updates)
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
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
