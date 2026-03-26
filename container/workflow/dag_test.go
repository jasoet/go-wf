package workflow

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"

	"github.com/jasoet/go-wf/container/payload"
)

func TestDAGWorkflow(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerContainerActivity(env)

	// Mock the activity
	env.OnActivity("StartContainerActivity", mock.Anything, mock.Anything).Return(
		&payload.ContainerExecutionOutput{
			ContainerID: "container-123",
			ExitCode:    0,
			Success:     true,
			Duration:    1 * time.Second,
			StartedAt:   time.Now(),
			FinishedAt:  time.Now().Add(1 * time.Second),
		}, nil)

	input := payload.DAGWorkflowInput{
		Nodes: []payload.DAGNode{
			{
				Name: "build",
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{
						Image:   "golang:1.25",
						Command: []string{"go", "build"},
					},
				},
			},
			{
				Name: "test",
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{
						Image:   "golang:1.25",
						Command: []string{"go", "test"},
					},
				},
				Dependencies: []string{"build"},
			},
		},
		FailFast: false,
	}

	env.ExecuteWorkflow(DAGWorkflow, input)
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())
}

func TestDAGWorkflowValidation(t *testing.T) {
	tests := []struct {
		name        string
		input       payload.DAGWorkflowInput
		expectError bool
	}{
		{
			name: "valid DAG",
			input: payload.DAGWorkflowInput{
				Nodes: []payload.DAGNode{
					{
						Name: "task1",
						Container: payload.ExtendedContainerInput{
							ContainerExecutionInput: payload.ContainerExecutionInput{
								Image: "alpine:latest",
							},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "empty nodes",
			input: payload.DAGWorkflowInput{
				Nodes: []payload.DAGNode{},
			},
			expectError: true,
		},
		{
			name: "missing dependency",
			input: payload.DAGWorkflowInput{
				Nodes: []payload.DAGNode{
					{
						Name: "task1",
						Container: payload.ExtendedContainerInput{
							ContainerExecutionInput: payload.ContainerExecutionInput{
								Image: "alpine:latest",
							},
						},
						Dependencies: []string{"nonexistent"},
					},
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDAGWorkflowFailFast(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerContainerActivity(env)

	// Mock activity to return failure for first call, success for second
	env.OnActivity("StartContainerActivity", mock.Anything, mock.Anything).
		Return(&payload.ContainerExecutionOutput{
			ContainerID: "container-fail",
			ExitCode:    1,
			Success:     false,
			Duration:    1 * time.Second,
		}, nil).Once()

	env.OnActivity("StartContainerActivity", mock.Anything, mock.Anything).
		Return(&payload.ContainerExecutionOutput{
			ContainerID: "container-ok",
			ExitCode:    0,
			Success:     true,
			Duration:    1 * time.Second,
		}, nil)

	input := payload.DAGWorkflowInput{
		Nodes: []payload.DAGNode{
			{
				Name: "task1",
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{
						Image: "alpine:latest",
					},
				},
			},
			{
				Name: "task2",
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{
						Image: "alpine:latest",
					},
				},
				Dependencies: []string{"task1"},
			},
		},
		FailFast: true,
	}

	env.ExecuteWorkflow(DAGWorkflow, input)
	assert.True(t, env.IsWorkflowCompleted())
	// Should have error due to fail-fast
	assert.Error(t, env.GetWorkflowError())
}

func TestWorkflowWithParameters(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerContainerActivity(env)

	env.OnActivity("StartContainerActivity", mock.Anything, mock.Anything).Return(
		&payload.ContainerExecutionOutput{
			ContainerID: "container-123",
			ExitCode:    0,
			Duration:    1 * time.Second,
		}, nil)

	input := payload.ContainerExecutionInput{
		Image:   "alpine:latest",
		Command: []string{"echo", "{{.version}}"},
		Env: map[string]string{
			"VERSION": "{{.version}}",
		},
	}

	params := []payload.WorkflowParameter{
		{Name: "version", Value: "v1.2.3"},
	}

	env.ExecuteWorkflow(WorkflowWithParameters, input, params)
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())
}

func TestHelperFunctions(t *testing.T) {
	t.Run("replaceAll", func(t *testing.T) {
		result := strings.ReplaceAll("Hello {{name}}, welcome to {{place}}", "{{name}}", "World")
		assert.Contains(t, result, "World")
		assert.NotContains(t, result, "{{name}}")
	})

	t.Run("indexOf", func(t *testing.T) {
		index := strings.Index("hello world", "world")
		assert.Equal(t, 6, index)

		index = strings.Index("hello world", "notfound")
		assert.Equal(t, -1, index)
	})
}

func TestDAGWorkflow_DiamondDependency(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerContainerActivity(env)

	env.OnActivity("StartContainerActivity", mock.Anything, mock.Anything).Return(
		&payload.ContainerExecutionOutput{
			ContainerID: "container-diamond",
			ExitCode:    0,
			Success:     true,
			Duration:    1 * time.Second,
			StartedAt:   time.Now(),
			FinishedAt:  time.Now().Add(1 * time.Second),
		}, nil)

	input := payload.DAGWorkflowInput{
		Nodes: []payload.DAGNode{
			{
				Name: "A",
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{
						Image:   "alpine:latest",
						Command: []string{"echo", "A"},
					},
				},
			},
			{
				Name: "B",
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{
						Image:   "alpine:latest",
						Command: []string{"echo", "B"},
					},
				},
				Dependencies: []string{"A"},
			},
			{
				Name: "C",
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{
						Image:   "alpine:latest",
						Command: []string{"echo", "C"},
					},
				},
				Dependencies: []string{"A"},
			},
			{
				Name: "D",
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{
						Image:   "alpine:latest",
						Command: []string{"echo", "D"},
					},
				},
				Dependencies: []string{"B", "C"},
			},
		},
		FailFast: false,
	}

	env.ExecuteWorkflow(DAGWorkflow, input)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.DAGWorkflowOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 4, result.TotalSuccess)
	assert.Equal(t, 0, result.TotalFailed)
}

func TestDAGWorkflow_ParallelBranches(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerContainerActivity(env)

	env.OnActivity("StartContainerActivity", mock.Anything, mock.Anything).Return(
		&payload.ContainerExecutionOutput{
			ContainerID: "container-parallel",
			ExitCode:    0,
			Success:     true,
			Duration:    1 * time.Second,
			StartedAt:   time.Now(),
			FinishedAt:  time.Now().Add(1 * time.Second),
		}, nil)

	input := payload.DAGWorkflowInput{
		Nodes: []payload.DAGNode{
			{
				Name: "root",
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{
						Image:   "alpine:latest",
						Command: []string{"echo", "root"},
					},
				},
			},
			{
				Name: "branch1",
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{
						Image:   "alpine:latest",
						Command: []string{"echo", "branch1"},
					},
				},
				Dependencies: []string{"root"},
			},
			{
				Name: "branch2",
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{
						Image:   "alpine:latest",
						Command: []string{"echo", "branch2"},
					},
				},
				Dependencies: []string{"root"},
			},
		},
		FailFast: false,
	}

	env.ExecuteWorkflow(DAGWorkflow, input)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.DAGWorkflowOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 3, result.TotalSuccess)
}

func TestDAGWorkflow_FailFastFalseWithFailure(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerContainerActivity(env)

	env.OnActivity("StartContainerActivity", mock.Anything, mock.MatchedBy(func(input payload.ContainerExecutionInput) bool {
		return input.Image == "alpine:fail"
	})).Return(
		&payload.ContainerExecutionOutput{
			ContainerID: "container-fail",
			ExitCode:    1,
			Success:     false,
			Duration:    1 * time.Second,
		}, nil).Once()

	env.OnActivity("StartContainerActivity", mock.Anything, mock.MatchedBy(func(input payload.ContainerExecutionInput) bool {
		return input.Image == "alpine:pass"
	})).Return(
		&payload.ContainerExecutionOutput{
			ContainerID: "container-pass",
			ExitCode:    0,
			Success:     true,
			Duration:    1 * time.Second,
		}, nil).Once()

	input := payload.DAGWorkflowInput{
		Nodes: []payload.DAGNode{
			{
				Name: "failing",
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{
						Image:   "alpine:fail",
						Command: []string{"exit", "1"},
					},
				},
			},
			{
				Name: "passing",
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{
						Image:   "alpine:pass",
						Command: []string{"echo", "ok"},
					},
				},
			},
		},
		FailFast: false,
	}

	env.ExecuteWorkflow(DAGWorkflow, input)
	require.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())

	var result payload.DAGWorkflowOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 1, result.TotalSuccess)
	assert.Equal(t, 1, result.TotalFailed)
}

func TestDAGWorkflow_DependencyFailureBlocksDownstream(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerContainerActivity(env)

	env.OnActivity("StartContainerActivity", mock.Anything, mock.Anything).Return(
		&payload.ContainerExecutionOutput{
			ContainerID: "container-fail",
			ExitCode:    1,
			Success:     false,
			Duration:    1 * time.Second,
		}, nil)

	input := payload.DAGWorkflowInput{
		Nodes: []payload.DAGNode{
			{
				Name: "build",
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{
						Image:   "golang:1.25",
						Command: []string{"go", "build"},
					},
				},
			},
			{
				Name: "test",
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{
						Image:   "golang:1.25",
						Command: []string{"go", "test"},
					},
				},
				Dependencies: []string{"build"},
			},
		},
		FailFast: true,
	}

	env.ExecuteWorkflow(DAGWorkflow, input)
	require.True(t, env.IsWorkflowCompleted())

	err := env.GetWorkflowError()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dependency build failed")
}

func TestDAGWorkflow_OutputExtractionAndSubstitution(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerContainerActivity(env)

	// Mock build activity to return JSON stdout
	env.OnActivity("StartContainerActivity", mock.Anything, mock.MatchedBy(func(input payload.ContainerExecutionInput) bool {
		return input.Image == "golang:1.25-build"
	})).Return(
		&payload.ContainerExecutionOutput{
			ContainerID: "container-build",
			ExitCode:    0,
			Success:     true,
			Duration:    1 * time.Second,
			Stdout:      `{"version":"1.2.3"}`,
		}, nil)

	// Mock deploy activity
	env.OnActivity("StartContainerActivity", mock.Anything, mock.Anything).Return(
		&payload.ContainerExecutionOutput{
			ContainerID: "container-deploy",
			ExitCode:    0,
			Success:     true,
			Duration:    1 * time.Second,
		}, nil)

	input := payload.DAGWorkflowInput{
		Nodes: []payload.DAGNode{
			{
				Name: "build",
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{
						Image:   "golang:1.25-build",
						Command: []string{"go", "build"},
					},
					Outputs: []payload.OutputDefinition{
						{
							Name:      "version",
							ValueFrom: "stdout",
							JSONPath:  "$.version",
						},
					},
				},
			},
			{
				Name: "deploy",
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{
						Image:   "alpine:deploy",
						Command: []string{"deploy"},
					},
					Inputs: []payload.InputMapping{
						{
							Name:     "BUILD_VERSION",
							From:     "build.version",
							Required: true,
						},
					},
				},
				Dependencies: []string{"build"},
			},
		},
		FailFast: false,
	}

	env.ExecuteWorkflow(DAGWorkflow, input)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.DAGWorkflowOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 2, result.TotalSuccess)
	assert.Equal(t, "1.2.3", result.StepOutputs["build"]["version"])
}

func TestDAGWorkflow_ActivityError(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerContainerActivity(env)

	env.OnActivity("StartContainerActivity", mock.Anything, mock.Anything).Return(
		nil, fmt.Errorf("docker daemon error"))

	input := payload.DAGWorkflowInput{
		Nodes: []payload.DAGNode{
			{
				Name: "broken",
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{
						Image:   "alpine:latest",
						Command: []string{"echo", "hello"},
					},
				},
			},
		},
		FailFast: true,
	}

	env.ExecuteWorkflow(DAGWorkflow, input)
	require.True(t, env.IsWorkflowCompleted())
	assert.Error(t, env.GetWorkflowError())
}

func TestDAGWorkflow_MultipleIndependentRoots(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerContainerActivity(env)

	env.OnActivity("StartContainerActivity", mock.Anything, mock.Anything).Return(
		&payload.ContainerExecutionOutput{
			ContainerID: "container-ok",
			ExitCode:    0,
			Success:     true,
			Duration:    1 * time.Second,
			StartedAt:   time.Now(),
			FinishedAt:  time.Now().Add(1 * time.Second),
		}, nil)

	input := payload.DAGWorkflowInput{
		Nodes: []payload.DAGNode{
			{
				Name: "lint",
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{
						Image:   "golangci/golangci-lint:latest",
						Command: []string{"golangci-lint", "run"},
					},
				},
			},
			{
				Name: "test",
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{
						Image:   "golang:1.25",
						Command: []string{"go", "test", "./..."},
					},
				},
			},
			{
				Name: "scan",
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{
						Image:   "securego/gosec:latest",
						Command: []string{"gosec", "./..."},
					},
				},
			},
		},
		FailFast: false,
	}

	env.ExecuteWorkflow(DAGWorkflow, input)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.DAGWorkflowOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 3, result.TotalSuccess)
	assert.Equal(t, 0, result.TotalFailed)
}

func TestDAGWorkflow_AlreadyExecutedGuard(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerContainerActivity(env)

	callCount := 0
	env.OnActivity("StartContainerActivity", mock.Anything, mock.Anything).Return(
		func(_ context.Context, _ payload.ContainerExecutionInput) (*payload.ContainerExecutionOutput, error) {
			callCount++
			return &payload.ContainerExecutionOutput{
				ContainerID: fmt.Sprintf("container-%d", callCount),
				ExitCode:    0,
				Success:     true,
				Duration:    1 * time.Second,
				StartedAt:   time.Now(),
				FinishedAt:  time.Now().Add(1 * time.Second),
			}, nil
		})

	input := payload.DAGWorkflowInput{
		Nodes: []payload.DAGNode{
			{
				Name: "A",
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{
						Image:   "alpine:latest",
						Command: []string{"echo", "A"},
					},
				},
			},
			{
				Name: "B",
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{
						Image:   "alpine:latest",
						Command: []string{"echo", "B"},
					},
				},
				Dependencies: []string{"A"},
			},
			{
				Name: "C",
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{
						Image:   "alpine:latest",
						Command: []string{"echo", "C"},
					},
				},
				Dependencies: []string{"A"},
			},
		},
		FailFast: false,
	}

	env.ExecuteWorkflow(DAGWorkflow, input)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.DAGWorkflowOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 3, callCount, "activity should be called exactly 3 times (A once, B once, C once)")
	assert.Equal(t, 3, result.TotalSuccess)
}
