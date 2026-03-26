package workflow

import (
	wf "go.temporal.io/sdk/workflow"

	"github.com/jasoet/go-wf/container/payload"
	generic "github.com/jasoet/go-wf/workflow"
)

// containerSubstitutor returns a substitutor function for container inputs.
func containerSubstitutor() func(*payload.ContainerExecutionInput, string, int, map[string]string) *payload.ContainerExecutionInput {
	return func(tmpl *payload.ContainerExecutionInput, item string, index int, params map[string]string) *payload.ContainerExecutionInput {
		result := substituteContainerInput(*tmpl, item, index, params)
		return &result
	}
}

// toLoopOutput converts a generic loop output to a docker-specific loop output.
func toLoopOutput(g *generic.LoopOutput[payload.ContainerExecutionOutput], err error) (*payload.LoopOutput, error) {
	if g == nil {
		return nil, err
	}
	return &payload.LoopOutput{
		Results:       g.Results,
		TotalSuccess:  g.TotalSuccess,
		TotalFailed:   g.TotalFailed,
		TotalDuration: g.TotalDuration,
		ItemCount:     g.ItemCount,
	}, err
}

// LoopWorkflow executes containers in a loop over items (withItems pattern).
func LoopWorkflow(ctx wf.Context, input payload.LoopInput) (*payload.LoopOutput, error) {
	genericInput := generic.LoopInput[*payload.ContainerExecutionInput]{
		Items:           input.Items,
		Template:        &input.Template,
		Parallel:        input.Parallel,
		MaxConcurrency:  input.MaxConcurrency,
		FailureStrategy: input.FailureStrategy,
	}

	return toLoopOutput(
		generic.InstrumentedLoopWorkflow[*payload.ContainerExecutionInput, payload.ContainerExecutionOutput](ctx, genericInput, containerSubstitutor()),
	)
}

// ParameterizedLoopWorkflow executes containers with parameterized loops (withParam pattern).
func ParameterizedLoopWorkflow(ctx wf.Context, input payload.ParameterizedLoopInput) (*payload.LoopOutput, error) {
	genericInput := generic.ParameterizedLoopInput[*payload.ContainerExecutionInput]{
		Parameters:      input.Parameters,
		Template:        &input.Template,
		Parallel:        input.Parallel,
		MaxConcurrency:  input.MaxConcurrency,
		FailureStrategy: input.FailureStrategy,
	}

	return toLoopOutput(
		generic.InstrumentedParameterizedLoopWorkflow[*payload.ContainerExecutionInput, payload.ContainerExecutionOutput](ctx, genericInput, containerSubstitutor()),
	)
}

// FailureStrategyFailFast indicates that workflow should stop on first failure.
// Deprecated: Use generic workflow.FailureStrategyFailFast instead.
const FailureStrategyFailFast = generic.FailureStrategyFailFast
