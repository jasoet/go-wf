package workflow

import (
	wf "go.temporal.io/sdk/workflow"

	"github.com/jasoet/go-wf/container/payload"
	generic "github.com/jasoet/go-wf/workflow"
)

// ContainerPipelineWorkflow executes containers sequentially.
func ContainerPipelineWorkflow(ctx wf.Context, input payload.PipelineInput) (*payload.PipelineOutput, error) {
	genericInput := generic.PipelineInput[*payload.ContainerExecutionInput]{
		Tasks:       toTaskPtrs(input.Containers),
		StopOnError: input.StopOnError,
		Cleanup:     input.Cleanup,
	}

	genericOutput, err := generic.InstrumentedPipelineWorkflow[*payload.ContainerExecutionInput, payload.ContainerExecutionOutput](ctx, genericInput)

	return toPipelineOutput(genericOutput, err)
}

// GenericContainerPipelineWorkflow executes containers sequentially using generic types directly.
// This is the preferred entry point for new code.
func GenericContainerPipelineWorkflow(
	ctx wf.Context,
	input generic.PipelineInput[*payload.ContainerExecutionInput],
) (*generic.PipelineOutput[payload.ContainerExecutionOutput], error) {
	return generic.InstrumentedPipelineWorkflow[*payload.ContainerExecutionInput, payload.ContainerExecutionOutput](ctx, input)
}
