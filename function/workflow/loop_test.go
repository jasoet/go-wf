package workflow

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"

	"github.com/jasoet/go-wf/function/payload"
)

func TestLoopWorkflow_Success(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	input := payload.LoopInput{
		Items:    []string{"a", "b", "c"},
		Template: payload.FunctionExecutionInput{Name: "process-{{item}}"},
	}

	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.Anything).Return(&payload.FunctionExecutionOutput{
		Success:  true,
		Duration: 1 * time.Second,
	}, nil)

	env.ExecuteWorkflow(LoopWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.LoopOutput
	require.NoError(t, env.GetWorkflowResult(&result))

	assert.Equal(t, 3, result.TotalSuccess)
	assert.Equal(t, 3, result.ItemCount)
}

func TestParameterizedLoopWorkflow_Success(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	input := payload.ParameterizedLoopInput{
		Parameters: map[string][]string{
			"env": {"dev", "prod"},
		},
		Template: payload.FunctionExecutionInput{
			Name: "deploy",
			Args: map[string]string{"target": "{{.env}}"},
		},
	}

	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.Anything).Return(&payload.FunctionExecutionOutput{
		Success:  true,
		Duration: 1 * time.Second,
	}, nil)

	env.ExecuteWorkflow(ParameterizedLoopWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.LoopOutput
	require.NoError(t, env.GetWorkflowResult(&result))

	assert.Equal(t, 2, result.TotalSuccess)
	assert.Equal(t, 2, result.ItemCount)
}

func TestLoopWorkflow_Sequential(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.Anything).Return(
		&payload.FunctionExecutionOutput{Success: true, Duration: 1 * time.Second}, nil)

	input := payload.LoopInput{
		Items:           []string{"step1", "step2"},
		Template:        payload.FunctionExecutionInput{Name: "process-{{item}}"},
		Parallel:        false,
		FailureStrategy: "fail_fast",
	}

	env.ExecuteWorkflow(LoopWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.LoopOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 2, result.ItemCount)
	assert.Equal(t, 2, result.TotalSuccess)
}

func TestLoopWorkflow_SequentialFailFast(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	callCount := 0
	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.Anything).Return(
		func(_ context.Context, _ payload.FunctionExecutionInput) (*payload.FunctionExecutionOutput, error) {
			callCount++
			if callCount == 2 {
				return &payload.FunctionExecutionOutput{Success: false, Error: "failed"}, nil
			}
			return &payload.FunctionExecutionOutput{Success: true}, nil
		})

	input := payload.LoopInput{
		Items:           []string{"a", "b", "c"},
		Template:        payload.FunctionExecutionInput{Name: "process-{{item}}"},
		Parallel:        false,
		FailureStrategy: "fail_fast",
	}

	env.ExecuteWorkflow(LoopWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.Error(t, env.GetWorkflowError())
	assert.Equal(t, 2, callCount, "third item should not execute")
}

func TestLoopWorkflow_SequentialContinueOnFailure(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	callCount := 0
	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.Anything).Return(
		func(_ context.Context, _ payload.FunctionExecutionInput) (*payload.FunctionExecutionOutput, error) {
			callCount++
			if callCount == 2 {
				return &payload.FunctionExecutionOutput{Success: false, Error: "failed"}, nil
			}
			return &payload.FunctionExecutionOutput{Success: true}, nil
		})

	input := payload.LoopInput{
		Items:           []string{"a", "b", "c"},
		Template:        payload.FunctionExecutionInput{Name: "process-{{item}}"},
		Parallel:        false,
		FailureStrategy: "continue",
	}

	env.ExecuteWorkflow(LoopWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.LoopOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 2, result.TotalSuccess)
	assert.Equal(t, 1, result.TotalFailed)
	assert.Len(t, result.Results, 3)
}

func TestLoopWorkflow_ParallelFailFast(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	var callCount atomic.Int32
	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.Anything).Return(
		func(_ context.Context, _ payload.FunctionExecutionInput) (*payload.FunctionExecutionOutput, error) {
			c := callCount.Add(1)
			if c == 2 {
				return &payload.FunctionExecutionOutput{Success: false, Error: "failed"}, nil
			}
			return &payload.FunctionExecutionOutput{Success: true}, nil
		})

	input := payload.LoopInput{
		Items:           []string{"a", "b", "c"},
		Template:        payload.FunctionExecutionInput{Name: "process-{{item}}"},
		Parallel:        true,
		FailureStrategy: "fail_fast",
	}

	env.ExecuteWorkflow(LoopWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.Error(t, env.GetWorkflowError())
}

func TestLoopWorkflow_ParallelContinueMultipleFailures(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	var callCount atomic.Int32
	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.Anything).Return(
		func(_ context.Context, _ payload.FunctionExecutionInput) (*payload.FunctionExecutionOutput, error) {
			c := callCount.Add(1)
			if c == 2 || c == 4 {
				return &payload.FunctionExecutionOutput{Success: false, Error: "failed"}, nil
			}
			return &payload.FunctionExecutionOutput{Success: true}, nil
		})

	input := payload.LoopInput{
		Items:           []string{"a", "b", "c", "d"},
		Template:        payload.FunctionExecutionInput{Name: "process-{{item}}"},
		Parallel:        true,
		FailureStrategy: "continue",
	}

	env.ExecuteWorkflow(LoopWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.LoopOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 2, result.TotalSuccess)
	assert.Equal(t, 2, result.TotalFailed)
	assert.Len(t, result.Results, 4)
}

func TestLoopWorkflow_AllFailContinue(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.Anything).Return(
		&payload.FunctionExecutionOutput{Success: false, Error: "failed"}, nil)

	input := payload.LoopInput{
		Items:           []string{"a", "b", "c"},
		Template:        payload.FunctionExecutionInput{Name: "process-{{item}}"},
		Parallel:        false,
		FailureStrategy: "continue",
	}

	env.ExecuteWorkflow(LoopWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.LoopOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 3, result.TotalFailed)
	assert.Equal(t, 0, result.TotalSuccess)
}

func TestLoopWorkflow_SingleItem(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.Anything).Return(
		&payload.FunctionExecutionOutput{Success: true}, nil)

	input := payload.LoopInput{
		Items:           []string{"only"},
		Template:        payload.FunctionExecutionInput{Name: "process-{{item}}"},
		Parallel:        false,
		FailureStrategy: "continue",
	}

	env.ExecuteWorkflow(LoopWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.LoopOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 1, result.ItemCount)
	assert.Equal(t, 1, result.TotalSuccess)
}

func TestParameterizedLoopWorkflow_Sequential(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.Anything).Return(
		&payload.FunctionExecutionOutput{Success: true, Duration: 1 * time.Second}, nil)

	input := payload.ParameterizedLoopInput{
		Parameters: map[string][]string{
			"version": {"1.0", "2.0"},
		},
		Template: payload.FunctionExecutionInput{
			Name: "build",
			Args: map[string]string{"version": "{{.version}}"},
		},
		Parallel:        false,
		FailureStrategy: "continue",
	}

	env.ExecuteWorkflow(ParameterizedLoopWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.LoopOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 2, result.ItemCount)
	assert.Equal(t, 2, result.TotalSuccess)
}

func TestParameterizedLoopWorkflow_FailFast(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	callCount := 0
	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.Anything).Return(
		func(_ context.Context, _ payload.FunctionExecutionInput) (*payload.FunctionExecutionOutput, error) {
			callCount++
			if callCount == 2 {
				return &payload.FunctionExecutionOutput{Success: false, Error: "failed"}, nil
			}
			return &payload.FunctionExecutionOutput{Success: true}, nil
		})

	input := payload.ParameterizedLoopInput{
		Parameters: map[string][]string{
			"env": {"dev", "prod"},
		},
		Template: payload.FunctionExecutionInput{
			Name: "deploy",
			Args: map[string]string{"target": "{{.env}}"},
		},
		Parallel:        false,
		FailureStrategy: "fail_fast",
	}

	env.ExecuteWorkflow(ParameterizedLoopWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.Error(t, env.GetWorkflowError())
}

func TestParameterizedLoopWorkflow_ContinueWithFailures(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	var callCount atomic.Int32
	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.Anything).Return(
		func(_ context.Context, _ payload.FunctionExecutionInput) (*payload.FunctionExecutionOutput, error) {
			c := callCount.Add(1)
			if c%2 == 0 {
				return &payload.FunctionExecutionOutput{Success: false, Error: "failed"}, nil
			}
			return &payload.FunctionExecutionOutput{Success: true}, nil
		})

	input := payload.ParameterizedLoopInput{
		Parameters: map[string][]string{
			"env":    {"dev", "prod"},
			"region": {"us", "eu"},
		},
		Template: payload.FunctionExecutionInput{
			Name: "deploy",
			Args: map[string]string{"target": "{{.env}}", "region": "{{.region}}"},
		},
		Parallel:        true,
		FailureStrategy: "continue",
	}

	env.ExecuteWorkflow(ParameterizedLoopWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.LoopOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 4, result.ItemCount)
	assert.Equal(t, 2, result.TotalSuccess)
	assert.Equal(t, 2, result.TotalFailed)
}

func TestSubstituteFunctionInput(t *testing.T) {
	tests := []struct {
		name     string
		template payload.FunctionExecutionInput
		item     string
		index    int
		params   map[string]string
		validate func(*testing.T, payload.FunctionExecutionInput)
	}{
		{
			name: "substitute in name",
			template: payload.FunctionExecutionInput{
				Name: "process-{{item}}",
			},
			item:   "data",
			index:  0,
			params: nil,
			validate: func(t *testing.T, result payload.FunctionExecutionInput) {
				assert.Equal(t, "process-data", result.Name)
			},
		},
		{
			name: "substitute in args",
			template: payload.FunctionExecutionInput{
				Name: "deploy",
				Args: map[string]string{
					"target":   "{{item}}",
					"index":    "{{index}}",
					"{{item}}": "value-{{index}}",
				},
			},
			item:   "prod",
			index:  3,
			params: nil,
			validate: func(t *testing.T, result payload.FunctionExecutionInput) {
				assert.Equal(t, "prod", result.Args["target"])
				assert.Equal(t, "3", result.Args["index"])
				assert.Equal(t, "value-3", result.Args["prod"])
			},
		},
		{
			name: "substitute in env",
			template: payload.FunctionExecutionInput{
				Name: "run",
				Env: map[string]string{
					"ITEM":  "{{item}}",
					"INDEX": "{{index}}",
					"ENV":   "{{.env}}",
				},
			},
			item:   "file.csv",
			index:  2,
			params: map[string]string{"env": "production"},
			validate: func(t *testing.T, result payload.FunctionExecutionInput) {
				assert.Equal(t, "file.csv", result.Env["ITEM"])
				assert.Equal(t, "2", result.Env["INDEX"])
				assert.Equal(t, "production", result.Env["ENV"])
			},
		},
		{
			name: "substitute in workdir",
			template: payload.FunctionExecutionInput{
				Name:    "build",
				WorkDir: "/app/{{item}}",
			},
			item:   "myproject",
			index:  0,
			params: nil,
			validate: func(t *testing.T, result payload.FunctionExecutionInput) {
				assert.Equal(t, "/app/myproject", result.WorkDir)
			},
		},
		{
			name: "no substitution needed",
			template: payload.FunctionExecutionInput{
				Name: "simple",
			},
			item:   "",
			index:  0,
			params: nil,
			validate: func(t *testing.T, result payload.FunctionExecutionInput) {
				assert.Equal(t, "simple", result.Name)
			},
		},
		{
			name: "substitute with params dot syntax",
			template: payload.FunctionExecutionInput{
				Name: "deploy-{{.env}}-{{.region}}",
				Args: map[string]string{"target": "{{.env}}"},
			},
			item:   "",
			index:  0,
			params: map[string]string{"env": "prod", "region": "us-west"},
			validate: func(t *testing.T, result payload.FunctionExecutionInput) {
				assert.Equal(t, "deploy-prod-us-west", result.Name)
				assert.Equal(t, "prod", result.Args["target"])
			},
		},
		{
			name: "empty workdir not substituted",
			template: payload.FunctionExecutionInput{
				Name:    "run",
				WorkDir: "",
			},
			item:   "test",
			index:  0,
			params: nil,
			validate: func(t *testing.T, result payload.FunctionExecutionInput) {
				assert.Equal(t, "", result.WorkDir)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := substituteFunctionInput(tt.template, tt.item, tt.index, tt.params)
			tt.validate(t, result)
		})
	}
}
