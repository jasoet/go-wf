package workflow

import (
	wf "go.temporal.io/sdk/workflow"

	"github.com/jasoet/go-wf/function/payload"
	generic "github.com/jasoet/go-wf/workflow"
)

// FunctionPipelineWorkflow executes functions sequentially.
func FunctionPipelineWorkflow(ctx wf.Context, input payload.PipelineInput) (*payload.PipelineOutput, error) {
	genericInput := generic.PipelineInput[*payload.FunctionExecutionInput]{
		Tasks:       toTaskPtrs(input.Functions),
		StopOnError: input.StopOnError,
	}

	genericOutput, err := generic.PipelineWorkflow[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput](ctx, genericInput)

	return toPipelineOutput(genericOutput, err)
}
