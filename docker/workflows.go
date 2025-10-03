package docker

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// ExecuteContainerWorkflow runs a single container and returns results.
func ExecuteContainerWorkflow(ctx workflow.Context, input ContainerExecutionInput) (*ContainerExecutionOutput, error) {
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
	var output ContainerExecutionOutput
	err := workflow.ExecuteActivity(ctx, StartContainerActivity, input).Get(ctx, &output)
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

// ContainerPipelineWorkflow executes containers sequentially.
func ContainerPipelineWorkflow(ctx workflow.Context, input PipelineInput) (*PipelineOutput, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting container pipeline workflow", "steps", len(input.Containers))

	// Validate input
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	startTime := workflow.Now(ctx)
	output := &PipelineOutput{
		Results: make([]ContainerExecutionOutput, 0, len(input.Containers)),
	}

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

	for i, containerInput := range input.Containers {
		stepName := containerInput.Name
		if stepName == "" {
			stepName = fmt.Sprintf("step-%d", i+1)
		}

		logger.Info("Executing pipeline step",
			"step", i+1,
			"name", stepName,
			"image", containerInput.Image)

		// Execute step
		var result ContainerExecutionOutput
		err := workflow.ExecuteActivity(ctx, StartContainerActivity, containerInput).Get(ctx, &result)

		output.Results = append(output.Results, result)

		if err != nil || !result.Success {
			output.TotalFailed++
			logger.Error("Pipeline step failed",
				"step", i+1,
				"name", stepName,
				"error", err)

			if input.StopOnError {
				output.TotalDuration = workflow.Now(ctx).Sub(startTime)
				return output, fmt.Errorf("pipeline stopped at step %d: %w", i+1, err)
			}
			continue
		}

		output.TotalSuccess++
		logger.Info("Pipeline step completed",
			"step", i+1,
			"name", stepName,
			"duration", result.Duration)
	}

	output.TotalDuration = workflow.Now(ctx).Sub(startTime)

	logger.Info("Pipeline workflow completed",
		"success", output.TotalSuccess,
		"failed", output.TotalFailed,
		"totalDuration", output.TotalDuration)

	return output, nil
}

// ParallelContainersWorkflow executes multiple containers in parallel.
func ParallelContainersWorkflow(ctx workflow.Context, input ParallelInput) (*ParallelOutput, error) {
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
		futures[i] = workflow.ExecuteActivity(ctx, StartContainerActivity, containerInput)
	}

	// Collect results
	output := &ParallelOutput{
		Results: make([]ContainerExecutionOutput, 0, len(input.Containers)),
	}

	for i, future := range futures {
		var result ContainerExecutionOutput
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
