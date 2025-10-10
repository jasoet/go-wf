package workflow

import (
	"fmt"
	"time"

	"github.com/jasoet/go-wf/docker"
	"github.com/jasoet/go-wf/docker/activity"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// ContainerPipelineWorkflow executes containers sequentially.
func ContainerPipelineWorkflow(ctx workflow.Context, input docker.PipelineInput) (*docker.PipelineOutput, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting container pipeline workflow", "steps", len(input.Containers))

	// Validate input
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	startTime := workflow.Now(ctx)
	output := &docker.PipelineOutput{
		Results: make([]docker.ContainerExecutionOutput, 0, len(input.Containers)),
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
		var result docker.ContainerExecutionOutput
		err := workflow.ExecuteActivity(ctx, activity.StartContainerActivity, containerInput).Get(ctx, &result)

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
