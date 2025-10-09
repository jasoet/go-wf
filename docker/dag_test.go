package docker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/testsuite"
)

func TestDAGWorkflow(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Mock the activity
	env.OnActivity(StartContainerActivity, mock.Anything, mock.Anything).Return(
		func(_ interface{}, input ContainerExecutionInput) ContainerExecutionOutput {
			return ContainerExecutionOutput{
				ContainerID: "container-123",
				ExitCode:    0,
				Duration:    1 * time.Second,
				StartedAt:   time.Now(),
				FinishedAt:  time.Now().Add(1 * time.Second),
			}
		}, nil)

	input := DAGWorkflowInput{
		Nodes: []DAGNode{
			{
				Name: "build",
				Container: ExtendedContainerInput{
					ContainerExecutionInput: ContainerExecutionInput{
						Image:   "golang:1.25",
						Command: []string{"go", "build"},
					},
				},
			},
			{
				Name: "test",
				Container: ExtendedContainerInput{
					ContainerExecutionInput: ContainerExecutionInput{
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
		input       DAGWorkflowInput
		expectError bool
	}{
		{
			name: "valid DAG",
			input: DAGWorkflowInput{
				Nodes: []DAGNode{
					{
						Name: "task1",
						Container: ExtendedContainerInput{
							ContainerExecutionInput: ContainerExecutionInput{
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
			input: DAGWorkflowInput{
				Nodes: []DAGNode{},
			},
			expectError: true,
		},
		{
			name: "missing dependency",
			input: DAGWorkflowInput{
				Nodes: []DAGNode{
					{
						Name: "task1",
						Container: ExtendedContainerInput{
							ContainerExecutionInput: ContainerExecutionInput{
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

	callCount := 0
	env.OnActivity(StartContainerActivity, mock.Anything, mock.Anything).Return(
		func(_ interface{}, input ContainerExecutionInput) ContainerExecutionOutput {
			callCount++
			// First task fails
			if callCount == 1 {
				return ContainerExecutionOutput{
					ContainerID: "container-fail",
					ExitCode:    1,
					Duration:    1 * time.Second,
				}
			}
			return ContainerExecutionOutput{
				ContainerID: "container-ok",
				ExitCode:    0,
				Duration:    1 * time.Second,
			}
		}, nil)

	input := DAGWorkflowInput{
		Nodes: []DAGNode{
			{
				Name: "task1",
				Container: ExtendedContainerInput{
					ContainerExecutionInput: ContainerExecutionInput{
						Image: "alpine:latest",
					},
				},
			},
			{
				Name: "task2",
				Container: ExtendedContainerInput{
					ContainerExecutionInput: ContainerExecutionInput{
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

	env.OnActivity(StartContainerActivity, mock.Anything, mock.Anything).Return(
		&ContainerExecutionOutput{
			ContainerID: "container-123",
			ExitCode:    0,
			Duration:    1 * time.Second,
		}, nil)

	input := ContainerExecutionInput{
		Image:   "alpine:latest",
		Command: []string{"echo", "{{.version}}"},
		Env: map[string]string{
			"VERSION": "{{.version}}",
		},
	}

	params := []WorkflowParameter{
		{Name: "version", Value: "v1.2.3"},
	}

	env.ExecuteWorkflow(WorkflowWithParameters, input, params)
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())
}

func TestHelperFunctions(t *testing.T) {
	t.Run("replaceAll", func(t *testing.T) {
		result := replaceAll("Hello {{name}}, welcome to {{place}}", "{{name}}", "World")
		assert.Contains(t, result, "World")
		assert.NotContains(t, result, "{{name}}")
	})

	t.Run("indexOf", func(t *testing.T) {
		index := indexOf("hello world", "world")
		assert.Equal(t, 6, index)

		index = indexOf("hello world", "notfound")
		assert.Equal(t, -1, index)
	})
}
