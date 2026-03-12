package workflow

import (
	wf "go.temporal.io/sdk/workflow"
)

// InstrumentedPipelineWorkflow wraps PipelineWorkflow with structured logging at boundaries.
func InstrumentedPipelineWorkflow[I TaskInput, O TaskOutput](ctx wf.Context, input PipelineInput[I]) (*PipelineOutput[O], error) {
	logger := wf.GetLogger(ctx)
	logger.Info("pipeline.start",
		"step_count", len(input.Tasks),
		"stop_on_error", input.StopOnError,
	)

	startTime := wf.Now(ctx)
	result, err := PipelineWorkflow[I, O](ctx, input)

	duration := wf.Now(ctx).Sub(startTime)

	if err != nil {
		logger.Error("pipeline.failed",
			"error", err,
			"duration", duration,
		)
		return result, err
	}

	logger.Info("pipeline.complete",
		"total_steps", len(input.Tasks),
		"success_count", result.TotalSuccess,
		"failure_count", result.TotalFailed,
		"duration", duration,
	)

	return result, nil
}

// InstrumentedParallelWorkflow wraps ParallelWorkflow with structured logging at boundaries.
func InstrumentedParallelWorkflow[I TaskInput, O TaskOutput](ctx wf.Context, input ParallelInput[I]) (*ParallelOutput[O], error) {
	logger := wf.GetLogger(ctx)
	logger.Info("parallel.start",
		"task_count", len(input.Tasks),
		"max_concurrency", input.MaxConcurrency,
		"failure_strategy", input.FailureStrategy,
	)

	startTime := wf.Now(ctx)
	result, err := ParallelWorkflow[I, O](ctx, input)

	duration := wf.Now(ctx).Sub(startTime)

	if err != nil {
		logger.Error("parallel.failed",
			"error", err,
			"duration", duration,
		)
		return result, err
	}

	logger.Info("parallel.complete",
		"total_tasks", len(input.Tasks),
		"success_count", result.TotalSuccess,
		"failure_count", result.TotalFailed,
		"duration", duration,
	)

	return result, nil
}

// InstrumentedLoopWorkflow wraps LoopWorkflow with structured logging at boundaries.
func InstrumentedLoopWorkflow[I TaskInput, O TaskOutput](ctx wf.Context, input LoopInput[I], substitutor Substitutor[I]) (*LoopOutput[O], error) {
	logger := wf.GetLogger(ctx)
	logger.Info("loop.start",
		"item_count", len(input.Items),
		"parallel", input.Parallel,
		"failure_strategy", input.FailureStrategy,
	)

	startTime := wf.Now(ctx)
	result, err := LoopWorkflow[I, O](ctx, input, substitutor)

	duration := wf.Now(ctx).Sub(startTime)

	if err != nil {
		logger.Error("loop.failed",
			"error", err,
			"duration", duration,
		)
		return result, err
	}

	logger.Info("loop.complete",
		"iterations", len(input.Items),
		"success_count", result.TotalSuccess,
		"failure_count", result.TotalFailed,
		"duration", duration,
	)

	return result, nil
}

// InstrumentedParameterizedLoopWorkflow wraps ParameterizedLoopWorkflow with structured logging at boundaries.
func InstrumentedParameterizedLoopWorkflow[I TaskInput, O TaskOutput](ctx wf.Context, input ParameterizedLoopInput[I], substitutor Substitutor[I]) (*LoopOutput[O], error) {
	logger := wf.GetLogger(ctx)
	combinationCount := len(GenerateParameterCombinations(input.Parameters))
	logger.Info("parameterized_loop.start",
		"combination_count", combinationCount,
		"parallel", input.Parallel,
		"failure_strategy", input.FailureStrategy,
	)

	startTime := wf.Now(ctx)
	result, err := ParameterizedLoopWorkflow[I, O](ctx, input, substitutor)

	duration := wf.Now(ctx).Sub(startTime)

	if err != nil {
		logger.Error("parameterized_loop.failed",
			"error", err,
			"duration", duration,
		)
		return result, err
	}

	logger.Info("parameterized_loop.complete",
		"iterations", combinationCount,
		"success_count", result.TotalSuccess,
		"failure_count", result.TotalFailed,
		"duration", duration,
	)

	return result, nil
}
