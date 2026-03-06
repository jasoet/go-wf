package workflow

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
)

// testInput is a mock TaskInput for testing generic workflows.
type testInput struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Activity string `json:"activity"`
}

// Validate validates the test input.
func (t testInput) Validate() error {
	if t.Value == "" {
		return fmt.Errorf("value is required")
	}
	return nil
}

// ActivityName returns the activity name.
func (t testInput) ActivityName() string {
	if t.Activity != "" {
		return t.Activity
	}
	return "TestActivity"
}

// testOutput is a mock TaskOutput for testing generic workflows.
type testOutput struct {
	Result  string `json:"result"`
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

// IsSuccess returns whether the output indicates success.
func (t testOutput) IsSuccess() bool { return t.Success }

// GetError returns the error message.
func (t testOutput) GetError() string { return t.Error }

// stubTestActivity is a stub activity function used to register with the test environment.
func stubTestActivity(_ context.Context, _ testInput) (*testOutput, error) {
	return nil, nil
}

// registerTestActivity registers the stub activity with the test workflow environment.
func registerTestActivity(env *testsuite.TestWorkflowEnvironment) {
	env.RegisterActivityWithOptions(stubTestActivity, activity.RegisterOptions{Name: "TestActivity"})
}
