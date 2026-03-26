package workflow

import (
	"fmt"

	wf "go.temporal.io/sdk/workflow"
)

// ParallelWorkflow executes tasks in parallel.
func ParallelWorkflow[I TaskInput, O TaskOutput](ctx wf.Context, input ParallelInput[I]) (*ParallelOutput[O], error) {
	logger := wf.GetLogger(ctx)
	logger.Info("Starting parallel workflow", "tasks", len(input.Tasks))

	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	startTime := wf.Now(ctx)
	ctx = wf.WithActivityOptions(ctx, DefaultActivityOptions())

	// NOTE: All tasks are launched simultaneously. MaxConcurrency is not yet enforced;
	// use Temporal's MaxConcurrentActivityExecutionSize for worker-level limiting.
	futures := make([]wf.Future, len(input.Tasks))
	for i, task := range input.Tasks {
		futures[i] = wf.ExecuteActivity(ctx, task.ActivityName(), task)
	}

	output := &ParallelOutput[O]{
		Results: make([]O, 0, len(input.Tasks)),
	}

	for i, future := range futures {
		var result O
		err := future.Get(ctx, &result)
		output.Results = append(output.Results, result)

		if err != nil || !result.IsSuccess() {
			output.TotalFailed++
			if input.FailureStrategy == FailureStrategyFailFast {
				output.TotalDuration = wf.Now(ctx).Sub(startTime)
				return output, fmt.Errorf("parallel execution failed at task %d: %w", i, err)
			}
		} else {
			output.TotalSuccess++
		}
	}

	output.TotalDuration = wf.Now(ctx).Sub(startTime)
	return output, nil
}
