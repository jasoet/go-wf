package workflow

import (
	"testing"
	"time"

	"github.com/jasoet/go-wf/docker"
	"github.com/jasoet/go-wf/docker/activity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/testsuite"
)

func TestDAGWorkflow(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Mock the activity
	env.OnActivity(activity.StartContainerActivity, mock.Anything, mock.Anything).Return(
		&docker.ContainerExecutionOutput{
			ContainerID: "container-123",
			ExitCode:    0,
			Success:     true,
			Duration:    1 * time.Second,
			StartedAt:   time.Now(),
			FinishedAt:  time.Now().Add(1 * time.Second),
		}, nil)

	input := docker.DAGWorkflowInput{
		Nodes: []docker.DAGNode{
			{
				Name: "build",
				Container: docker.ExtendedContainerInput{
					ContainerExecutionInput: docker.ContainerExecutionInput{
						Image:   "golang:1.25",
						Command: []string{"go", "build"},
					},
				},
			},
			{
				Name: "test",
				Container: docker.ExtendedContainerInput{
					ContainerExecutionInput: docker.ContainerExecutionInput{
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
		input       docker.DAGWorkflowInput
		expectError bool
	}{
		{
			name: "valid DAG",
			input: docker.DAGWorkflowInput{
				Nodes: []docker.DAGNode{
					{
						Name: "task1",
						Container: docker.ExtendedContainerInput{
							ContainerExecutionInput: docker.ContainerExecutionInput{
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
			input: docker.DAGWorkflowInput{
				Nodes: []docker.DAGNode{},
			},
			expectError: true,
		},
		{
			name: "missing dependency",
			input: docker.DAGWorkflowInput{
				Nodes: []docker.DAGNode{
					{
						Name: "task1",
						Container: docker.ExtendedContainerInput{
							ContainerExecutionInput: docker.ContainerExecutionInput{
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

	// Mock activity to return failure for first call, success for second
	env.OnActivity(activity.StartContainerActivity, mock.Anything, mock.Anything).
		Return(&docker.ContainerExecutionOutput{
			ContainerID: "container-fail",
			ExitCode:    1,
			Success:     false,
			Duration:    1 * time.Second,
		}, nil).Once()

	env.OnActivity(activity.StartContainerActivity, mock.Anything, mock.Anything).
		Return(&docker.ContainerExecutionOutput{
			ContainerID: "container-ok",
			ExitCode:    0,
			Success:     true,
			Duration:    1 * time.Second,
		}, nil)

	input := docker.DAGWorkflowInput{
		Nodes: []docker.DAGNode{
			{
				Name: "task1",
				Container: docker.ExtendedContainerInput{
					ContainerExecutionInput: docker.ContainerExecutionInput{
						Image: "alpine:latest",
					},
				},
			},
			{
				Name: "task2",
				Container: docker.ExtendedContainerInput{
					ContainerExecutionInput: docker.ContainerExecutionInput{
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

	env.OnActivity(activity.StartContainerActivity, mock.Anything, mock.Anything).Return(
		&docker.ContainerExecutionOutput{
			ContainerID: "container-123",
			ExitCode:    0,
			Duration:    1 * time.Second,
		}, nil)

	input := docker.ContainerExecutionInput{
		Image:   "alpine:latest",
		Command: []string{"echo", "{{.version}}"},
		Env: map[string]string{
			"VERSION": "{{.version}}",
		},
	}

	params := []docker.WorkflowParameter{
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
