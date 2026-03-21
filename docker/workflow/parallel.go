package workflow

import (
	wf "go.temporal.io/sdk/workflow"

	"github.com/jasoet/go-wf/docker/payload"
	generic "github.com/jasoet/go-wf/workflow"
)

// ParallelContainersWorkflow executes multiple containers in parallel.
func ParallelContainersWorkflow(ctx wf.Context, input payload.ParallelInput) (*payload.ParallelOutput, error) {
	genericInput := generic.ParallelInput[*payload.ContainerExecutionInput]{
		Tasks:           toTaskPtrs(input.Containers),
		MaxConcurrency:  input.MaxConcurrency,
		FailureStrategy: input.FailureStrategy,
	}

	genericOutput, err := generic.InstrumentedParallelWorkflow[*payload.ContainerExecutionInput, payload.ContainerExecutionOutput](ctx, genericInput)

	return toParallelOutput(genericOutput, err)
}
