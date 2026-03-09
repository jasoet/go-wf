package builder

// BuilderOption is a functional option for configuring WorkflowBuilder.
type BuilderOption func(*WorkflowBuilder)

// WithStopOnError configures whether the workflow should stop on first error.
func WithStopOnError(stop bool) BuilderOption {
	return func(b *WorkflowBuilder) {
		b.stopOnError = stop
	}
}

// WithParallelMode enables parallel execution mode.
func WithParallelMode(parallel bool) BuilderOption {
	return func(b *WorkflowBuilder) {
		b.parallelMode = parallel
	}
}

// WithFailFast enables fail-fast behavior for parallel workflows.
func WithFailFast(failFast bool) BuilderOption {
	return func(b *WorkflowBuilder) {
		b.failFast = failFast
	}
}

// WithMaxConcurrency sets maximum concurrent functions for parallel workflows.
func WithMaxConcurrency(max int) BuilderOption {
	return func(b *WorkflowBuilder) {
		b.maxConcurrency = max
	}
}
