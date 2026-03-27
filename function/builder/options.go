package builder

import (
	"github.com/jasoet/go-wf/function/payload"
	"github.com/jasoet/go-wf/workflow"
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
func WithParallelMode[I workflow.TaskInput, O workflow.TaskOutput](parallel bool) BuilderOption[I, O] {
	return func(b *WorkflowBuilder[I, O]) {
		b.parallelMode = parallel
	}
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
