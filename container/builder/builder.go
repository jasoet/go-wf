package builder

import (
	"context"
	"fmt"
	"time"

	"github.com/jasoet/pkg/v2/temporal/job"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"github.com/jasoet/go-wf/container"
	"github.com/jasoet/go-wf/container/payload"
	"github.com/jasoet/go-wf/workflow"
)

const (
	// FailureStrategyContinue indicates that workflow should continue after failures.
	FailureStrategyContinue = "continue"
	// FailureStrategyFailFast indicates that workflow should stop on first failure.
	FailureStrategyFailFast = "fail_fast"
)

// runTimeoutMargin is added to the max container RunTimeout when Build derives
// StartToCloseTimeout, so the activity outlives the container's own timeout.
const runTimeoutMargin = 2 * time.Minute

// resolveExecutionOptions derives activity options from the max container
// RunTimeout when none are set explicitly, and validates that an explicit
// StartToCloseTimeout exceeds the max RunTimeout. Explicit options that set
// other fields (e.g. RetryPolicy) but leave StartToCloseTimeout zero also get
// a derived StartToCloseTimeout; the caller's struct is never mutated.
func resolveExecutionOptions(explicit *workflow.ExecutionOptions, maxRunTimeout time.Duration) (*workflow.ExecutionOptions, error) {
	if explicit != nil && explicit.StartToCloseTimeout == 0 && maxRunTimeout > 0 {
		derived := *explicit
		derived.StartToCloseTimeout = maxRunTimeout + runTimeoutMargin
		return &derived, nil
	}
	opts := explicit
	if opts == nil && maxRunTimeout > 0 {
		opts = &workflow.ExecutionOptions{StartToCloseTimeout: maxRunTimeout + runTimeoutMargin}
	}
	if opts != nil && maxRunTimeout > 0 && opts.StartToCloseTimeout > 0 && opts.StartToCloseTimeout <= maxRunTimeout {
		return opts, fmt.Errorf("start_to_close_timeout (%s) must exceed max container run_timeout (%s)", opts.StartToCloseTimeout, maxRunTimeout)
	}
	return opts, nil
}

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

// GenericBuilder provides a fluent API for constructing generic workflow inputs.
// It supports any input/output types that satisfy the workflow.TaskInput and
// workflow.TaskOutput constraints.
type GenericBuilder[I workflow.TaskInput, O workflow.TaskOutput] struct {
	inputs         []I
	stopOnError    bool
	cleanup        bool
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
func (b *GenericBuilder[I, O]) BuildPipeline() (*workflow.PipelineInput[I, O], error) {
	if len(b.errors) > 0 {
		return nil, b.errors[0]
	}
	if len(b.inputs) == 0 {
		return nil, fmt.Errorf("pipeline requires at least one input")
	}

	input := &workflow.PipelineInput[I, O]{
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
func (b *GenericBuilder[I, O]) BuildParallel() (*workflow.ParallelInput[I, O], error) {
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

// WorkflowBuilder provides a fluent API for constructing container workflow inputs
// and producing a *job.Definition ready for registration with a Temporal worker.
//
// Call Name(...), one of Pipeline()/Parallel()/Single() to select a mode, add
// containers via Add/AddInput, then call Build() to get a *job.Definition.
//
// Example usage:
//
//	def, err := NewWorkflowBuilder().
//	    Name("deployment").
//	    Pipeline().
//	    Add(buildStep).
//	    Add(testStep).
//	    Add(deployStep).
//	    AddExitHandler(cleanupStep).
//	    Build()
type WorkflowBuilder struct {
	name             string
	taskQueue        string
	mode             workflowMode
	containers       []payload.ContainerExecutionInput
	exitHandlers     []payload.ContainerExecutionInput
	stopOnError      bool
	cleanup          bool
	failFast         bool
	maxConcurrency   int
	executionOptions *workflow.ExecutionOptions
	errors           []error
}

// NewWorkflowBuilder creates a new workflow builder.
//
// Example:
//
//	builder := NewWorkflowBuilder().Name("ci-pipeline").Pipeline()
func NewWorkflowBuilder(opts ...BuilderOption) *WorkflowBuilder {
	b := &WorkflowBuilder{
		containers:   make([]payload.ContainerExecutionInput, 0),
		exitHandlers: make([]payload.ContainerExecutionInput, 0),
		stopOnError:  true,
		cleanup:      false,
		failFast:     false,
	}

	// Apply options
	for _, opt := range opts {
		opt(b)
	}

	return b
}

// Name sets the job name. Required before calling Build.
func (b *WorkflowBuilder) Name(n string) *WorkflowBuilder {
	b.name = n
	return b
}

// TaskQueue overrides the default task queue (which is "container-<name>").
func (b *WorkflowBuilder) TaskQueue(tq string) *WorkflowBuilder {
	b.taskQueue = tq
	return b
}

// Pipeline selects sequential pipeline execution mode.
func (b *WorkflowBuilder) Pipeline() *WorkflowBuilder {
	b.mode = modePipeline
	return b
}

// Parallel selects concurrent parallel execution mode.
func (b *WorkflowBuilder) Parallel() *WorkflowBuilder {
	b.mode = modeParallel
	return b
}

// Single selects single-container execution mode.
func (b *WorkflowBuilder) Single() *WorkflowBuilder {
	b.mode = modeSingle
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

// WithExecutionOptions sets Temporal activity options for the built workflow.
// It applies to Pipeline/Parallel/Loop modes only — Single mode has no Options
// field. When nil and any container sets RunTimeout, Build derives
// StartToCloseTimeout = max(RunTimeout) + 2 minutes. Explicit options that
// leave StartToCloseTimeout zero (e.g. RetryPolicy-only) also get
// StartToCloseTimeout derived from RunTimeout; the caller's struct is not
// mutated.
func (b *WorkflowBuilder) WithExecutionOptions(opts *workflow.ExecutionOptions) *WorkflowBuilder {
	b.executionOptions = opts
	return b
}

// buildPipelineInput constructs a PipelineInput from the current builder state.
func (b *WorkflowBuilder) buildPipelineInput() (*payload.PipelineInput, error) {
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

// buildParallelInput constructs a ParallelInput from the current builder state.
func (b *WorkflowBuilder) buildParallelInput() (*payload.ParallelInput, error) {
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

// buildSingleInput constructs a single ContainerExecutionInput from the current builder state.
func (b *WorkflowBuilder) buildSingleInput() (*payload.ContainerExecutionInput, error) {
	if len(b.errors) > 0 {
		return nil, b.errors[0]
	}

	if len(b.containers) == 0 {
		return nil, fmt.Errorf("single workflow requires at least one container")
	}

	input := b.containers[0]

	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("single container validation failed: %w", err)
	}

	return &input, nil
}

// BuildPipeline creates a pipeline workflow configuration.
// Kept for callers that only need the raw input without a full job.Definition.
func (b *WorkflowBuilder) BuildPipeline() (*payload.PipelineInput, error) {
	return b.buildPipelineInput()
}

// BuildParallel creates a parallel workflow configuration.
// Kept for callers that only need the raw input without a full job.Definition.
func (b *WorkflowBuilder) BuildParallel() (*payload.ParallelInput, error) {
	return b.buildParallelInput()
}

// BuildSingle creates a single container execution workflow.
// Kept for callers that only need the raw input without a full job.Definition.
func (b *WorkflowBuilder) BuildSingle() (*payload.ContainerExecutionInput, error) {
	return b.buildSingleInput()
}

// BuildGenericPipeline creates a generic pipeline input using workflow.PipelineInput.
// Kept for callers that only need the raw typed input.
func (b *WorkflowBuilder) BuildGenericPipeline() (*workflow.PipelineInput[*payload.ContainerExecutionInput, payload.ContainerExecutionOutput], error) {
	return b.buildGenericPipelineInput()
}

// BuildGenericParallel creates a generic parallel input using workflow.ParallelInput.
// Kept for callers that only need the raw typed input.
func (b *WorkflowBuilder) BuildGenericParallel() (*workflow.ParallelInput[*payload.ContainerExecutionInput, payload.ContainerExecutionOutput], error) {
	return b.buildGenericParallelInput()
}

// buildGenericPipelineInput creates a generic pipeline input using workflow.PipelineInput.
func (b *WorkflowBuilder) buildGenericPipelineInput() (*workflow.PipelineInput[*payload.ContainerExecutionInput, payload.ContainerExecutionOutput], error) {
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

	input := &workflow.PipelineInput[*payload.ContainerExecutionInput, payload.ContainerExecutionOutput]{
		Tasks:       ptrs,
		StopOnError: b.stopOnError,
		Cleanup:     b.cleanup,
	}

	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("pipeline validation failed: %w", err)
	}

	return input, nil
}

// buildGenericParallelInput creates a generic parallel input using workflow.ParallelInput.
func (b *WorkflowBuilder) buildGenericParallelInput() (*workflow.ParallelInput[*payload.ContainerExecutionInput, payload.ContainerExecutionOutput], error) {
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

	input := &workflow.ParallelInput[*payload.ContainerExecutionInput, payload.ContainerExecutionOutput]{
		Tasks:           ptrs,
		MaxConcurrency:  b.maxConcurrency,
		FailureStrategy: failureStrategy,
	}

	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("parallel validation failed: %w", err)
	}

	return input, nil
}

// Build validates the configuration and returns a *job.Definition ready for
// registration with a Temporal worker and execution via the job registry.
//
// Required before calling Build:
//   - Name(...) — sets the job name
//   - One of Pipeline(), Parallel(), Single() — selects the execution mode
func (b *WorkflowBuilder) Build() (*job.Definition, error) {
	if b.name == "" {
		return nil, fmt.Errorf("container.WorkflowBuilder: Name is required")
	}
	if b.mode == modeUnset {
		return nil, fmt.Errorf("container.WorkflowBuilder: call .Pipeline()/.Parallel()/.Single() before Build")
	}

	tq := b.taskQueue
	if tq == "" {
		tq = "container-" + b.name
	}

	maxRunTimeout := time.Duration(0)
	for _, c := range b.containers {
		if c.RunTimeout > maxRunTimeout {
			maxRunTimeout = c.RunTimeout
		}
	}
	execOpts, err := resolveExecutionOptions(b.executionOptions, maxRunTimeout)
	if err != nil {
		b.errors = append(b.errors, err)
	}

	var wfType string
	var newInputFn func() any

	switch b.mode {
	case modePipeline:
		wfType = "ContainerPipelineWorkflow"
		if _, err := b.buildPipelineInput(); err != nil {
			return nil, err
		}
		containers := b.containers
		stopOnError := b.stopOnError
		cleanup := b.cleanup
		opts := execOpts
		newInputFn = func() any {
			return payload.PipelineInput{
				Containers:  containers,
				StopOnError: stopOnError,
				Cleanup:     cleanup,
				Options:     opts,
			}
		}

	case modeParallel:
		wfType = "ParallelContainersWorkflow"
		if _, err := b.buildParallelInput(); err != nil {
			return nil, err
		}
		failureStrategy := FailureStrategyContinue
		if b.failFast {
			failureStrategy = FailureStrategyFailFast
		}
		containers := b.containers
		maxConcurrency := b.maxConcurrency
		fs := failureStrategy
		opts := execOpts
		newInputFn = func() any {
			return payload.ParallelInput{
				Containers:      containers,
				MaxConcurrency:  maxConcurrency,
				FailureStrategy: fs,
				Options:         opts,
			}
		}

	case modeSingle:
		wfType = "ExecuteContainerWorkflow"
		if _, err := b.buildSingleInput(); err != nil {
			return nil, err
		}
		snapshot := b.containers[0]
		newInputFn = func() any {
			cp := snapshot
			return cp
		}
	}

	return job.New(b.name, tq,
		job.WithRegister(func(w worker.Worker) { container.RegisterAll(w) }),
		job.WithExecute(func(ctx context.Context, c client.Client, opts client.StartWorkflowOptions, in any) (client.WorkflowRun, error) {
			return c.ExecuteWorkflow(ctx, opts, wfType, in)
		}),
		job.WithNewInput(newInputFn),
	)
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

// LoopBuilder provides a fluent API for constructing loop workflow inputs
// and producing a *job.Definition ready for registration with a Temporal worker.
type LoopBuilder struct {
	name             string
	taskQueue        string
	mode             loopMode
	items            []string
	parameters       map[string][]string
	template         payload.ContainerExecutionInput
	parallel         bool
	maxConcurrency   int
	failFast         bool
	executionOptions *workflow.ExecutionOptions
	errors           []error
}

// NewLoopBuilder creates a new loop builder with the specified items.
func NewLoopBuilder(items []string) *LoopBuilder {
	return &LoopBuilder{
		items:    items,
		mode:     loopModeLoop,
		parallel: false,
		failFast: false,
	}
}

// NewParameterizedLoopBuilder creates a new parameterized loop builder.
func NewParameterizedLoopBuilder(parameters map[string][]string) *LoopBuilder {
	return &LoopBuilder{
		parameters: parameters,
		mode:       loopModeParameterizedLoop,
		parallel:   false,
		failFast:   false,
	}
}

// Name sets the job name. Required before calling Build.
func (lb *LoopBuilder) Name(n string) *LoopBuilder {
	lb.name = n
	return lb
}

// TaskQueue overrides the default task queue (which is "container-<name>").
func (lb *LoopBuilder) TaskQueue(tq string) *LoopBuilder {
	lb.taskQueue = tq
	return lb
}

// Loop selects simple item-iteration mode (default for NewLoopBuilder).
func (lb *LoopBuilder) Loop() *LoopBuilder {
	lb.mode = loopModeLoop
	return lb
}

// ParameterizedLoop selects parameter-combination mode (default for NewParameterizedLoopBuilder).
func (lb *LoopBuilder) ParameterizedLoop() *LoopBuilder {
	lb.mode = loopModeParameterizedLoop
	return lb
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

// WithExecutionOptions sets Temporal activity options for the built workflow.
// When nil and the container template sets RunTimeout, Build derives
// StartToCloseTimeout = RunTimeout + 2 minutes. Explicit options that leave
// StartToCloseTimeout zero (e.g. RetryPolicy-only) also get
// StartToCloseTimeout derived from RunTimeout; the caller's struct is not
// mutated.
func (lb *LoopBuilder) WithExecutionOptions(opts *workflow.ExecutionOptions) *LoopBuilder {
	lb.executionOptions = opts
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

// Build validates the configuration and returns a *job.Definition ready for
// registration with a Temporal worker and execution via the job registry.
//
// Required before calling Build:
//   - Name(...) — sets the job name
//   - The mode is inferred from the constructor (NewLoopBuilder → Loop,
//     NewParameterizedLoopBuilder → ParameterizedLoop). Override with .Loop()
//     or .ParameterizedLoop().
func (lb *LoopBuilder) Build() (*job.Definition, error) {
	if lb.name == "" {
		return nil, fmt.Errorf("container.LoopBuilder: Name is required")
	}
	if lb.mode == loopModeUnset {
		return nil, fmt.Errorf("container.LoopBuilder: call .Loop() or .ParameterizedLoop() before Build")
	}

	tq := lb.taskQueue
	if tq == "" {
		tq = "container-" + lb.name
	}

	execOpts, err := resolveExecutionOptions(lb.executionOptions, lb.template.RunTimeout)
	if err != nil {
		lb.errors = append(lb.errors, err)
	}

	var wfType string
	var newInputFn func() any

	switch lb.mode {
	case loopModeLoop:
		wfType = "LoopWorkflow"
		if _, err := lb.BuildLoop(); err != nil {
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
		opts := execOpts
		newInputFn = func() any {
			return payload.LoopInput{
				Items:           items,
				Template:        tmpl,
				Parallel:        parallel,
				MaxConcurrency:  maxConcurrency,
				FailureStrategy: fs,
				Options:         opts,
			}
		}

	case loopModeParameterizedLoop:
		wfType = "ParameterizedLoopWorkflow"
		if _, err := lb.BuildParameterizedLoop(); err != nil {
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
		opts := execOpts
		newInputFn = func() any {
			return payload.ParameterizedLoopInput{
				Parameters:      params,
				Template:        tmpl,
				Parallel:        parallel,
				MaxConcurrency:  maxConcurrency,
				FailureStrategy: fs,
				Options:         opts,
			}
		}
	}

	return job.New(lb.name, tq,
		job.WithRegister(func(w worker.Worker) { container.RegisterAll(w) }),
		job.WithExecute(func(ctx context.Context, c client.Client, opts client.StartWorkflowOptions, in any) (client.WorkflowRun, error) {
			return c.ExecuteWorkflow(ctx, opts, wfType, in)
		}),
		job.WithNewInput(newInputFn),
	)
}

// ForEach creates a loop builder for iterating over items.
func ForEach(items []string, template payload.ContainerExecutionInput) *LoopBuilder {
	return NewLoopBuilder(items).WithTemplate(template)
}

// ForEachParam creates a parameterized loop builder.
func ForEachParam(parameters map[string][]string, template payload.ContainerExecutionInput) *LoopBuilder {
	return NewParameterizedLoopBuilder(parameters).WithTemplate(template)
}
