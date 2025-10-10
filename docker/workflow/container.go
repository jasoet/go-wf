package workflow

import (
	"fmt"
	"time"

	"github.com/jasoet/go-wf/docker"
	"github.com/jasoet/go-wf/docker/activity"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// ExecuteContainerWorkflow runs a single container and returns results.
func ExecuteContainerWorkflow(ctx workflow.Context, input docker.ContainerExecutionInput) (*docker.ContainerExecutionOutput, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting container execution workflow",
		"image", input.Image,
		"name", input.Name)

	// Validate input
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	// Set default timeout if not specified
	timeout := input.RunTimeout
	if timeout == 0 {
		timeout = 10 * time.Minute
	}

	// Activity options with retry policy
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: timeout,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Execute container activity
	var output docker.ContainerExecutionOutput
	err := workflow.ExecuteActivity(ctx, activity.StartContainerActivity, input).Get(ctx, &output)
	if err != nil {
		logger.Error("Container execution failed", "error", err)
		return nil, err
	}

	logger.Info("Container execution completed",
		"success", output.Success,
		"exitCode", output.ExitCode,
		"duration", output.Duration)

	return &output, nil
}
