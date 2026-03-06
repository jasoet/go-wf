package workflow

import (
	wf "go.temporal.io/sdk/workflow"

	"github.com/jasoet/go-wf/docker/payload"
	generic "github.com/jasoet/go-wf/workflow"
)

// ExecuteContainerWorkflow runs a single container and returns results.
func ExecuteContainerWorkflow(ctx wf.Context, input payload.ContainerExecutionInput) (*payload.ContainerExecutionOutput, error) {
	return generic.ExecuteTaskWorkflow[*payload.ContainerExecutionInput, payload.ContainerExecutionOutput](ctx, &input)
}
