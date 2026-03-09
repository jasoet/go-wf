package workflow

import (
	wf "go.temporal.io/sdk/workflow"

	"github.com/jasoet/go-wf/function/payload"
	generic "github.com/jasoet/go-wf/workflow"
)

// ParallelFunctionsWorkflow executes multiple functions in parallel.
func ParallelFunctionsWorkflow(ctx wf.Context, input payload.ParallelInput) (*payload.ParallelOutput, error) {
	genericInput := generic.ParallelInput[*payload.FunctionExecutionInput]{
		Tasks:           toTaskPtrs(input.Functions),
		MaxConcurrency:  input.MaxConcurrency,
		FailureStrategy: input.FailureStrategy,
	}

	genericOutput, err := generic.ParallelWorkflow[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput](ctx, genericInput)

	return toParallelOutput(genericOutput, err)
}
