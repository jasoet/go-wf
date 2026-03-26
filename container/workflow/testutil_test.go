package workflow

import (
	"context"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"

	"github.com/jasoet/go-wf/container/payload"
)

// stubStartContainerActivity is a stub activity function used to register with the test environment.
func stubStartContainerActivity(_ context.Context, _ payload.ContainerExecutionInput) (*payload.ContainerExecutionOutput, error) {
	return nil, nil
}

// registerContainerActivity registers the stub StartContainerActivity with the test workflow environment.
func registerContainerActivity(env *testsuite.TestWorkflowEnvironment) {
	env.RegisterActivityWithOptions(stubStartContainerActivity, activity.RegisterOptions{Name: "StartContainerActivity"})
}
