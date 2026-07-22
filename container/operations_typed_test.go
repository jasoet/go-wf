package container

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/mocks"

	"github.com/jasoet/go-wf/container/payload"
)

func TestSubmitTypedWorkflow(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockClient := new(mocks.Client)
		mockRun := new(mocks.WorkflowRun)

		mockRun.On("GetID").Return("workflow-typed-1")
		mockRun.On("GetRunID").Return("run-typed-1")

		mockClient.On("ExecuteWorkflow",
			mock.Anything,
			mock.Anything,
			mock.Anything,
			mock.Anything,
		).Return(mockRun, nil)

		input := payload.ContainerExecutionInput{Image: "alpine:latest"}

		status, err := SubmitTypedWorkflow(context.Background(), mockClient, "SomeWorkflow", input, "container-queue")
		assert.NoError(t, err)
		assert.NotNil(t, status)
		assert.Equal(t, "workflow-typed-1", status.WorkflowID)
		assert.Equal(t, "run-typed-1", status.RunID)
		assert.Equal(t, "Running", status.Status)

		mockClient.AssertExpectations(t)
	})

	t.Run("start failure", func(t *testing.T) {
		mockClient := new(mocks.Client)
		startErr := errors.New("temporal unavailable")

		mockClient.On("ExecuteWorkflow",
			mock.Anything,
			mock.Anything,
			mock.Anything,
			mock.Anything,
		).Return(nil, startErr)

		status, err := SubmitTypedWorkflow(context.Background(), mockClient, "SomeWorkflow", "input", "container-queue")
		assert.Nil(t, status)
		assert.ErrorIs(t, err, startErr)
		assert.Contains(t, err.Error(), "failed to start workflow")

		mockClient.AssertExpectations(t)
	})
}

func TestSubmitAndWaitTyped(t *testing.T) {
	t.Run("completed", func(t *testing.T) {
		mockClient := new(mocks.Client)
		mockRun := new(mocks.WorkflowRun)

		mockRun.On("GetID").Return("workflow-typed-2")
		mockRun.On("GetRunID").Return("run-typed-2")
		mockRun.On("Get", mock.Anything, mock.Anything).Return(nil)

		mockClient.On("ExecuteWorkflow",
			mock.Anything,
			mock.Anything,
			mock.Anything,
			mock.Anything,
		).Return(mockRun, nil)
		mockClient.On("GetWorkflow", mock.Anything, "workflow-typed-2", "run-typed-2").Return(mockRun)

		status, result, err := SubmitAndWaitTyped[payload.ContainerExecutionOutput](
			context.Background(), mockClient, "SomeWorkflow",
			payload.ContainerExecutionInput{Image: "alpine:latest"}, "container-queue", time.Minute)

		assert.NoError(t, err)
		assert.NotNil(t, status)
		assert.Equal(t, "Completed", status.Status)
		assert.NotNil(t, status.CloseTime)
		assert.NotNil(t, result)

		mockClient.AssertExpectations(t)
	})

	t.Run("start failure propagates", func(t *testing.T) {
		mockClient := new(mocks.Client)
		startErr := errors.New("temporal unavailable")

		mockClient.On("ExecuteWorkflow",
			mock.Anything,
			mock.Anything,
			mock.Anything,
			mock.Anything,
		).Return(nil, startErr)

		status, result, err := SubmitAndWaitTyped[payload.ContainerExecutionOutput](
			context.Background(), mockClient, "SomeWorkflow", "input", "container-queue", time.Minute)

		assert.Nil(t, status)
		assert.Nil(t, result)
		assert.ErrorIs(t, err, startErr)
	})

	t.Run("workflow result error marks status failed", func(t *testing.T) {
		mockClient := new(mocks.Client)
		mockRun := new(mocks.WorkflowRun)
		runErr := errors.New("activity failed")

		mockRun.On("GetID").Return("workflow-typed-3")
		mockRun.On("GetRunID").Return("run-typed-3")
		mockRun.On("Get", mock.Anything, mock.Anything).Return(runErr)

		mockClient.On("ExecuteWorkflow",
			mock.Anything,
			mock.Anything,
			mock.Anything,
			mock.Anything,
		).Return(mockRun, nil)
		mockClient.On("GetWorkflow", mock.Anything, "workflow-typed-3", "run-typed-3").Return(mockRun)

		status, result, err := SubmitAndWaitTyped[payload.ContainerExecutionOutput](
			context.Background(), mockClient, "SomeWorkflow", "input", "container-queue", time.Minute)

		assert.ErrorIs(t, err, runErr)
		assert.Nil(t, result)
		assert.NotNil(t, status)
		assert.Equal(t, "Failed", status.Status)
		assert.ErrorIs(t, status.Error, runErr)
		assert.NotNil(t, status.CloseTime)
	})
}
