package builder

import "github.com/jasoet/go-wf/function/payload"

// WorkflowSource represents a composable workflow component that can generate function execution inputs.
type WorkflowSource interface {
	// ToInput converts the source into a FunctionExecutionInput.
	ToInput() payload.FunctionExecutionInput
}

// WorkflowSourceFunc is a function adapter for WorkflowSource interface.
type WorkflowSourceFunc func() payload.FunctionExecutionInput

// ToInput implements WorkflowSource interface.
func (f WorkflowSourceFunc) ToInput() payload.FunctionExecutionInput {
	return f()
}

// FunctionSource creates a WorkflowSource from a FunctionExecutionInput.
type FunctionSource struct {
	input payload.FunctionExecutionInput
}

// NewFunctionSource creates a new function source.
func NewFunctionSource(input payload.FunctionExecutionInput) *FunctionSource {
	return &FunctionSource{input: input}
}

// ToInput implements WorkflowSource interface.
func (f *FunctionSource) ToInput() payload.FunctionExecutionInput {
	return f.input
}
