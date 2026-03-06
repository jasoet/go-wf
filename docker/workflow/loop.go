package workflow

import (
	wf "go.temporal.io/sdk/workflow"

	"github.com/jasoet/go-wf/docker/payload"
	generic "github.com/jasoet/go-wf/workflow"
)

// LoopWorkflow executes containers in a loop over items (withItems pattern).
func LoopWorkflow(ctx wf.Context, input payload.LoopInput) (*payload.LoopOutput, error) {
	// Convert to generic input
	genericInput := generic.LoopInput[*payload.ContainerExecutionInput]{
		Items:           input.Items,
		Template:        &input.Template,
		Parallel:        input.Parallel,
		MaxConcurrency:  input.MaxConcurrency,
		FailureStrategy: input.FailureStrategy,
	}

	substitutor := func(tmpl *payload.ContainerExecutionInput, item string, index int, params map[string]string) *payload.ContainerExecutionInput {
		result := substituteContainerInput(*tmpl, item, index, params)
		return &result
	}

	genericOutput, err := generic.LoopWorkflow[*payload.ContainerExecutionInput, payload.ContainerExecutionOutput](ctx, genericInput, substitutor)

	// Convert generic output back to docker output
	if genericOutput != nil {
		output := &payload.LoopOutput{
			Results:       genericOutput.Results,
			TotalSuccess:  genericOutput.TotalSuccess,
			TotalFailed:   genericOutput.TotalFailed,
			TotalDuration: genericOutput.TotalDuration,
			ItemCount:     genericOutput.ItemCount,
		}
		return output, err
	}
	return nil, err
}

// ParameterizedLoopWorkflow executes containers with parameterized loops (withParam pattern).
func ParameterizedLoopWorkflow(ctx wf.Context, input payload.ParameterizedLoopInput) (*payload.LoopOutput, error) {
	// Convert to generic input
	genericInput := generic.ParameterizedLoopInput[*payload.ContainerExecutionInput]{
		Parameters:      input.Parameters,
		Template:        &input.Template,
		Parallel:        input.Parallel,
		MaxConcurrency:  input.MaxConcurrency,
		FailureStrategy: input.FailureStrategy,
	}

	substitutor := func(tmpl *payload.ContainerExecutionInput, item string, index int, params map[string]string) *payload.ContainerExecutionInput {
		result := substituteContainerInput(*tmpl, item, index, params)
		return &result
	}

	genericOutput, err := generic.ParameterizedLoopWorkflow[*payload.ContainerExecutionInput, payload.ContainerExecutionOutput](ctx, genericInput, substitutor)

	// Convert generic output back to docker output
	if genericOutput != nil {
		output := &payload.LoopOutput{
			Results:       genericOutput.Results,
			TotalSuccess:  genericOutput.TotalSuccess,
			TotalFailed:   genericOutput.TotalFailed,
			TotalDuration: genericOutput.TotalDuration,
			ItemCount:     genericOutput.ItemCount,
		}
		return output, err
	}
	return nil, err
}

// FailureStrategyFailFast indicates that workflow should stop on first failure.
// Deprecated: Use generic workflow.FailureStrategyFailFast instead.
const FailureStrategyFailFast = generic.FailureStrategyFailFast
