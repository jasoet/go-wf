package builder

import (
	"github.com/jasoet/go-wf/v2/function/payload"
	"github.com/jasoet/go-wf/v2/workflow"
)

// BuilderOption is a functional option for configuring WorkflowBuilder.
type BuilderOption[I workflow.TaskInput, O workflow.TaskOutput] func(*WorkflowBuilder[I, O])

// FunctionBuilderOption is a functional option for configuring function workflow builders.
type FunctionBuilderOption = BuilderOption[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput]

// WithStopOnError configures whether the workflow should stop on first error.
func WithStopOnError[I workflow.TaskInput, O workflow.TaskOutput](stop bool) BuilderOption[I, O] {
	return func(b *WorkflowBuilder[I, O]) {
		b.stopOnError = stop
	}
}

// WithParallelMode enables parallel execution mode.
// Deprecated: call .Parallel() on the builder directly; this option is a no-op.
func WithParallelMode[I workflow.TaskInput, O workflow.TaskOutput](_ bool) BuilderOption[I, O] {
	return func(_ *WorkflowBuilder[I, O]) {}
}

// WithFailFast enables fail-fast behavior for parallel workflows.
func WithFailFast[I workflow.TaskInput, O workflow.TaskOutput](failFast bool) BuilderOption[I, O] {
	return func(b *WorkflowBuilder[I, O]) {
		b.failFast = failFast
	}
}

// WithMaxConcurrency sets maximum concurrent functions for parallel workflows.
func WithMaxConcurrency[I workflow.TaskInput, O workflow.TaskOutput](max int) BuilderOption[I, O] {
	return func(b *WorkflowBuilder[I, O]) {
		b.maxConcurrency = max
	}
}

// WithExecutionOptions sets Temporal activity options for the built workflow.
func WithExecutionOptions[I workflow.TaskInput, O workflow.TaskOutput](opts *workflow.ExecutionOptions) BuilderOption[I, O] {
	return func(b *WorkflowBuilder[I, O]) {
		b.executionOptions = opts
	}
}
