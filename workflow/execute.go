package workflow

import (
	"fmt"
	"time"

	wf "go.temporal.io/sdk/workflow"
)

// ExecuteTaskWorkflow runs a single task and returns results.
func ExecuteTaskWorkflow[I TaskInput, O TaskOutput](ctx wf.Context, input I) (*O, error) {
	logger := wf.GetLogger(ctx)
	logger.Info("Starting task execution workflow", "activity", input.ActivityName())

	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	ao := DefaultActivityOptions()
	ctx = wf.WithActivityOptions(ctx, ao)

	var output O
	err := wf.ExecuteActivity(ctx, input.ActivityName(), input).Get(ctx, &output)
	if err != nil {
		logger.Error("Task execution failed", "error", err)
		return nil, err
	}

	logger.Info("Task execution completed", "success", output.IsSuccess())
	return &output, nil
}

// ExecuteTaskWorkflowWithTimeout runs a single task with a custom timeout.
func ExecuteTaskWorkflowWithTimeout[I TaskInput, O TaskOutput](ctx wf.Context, input I, timeout time.Duration) (*O, error) {
	logger := wf.GetLogger(ctx)
	logger.Info("Starting task execution workflow", "activity", input.ActivityName(), "timeout", timeout)

	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	ao := DefaultActivityOptions()
	ao.StartToCloseTimeout = timeout
	ctx = wf.WithActivityOptions(ctx, ao)

	var output O
	err := wf.ExecuteActivity(ctx, input.ActivityName(), input).Get(ctx, &output)
	if err != nil {
		logger.Error("Task execution failed", "error", err)
		return nil, err
	}

	return &output, nil
}
