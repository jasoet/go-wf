package function

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.temporal.io/sdk/testsuite"

	"github.com/jasoet/go-wf/function/payload"
)

func TestRegisterWorkflows(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	assert.NotPanics(t, func() {
		RegisterWorkflows(env)
	})
}

func TestRegisterActivity(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Use a stub activity function since importing function/activity would create a cycle.
	stubActivity := func(_ context.Context, _ payload.FunctionExecutionInput) (*payload.FunctionExecutionOutput, error) {
		return nil, nil
	}

	assert.NotPanics(t, func() {
		RegisterActivity(env, stubActivity)
	})
}
