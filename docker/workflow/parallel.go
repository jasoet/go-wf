package workflow

import (
	"fmt"
	"time"

	"github.com/jasoet/go-wf/docker/activity"
	"github.com/jasoet/go-wf/docker/payload"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// ParallelContainersWorkflow executes multiple containers in parallel.
func ParallelContainersWorkflow(ctx workflow.Context, input payload.ParallelInput) (*payload.ParallelOutput, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting parallel containers workflow",
		"containers", len(input.Containers),
		"maxConcurrency", input.MaxConcurrency)

	// Validate input
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	startTime := workflow.Now(ctx)

	// Default activity options
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Execute containers in parallel
	futures := make([]workflow.Future, len(input.Containers))

	for i, containerInput := range input.Containers {
		// Execute each container as an activity
		futures[i] = workflow.ExecuteActivity(ctx, activity.StartContainerActivity, containerInput)
	}

	// Collect results
	output := &payload.ParallelOutput{
		Results: make([]payload.ContainerExecutionOutput, 0, len(input.Containers)),
	}

	for i, future := range futures {
		var result payload.ContainerExecutionOutput
		err := future.Get(ctx, &result)

		output.Results = append(output.Results, result)

		if err != nil || !result.Success {
			output.TotalFailed++

			if input.FailureStrategy == "fail_fast" {
				output.TotalDuration = workflow.Now(ctx).Sub(startTime)
				return output, fmt.Errorf("parallel execution failed at container %d: %w", i, err)
			}
		} else {
			output.TotalSuccess++
		}
	}

	output.TotalDuration = workflow.Now(ctx).Sub(startTime)

	logger.Info("Parallel workflow completed",
		"success", output.TotalSuccess,
		"failed", output.TotalFailed,
		"totalDuration", output.TotalDuration)

	return output, nil
}
