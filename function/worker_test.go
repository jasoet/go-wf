package function

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.temporal.io/sdk/testsuite"
)

func TestRegisterWorkflows(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	assert.NotPanics(t, func() {
		RegisterWorkflows(env)
	})
}

func TestRegisterActivities(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registry := NewRegistry()

	assert.NotPanics(t, func() {
		RegisterActivities(env, registry)
	})
}

func TestRegisterAll(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registry := NewRegistry()

	assert.NotPanics(t, func() {
		RegisterAll(env, registry)
	})
}
