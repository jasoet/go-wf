package workflow

import (
	wf "go.temporal.io/sdk/workflow"

	"github.com/jasoet/go-wf/container/payload"
	generic "github.com/jasoet/go-wf/workflow"
)

// ParallelContainersWorkflow executes multiple containers in parallel.
func ParallelContainersWorkflow(ctx wf.Context, input payload.ParallelInput) (*payload.ParallelOutput, error) {
	genericInput := generic.ParallelInput[*payload.ContainerExecutionInput, payload.ContainerExecutionOutput]{
		Tasks:           toTaskPtrs(input.Containers),
		MaxConcurrency:  input.MaxConcurrency,
		FailureStrategy: input.FailureStrategy,
	}

	genericOutput, err := generic.InstrumentedParallelWorkflow[*payload.ContainerExecutionInput, payload.ContainerExecutionOutput](ctx, genericInput)

	return toParallelOutput(genericOutput, err)
}

// GenericParallelContainersWorkflow executes multiple containers in parallel using generic types directly.
// This is the preferred entry point for new code.
func GenericParallelContainersWorkflow(
	ctx wf.Context,
	input generic.ParallelInput[*payload.ContainerExecutionInput, payload.ContainerExecutionOutput],
) (*generic.ParallelOutput[payload.ContainerExecutionOutput], error) {
	return generic.InstrumentedParallelWorkflow[*payload.ContainerExecutionInput, payload.ContainerExecutionOutput](ctx, input)
}
