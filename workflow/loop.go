package workflow

import (
	"fmt"

	wf "go.temporal.io/sdk/workflow"
)

// Substitutor is a function that creates a new task input with template variables substituted.
type Substitutor[I TaskInput] func(template I, item string, index int, params map[string]string) I

// LoopWorkflow executes a task template for each item.
func LoopWorkflow[I TaskInput, O TaskOutput](ctx wf.Context, input LoopInput[I], substitutor Substitutor[I]) (*LoopOutput[O], error) {
	logger := wf.GetLogger(ctx)
	logger.Info("Starting loop workflow", "items", len(input.Items), "parallel", input.Parallel)

	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	ctx = wf.WithActivityOptions(ctx, DefaultActivityOptions())
	startTime := wf.Now(ctx)

	output := &LoopOutput[O]{
		Results:   make([]O, 0, len(input.Items)),
		ItemCount: len(input.Items),
	}

	if input.Parallel {
		executeParallelLoop(ctx, input, substitutor, output)
	} else {
		executeSequentialLoop(ctx, input, substitutor, output)
	}

	output.TotalDuration = wf.Now(ctx).Sub(startTime)

	if output.TotalFailed > 0 && input.FailureStrategy == FailureStrategyFailFast {
		return output, fmt.Errorf("loop failed: %d iterations failed", output.TotalFailed)
	}

	return output, nil
}

func executeParallelLoop[I TaskInput, O TaskOutput](ctx wf.Context, input LoopInput[I], substitutor Substitutor[I], output *LoopOutput[O]) {
	futures := make([]wf.Future, len(input.Items))
	for i, item := range input.Items {
		taskInput := substitutor(input.Template, item, i, nil)
		futures[i] = wf.ExecuteActivity(ctx, taskInput.ActivityName(), taskInput)
	}

	for _, future := range futures {
		var result O
		err := future.Get(ctx, &result)
		output.Results = append(output.Results, result)
		if err != nil || !result.IsSuccess() {
			output.TotalFailed++
			if input.FailureStrategy == FailureStrategyFailFast {
				return
			}
		} else {
			output.TotalSuccess++
		}
	}
}

func executeSequentialLoop[I TaskInput, O TaskOutput](ctx wf.Context, input LoopInput[I], substitutor Substitutor[I], output *LoopOutput[O]) {
	for i, item := range input.Items {
		taskInput := substitutor(input.Template, item, i, nil)
		var result O
		err := wf.ExecuteActivity(ctx, taskInput.ActivityName(), taskInput).Get(ctx, &result)
		output.Results = append(output.Results, result)
		if err != nil || !result.IsSuccess() {
			output.TotalFailed++
			if input.FailureStrategy == FailureStrategyFailFast {
				return
			}
		} else {
			output.TotalSuccess++
		}
	}
}

// ParameterizedLoopWorkflow executes a task template for each parameter combination.
func ParameterizedLoopWorkflow[I TaskInput, O TaskOutput](ctx wf.Context, input ParameterizedLoopInput[I], substitutor Substitutor[I]) (*LoopOutput[O], error) {
	logger := wf.GetLogger(ctx)
	logger.Info("Starting parameterized loop workflow", "parameters", len(input.Parameters))

	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	combinations := GenerateParameterCombinations(input.Parameters)
	ctx = wf.WithActivityOptions(ctx, DefaultActivityOptions())
	startTime := wf.Now(ctx)

	output := &LoopOutput[O]{
		Results:   make([]O, 0, len(combinations)),
		ItemCount: len(combinations),
	}

	if input.Parallel {
		futures := make([]wf.Future, len(combinations))
		for i, params := range combinations {
			taskInput := substitutor(input.Template, "", i, params)
			futures[i] = wf.ExecuteActivity(ctx, taskInput.ActivityName(), taskInput)
		}
		for _, future := range futures {
			var result O
			err := future.Get(ctx, &result)
			output.Results = append(output.Results, result)
			if err != nil || !result.IsSuccess() {
				output.TotalFailed++
				if input.FailureStrategy == FailureStrategyFailFast {
					break
				}
			} else {
				output.TotalSuccess++
			}
		}
	} else {
		for i, params := range combinations {
			taskInput := substitutor(input.Template, "", i, params)
			var result O
			err := wf.ExecuteActivity(ctx, taskInput.ActivityName(), taskInput).Get(ctx, &result)
			output.Results = append(output.Results, result)
			if err != nil || !result.IsSuccess() {
				output.TotalFailed++
				if input.FailureStrategy == FailureStrategyFailFast {
					break
				}
			} else {
				output.TotalSuccess++
			}
		}
	}

	output.TotalDuration = wf.Now(ctx).Sub(startTime)

	if output.TotalFailed > 0 && input.FailureStrategy == FailureStrategyFailFast {
		return output, fmt.Errorf("parameterized loop failed: %d iterations failed", output.TotalFailed)
	}

	return output, nil
}
