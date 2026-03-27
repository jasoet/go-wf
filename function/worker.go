package function

import (
	"context"
	"sync"

	"go.temporal.io/sdk/activity"

	wf "github.com/jasoet/go-wf/function/workflow"
)

// activityType is the function signature for the function execution activity.
type activityType = func(context.Context, FunctionExecutionInput) (*FunctionExecutionOutput, error)

var (
	// instrumentActivity is a hook that function/activity can set to provide OTel instrumentation.
	// When non-nil, RegisterActivity uses it to wrap the activity function.
	instrumentActivity func(activityType) activityType

	setInstrumenterOnce sync.Once
)

// SetActivityInstrumenter sets the function that wraps activities with instrumentation.
// This must only be called once during initialization; subsequent calls are ignored.
func SetActivityInstrumenter(wrapper func(activityType) activityType) {
	setInstrumenterOnce.Do(func() {
		instrumentActivity = wrapper
	})
}

// WorkflowRegistrar is the interface for registering workflows and activities.
type WorkflowRegistrar interface {
	RegisterWorkflow(w interface{})
	RegisterActivityWithOptions(a interface{}, options activity.RegisterOptions)
}

// RegisterWorkflows registers all function workflows.
func RegisterWorkflows(w WorkflowRegistrar) {
	w.RegisterWorkflow(wf.ExecuteFunctionWorkflow)
	w.RegisterWorkflow(wf.FunctionPipelineWorkflow)
	w.RegisterWorkflow(wf.ParallelFunctionsWorkflow)
	w.RegisterWorkflow(wf.LoopWorkflow)
	w.RegisterWorkflow(wf.ParameterizedLoopWorkflow)
	w.RegisterWorkflow(wf.InstrumentedDAGWorkflow)
}

// RegisterActivity registers a function execution activity.
// Create the activity with activity.NewExecuteFunctionActivity(registry).
func RegisterActivity(w WorkflowRegistrar, activityFn interface{}) {
	if typed, ok := activityFn.(func(context.Context, FunctionExecutionInput) (*FunctionExecutionOutput, error)); ok {
		if instrumentActivity != nil {
			activityFn = instrumentActivity(typed)
		}
	}
	w.RegisterActivityWithOptions(activityFn, activity.RegisterOptions{
		Name: "ExecuteFunctionActivity",
	})
}

// RegisterAll registers all function workflows and the given activity.
// Create the activity with activity.NewExecuteFunctionActivity(registry).
func RegisterAll(w WorkflowRegistrar, activityFn interface{}) {
	RegisterWorkflows(w)
	RegisterActivity(w, activityFn)
}
