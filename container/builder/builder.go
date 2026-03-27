package builder

import (
	"fmt"
	"time"

	"github.com/jasoet/go-wf/container/payload"
	"github.com/jasoet/go-wf/workflow"
)

const (
	// FailureStrategyContinue indicates that workflow should continue after failures.
	FailureStrategyContinue = "continue"
	// FailureStrategyFailFast indicates that workflow should stop on first failure.
	FailureStrategyFailFast = "fail_fast"
)

// GenericBuilder provides a fluent API for constructing generic workflow inputs.
// It supports any input/output types that satisfy the workflow.TaskInput and
// workflow.TaskOutput constraints.
type GenericBuilder[I workflow.TaskInput, O workflow.TaskOutput] struct {
	inputs         []I
	stopOnError    bool
	cleanup        bool
	parallelMode   bool
	failFast       bool
	maxConcurrency int
	errors         []error
}

// NewGenericBuilder creates a new generic workflow builder.
func NewGenericBuilder[I workflow.TaskInput, O workflow.TaskOutput]() *GenericBuilder[I, O] {
	return &GenericBuilder[I, O]{
		inputs:      make([]I, 0),
		stopOnError: true,
	}
}

// Add adds an input to the generic builder.
func (b *GenericBuilder[I, O]) Add(input I) *GenericBuilder[I, O] {
	b.inputs = append(b.inputs, input)
	return b
}

// StopOnError configures whether the workflow should stop on first error.
func (b *GenericBuilder[I, O]) StopOnError(stop bool) *GenericBuilder[I, O] {
	b.stopOnError = stop
	return b
}

// Cleanup enables cleanup after each step.
func (b *GenericBuilder[I, O]) Cleanup(cleanup bool) *GenericBuilder[I, O] {
	b.cleanup = cleanup
	return b
}

// FailFast configures fail-fast behavior.
func (b *GenericBuilder[I, O]) FailFast(failFast bool) *GenericBuilder[I, O] {
	b.failFast = failFast
	return b
}

// MaxConcurrency sets the maximum number of concurrent tasks.
func (b *GenericBuilder[I, O]) MaxConcurrency(max int) *GenericBuilder[I, O] {
	b.maxConcurrency = max
	return b
}

// BuildPipeline creates a generic pipeline input.
func (b *GenericBuilder[I, O]) BuildPipeline() (*workflow.PipelineInput[I], error) {
	if len(b.errors) > 0 {
		return nil, b.errors[0]
	}
	if len(b.inputs) == 0 {
		return nil, fmt.Errorf("pipeline requires at least one input")
	}

	input := &workflow.PipelineInput[I]{
		Tasks:       b.inputs,
		StopOnError: b.stopOnError,
		Cleanup:     b.cleanup,
	}
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("pipeline validation failed: %w", err)
	}
	return input, nil
}

// BuildParallel creates a generic parallel input.
func (b *GenericBuilder[I, O]) BuildParallel() (*workflow.ParallelInput[I], error) {
	if len(b.errors) > 0 {
		return nil, b.errors[0]
	}
	if len(b.inputs) == 0 {
		return nil, fmt.Errorf("parallel requires at least one input")
	}

	failureStrategy := FailureStrategyContinue
	if b.failFast {
		failureStrategy = FailureStrategyFailFast
	}

	input := &workflow.ParallelInput[I]{
		Tasks:           b.inputs,
		MaxConcurrency:  b.maxConcurrency,
		FailureStrategy: failureStrategy,
	}
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("parallel validation failed: %w", err)
	}
	return input, nil
}

// BuildSingle returns the first input.
func (b *GenericBuilder[I, O]) BuildSingle() (*I, error) {
	if len(b.errors) > 0 {
		return nil, b.errors[0]
	}
	if len(b.inputs) == 0 {
		return nil, fmt.Errorf("single requires at least one input")
	}
	input := &b.inputs[0]
	if err := (*input).Validate(); err != nil {
		return nil, fmt.Errorf("single validation failed: %w", err)
	}
	return input, nil
}

// Count returns the number of inputs.
func (b *GenericBuilder[I, O]) Count() int {
	return len(b.inputs)
}

// Errors returns accumulated errors.
func (b *GenericBuilder[I, O]) Errors() []error {
	return b.errors
}

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
	containers     []payload.ContainerExecutionInput
	exitHandlers   []payload.ContainerExecutionInput
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
		containers:   make([]payload.ContainerExecutionInput, 0),
		exitHandlers: make([]payload.ContainerExecutionInput, 0),
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
func (b *WorkflowBuilder) AddInput(input payload.ContainerExecutionInput) *WorkflowBuilder {
	b.containers = append(b.containers, input)
	return b
}

// AddExitHandler adds a workflow source that executes on workflow exit.
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
func (b *WorkflowBuilder) AddExitHandlerInput(input payload.ContainerExecutionInput) *WorkflowBuilder {
	b.exitHandlers = append(b.exitHandlers, input)
	return b
}

// StopOnError configures whether the workflow should stop on first error.
func (b *WorkflowBuilder) StopOnError(stop bool) *WorkflowBuilder {
	b.stopOnError = stop
	return b
}

// Cleanup enables cleanup after each step (for pipeline mode).
func (b *WorkflowBuilder) Cleanup(cleanup bool) *WorkflowBuilder {
	b.cleanup = cleanup
	return b
}

// Parallel configures the builder to create a parallel execution workflow.
func (b *WorkflowBuilder) Parallel(parallel bool) *WorkflowBuilder {
	b.parallelMode = parallel
	return b
}

// FailFast configures fail-fast behavior for parallel workflows.
func (b *WorkflowBuilder) FailFast(failFast bool) *WorkflowBuilder {
	b.failFast = failFast
	return b
}

// MaxConcurrency sets the maximum number of concurrent containers for parallel workflows.
func (b *WorkflowBuilder) MaxConcurrency(max int) *WorkflowBuilder {
	b.maxConcurrency = max
	return b
}

// BuildPipeline creates a pipeline workflow configuration.
func (b *WorkflowBuilder) BuildPipeline() (*payload.PipelineInput, error) {
	if len(b.errors) > 0 {
		return nil, b.errors[0]
	}

	if len(b.containers) == 0 {
		return nil, fmt.Errorf("pipeline workflow requires at least one container")
	}

	input := &payload.PipelineInput{
		Containers:  b.containers,
		StopOnError: b.stopOnError,
		Cleanup:     b.cleanup,
	}

	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("pipeline validation failed: %w", err)
	}

	return input, nil
}

// BuildGenericPipeline creates a generic pipeline input using workflow.PipelineInput.
func (b *WorkflowBuilder) BuildGenericPipeline() (*workflow.PipelineInput[*payload.ContainerExecutionInput], error) {
	if len(b.errors) > 0 {
		return nil, b.errors[0]
	}

	if len(b.containers) == 0 {
		return nil, fmt.Errorf("pipeline workflow requires at least one container")
	}

	// Convert to pointer slice for generic type compatibility
	ptrs := make([]*payload.ContainerExecutionInput, len(b.containers))
	for i := range b.containers {
		ptrs[i] = &b.containers[i]
	}

	input := &workflow.PipelineInput[*payload.ContainerExecutionInput]{
		Tasks:       ptrs,
		StopOnError: b.stopOnError,
		Cleanup:     b.cleanup,
	}

	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("pipeline validation failed: %w", err)
	}

	return input, nil
}

// BuildParallel creates a parallel workflow configuration.
func (b *WorkflowBuilder) BuildParallel() (*payload.ParallelInput, error) {
	if len(b.errors) > 0 {
		return nil, b.errors[0]
	}

	if len(b.containers) == 0 {
		return nil, fmt.Errorf("parallel workflow requires at least one container")
	}

	failureStrategy := FailureStrategyContinue
	if b.failFast {
		failureStrategy = FailureStrategyFailFast
	}

	input := &payload.ParallelInput{
		Containers:      b.containers,
		MaxConcurrency:  b.maxConcurrency,
		FailureStrategy: failureStrategy,
	}

	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("parallel validation failed: %w", err)
	}

	return input, nil
}

// BuildGenericParallel creates a generic parallel input using workflow.ParallelInput.
func (b *WorkflowBuilder) BuildGenericParallel() (*workflow.ParallelInput[*payload.ContainerExecutionInput], error) {
	if len(b.errors) > 0 {
		return nil, b.errors[0]
	}

	if len(b.containers) == 0 {
		return nil, fmt.Errorf("parallel workflow requires at least one container")
	}

	failureStrategy := FailureStrategyContinue
	if b.failFast {
		failureStrategy = FailureStrategyFailFast
	}

	// Convert to pointer slice for generic type compatibility
	ptrs := make([]*payload.ContainerExecutionInput, len(b.containers))
	for i := range b.containers {
		ptrs[i] = &b.containers[i]
	}

	input := &workflow.ParallelInput[*payload.ContainerExecutionInput]{
		Tasks:           ptrs,
		MaxConcurrency:  b.maxConcurrency,
		FailureStrategy: failureStrategy,
	}

	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("parallel validation failed: %w", err)
	}

	return input, nil
}

// Build creates the appropriate workflow configuration based on the builder's mode.
func (b *WorkflowBuilder) Build() (interface{}, error) {
	if b.parallelMode {
		return b.BuildParallel()
	}
	return b.BuildPipeline()
}

// BuildSingle creates a single container execution workflow.
func (b *WorkflowBuilder) BuildSingle() (*payload.ContainerExecutionInput, error) {
	if len(b.errors) > 0 {
		return nil, b.errors[0]
	}

	if len(b.containers) == 0 {
		return nil, fmt.Errorf("single workflow requires at least one container")
	}

	input := &b.containers[0]

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
func (b *WorkflowBuilder) WithTimeout(timeout time.Duration) *WorkflowBuilder {
	for i := range b.containers {
		b.containers[i].RunTimeout = timeout
	}
	return b
}

// WithAutoRemove enables auto-remove for all containers in the builder.
func (b *WorkflowBuilder) WithAutoRemove(autoRemove bool) *WorkflowBuilder {
	for i := range b.containers {
		b.containers[i].AutoRemove = autoRemove
	}
	return b
}

// LoopBuilder provides a fluent API for constructing loop workflow inputs.
type LoopBuilder struct {
	items          []string
	parameters     map[string][]string
	template       payload.ContainerExecutionInput
	parallel       bool
	maxConcurrency int
	failFast       bool
	errors         []error
}

// NewLoopBuilder creates a new loop builder with the specified items.
func NewLoopBuilder(items []string) *LoopBuilder {
	return &LoopBuilder{
		items:    items,
		parallel: false,
		failFast: false,
	}
}

// NewParameterizedLoopBuilder creates a new parameterized loop builder.
func NewParameterizedLoopBuilder(parameters map[string][]string) *LoopBuilder {
	return &LoopBuilder{
		parameters: parameters,
		parallel:   false,
		failFast:   false,
	}
}

// WithTemplate sets the container template for the loop.
func (lb *LoopBuilder) WithTemplate(template payload.ContainerExecutionInput) *LoopBuilder {
	lb.template = template
	return lb
}

// WithSource sets the container template from a workflow source.
func (lb *LoopBuilder) WithSource(source WorkflowSource) *LoopBuilder {
	if source == nil {
		lb.errors = append(lb.errors, fmt.Errorf("cannot use nil source"))
		return lb
	}
	lb.template = source.ToInput()
	return lb
}

// Parallel configures the loop to execute in parallel.
func (lb *LoopBuilder) Parallel(parallel bool) *LoopBuilder {
	lb.parallel = parallel
	return lb
}

// MaxConcurrency sets the maximum number of concurrent iterations.
func (lb *LoopBuilder) MaxConcurrency(max int) *LoopBuilder {
	lb.maxConcurrency = max
	return lb
}

// FailFast configures fail-fast behavior.
func (lb *LoopBuilder) FailFast(failFast bool) *LoopBuilder {
	lb.failFast = failFast
	return lb
}

// checkAndStrategy validates the builder state and returns the resolved failure strategy.
func (lb *LoopBuilder) checkAndStrategy() (string, error) {
	if len(lb.errors) > 0 {
		return "", lb.errors[0]
	}

	failureStrategy := FailureStrategyContinue
	if lb.failFast {
		failureStrategy = FailureStrategyFailFast
	}
	return failureStrategy, nil
}

// BuildLoop creates a loop workflow configuration for simple item iteration.
//
//nolint:dupl // BuildLoop and BuildParameterizedLoop construct different types with the same pattern
func (lb *LoopBuilder) BuildLoop() (*payload.LoopInput, error) {
	failureStrategy, err := lb.checkAndStrategy()
	if err != nil {
		return nil, err
	}

	if len(lb.items) == 0 {
		return nil, fmt.Errorf("loop requires at least one item")
	}

	input := &payload.LoopInput{
		Items:           lb.items,
		Template:        lb.template,
		Parallel:        lb.parallel,
		MaxConcurrency:  lb.maxConcurrency,
		FailureStrategy: failureStrategy,
	}

	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("loop validation failed: %w", err)
	}

	return input, nil
}

// BuildParameterizedLoop creates a parameterized loop workflow configuration.
//
//nolint:dupl // BuildParameterizedLoop and BuildLoop construct different types with the same pattern
func (lb *LoopBuilder) BuildParameterizedLoop() (*payload.ParameterizedLoopInput, error) {
	failureStrategy, err := lb.checkAndStrategy()
	if err != nil {
		return nil, err
	}

	if len(lb.parameters) == 0 {
		return nil, fmt.Errorf("parameterized loop requires at least one parameter")
	}

	input := &payload.ParameterizedLoopInput{
		Parameters:      lb.parameters,
		Template:        lb.template,
		Parallel:        lb.parallel,
		MaxConcurrency:  lb.maxConcurrency,
		FailureStrategy: failureStrategy,
	}

	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("parameterized loop validation failed: %w", err)
	}

	return input, nil
}

// ForEach creates a loop builder for iterating over items.
func ForEach(items []string, template payload.ContainerExecutionInput) *LoopBuilder {
	return NewLoopBuilder(items).WithTemplate(template)
}

// ForEachParam creates a parameterized loop builder.
func ForEachParam(parameters map[string][]string, template payload.ContainerExecutionInput) *LoopBuilder {
	return NewParameterizedLoopBuilder(parameters).WithTemplate(template)
}
