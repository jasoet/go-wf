package builder

import (
	"fmt"

	"github.com/jasoet/go-wf/function/payload"
	"github.com/jasoet/go-wf/workflow"
)

const (
	// FailureStrategyContinue indicates that workflow should continue after failures.
	FailureStrategyContinue = "continue"
	// FailureStrategyFailFast indicates that workflow should stop on first failure.
	FailureStrategyFailFast = "fail_fast"
)

// WorkflowBuilder provides a fluent API for constructing generic workflow inputs.
type WorkflowBuilder[I workflow.TaskInput, O workflow.TaskOutput] struct {
	name           string
	inputs         []I
	stopOnError    bool
	parallelMode   bool
	failFast       bool
	maxConcurrency int
	errors         []error
}

// NewWorkflowBuilder creates a new generic workflow builder with the specified name.
func NewWorkflowBuilder[I workflow.TaskInput, O workflow.TaskOutput](name string) *WorkflowBuilder[I, O] {
	b := &WorkflowBuilder[I, O]{
		name:        name,
		inputs:      make([]I, 0),
		stopOnError: true,
	}
	return b
}

// NewFunctionBuilder creates a new workflow builder specialized for function execution.
func NewFunctionBuilder(name string) *WorkflowBuilder[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput] {
	return NewWorkflowBuilder[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput](name)
}

// Add adds an input to the builder.
func (b *WorkflowBuilder[I, O]) Add(input I) *WorkflowBuilder[I, O] {
	b.inputs = append(b.inputs, input)
	return b
}

// StopOnError configures whether the workflow should stop on first error.
func (b *WorkflowBuilder[I, O]) StopOnError(stop bool) *WorkflowBuilder[I, O] {
	b.stopOnError = stop
	return b
}

// Parallel configures the builder to create a parallel execution workflow.
func (b *WorkflowBuilder[I, O]) Parallel(parallel bool) *WorkflowBuilder[I, O] {
	b.parallelMode = parallel
	return b
}

// FailFast configures fail-fast behavior for parallel workflows.
func (b *WorkflowBuilder[I, O]) FailFast(failFast bool) *WorkflowBuilder[I, O] {
	b.failFast = failFast
	return b
}

// MaxConcurrency sets the maximum number of concurrent tasks for parallel workflows.
func (b *WorkflowBuilder[I, O]) MaxConcurrency(max int) *WorkflowBuilder[I, O] {
	b.maxConcurrency = max
	return b
}

// BuildPipeline creates a pipeline workflow configuration.
func (b *WorkflowBuilder[I, O]) BuildPipeline() (*workflow.PipelineInput[I, O], error) {
	if len(b.errors) > 0 {
		return nil, b.errors[0]
	}

	if len(b.inputs) == 0 {
		return nil, fmt.Errorf("pipeline workflow requires at least one function")
	}

	input := &workflow.PipelineInput[I, O]{
		Tasks:       b.inputs,
		StopOnError: b.stopOnError,
	}

	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("pipeline validation failed: %w", err)
	}

	return input, nil
}

// BuildParallel creates a parallel workflow configuration.
func (b *WorkflowBuilder[I, O]) BuildParallel() (*workflow.ParallelInput[I, O], error) {
	if len(b.errors) > 0 {
		return nil, b.errors[0]
	}

	if len(b.inputs) == 0 {
		return nil, fmt.Errorf("parallel workflow requires at least one function")
	}

	failureStrategy := FailureStrategyContinue
	if b.failFast {
		failureStrategy = FailureStrategyFailFast
	}

	input := &workflow.ParallelInput[I, O]{
		Tasks:           b.inputs,
		MaxConcurrency:  b.maxConcurrency,
		FailureStrategy: failureStrategy,
	}

	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("parallel validation failed: %w", err)
	}

	return input, nil
}

// Build creates the appropriate workflow configuration based on the builder's mode.
func (b *WorkflowBuilder[I, O]) Build() (interface{}, error) {
	if b.parallelMode {
		return b.BuildParallel()
	}
	return b.BuildPipeline()
}

// BuildSingle creates a single task execution workflow.
func (b *WorkflowBuilder[I, O]) BuildSingle() (*I, error) {
	if len(b.errors) > 0 {
		return nil, b.errors[0]
	}

	if len(b.inputs) == 0 {
		return nil, fmt.Errorf("single workflow requires at least one function")
	}

	input := b.inputs[0] // value copy to avoid pointer to internal slice element

	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("single function validation failed: %w", err)
	}

	return &input, nil
}

// Count returns the number of inputs added to the builder.
func (b *WorkflowBuilder[I, O]) Count() int {
	return len(b.inputs)
}

// Errors returns all errors accumulated during building.
func (b *WorkflowBuilder[I, O]) Errors() []error {
	return b.errors
}

// LoopBuilder provides a fluent API for constructing loop workflow inputs.
type LoopBuilder[I workflow.TaskInput, O workflow.TaskOutput] struct {
	items          []string
	parameters     map[string][]string
	template       I
	parallel       bool
	maxConcurrency int
	failFast       bool
	errors         []error
}

// NewLoopBuilder creates a new generic loop builder with the specified items.
func NewLoopBuilder[I workflow.TaskInput, O workflow.TaskOutput](items []string) *LoopBuilder[I, O] {
	return &LoopBuilder[I, O]{
		items: items,
	}
}

// NewParameterizedLoopBuilder creates a new generic parameterized loop builder.
func NewParameterizedLoopBuilder[I workflow.TaskInput, O workflow.TaskOutput](parameters map[string][]string) *LoopBuilder[I, O] {
	return &LoopBuilder[I, O]{
		parameters: parameters,
	}
}

// NewFunctionLoopBuilder creates a new loop builder specialized for function execution.
func NewFunctionLoopBuilder(items []string) *LoopBuilder[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput] {
	return NewLoopBuilder[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput](items)
}

// NewFunctionParameterizedLoopBuilder creates a new parameterized loop builder specialized for function execution.
func NewFunctionParameterizedLoopBuilder(parameters map[string][]string) *LoopBuilder[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput] {
	return NewParameterizedLoopBuilder[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput](parameters)
}

// WithTemplate sets the template for the loop.
func (lb *LoopBuilder[I, O]) WithTemplate(template I) *LoopBuilder[I, O] {
	lb.template = template
	return lb
}

// Parallel configures the loop to execute in parallel.
func (lb *LoopBuilder[I, O]) Parallel(parallel bool) *LoopBuilder[I, O] {
	lb.parallel = parallel
	return lb
}

// MaxConcurrency sets the maximum number of concurrent iterations.
func (lb *LoopBuilder[I, O]) MaxConcurrency(max int) *LoopBuilder[I, O] {
	lb.maxConcurrency = max
	return lb
}

// FailFast configures fail-fast behavior.
func (lb *LoopBuilder[I, O]) FailFast(failFast bool) *LoopBuilder[I, O] {
	lb.failFast = failFast
	return lb
}

// checkAndStrategy validates the builder state and returns the resolved failure strategy.
func (lb *LoopBuilder[I, O]) checkAndStrategy() (string, error) {
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
func (lb *LoopBuilder[I, O]) BuildLoop() (*workflow.LoopInput[I, O], error) {
	failureStrategy, err := lb.checkAndStrategy()
	if err != nil {
		return nil, err
	}

	if len(lb.items) == 0 {
		return nil, fmt.Errorf("loop requires at least one item")
	}

	input := &workflow.LoopInput[I, O]{
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
func (lb *LoopBuilder[I, O]) BuildParameterizedLoop() (*workflow.ParameterizedLoopInput[I, O], error) {
	failureStrategy, err := lb.checkAndStrategy()
	if err != nil {
		return nil, err
	}

	if len(lb.parameters) == 0 {
		return nil, fmt.Errorf("parameterized loop requires at least one parameter")
	}

	input := &workflow.ParameterizedLoopInput[I, O]{
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

// ForEach creates a function-specific loop builder for iterating over items.
func ForEach(items []string, template payload.FunctionExecutionInput) *LoopBuilder[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput] {
	return NewFunctionLoopBuilder(items).WithTemplate(&template)
}

// ForEachParam creates a function-specific parameterized loop builder.
func ForEachParam(parameters map[string][]string, template payload.FunctionExecutionInput) *LoopBuilder[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput] {
	return NewFunctionParameterizedLoopBuilder(parameters).WithTemplate(&template)
}
