package builder

import (
	"fmt"

	"github.com/jasoet/go-wf/function/payload"
)

const (
	// FailureStrategyContinue indicates that workflow should continue after failures.
	FailureStrategyContinue = "continue"
	// FailureStrategyFailFast indicates that workflow should stop on first failure.
	FailureStrategyFailFast = "fail_fast"
)

// WorkflowBuilder provides a fluent API for constructing function workflow inputs.
type WorkflowBuilder struct {
	name           string
	functions      []payload.FunctionExecutionInput
	stopOnError    bool
	parallelMode   bool
	failFast       bool
	maxConcurrency int
	errors         []error
}

// NewWorkflowBuilder creates a new workflow builder with the specified name.
func NewWorkflowBuilder(name string, opts ...BuilderOption) *WorkflowBuilder {
	b := &WorkflowBuilder{
		name:        name,
		functions:   make([]payload.FunctionExecutionInput, 0),
		stopOnError: true,
	}

	for _, opt := range opts {
		opt(b)
	}

	return b
}

// Add adds a workflow source to the builder.
func (b *WorkflowBuilder) Add(source WorkflowSource) *WorkflowBuilder {
	if source == nil {
		b.errors = append(b.errors, fmt.Errorf("cannot add nil source"))
		return b
	}

	input := source.ToInput()
	b.functions = append(b.functions, input)
	return b
}

// AddInput adds a function execution input directly to the builder.
func (b *WorkflowBuilder) AddInput(input payload.FunctionExecutionInput) *WorkflowBuilder {
	b.functions = append(b.functions, input)
	return b
}

// StopOnError configures whether the workflow should stop on first error.
func (b *WorkflowBuilder) StopOnError(stop bool) *WorkflowBuilder {
	b.stopOnError = stop
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

// MaxConcurrency sets the maximum number of concurrent functions for parallel workflows.
func (b *WorkflowBuilder) MaxConcurrency(max int) *WorkflowBuilder {
	b.maxConcurrency = max
	return b
}

// BuildPipeline creates a pipeline workflow configuration.
func (b *WorkflowBuilder) BuildPipeline() (*payload.PipelineInput, error) {
	if len(b.errors) > 0 {
		return nil, b.errors[0]
	}

	if len(b.functions) == 0 {
		return nil, fmt.Errorf("pipeline workflow requires at least one function")
	}

	input := &payload.PipelineInput{
		Functions:   b.functions,
		StopOnError: b.stopOnError,
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

	if len(b.functions) == 0 {
		return nil, fmt.Errorf("parallel workflow requires at least one function")
	}

	failureStrategy := FailureStrategyContinue
	if b.failFast {
		failureStrategy = FailureStrategyFailFast
	}

	input := &payload.ParallelInput{
		Functions:       b.functions,
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

// BuildSingle creates a single function execution workflow.
func (b *WorkflowBuilder) BuildSingle() (*payload.FunctionExecutionInput, error) {
	if len(b.errors) > 0 {
		return nil, b.errors[0]
	}

	if len(b.functions) == 0 {
		return nil, fmt.Errorf("single workflow requires at least one function")
	}

	input := &b.functions[0]

	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("single function validation failed: %w", err)
	}

	return input, nil
}

// Count returns the number of functions added to the builder.
func (b *WorkflowBuilder) Count() int {
	return len(b.functions)
}

// Errors returns all errors accumulated during building.
func (b *WorkflowBuilder) Errors() []error {
	return b.errors
}

// LoopBuilder provides a fluent API for constructing loop workflow inputs.
type LoopBuilder struct {
	items          []string
	parameters     map[string][]string
	template       payload.FunctionExecutionInput
	parallel       bool
	maxConcurrency int
	failFast       bool
	errors         []error
}

// NewLoopBuilder creates a new loop builder with the specified items.
func NewLoopBuilder(items []string) *LoopBuilder {
	return &LoopBuilder{
		items: items,
	}
}

// NewParameterizedLoopBuilder creates a new parameterized loop builder.
func NewParameterizedLoopBuilder(parameters map[string][]string) *LoopBuilder {
	return &LoopBuilder{
		parameters: parameters,
	}
}

// WithTemplate sets the function template for the loop.
func (lb *LoopBuilder) WithTemplate(template payload.FunctionExecutionInput) *LoopBuilder {
	lb.template = template
	return lb
}

// WithSource sets the function template from a workflow source.
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
//nolint:dupl // BuildLoop and BuildParameterizedLoop construct different types with the same pattern.
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
//nolint:dupl // BuildParameterizedLoop and BuildLoop construct different types with the same pattern.
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
func ForEach(items []string, template payload.FunctionExecutionInput) *LoopBuilder {
	return NewLoopBuilder(items).WithTemplate(template)
}

// ForEachParam creates a parameterized loop builder.
func ForEachParam(parameters map[string][]string, template payload.FunctionExecutionInput) *LoopBuilder {
	return NewParameterizedLoopBuilder(parameters).WithTemplate(template)
}
