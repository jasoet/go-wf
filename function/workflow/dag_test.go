package workflow

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"

	"github.com/jasoet/go-wf/function/payload"
)

func TestDAGWorkflow_Success(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.Anything).Return(
		&payload.FunctionExecutionOutput{
			Name:     "test-func",
			Success:  true,
			Result:   map[string]string{"status": "ok"},
			Duration: 1 * time.Second,
		}, nil)

	input := payload.DAGWorkflowInput{
		Nodes: []payload.FunctionDAGNode{
			{
				Name:     "build",
				Function: payload.FunctionExecutionInput{Name: "build-func", Args: map[string]string{"target": "all"}},
			},
			{
				Name:         "test",
				Function:     payload.FunctionExecutionInput{Name: "test-func"},
				Dependencies: []string{"build"},
			},
		},
		FailFast: false,
	}

	env.ExecuteWorkflow(DAGWorkflow, input)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.FunctionDAGWorkflowOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 2, result.TotalSuccess)
	assert.Equal(t, 0, result.TotalFailed)
}

func TestDAGWorkflow_ValidationError(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := payload.DAGWorkflowInput{
		Nodes: []payload.FunctionDAGNode{},
	}

	env.ExecuteWorkflow(DAGWorkflow, input)
	require.True(t, env.IsWorkflowCompleted())
	assert.Error(t, env.GetWorkflowError())
}

func TestDAGWorkflow_FailFast(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.Anything).Return(
		&payload.FunctionExecutionOutput{
			Name:     "fail-func",
			Success:  false,
			Error:    "execution failed",
			Duration: 1 * time.Second,
		}, nil).Once()

	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.Anything).Return(
		&payload.FunctionExecutionOutput{
			Name:     "ok-func",
			Success:  true,
			Duration: 1 * time.Second,
		}, nil)

	input := payload.DAGWorkflowInput{
		Nodes: []payload.FunctionDAGNode{
			{
				Name:     "first",
				Function: payload.FunctionExecutionInput{Name: "fail-func"},
			},
			{
				Name:         "second",
				Function:     payload.FunctionExecutionInput{Name: "ok-func"},
				Dependencies: []string{"first"},
			},
		},
		FailFast: true,
	}

	env.ExecuteWorkflow(DAGWorkflow, input)
	require.True(t, env.IsWorkflowCompleted())
	assert.Error(t, env.GetWorkflowError())
}

func TestDAGWorkflow_DiamondDependency(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.Anything).Return(
		&payload.FunctionExecutionOutput{
			Name:     "diamond-func",
			Success:  true,
			Result:   map[string]string{"status": "ok"},
			Duration: 1 * time.Second,
		}, nil)

	input := payload.DAGWorkflowInput{
		Nodes: []payload.FunctionDAGNode{
			{Name: "A", Function: payload.FunctionExecutionInput{Name: "func-a"}},
			{Name: "B", Function: payload.FunctionExecutionInput{Name: "func-b"}, Dependencies: []string{"A"}},
			{Name: "C", Function: payload.FunctionExecutionInput{Name: "func-c"}, Dependencies: []string{"A"}},
			{Name: "D", Function: payload.FunctionExecutionInput{Name: "func-d"}, Dependencies: []string{"B", "C"}},
		},
		FailFast: false,
	}

	env.ExecuteWorkflow(DAGWorkflow, input)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.FunctionDAGWorkflowOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 4, result.TotalSuccess)
	assert.Equal(t, 0, result.TotalFailed)
}

func TestDAGWorkflow_AlreadyExecutedGuard(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	callCount := 0
	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.Anything).Return(
		func(_ context.Context, _ payload.FunctionExecutionInput) (*payload.FunctionExecutionOutput, error) {
			callCount++
			return &payload.FunctionExecutionOutput{
				Name:     fmt.Sprintf("func-%d", callCount),
				Success:  true,
				Duration: 1 * time.Second,
			}, nil
		})

	// A is depended on by both B and C. A should execute only once.
	input := payload.DAGWorkflowInput{
		Nodes: []payload.FunctionDAGNode{
			{Name: "A", Function: payload.FunctionExecutionInput{Name: "func-a"}},
			{Name: "B", Function: payload.FunctionExecutionInput{Name: "func-b"}, Dependencies: []string{"A"}},
			{Name: "C", Function: payload.FunctionExecutionInput{Name: "func-c"}, Dependencies: []string{"A"}},
		},
		FailFast: false,
	}

	env.ExecuteWorkflow(DAGWorkflow, input)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.FunctionDAGWorkflowOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 3, callCount, "activity should be called exactly 3 times (A once, B once, C once)")
	assert.Equal(t, 3, result.TotalSuccess)
}

func TestDAGWorkflow_InputMapping(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	// Build node returns result with version key.
	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.MatchedBy(func(input payload.FunctionExecutionInput) bool {
		return input.Name == "build-func"
	})).Return(
		&payload.FunctionExecutionOutput{
			Name:     "build-func",
			Success:  true,
			Result:   map[string]string{"version": "1.2.3", "artifact": "app.tar.gz"},
			Duration: 1 * time.Second,
		}, nil)

	// Deploy node receives the version via input mapping in args.
	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.MatchedBy(func(input payload.FunctionExecutionInput) bool {
		return input.Name == "deploy-func"
	})).Return(
		&payload.FunctionExecutionOutput{
			Name:     "deploy-func",
			Success:  true,
			Duration: 1 * time.Second,
		}, nil)

	input := payload.DAGWorkflowInput{
		Nodes: []payload.FunctionDAGNode{
			{
				Name:     "build",
				Function: payload.FunctionExecutionInput{Name: "build-func"},
				Outputs: []payload.OutputMapping{
					{Name: "version", ResultKey: "version"},
					{Name: "artifact", ResultKey: "artifact"},
				},
			},
			{
				Name:     "deploy",
				Function: payload.FunctionExecutionInput{Name: "deploy-func"},
				Inputs: []payload.FunctionInputMapping{
					{Name: "BUILD_VERSION", From: "build.version", Required: true},
				},
				Dependencies: []string{"build"},
			},
		},
		FailFast: false,
	}

	env.ExecuteWorkflow(DAGWorkflow, input)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.FunctionDAGWorkflowOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 2, result.TotalSuccess)
	assert.Equal(t, "1.2.3", result.StepOutputs["build"]["version"])
	assert.Equal(t, "app.tar.gz", result.StepOutputs["build"]["artifact"])
}

func TestDAGWorkflow_DataMapping(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	extractedData := []byte(`{"key": "value", "count": 42}`)

	// Extract node returns data bytes.
	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.MatchedBy(func(input payload.FunctionExecutionInput) bool {
		return input.Name == "extract-func"
	})).Return(
		&payload.FunctionExecutionOutput{
			Name:     "extract-func",
			Success:  true,
			Data:     extractedData,
			Duration: 1 * time.Second,
		}, nil)

	// Transform node receives data from extract via data mapping.
	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.MatchedBy(func(input payload.FunctionExecutionInput) bool {
		return input.Name == "transform-func"
	})).Return(
		&payload.FunctionExecutionOutput{
			Name:     "transform-func",
			Success:  true,
			Duration: 1 * time.Second,
		}, nil)

	input := payload.DAGWorkflowInput{
		Nodes: []payload.FunctionDAGNode{
			{
				Name:     "extract",
				Function: payload.FunctionExecutionInput{Name: "extract-func"},
			},
			{
				Name:     "transform",
				Function: payload.FunctionExecutionInput{Name: "transform-func"},
				DataInput: &payload.DataMapping{
					FromNode: "extract",
				},
				Dependencies: []string{"extract"},
			},
		},
		FailFast: false,
	}

	env.ExecuteWorkflow(DAGWorkflow, input)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.FunctionDAGWorkflowOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 2, result.TotalSuccess)
}

func TestDAGWorkflow_ActivityError(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.Anything).Return(
		nil, fmt.Errorf("function runtime error"))

	input := payload.DAGWorkflowInput{
		Nodes: []payload.FunctionDAGNode{
			{
				Name:     "broken",
				Function: payload.FunctionExecutionInput{Name: "broken-func"},
			},
		},
		FailFast: true,
	}

	env.ExecuteWorkflow(DAGWorkflow, input)
	require.True(t, env.IsWorkflowCompleted())
	assert.Error(t, env.GetWorkflowError())
}

func TestDAGWorkflow_ContinueOnFailure(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.MatchedBy(func(input payload.FunctionExecutionInput) bool {
		return input.Name == "fail-func"
	})).Return(
		&payload.FunctionExecutionOutput{
			Name:     "fail-func",
			Success:  false,
			Error:    "something went wrong",
			Duration: 1 * time.Second,
		}, nil).Once()

	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.MatchedBy(func(input payload.FunctionExecutionInput) bool {
		return input.Name == "pass-func"
	})).Return(
		&payload.FunctionExecutionOutput{
			Name:     "pass-func",
			Success:  true,
			Duration: 1 * time.Second,
		}, nil).Once()

	input := payload.DAGWorkflowInput{
		Nodes: []payload.FunctionDAGNode{
			{
				Name:     "failing",
				Function: payload.FunctionExecutionInput{Name: "fail-func"},
			},
			{
				Name:     "passing",
				Function: payload.FunctionExecutionInput{Name: "pass-func"},
			},
		},
		FailFast: false,
	}

	env.ExecuteWorkflow(DAGWorkflow, input)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.FunctionDAGWorkflowOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 1, result.TotalSuccess)
	assert.Equal(t, 1, result.TotalFailed)
}
