package workflow

import (
	wf "go.temporal.io/sdk/workflow"

	"github.com/jasoet/go-wf/v2/function/payload"
	generic "github.com/jasoet/go-wf/v2/workflow"
)

// ParallelFunctionsWorkflow executes multiple functions in parallel.
// Accepts the generic ParallelInput directly.
func ParallelFunctionsWorkflow(
	ctx wf.Context,
	input generic.ParallelInput[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput],
) (*generic.ParallelOutput[payload.FunctionExecutionOutput], error) {
	return generic.InstrumentedParallelWorkflow[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput](ctx, input)
}
