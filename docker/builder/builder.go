package builder

import (
	"fmt"
	"time"

	"github.com/jasoet/go-wf/docker"
)

// WorkflowBuilder provides a fluent API for constructing Docker workflow inputs.
// It supports composing workflows from reusable sources and adding exit handlers.
//
// Example usage:
//
//	builder := NewWorkflowBuilder("deployment").
//	    Add(buildStep).
//	    Add(testStep).
//	    Add(deployStep).
//	    AddExitHandler(cleanupStep).
//	    Build()
type WorkflowBuilder struct {
	name           string
	containers     []docker.ContainerExecutionInput
	exitHandlers   []docker.ContainerExecutionInput
	stopOnError    bool
	cleanup        bool
	parallelMode   bool
	failFast       bool
	maxConcurrency int
	errors         []error
}

// NewWorkflowBuilder creates a new workflow builder with the specified name.
//
// Parameters:
//   - name: Workflow name for identification and logging
//
// Example:
//
//	builder := NewWorkflowBuilder("ci-pipeline")
func NewWorkflowBuilder(name string, opts ...BuilderOption) *WorkflowBuilder {
	b := &WorkflowBuilder{
		name:         name,
		containers:   make([]docker.ContainerExecutionInput, 0),
		exitHandlers: make([]docker.ContainerExecutionInput, 0),
		stopOnError:  true,
		cleanup:      false,
		parallelMode: false,
		failFast:     false,
	}

	// Apply options
	for _, opt := range opts {
		opt(b)
	}

	return b
}

// Add adds a workflow source to the builder.
// Sources are executed in the order they are added (for pipeline mode)
// or concurrently (for parallel mode).
//
// Example:
//
//	builder.Add(buildSource).Add(testSource).Add(deploySource)
func (b *WorkflowBuilder) Add(source WorkflowSource) *WorkflowBuilder {
	if source == nil {
		b.errors = append(b.errors, fmt.Errorf("cannot add nil source"))
		return b
	}

	input := source.ToInput()
	b.containers = append(b.containers, input)
	return b
}

// AddInput adds a container execution input directly to the builder.
// This is useful when you already have a configured input.
//
// Example:
//
//	builder.AddInput(docker.ContainerExecutionInput{
//	    Image: "alpine:latest",
//	    Command: []string{"echo", "hello"},
//	})
func (b *WorkflowBuilder) AddInput(input docker.ContainerExecutionInput) *WorkflowBuilder {
	b.containers = append(b.containers, input)
	return b
}

// AddExitHandler adds a workflow source that executes on workflow exit.
// Exit handlers always run regardless of workflow success or failure.
// They are useful for cleanup operations and notifications.
//
// Example:
//
//	builder.AddExitHandler(cleanupSource).AddExitHandler(notifySource)
func (b *WorkflowBuilder) AddExitHandler(source WorkflowSource) *WorkflowBuilder {
	if source == nil {
		b.errors = append(b.errors, fmt.Errorf("cannot add nil exit handler"))
		return b
	}

	input := source.ToInput()
	b.exitHandlers = append(b.exitHandlers, input)
	return b
}

// AddExitHandlerInput adds a container execution input as an exit handler.
//
// Example:
//
//	builder.AddExitHandlerInput(docker.ContainerExecutionInput{
//	    Image: "alpine:latest",
//	    Command: []string{"cleanup.sh"},
//	})
func (b *WorkflowBuilder) AddExitHandlerInput(input docker.ContainerExecutionInput) *WorkflowBuilder {
	b.exitHandlers = append(b.exitHandlers, input)
	return b
}

// StopOnError configures whether the workflow should stop on first error.
// Default is true for pipeline mode.
//
// Example:
//
//	builder.StopOnError(false) // Continue executing all steps even if one fails
func (b *WorkflowBuilder) StopOnError(stop bool) *WorkflowBuilder {
	b.stopOnError = stop
	return b
}

// Cleanup enables cleanup after each step (for pipeline mode).
//
// Example:
//
//	builder.Cleanup(true)
func (b *WorkflowBuilder) Cleanup(cleanup bool) *WorkflowBuilder {
	b.cleanup = cleanup
	return b
}

// Parallel configures the builder to create a parallel execution workflow.
// By default, workflows execute containers sequentially.
//
// Example:
//
//	builder.Parallel(true).FailFast(true)
func (b *WorkflowBuilder) Parallel(parallel bool) *WorkflowBuilder {
	b.parallelMode = parallel
	return b
}

// FailFast configures fail-fast behavior for parallel workflows.
// Only applicable when Parallel(true) is set.
//
// Example:
//
//	builder.Parallel(true).FailFast(true)
func (b *WorkflowBuilder) FailFast(failFast bool) *WorkflowBuilder {
	b.failFast = failFast
	return b
}

// MaxConcurrency sets the maximum number of concurrent containers for parallel workflows.
// A value of 0 means unlimited concurrency.
//
// Example:
//
//	builder.Parallel(true).MaxConcurrency(5)
func (b *WorkflowBuilder) MaxConcurrency(max int) *WorkflowBuilder {
	b.maxConcurrency = max
	return b
}

// BuildPipeline creates a pipeline workflow configuration.
// Containers execute sequentially in the order they were added.
//
// Returns:
//   - PipelineInput configured with all added containers
//   - error if validation fails
//
// Example:
//
//	input, err := builder.BuildPipeline()
//	if err != nil {
//	    return err
//	}
//	output, err := docker.ContainerPipelineWorkflow(ctx, input)
func (b *WorkflowBuilder) BuildPipeline() (*docker.PipelineInput, error) {
	// Check for errors
	if len(b.errors) > 0 {
		return nil, b.errors[0]
	}

	// Validate at least one container
	if len(b.containers) == 0 {
		return nil, fmt.Errorf("pipeline workflow requires at least one container")
	}

	// Create pipeline input
	input := &docker.PipelineInput{
		Containers:  b.containers,
		StopOnError: b.stopOnError,
		Cleanup:     b.cleanup,
	}

	// Validate input
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("pipeline validation failed: %w", err)
	}

	return input, nil
}

// BuildParallel creates a parallel workflow configuration.
// Containers execute concurrently.
//
// Returns:
//   - ParallelInput configured with all added containers
//   - error if validation fails
//
// Example:
//
//	input, err := builder.Parallel(true).FailFast(true).BuildParallel()
//	if err != nil {
//	    return err
//	}
//	output, err := docker.ParallelContainersWorkflow(ctx, input)
func (b *WorkflowBuilder) BuildParallel() (*docker.ParallelInput, error) {
	// Check for errors
	if len(b.errors) > 0 {
		return nil, b.errors[0]
	}

	// Validate at least one container
	if len(b.containers) == 0 {
		return nil, fmt.Errorf("parallel workflow requires at least one container")
	}

	// Determine failure strategy
	failureStrategy := "continue"
	if b.failFast {
		failureStrategy = "fail_fast"
	}

	// Create parallel input
	input := &docker.ParallelInput{
		Containers:      b.containers,
		MaxConcurrency:  b.maxConcurrency,
		FailureStrategy: failureStrategy,
	}

	// Validate input
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("parallel validation failed: %w", err)
	}

	return input, nil
}

// Build creates the appropriate workflow configuration based on the builder's mode.
// If parallel mode is enabled, creates a ParallelInput, otherwise creates a PipelineInput.
//
// Returns:
//   - interface{} containing either *PipelineInput or *ParallelInput
//   - error if validation fails
//
// Example:
//
//	input, err := builder.Build()
//	if err != nil {
//	    return err
//	}
//	switch v := input.(type) {
//	case *docker.PipelineInput:
//	    output, err := docker.ContainerPipelineWorkflow(ctx, *v)
//	case *docker.ParallelInput:
//	    output, err := docker.ParallelContainersWorkflow(ctx, *v)
//	}
func (b *WorkflowBuilder) Build() (interface{}, error) {
	if b.parallelMode {
		return b.BuildParallel()
	}
	return b.BuildPipeline()
}

// BuildSingle creates a single container execution workflow.
// Only the first container added will be used.
//
// Returns:
//   - ContainerExecutionInput for single container execution
//   - error if no containers were added
//
// Example:
//
//	input, err := builder.AddInput(containerInput).BuildSingle()
//	if err != nil {
//	    return err
//	}
//	output, err := docker.ExecuteContainerWorkflow(ctx, input)
func (b *WorkflowBuilder) BuildSingle() (*docker.ContainerExecutionInput, error) {
	// Check for errors
	if len(b.errors) > 0 {
		return nil, b.errors[0]
	}

	// Validate at least one container
	if len(b.containers) == 0 {
		return nil, fmt.Errorf("single workflow requires at least one container")
	}

	input := &b.containers[0]

	// Validate input
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("single container validation failed: %w", err)
	}

	return input, nil
}

// Count returns the number of containers added to the builder.
func (b *WorkflowBuilder) Count() int {
	return len(b.containers)
}

// ExitHandlerCount returns the number of exit handlers added to the builder.
func (b *WorkflowBuilder) ExitHandlerCount() int {
	return len(b.exitHandlers)
}

// Errors returns all errors accumulated during building.
func (b *WorkflowBuilder) Errors() []error {
	return b.errors
}

// WithTimeout adds a timeout to all containers in the builder.
// This is a convenience method to set RunTimeout on all containers.
//
// Example:
//
//	builder.WithTimeout(5 * time.Minute)
func (b *WorkflowBuilder) WithTimeout(timeout time.Duration) *WorkflowBuilder {
	for i := range b.containers {
		b.containers[i].RunTimeout = timeout
	}
	return b
}

// WithAutoRemove enables auto-remove for all containers in the builder.
//
// Example:
//
//	builder.WithAutoRemove(true)
func (b *WorkflowBuilder) WithAutoRemove(autoRemove bool) *WorkflowBuilder {
	for i := range b.containers {
		b.containers[i].AutoRemove = autoRemove
	}
	return b
}
