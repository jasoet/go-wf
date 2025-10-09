package builder

import "time"

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

// WithParallelMode enables parallel execution mode.
//
// Example:
//
//	builder := NewWorkflowBuilder("test", WithParallelMode(true))
func WithParallelMode(parallel bool) BuilderOption {
	return func(b *WorkflowBuilder) {
		b.parallelMode = parallel
	}
}

// WithFailFast enables fail-fast behavior for parallel workflows.
//
// Example:
//
//	builder := NewWorkflowBuilder("test",
//	    WithParallelMode(true),
//	    WithFailFast(true))
func WithFailFast(failFast bool) BuilderOption {
	return func(b *WorkflowBuilder) {
		b.failFast = failFast
	}
}

// WithMaxConcurrency sets maximum concurrent containers for parallel workflows.
//
// Example:
//
//	builder := NewWorkflowBuilder("test",
//	    WithParallelMode(true),
//	    WithMaxConcurrency(10))
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
