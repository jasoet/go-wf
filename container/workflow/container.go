package workflow

import (
	wf "go.temporal.io/sdk/workflow"

	"github.com/jasoet/go-wf/v2/container/payload"
	generic "github.com/jasoet/go-wf/v2/workflow"
)

// ExecuteContainerWorkflow runs a single container and returns results.
func ExecuteContainerWorkflow(ctx wf.Context, input payload.ContainerExecutionInput) (*payload.ContainerExecutionOutput, error) {
	return generic.ExecuteTaskWorkflow[*payload.ContainerExecutionInput, payload.ContainerExecutionOutput](ctx, &input)
}
