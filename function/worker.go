package function

import (
	"context"
	"time"

	"go.temporal.io/sdk/activity"

	"github.com/jasoet/go-wf/function/payload"
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

// RegisterActivities registers the function execution activity.
func RegisterActivities(w WorkflowRegistrar, registry *Registry) {
	executeFn := func(ctx context.Context, input payload.FunctionExecutionInput) (*payload.FunctionExecutionOutput, error) {
		startTime := time.Now()

		if err := input.Validate(); err != nil {
			return &payload.FunctionExecutionOutput{
				Name:       input.Name,
				StartedAt:  startTime,
				FinishedAt: time.Now(),
				Success:    false,
				Error:      err.Error(),
			}, err
		}

		handler, err := registry.Get(input.Name)
		if err != nil {
			return &payload.FunctionExecutionOutput{
				Name:       input.Name,
				StartedAt:  startTime,
				FinishedAt: time.Now(),
				Success:    false,
				Error:      err.Error(),
			}, err
		}

		fnInput := FunctionInput{
			Args:    input.Args,
			Data:    input.Data,
			Env:     input.Env,
			WorkDir: input.WorkDir,
		}

		fnOutput, handlerErr := handler(ctx, fnInput)
		finishTime := time.Now()

		output := &payload.FunctionExecutionOutput{
			Name:       input.Name,
			StartedAt:  startTime,
			FinishedAt: finishTime,
			Duration:   finishTime.Sub(startTime),
		}

		if handlerErr != nil {
			output.Success = false
			output.Error = handlerErr.Error()
			return output, nil
		}

		output.Success = true
		if fnOutput != nil {
			output.Result = fnOutput.Result
			output.Data = fnOutput.Data
		}

		return output, nil
	}

	w.RegisterActivityWithOptions(executeFn, activity.RegisterOptions{
		Name: "ExecuteFunctionActivity",
	})
}

// RegisterAll registers both workflows and activities.
func RegisterAll(w WorkflowRegistrar, registry *Registry) {
	RegisterWorkflows(w)
	RegisterActivities(w, registry)
}
