package workflow

import (
	wf "go.temporal.io/sdk/workflow"

	"github.com/jasoet/go-wf/docker/payload"
	generic "github.com/jasoet/go-wf/workflow"
)

// ParallelContainersWorkflow executes multiple containers in parallel.
func ParallelContainersWorkflow(ctx wf.Context, input payload.ParallelInput) (*payload.ParallelOutput, error) {
	// Convert docker ParallelInput to generic ParallelInput
	genericInput := generic.ParallelInput[*payload.ContainerExecutionInput]{
		MaxConcurrency:  input.MaxConcurrency,
		FailureStrategy: input.FailureStrategy,
	}
	// Convert []ContainerExecutionInput to []*ContainerExecutionInput
	genericInput.Tasks = make([]*payload.ContainerExecutionInput, len(input.Containers))
	for i := range input.Containers {
		genericInput.Tasks[i] = &input.Containers[i]
	}

	genericOutput, err := generic.ParallelWorkflow[*payload.ContainerExecutionInput, payload.ContainerExecutionOutput](ctx, genericInput)

	// Convert generic output back to docker output
	if genericOutput != nil {
		output := &payload.ParallelOutput{
			Results:       genericOutput.Results,
			TotalSuccess:  genericOutput.TotalSuccess,
			TotalFailed:   genericOutput.TotalFailed,
			TotalDuration: genericOutput.TotalDuration,
		}
		return output, err
	}
	return nil, err
}
