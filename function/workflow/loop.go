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

// LoopWorkflow executes functions in a loop over items.
// Accepts the generic LoopInput directly.
func LoopWorkflow(
	ctx wf.Context,
	input generic.LoopInput[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput],
) (*generic.LoopOutput[payload.FunctionExecutionOutput], error) {
	return generic.InstrumentedLoopWorkflow[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput](ctx, input, functionSubstitutor())
}

// ParameterizedLoopWorkflow executes functions with parameterized loops.
// Accepts the generic ParameterizedLoopInput directly.
func ParameterizedLoopWorkflow(
	ctx wf.Context,
	input generic.ParameterizedLoopInput[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput],
) (*generic.LoopOutput[payload.FunctionExecutionOutput], error) {
	return generic.InstrumentedParameterizedLoopWorkflow[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput](ctx, input, functionSubstitutor())
}
