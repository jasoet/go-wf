package workflow

import (
	"fmt"

	wf "go.temporal.io/sdk/workflow"
)

// PipelineWorkflow executes tasks sequentially.
func PipelineWorkflow[I TaskInput, O TaskOutput](ctx wf.Context, input PipelineInput[I]) (*PipelineOutput[O], error) {
	logger := wf.GetLogger(ctx)
	logger.Info("Starting pipeline workflow", "steps", len(input.Tasks))

	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	startTime := wf.Now(ctx)
	output := &PipelineOutput[O]{
		Results: make([]O, 0, len(input.Tasks)),
	}

	ctx = wf.WithActivityOptions(ctx, DefaultActivityOptions())

	for i, task := range input.Tasks {
		logger.Info("Executing pipeline step", "step", i+1)

		var result O
		err := wf.ExecuteActivity(ctx, task.ActivityName(), task).Get(ctx, &result)
		output.Results = append(output.Results, result)

		if err != nil || !result.IsSuccess() {
			output.TotalFailed++
			logger.Error("Pipeline step failed", "step", i+1, "error", err)
			if input.StopOnError {
				output.TotalDuration = wf.Now(ctx).Sub(startTime)
				return output, fmt.Errorf("pipeline stopped at step %d: %w", i+1, err)
			}
			continue
		}

		output.TotalSuccess++
	}

	output.TotalDuration = wf.Now(ctx).Sub(startTime)
	return output, nil
}
