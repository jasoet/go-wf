package builder

import (
	"context"
	"fmt"

	"github.com/jasoet/pkg/v2/temporal/job"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	fn "github.com/jasoet/go-wf/function"
	"github.com/jasoet/go-wf/function/payload"
	"github.com/jasoet/go-wf/workflow"
)

const (
	// FailureStrategyContinue indicates that workflow should continue after failures.
	FailureStrategyContinue = "continue"
	// FailureStrategyFailFast indicates that workflow should stop on first failure.
	FailureStrategyFailFast = "fail_fast"
)

// workflowMode selects the execution pattern for a WorkflowBuilder.
type workflowMode int

const (
	modeUnset    workflowMode = iota
	modePipeline              // sequential, stop-on-error
	modeParallel              // concurrent tasks
	modeSingle                // one task
)

// loopMode selects the execution pattern for a LoopBuilder.
type loopMode int

const (
	loopModeUnset             loopMode = iota
	loopModeLoop                       // iterate over string items
	loopModeParameterizedLoop          // iterate over parameter combinations
)

// WorkflowBuilder provides a fluent API for constructing generic workflow inputs
// and producing a *job.Definition ready for registration with a Temporal worker.
type WorkflowBuilder[I workflow.TaskInput, O workflow.TaskOutput] struct {
	name           string
	taskQueue      string
	activityFn     any
	mode           workflowMode
	inputs         []I
	stopOnError    bool
	failFast       bool
	maxConcurrency int
	errors         []error
}

// NewWorkflowBuilder creates a new generic workflow builder.
func NewWorkflowBuilder[I workflow.TaskInput, O workflow.TaskOutput]() *WorkflowBuilder[I, O] {
	return &WorkflowBuilder[I, O]{
		inputs:      make([]I, 0),
		stopOnError: true,
	}
}

// NewFunctionBuilder creates a new workflow builder specialised for function execution.
func NewFunctionBuilder() *WorkflowBuilder[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput] {
	return NewWorkflowBuilder[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput]()
}

// Name sets the job name. Required before calling Build.
func (b *WorkflowBuilder[I, O]) Name(n string) *WorkflowBuilder[I, O] {
	b.name = n
	return b
}

// TaskQueue overrides the default task queue (which is "function-<name>").
func (b *WorkflowBuilder[I, O]) TaskQueue(tq string) *WorkflowBuilder[I, O] {
	b.taskQueue = tq
	return b
}

// Activity sets the activity function to register with the worker. Required before calling Build.
func (b *WorkflowBuilder[I, O]) Activity(activityFn any) *WorkflowBuilder[I, O] {
	b.activityFn = activityFn
	return b
}

// Pipeline selects sequential pipeline execution mode.
func (b *WorkflowBuilder[I, O]) Pipeline() *WorkflowBuilder[I, O] {
	b.mode = modePipeline
	return b
}

// Parallel selects concurrent parallel execution mode.
func (b *WorkflowBuilder[I, O]) Parallel() *WorkflowBuilder[I, O] {
	b.mode = modeParallel
	return b
}

// Single selects single-task execution mode.
func (b *WorkflowBuilder[I, O]) Single() *WorkflowBuilder[I, O] {
	b.mode = modeSingle
	return b
}

// Add appends an input to the builder.
func (b *WorkflowBuilder[I, O]) Add(input I) *WorkflowBuilder[I, O] {
	b.inputs = append(b.inputs, input)
	return b
}

// StopOnError configures whether the workflow should stop on first error (pipeline mode).
func (b *WorkflowBuilder[I, O]) StopOnError(stop bool) *WorkflowBuilder[I, O] {
	b.stopOnError = stop
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

// Count returns the number of inputs added to the builder.
func (b *WorkflowBuilder[I, O]) Count() int {
	return len(b.inputs)
}

// Errors returns all errors accumulated during building.
func (b *WorkflowBuilder[I, O]) Errors() []error {
	return b.errors
}

// buildPipelineInput constructs a PipelineInput from the current builder state.
func (b *WorkflowBuilder[I, O]) buildPipelineInput() (*workflow.PipelineInput[I, O], error) {
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

// buildParallelInput constructs a ParallelInput from the current builder state.
func (b *WorkflowBuilder[I, O]) buildParallelInput() (*workflow.ParallelInput[I, O], error) {
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

// buildSingleInput returns the first input in the builder for single-task execution.
func (b *WorkflowBuilder[I, O]) buildSingleInput() (*I, error) {
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

// Build validates the configuration and returns a *job.Definition ready for
// registration with a Temporal worker and execution via the job registry.
//
// Required before calling Build:
//   - Name(...) — sets the job name
//   - Activity(...) — sets the activity function
//   - One of Pipeline(), Parallel(), Single() — selects the execution mode
func (b *WorkflowBuilder[I, O]) Build() (*job.Definition, error) {
	if b.name == "" {
		return nil, fmt.Errorf("function.WorkflowBuilder: Name is required")
	}
	if b.mode == modeUnset {
		return nil, fmt.Errorf("function.WorkflowBuilder: call .Pipeline()/.Parallel()/.Single() before Build")
	}
	if b.activityFn == nil {
		return nil, fmt.Errorf("function.WorkflowBuilder: Activity is required")
	}

	tq := b.taskQueue
	if tq == "" {
		tq = "function-" + b.name
	}

	var wfType string
	var newInputFn func() any

	switch b.mode {
	case modePipeline:
		wfType = "FunctionPipelineWorkflow"
		// Validate the input eagerly so Build returns an error immediately.
		if _, err := b.buildPipelineInput(); err != nil {
			return nil, err
		}
		snapshot := b.inputs
		stopOnError := b.stopOnError
		newInputFn = func() any {
			return &workflow.PipelineInput[I, O]{
				Tasks:       snapshot,
				StopOnError: stopOnError,
			}
		}

	case modeParallel:
		wfType = "ParallelFunctionsWorkflow"
		if _, err := b.buildParallelInput(); err != nil {
			return nil, err
		}
		failureStrategy := FailureStrategyContinue
		if b.failFast {
			failureStrategy = FailureStrategyFailFast
		}
		snapshot := b.inputs
		maxConcurrency := b.maxConcurrency
		fs := failureStrategy
		newInputFn = func() any {
			return &workflow.ParallelInput[I, O]{
				Tasks:           snapshot,
				MaxConcurrency:  maxConcurrency,
				FailureStrategy: fs,
			}
		}

	case modeSingle:
		wfType = "ExecuteFunctionWorkflow"
		if _, err := b.buildSingleInput(); err != nil {
			return nil, err
		}
		snapshot := b.inputs[0]
		newInputFn = func() any {
			cp := snapshot
			return &cp
		}
	}

	activityFn := b.activityFn
	return job.New(b.name, tq,
		job.WithRegister(func(w worker.Worker) {
			fn.RegisterAll(w, activityFn)
		}),
		job.WithExecute(func(ctx context.Context, c client.Client, opts client.StartWorkflowOptions, in any) (client.WorkflowRun, error) {
			return c.ExecuteWorkflow(ctx, opts, wfType, in)
		}),
		job.WithNewInput(newInputFn),
	)
}

// LoopBuilder provides a fluent API for constructing loop workflow inputs
// and producing a *job.Definition ready for registration with a Temporal worker.
type LoopBuilder[I workflow.TaskInput, O workflow.TaskOutput] struct {
	name           string
	taskQueue      string
	activityFn     any
	mode           loopMode
	items          []string
	parameters     map[string][]string
	template       I
	parallel       bool
	maxConcurrency int
	failFast       bool
	errors         []error
}

// NewLoopBuilder creates a new generic loop builder for iterating over string items.
func NewLoopBuilder[I workflow.TaskInput, O workflow.TaskOutput](items []string) *LoopBuilder[I, O] {
	return &LoopBuilder[I, O]{
		items: items,
		mode:  loopModeLoop,
	}
}

// NewParameterizedLoopBuilder creates a new generic parameterized loop builder.
func NewParameterizedLoopBuilder[I workflow.TaskInput, O workflow.TaskOutput](parameters map[string][]string) *LoopBuilder[I, O] {
	return &LoopBuilder[I, O]{
		parameters: parameters,
		mode:       loopModeParameterizedLoop,
	}
}

// NewFunctionLoopBuilder creates a new loop builder specialised for function execution.
func NewFunctionLoopBuilder(items []string) *LoopBuilder[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput] {
	return NewLoopBuilder[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput](items)
}

// NewFunctionParameterizedLoopBuilder creates a new parameterised loop builder specialised for function execution.
func NewFunctionParameterizedLoopBuilder(parameters map[string][]string) *LoopBuilder[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput] {
	return NewParameterizedLoopBuilder[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput](parameters)
}

// Name sets the job name. Required before calling Build.
func (lb *LoopBuilder[I, O]) Name(n string) *LoopBuilder[I, O] {
	lb.name = n
	return lb
}

// TaskQueue overrides the default task queue (which is "function-<name>").
func (lb *LoopBuilder[I, O]) TaskQueue(tq string) *LoopBuilder[I, O] {
	lb.taskQueue = tq
	return lb
}

// Activity sets the activity function to register with the worker. Required before calling Build.
func (lb *LoopBuilder[I, O]) Activity(activityFn any) *LoopBuilder[I, O] {
	lb.activityFn = activityFn
	return lb
}

// Loop selects simple item-iteration mode (default for NewLoopBuilder).
func (lb *LoopBuilder[I, O]) Loop() *LoopBuilder[I, O] {
	lb.mode = loopModeLoop
	return lb
}

// ParameterizedLoop selects parameter-combination mode (default for NewParameterizedLoopBuilder).
func (lb *LoopBuilder[I, O]) ParameterizedLoop() *LoopBuilder[I, O] {
	lb.mode = loopModeParameterizedLoop
	return lb
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

// resolveFailureStrategy returns the resolved failure strategy string.
func (lb *LoopBuilder[I, O]) resolveFailureStrategy() (string, error) {
	if len(lb.errors) > 0 {
		return "", lb.errors[0]
	}
	if lb.failFast {
		return FailureStrategyFailFast, nil
	}
	return FailureStrategyContinue, nil
}

// buildLoopInput constructs a LoopInput from the current builder state.
func (lb *LoopBuilder[I, O]) buildLoopInput() (*workflow.LoopInput[I, O], error) {
	failureStrategy, err := lb.resolveFailureStrategy()
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

// buildParameterizedLoopInput constructs a ParameterizedLoopInput from the current builder state.
func (lb *LoopBuilder[I, O]) buildParameterizedLoopInput() (*workflow.ParameterizedLoopInput[I, O], error) {
	failureStrategy, err := lb.resolveFailureStrategy()
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

// BuildLoop creates a loop workflow input for simple item iteration.
// Kept for backward compatibility with code that only needs the raw input.
func (lb *LoopBuilder[I, O]) BuildLoop() (*workflow.LoopInput[I, O], error) {
	return lb.buildLoopInput()
}

// BuildParameterizedLoop creates a parameterised loop workflow input.
// Kept for backward compatibility with code that only needs the raw input.
func (lb *LoopBuilder[I, O]) BuildParameterizedLoop() (*workflow.ParameterizedLoopInput[I, O], error) {
	return lb.buildParameterizedLoopInput()
}

// Build validates the configuration and returns a *job.Definition ready for
// registration with a Temporal worker and execution via the job registry.
//
// Required before calling Build:
//   - Name(...) — sets the job name
//   - Activity(...) — sets the activity function
//   - The mode is inferred from the constructor (NewLoopBuilder → Loop,
//     NewParameterizedLoopBuilder → ParameterizedLoop). Override with .Loop()
//     or .ParameterizedLoop().
func (lb *LoopBuilder[I, O]) Build() (*job.Definition, error) {
	if lb.name == "" {
		return nil, fmt.Errorf("function.LoopBuilder: Name is required")
	}
	if lb.mode == loopModeUnset {
		return nil, fmt.Errorf("function.LoopBuilder: call .Loop() or .ParameterizedLoop() before Build")
	}
	if lb.activityFn == nil {
		return nil, fmt.Errorf("function.LoopBuilder: Activity is required")
	}

	tq := lb.taskQueue
	if tq == "" {
		tq = "function-" + lb.name
	}

	var wfType string
	var newInputFn func() any

	switch lb.mode {
	case loopModeLoop:
		wfType = "LoopWorkflow"
		if _, err := lb.buildLoopInput(); err != nil {
			return nil, err
		}
		failureStrategy := FailureStrategyContinue
		if lb.failFast {
			failureStrategy = FailureStrategyFailFast
		}
		items := lb.items
		tmpl := lb.template
		parallel := lb.parallel
		maxConcurrency := lb.maxConcurrency
		fs := failureStrategy
		newInputFn = func() any {
			return &workflow.LoopInput[I, O]{
				Items:           items,
				Template:        tmpl,
				Parallel:        parallel,
				MaxConcurrency:  maxConcurrency,
				FailureStrategy: fs,
			}
		}

	case loopModeParameterizedLoop:
		wfType = "ParameterizedLoopWorkflow"
		if _, err := lb.buildParameterizedLoopInput(); err != nil {
			return nil, err
		}
		failureStrategy := FailureStrategyContinue
		if lb.failFast {
			failureStrategy = FailureStrategyFailFast
		}
		params := lb.parameters
		tmpl := lb.template
		parallel := lb.parallel
		maxConcurrency := lb.maxConcurrency
		fs := failureStrategy
		newInputFn = func() any {
			return &workflow.ParameterizedLoopInput[I, O]{
				Parameters:      params,
				Template:        tmpl,
				Parallel:        parallel,
				MaxConcurrency:  maxConcurrency,
				FailureStrategy: fs,
			}
		}
	}

	activityFn := lb.activityFn
	return job.New(lb.name, tq,
		job.WithRegister(func(w worker.Worker) {
			fn.RegisterAll(w, activityFn)
		}),
		job.WithExecute(func(ctx context.Context, c client.Client, opts client.StartWorkflowOptions, in any) (client.WorkflowRun, error) {
			return c.ExecuteWorkflow(ctx, opts, wfType, in)
		}),
		job.WithNewInput(newInputFn),
	)
}

// ForEach creates a function-specific loop builder for iterating over items.
func ForEach(items []string, template payload.FunctionExecutionInput) *LoopBuilder[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput] {
	return NewFunctionLoopBuilder(items).WithTemplate(&template)
}

// ForEachParam creates a function-specific parameterised loop builder.
func ForEachParam(parameters map[string][]string, template payload.FunctionExecutionInput) *LoopBuilder[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput] {
	return NewFunctionParameterizedLoopBuilder(parameters).WithTemplate(&template)
}
