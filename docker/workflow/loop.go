package workflow

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/workflow"

	"github.com/jasoet/go-wf/docker/activity"
	"github.com/jasoet/go-wf/docker/payload"
)

const (
	// FailureStrategyFailFast indicates that workflow should stop on first failure.
	FailureStrategyFailFast = "fail_fast"
)

// iterationInput holds the data for a single loop iteration.
type iterationInput struct {
	item   string
	index  int
	params map[string]string
}

// LoopWorkflow executes containers in a loop over items (withItems pattern).
func LoopWorkflow(ctx workflow.Context, input payload.LoopInput) (*payload.LoopOutput, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting loop workflow",
		"items", len(input.Items),
		"parallel", input.Parallel,
		"maxConcurrency", input.MaxConcurrency)

	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	iterations := make([]iterationInput, len(input.Items))
	for i, item := range input.Items {
		iterations[i] = iterationInput{item: item, index: i}
	}

	ctx = workflow.WithActivityOptions(ctx, loopActivityOptions())
	startTime := workflow.Now(ctx)

	output := executeIterations(ctx, input.Template, iterations, input.Parallel, input.FailureStrategy)
	output.TotalDuration = workflow.Now(ctx).Sub(startTime)

	logger.Info("Loop workflow completed",
		"success", output.TotalSuccess,
		"failed", output.TotalFailed,
		"totalDuration", output.TotalDuration,
		"itemCount", output.ItemCount)

	if output.TotalFailed > 0 && input.FailureStrategy == FailureStrategyFailFast {
		return output, fmt.Errorf("loop failed: %d iterations failed", output.TotalFailed)
	}

	return output, nil
}

// ParameterizedLoopWorkflow executes containers with parameterized loops (withParam pattern).
func ParameterizedLoopWorkflow(ctx workflow.Context, input payload.ParameterizedLoopInput) (*payload.LoopOutput, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting parameterized loop workflow",
		"parameters", len(input.Parameters),
		"parallel", input.Parallel,
		"maxConcurrency", input.MaxConcurrency)

	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	combinations := generateParameterCombinations(input.Parameters)
	logger.Info("Generated parameter combinations", "combinations", len(combinations))

	iterations := make([]iterationInput, len(combinations))
	for i, params := range combinations {
		iterations[i] = iterationInput{index: i, params: params}
	}

	ctx = workflow.WithActivityOptions(ctx, loopActivityOptions())
	startTime := workflow.Now(ctx)

	output := executeIterations(ctx, input.Template, iterations, input.Parallel, input.FailureStrategy)
	output.TotalDuration = workflow.Now(ctx).Sub(startTime)

	logger.Info("Parameterized loop workflow completed",
		"success", output.TotalSuccess,
		"failed", output.TotalFailed,
		"totalDuration", output.TotalDuration,
		"combinations", output.ItemCount)

	if output.TotalFailed > 0 && input.FailureStrategy == FailureStrategyFailFast {
		return output, fmt.Errorf("parameterized loop failed: %d iterations failed", output.TotalFailed)
	}

	return output, nil
}

func loopActivityOptions() workflow.ActivityOptions {
	return workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy:         defaultActivityOptions().RetryPolicy,
	}
}

func executeIterations(ctx workflow.Context, template payload.ContainerExecutionInput, iterations []iterationInput, parallel bool, failureStrategy string) *payload.LoopOutput {
	output := &payload.LoopOutput{
		Results:   make([]payload.ContainerExecutionOutput, 0, len(iterations)),
		ItemCount: len(iterations),
	}

	logger := workflow.GetLogger(ctx)

	if parallel {
		executeParallelIterations(ctx, logger, template, iterations, failureStrategy, output)
	} else {
		executeSequentialIterations(ctx, logger, template, iterations, failureStrategy, output)
	}

	return output
}

func executeParallelIterations(ctx workflow.Context, logger interface {
	Info(string, ...interface{})
	Error(string, ...interface{})
}, template payload.ContainerExecutionInput, iterations []iterationInput, failureStrategy string, output *payload.LoopOutput,
) {
	futures := make([]workflow.Future, len(iterations))

	for i, iter := range iterations {
		containerInput := substituteContainerInput(template, iter.item, iter.index, iter.params)
		logger.Info("Scheduling loop iteration", "index", i, "item", iter.item, "image", containerInput.Image)
		futures[i] = workflow.ExecuteActivity(ctx, activity.StartContainerActivity, containerInput)
	}

	for i, future := range futures {
		var result payload.ContainerExecutionOutput
		err := future.Get(ctx, &result)
		output.Results = append(output.Results, result)

		if err != nil || !result.Success {
			output.TotalFailed++
			logger.Error("Loop iteration failed", "index", i, "item", iterations[i].item, "error", err)
			if failureStrategy == FailureStrategyFailFast {
				return
			}
		} else {
			output.TotalSuccess++
		}
	}
}

func executeSequentialIterations(ctx workflow.Context, logger interface {
	Info(string, ...interface{})
	Error(string, ...interface{})
}, template payload.ContainerExecutionInput, iterations []iterationInput, failureStrategy string, output *payload.LoopOutput,
) {
	for i, iter := range iterations {
		containerInput := substituteContainerInput(template, iter.item, iter.index, iter.params)
		logger.Info("Executing loop iteration", "index", i, "item", iter.item, "image", containerInput.Image)

		var result payload.ContainerExecutionOutput
		err := workflow.ExecuteActivity(ctx, activity.StartContainerActivity, containerInput).Get(ctx, &result)
		output.Results = append(output.Results, result)

		if err != nil || !result.Success {
			output.TotalFailed++
			logger.Error("Loop iteration failed", "index", i, "item", iter.item, "error", err)
			if failureStrategy == FailureStrategyFailFast {
				return
			}
		} else {
			output.TotalSuccess++
		}
	}
}
