package activity

import (
	"context"
	"time"

	fn "github.com/jasoet/go-wf/function"
	"github.com/jasoet/go-wf/function/payload"
)

// NewExecuteFunctionActivity creates a Temporal activity that dispatches to registered handlers.
//
// Error handling semantics:
//   - Validation errors and registry lookup failures return an error, causing Temporal retries.
//   - Handler execution errors are captured in the output (Success=false, Error set) but return nil
//     error, so Temporal does NOT retry. This treats handler failures as business logic results.
func NewExecuteFunctionActivity(registry *fn.Registry) func(ctx context.Context, input payload.FunctionExecutionInput) (*payload.FunctionExecutionOutput, error) {
	return func(ctx context.Context, input payload.FunctionExecutionInput) (*payload.FunctionExecutionOutput, error) {
		startTime := time.Now()

		// Validate input
		if err := input.Validate(); err != nil {
			return &payload.FunctionExecutionOutput{
				Name:       input.Name,
				StartedAt:  startTime,
				FinishedAt: time.Now(),
				Success:    false,
				Error:      err.Error(),
			}, err
		}

		// Look up handler
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

		// Build function input from payload
		fnInput := fn.FunctionInput{
			Args:    input.Args,
			Data:    input.Data,
			Env:     input.Env,
			WorkDir: input.WorkDir,
		}

		// Call handler
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
}
