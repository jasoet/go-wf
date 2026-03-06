package workflow

import (
	wf "go.temporal.io/sdk/workflow"

	"github.com/jasoet/go-wf/docker/payload"
	generic "github.com/jasoet/go-wf/workflow"
)

// ContainerPipelineWorkflow executes containers sequentially.
func ContainerPipelineWorkflow(ctx wf.Context, input payload.PipelineInput) (*payload.PipelineOutput, error) {
	// Convert docker PipelineInput to generic PipelineInput
	genericInput := generic.PipelineInput[*payload.ContainerExecutionInput]{
		StopOnError: input.StopOnError,
		Cleanup:     input.Cleanup,
	}
	// Convert []ContainerExecutionInput to []*ContainerExecutionInput
	genericInput.Tasks = make([]*payload.ContainerExecutionInput, len(input.Containers))
	for i := range input.Containers {
		genericInput.Tasks[i] = &input.Containers[i]
	}

	genericOutput, err := generic.PipelineWorkflow[*payload.ContainerExecutionInput, payload.ContainerExecutionOutput](ctx, genericInput)

	// Convert generic output back to docker output
	if genericOutput != nil {
		output := &payload.PipelineOutput{
			Results:       genericOutput.Results,
			TotalSuccess:  genericOutput.TotalSuccess,
			TotalFailed:   genericOutput.TotalFailed,
			TotalDuration: genericOutput.TotalDuration,
		}
		return output, err
	}
	return nil, err
}
