package function

import (
	"context"

	"go.temporal.io/sdk/activity"

	"github.com/jasoet/go-wf/function/payload"
	wf "github.com/jasoet/go-wf/function/workflow"
)

// activityType is the function signature for the function execution activity.
type activityType = func(context.Context, payload.FunctionExecutionInput) (*payload.FunctionExecutionOutput, error)

// instrumentActivity is a hook that function/activity can set to provide OTel instrumentation.
// When non-nil, RegisterActivity uses it to wrap the activity function.
var instrumentActivity func(activityType) activityType

// SetActivityInstrumenter sets the function used to wrap activity functions with instrumentation.
// This must only be called during package init(), not at runtime.
// It is called by function/activity's init() to register OTel instrumentation.
func SetActivityInstrumenter(wrapper func(activityType) activityType) {
	instrumentActivity = wrapper
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
}

// RegisterActivity registers a function execution activity.
// Create the activity with activity.NewExecuteFunctionActivity(registry).
func RegisterActivity(w WorkflowRegistrar, activityFn interface{}) {
	if typed, ok := activityFn.(func(context.Context, payload.FunctionExecutionInput) (*payload.FunctionExecutionOutput, error)); ok {
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
