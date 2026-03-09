package function

import (
	"go.temporal.io/sdk/activity"

	wf "github.com/jasoet/go-wf/function/workflow"
)

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
	w.RegisterActivityWithOptions(activityFn, activity.RegisterOptions{
		Name: "ExecuteFunctionActivity",
	})
}
