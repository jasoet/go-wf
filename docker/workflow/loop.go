package workflow

import (
	"fmt"
	"time"

	"github.com/jasoet/go-wf/docker"
	"github.com/jasoet/go-wf/docker/activity"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// LoopWorkflow executes containers in a loop over items (withItems pattern).
func LoopWorkflow(ctx workflow.Context, input docker.LoopInput) (*docker.LoopOutput, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting loop workflow",
		"items", len(input.Items),
		"parallel", input.Parallel,
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

	output := &docker.LoopOutput{
		Results:   make([]docker.ContainerExecutionOutput, 0, len(input.Items)),
		ItemCount: len(input.Items),
	}

	if input.Parallel {
		// Execute in parallel
		futures := make([]workflow.Future, len(input.Items))

		for i, item := range input.Items {
			// Substitute template variables
			containerInput := substituteContainerInput(input.Template, item, i, nil)

			logger.Info("Scheduling loop iteration",
				"index", i,
				"item", item,
				"image", containerInput.Image)

			// Execute container activity
			futures[i] = workflow.ExecuteActivity(ctx, activity.StartContainerActivity, containerInput)
		}

		// Collect results
		for i, future := range futures {
			var result docker.ContainerExecutionOutput
			err := future.Get(ctx, &result)

			output.Results = append(output.Results, result)

			if err != nil || !result.Success {
				output.TotalFailed++
				logger.Error("Loop iteration failed",
					"index", i,
					"item", input.Items[i],
					"error", err)

				if input.FailureStrategy == "fail_fast" {
					output.TotalDuration = workflow.Now(ctx).Sub(startTime)
					return output, fmt.Errorf("loop failed at iteration %d: %w", i, err)
				}
			} else {
				output.TotalSuccess++
			}
		}
	} else {
		// Execute sequentially
		for i, item := range input.Items {
			// Substitute template variables
			containerInput := substituteContainerInput(input.Template, item, i, nil)

			logger.Info("Executing loop iteration",
				"index", i,
				"item", item,
				"image", containerInput.Image)

			// Execute container activity
			var result docker.ContainerExecutionOutput
			err := workflow.ExecuteActivity(ctx, activity.StartContainerActivity, containerInput).Get(ctx, &result)

			output.Results = append(output.Results, result)

			if err != nil || !result.Success {
				output.TotalFailed++
				logger.Error("Loop iteration failed",
					"index", i,
					"item", item,
					"error", err)

				if input.FailureStrategy == "fail_fast" {
					output.TotalDuration = workflow.Now(ctx).Sub(startTime)
					return output, fmt.Errorf("loop failed at iteration %d: %w", i, err)
				}
			} else {
				output.TotalSuccess++
			}
		}
	}

	output.TotalDuration = workflow.Now(ctx).Sub(startTime)

	logger.Info("Loop workflow completed",
		"success", output.TotalSuccess,
		"failed", output.TotalFailed,
		"totalDuration", output.TotalDuration,
		"itemCount", output.ItemCount)

	return output, nil
}

// ParameterizedLoopWorkflow executes containers with parameterized loops (withParam pattern).
func ParameterizedLoopWorkflow(ctx workflow.Context, input docker.ParameterizedLoopInput) (*docker.LoopOutput, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting parameterized loop workflow",
		"parameters", len(input.Parameters),
		"parallel", input.Parallel,
		"maxConcurrency", input.MaxConcurrency)

	// Validate input
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	// Generate all parameter combinations
	combinations := generateParameterCombinations(input.Parameters)

	logger.Info("Generated parameter combinations",
		"combinations", len(combinations))

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

	output := &docker.LoopOutput{
		Results:   make([]docker.ContainerExecutionOutput, 0, len(combinations)),
		ItemCount: len(combinations),
	}

	if input.Parallel {
		// Execute in parallel
		futures := make([]workflow.Future, len(combinations))

		for i, params := range combinations {
			// Substitute template variables
			containerInput := substituteContainerInput(input.Template, "", i, params)

			logger.Info("Scheduling parameterized iteration",
				"index", i,
				"params", params,
				"image", containerInput.Image)

			// Execute container activity
			futures[i] = workflow.ExecuteActivity(ctx, activity.StartContainerActivity, containerInput)
		}

		// Collect results
		for i, future := range futures {
			var result docker.ContainerExecutionOutput
			err := future.Get(ctx, &result)

			output.Results = append(output.Results, result)

			if err != nil || !result.Success {
				output.TotalFailed++
				logger.Error("Parameterized iteration failed",
					"index", i,
					"params", combinations[i],
					"error", err)

				if input.FailureStrategy == "fail_fast" {
					output.TotalDuration = workflow.Now(ctx).Sub(startTime)
					return output, fmt.Errorf("parameterized loop failed at iteration %d: %w", i, err)
				}
			} else {
				output.TotalSuccess++
			}
		}
	} else {
		// Execute sequentially
		for i, params := range combinations {
			// Substitute template variables
			containerInput := substituteContainerInput(input.Template, "", i, params)

			logger.Info("Executing parameterized iteration",
				"index", i,
				"params", params,
				"image", containerInput.Image)

			// Execute container activity
			var result docker.ContainerExecutionOutput
			err := workflow.ExecuteActivity(ctx, activity.StartContainerActivity, containerInput).Get(ctx, &result)

			output.Results = append(output.Results, result)

			if err != nil || !result.Success {
				output.TotalFailed++
				logger.Error("Parameterized iteration failed",
					"index", i,
					"params", params,
					"error", err)

				if input.FailureStrategy == "fail_fast" {
					output.TotalDuration = workflow.Now(ctx).Sub(startTime)
					return output, fmt.Errorf("parameterized loop failed at iteration %d: %w", i, err)
				}
			} else {
				output.TotalSuccess++
			}
		}
	}

	output.TotalDuration = workflow.Now(ctx).Sub(startTime)

	logger.Info("Parameterized loop workflow completed",
		"success", output.TotalSuccess,
		"failed", output.TotalFailed,
		"totalDuration", output.TotalDuration,
		"combinations", output.ItemCount)

	return output, nil
}
