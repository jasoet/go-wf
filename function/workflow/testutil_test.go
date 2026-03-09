package workflow

import (
	"context"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"

	"github.com/jasoet/go-wf/function/payload"
)

// stubExecuteFunctionActivity is a stub for test registration.
func stubExecuteFunctionActivity(_ context.Context, _ payload.FunctionExecutionInput) (*payload.FunctionExecutionOutput, error) {
	return nil, nil
}

// registerFunctionActivity registers the stub activity with the test environment.
func registerFunctionActivity(env *testsuite.TestWorkflowEnvironment) {
	env.RegisterActivityWithOptions(stubExecuteFunctionActivity, activity.RegisterOptions{Name: "ExecuteFunctionActivity"})
}
