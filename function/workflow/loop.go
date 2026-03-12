package workflow

import (
	wf "go.temporal.io/sdk/workflow"

	"github.com/jasoet/go-wf/function/payload"
	generic "github.com/jasoet/go-wf/workflow"
)

// functionSubstitutor returns a substitutor function for function inputs.
func functionSubstitutor() func(*payload.FunctionExecutionInput, string, int, map[string]string) *payload.FunctionExecutionInput {
	return func(tmpl *payload.FunctionExecutionInput, item string, index int, params map[string]string) *payload.FunctionExecutionInput {
		result := substituteFunctionInput(*tmpl, item, index, params)
		return &result
	}
}

// substituteFunctionInput creates a new function input with substituted values.
func substituteFunctionInput(template payload.FunctionExecutionInput, item string, index int, params map[string]string) payload.FunctionExecutionInput {
	result := template

	// Substitute in name.
	result.Name = generic.SubstituteTemplate(template.Name, item, index, params)

	// Substitute in args.
	if len(template.Args) > 0 {
		result.Args = make(map[string]string, len(template.Args))
		for key, value := range template.Args {
			newKey := generic.SubstituteTemplate(key, item, index, params)
			newValue := generic.SubstituteTemplate(value, item, index, params)
			result.Args[newKey] = newValue
		}
	}

	// Substitute in env.
	if len(template.Env) > 0 {
		result.Env = make(map[string]string, len(template.Env))
		for key, value := range template.Env {
			newKey := generic.SubstituteTemplate(key, item, index, params)
			newValue := generic.SubstituteTemplate(value, item, index, params)
			result.Env[newKey] = newValue
		}
	}

	// Substitute in work directory.
	if template.WorkDir != "" {
		result.WorkDir = generic.SubstituteTemplate(template.WorkDir, item, index, params)
	}

	return result
}

// toLoopOutput converts a generic loop output to a function-specific loop output.
func toLoopOutput(g *generic.LoopOutput[payload.FunctionExecutionOutput], err error) (*payload.LoopOutput, error) {
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

// LoopWorkflow executes functions in a loop over items.
func LoopWorkflow(ctx wf.Context, input payload.LoopInput) (*payload.LoopOutput, error) {
	genericInput := generic.LoopInput[*payload.FunctionExecutionInput]{
		Items:           input.Items,
		Template:        &input.Template,
		Parallel:        input.Parallel,
		MaxConcurrency:  input.MaxConcurrency,
		FailureStrategy: input.FailureStrategy,
	}

	return toLoopOutput(
		generic.InstrumentedLoopWorkflow[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput](ctx, genericInput, functionSubstitutor()),
	)
}

// ParameterizedLoopWorkflow executes functions with parameterized loops.
func ParameterizedLoopWorkflow(ctx wf.Context, input payload.ParameterizedLoopInput) (*payload.LoopOutput, error) {
	genericInput := generic.ParameterizedLoopInput[*payload.FunctionExecutionInput]{
		Parameters:      input.Parameters,
		Template:        &input.Template,
		Parallel:        input.Parallel,
		MaxConcurrency:  input.MaxConcurrency,
		FailureStrategy: input.FailureStrategy,
	}

	return toLoopOutput(
		generic.InstrumentedParameterizedLoopWorkflow[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput](ctx, genericInput, functionSubstitutor()),
	)
}
