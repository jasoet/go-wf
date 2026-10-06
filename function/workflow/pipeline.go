package workflow

import (
	wf "go.temporal.io/sdk/workflow"

	"github.com/jasoet/go-wf/v3/function/payload"
	generic "github.com/jasoet/go-wf/v3/workflow"
)

// FunctionPipelineWorkflow executes functions sequentially.
// Accepts the generic PipelineInput directly.
func FunctionPipelineWorkflow(
	ctx wf.Context,
	input generic.PipelineInput[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput],
) (*generic.PipelineOutput[payload.FunctionExecutionOutput], error) {
	return generic.InstrumentedPipelineWorkflow[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput](ctx, input)
}
