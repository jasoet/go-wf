package workflow

import (
	wf "go.temporal.io/sdk/workflow"

	"github.com/jasoet/go-wf/docker/payload"
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
