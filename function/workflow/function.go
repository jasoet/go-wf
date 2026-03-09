package workflow

import (
	wf "go.temporal.io/sdk/workflow"

	"github.com/jasoet/go-wf/function/payload"
	generic "github.com/jasoet/go-wf/workflow"
)

// ExecuteFunctionWorkflow runs a single function and returns results.
func ExecuteFunctionWorkflow(ctx wf.Context, input payload.FunctionExecutionInput) (*payload.FunctionExecutionOutput, error) {
	return generic.ExecuteTaskWorkflow[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput](ctx, &input)
}
