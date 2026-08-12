package function

import (
	"context"
	"sync"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	"github.com/jasoet/pkg/v2/temporal/job"

	wf "github.com/jasoet/go-wf/v2/function/workflow"
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

// RegisterWorkflows registers all function workflows with a worker.
// Calling this function multiple times on the same worker is a no-op for
// subsequent calls — each workflow type is registered at most once per worker.
func RegisterWorkflows(w worker.Worker) {
	job.RegisterWorkflowOnce(w, "ExecuteFunctionWorkflow", wf.ExecuteFunctionWorkflow, workflow.RegisterOptions{Name: "ExecuteFunctionWorkflow"})
	job.RegisterWorkflowOnce(w, "FunctionPipelineWorkflow", wf.FunctionPipelineWorkflow, workflow.RegisterOptions{Name: "FunctionPipelineWorkflow"})
	job.RegisterWorkflowOnce(w, "ParallelFunctionsWorkflow", wf.ParallelFunctionsWorkflow, workflow.RegisterOptions{Name: "ParallelFunctionsWorkflow"})
	job.RegisterWorkflowOnce(w, "LoopWorkflow", wf.LoopWorkflow, workflow.RegisterOptions{Name: "LoopWorkflow"})
	job.RegisterWorkflowOnce(w, "ParameterizedLoopWorkflow", wf.ParameterizedLoopWorkflow, workflow.RegisterOptions{Name: "ParameterizedLoopWorkflow"})
	job.RegisterWorkflowOnce(w, "InstrumentedDAGWorkflow", wf.InstrumentedDAGWorkflow, workflow.RegisterOptions{Name: "InstrumentedDAGWorkflow"})
}

// RegisterActivity registers a function execution activity with a worker.
// Create the activity with activity.NewExecuteFunctionActivity(registry).
// Calling this function multiple times on the same worker is a no-op for
// subsequent calls — the activity type is registered at most once per worker.
func RegisterActivity(w worker.Worker, activityFn interface{}) {
	if typed, ok := activityFn.(func(context.Context, FunctionExecutionInput) (*FunctionExecutionOutput, error)); ok {
		if instrumentActivity != nil {
			activityFn = instrumentActivity(typed)
		}
	}
	job.RegisterActivityOnce(w, "ExecuteFunctionActivity", activityFn, activity.RegisterOptions{
		Name: "ExecuteFunctionActivity",
	})
}

// RegisterAll registers all function workflows and the given activity with a worker.
// Create the activity with activity.NewExecuteFunctionActivity(registry).
// Calling this function multiple times on the same worker is a no-op for
// subsequent calls — idempotency is ensured via job.RegisterWorkflowOnce and
// job.RegisterActivityOnce.
func RegisterAll(w worker.Worker, activityFn interface{}) {
	RegisterWorkflows(w)
	RegisterActivity(w, activityFn)
}
