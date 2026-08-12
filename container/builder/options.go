package builder

import (
	"time"

	"github.com/jasoet/go-wf/v2/workflow"
)

// BuilderOption is a functional option for configuring WorkflowBuilder.
type BuilderOption func(*WorkflowBuilder)

// WithStopOnError configures whether the workflow should stop on first error.
//
// Example:
//
//	builder := NewWorkflowBuilder("test", WithStopOnError(false))
func WithStopOnError(stop bool) BuilderOption {
	return func(b *WorkflowBuilder) {
		b.stopOnError = stop
	}
}

// WithCleanup enables cleanup after each step.
//
// Example:
//
//	builder := NewWorkflowBuilder("test", WithCleanup(true))
func WithCleanup(cleanup bool) BuilderOption {
	return func(b *WorkflowBuilder) {
		b.cleanup = cleanup
	}
}

// WithParallelMode selects parallel execution mode.
//
// Example:
//
//	builder := NewWorkflowBuilder(WithParallelMode()).Name("test")
func WithParallelMode() BuilderOption {
	return func(b *WorkflowBuilder) {
		b.mode = modeParallel
	}
}

// WithPipelineMode selects sequential pipeline execution mode.
//
// Example:
//
//	builder := NewWorkflowBuilder(WithPipelineMode()).Name("test")
func WithPipelineMode() BuilderOption {
	return func(b *WorkflowBuilder) {
		b.mode = modePipeline
	}
}

// WithSingleMode selects single-container execution mode.
//
// Example:
//
//	builder := NewWorkflowBuilder(WithSingleMode()).Name("test")
func WithSingleMode() BuilderOption {
	return func(b *WorkflowBuilder) {
		b.mode = modeSingle
	}
}

// WithFailFast enables fail-fast behavior for parallel workflows.
//
// Example:
//
//	builder := NewWorkflowBuilder(WithParallelMode(), WithFailFast(true)).Name("test")
func WithFailFast(failFast bool) BuilderOption {
	return func(b *WorkflowBuilder) {
		b.failFast = failFast
	}
}

// WithMaxConcurrency sets maximum concurrent containers for parallel workflows.
//
// Example:
//
//	builder := NewWorkflowBuilder(WithParallelMode(), WithMaxConcurrency(10)).Name("test")
func WithMaxConcurrency(max int) BuilderOption {
	return func(b *WorkflowBuilder) {
		b.maxConcurrency = max
	}
}

// WithGlobalTimeout sets timeout for all containers.
//
// Example:
//
//	builder := NewWorkflowBuilder("test",
//	    WithGlobalTimeout(10 * time.Minute))
func WithGlobalTimeout(timeout time.Duration) BuilderOption {
	return func(b *WorkflowBuilder) {
		// This will be applied to containers as they are added
		// Store it for later application
		b.WithTimeout(timeout)
	}
}

// WithGlobalAutoRemove enables auto-remove for all containers.
//
// Example:
//
//	builder := NewWorkflowBuilder("test",
//	    WithGlobalAutoRemove(true))
func WithGlobalAutoRemove(autoRemove bool) BuilderOption {
	return func(b *WorkflowBuilder) {
		b.WithAutoRemove(autoRemove)
	}
}

// WithExecutionOptions sets Temporal activity options for the built workflow.
// When nil and any container sets RunTimeout, Build derives
// StartToCloseTimeout = max(RunTimeout) + 2 minutes.
//
// Example:
//
//	builder := NewWorkflowBuilder(
//	    WithExecutionOptions(&workflow.ExecutionOptions{StartToCloseTimeout: 45 * time.Minute}))
func WithExecutionOptions(opts *workflow.ExecutionOptions) BuilderOption {
	return func(b *WorkflowBuilder) {
		b.executionOptions = opts
	}
}
