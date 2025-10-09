package builder

import (
	"testing"
	"time"

	"github.com/jasoet/go-wf/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewWorkflowBuilder(t *testing.T) {
	tests := []struct {
		name string
		opts []BuilderOption
		want *WorkflowBuilder
	}{
		{
			name: "default configuration",
			opts: nil,
			want: &WorkflowBuilder{
				name:           "test",
				containers:     []docker.ContainerExecutionInput{},
				exitHandlers:   []docker.ContainerExecutionInput{},
				stopOnError:    true,
				cleanup:        false,
				parallelMode:   false,
				failFast:       false,
				maxConcurrency: 0,
			},
		},
		{
			name: "with options",
			opts: []BuilderOption{
				WithStopOnError(false),
				WithParallelMode(true),
				WithFailFast(true),
				WithMaxConcurrency(5),
			},
			want: &WorkflowBuilder{
				name:           "test",
				containers:     []docker.ContainerExecutionInput{},
				exitHandlers:   []docker.ContainerExecutionInput{},
				stopOnError:    false,
				cleanup:        false,
				parallelMode:   true,
				failFast:       true,
				maxConcurrency: 5,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewWorkflowBuilder("test", tt.opts...)
			assert.Equal(t, tt.want.name, got.name)
			assert.Equal(t, tt.want.stopOnError, got.stopOnError)
			assert.Equal(t, tt.want.parallelMode, got.parallelMode)
			assert.Equal(t, tt.want.failFast, got.failFast)
			assert.Equal(t, tt.want.maxConcurrency, got.maxConcurrency)
			assert.NotNil(t, got.containers)
			assert.NotNil(t, got.exitHandlers)
		})
	}
}

func TestWorkflowBuilder_Add(t *testing.T) {
	tests := []struct {
		name        string
		sources     []WorkflowSource
		wantCount   int
		expectError bool
	}{
		{
			name: "add single source",
			sources: []WorkflowSource{
				NewContainerSource(docker.ContainerExecutionInput{
					Image: "alpine:latest",
				}),
			},
			wantCount:   1,
			expectError: false,
		},
		{
			name: "add multiple sources",
			sources: []WorkflowSource{
				NewContainerSource(docker.ContainerExecutionInput{Image: "alpine:latest"}),
				NewContainerSource(docker.ContainerExecutionInput{Image: "busybox:latest"}),
				NewContainerSource(docker.ContainerExecutionInput{Image: "nginx:latest"}),
			},
			wantCount:   3,
			expectError: false,
		},
		{
			name: "add nil source",
			sources: []WorkflowSource{
				nil,
			},
			wantCount:   0,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewWorkflowBuilder("test")
			for _, source := range tt.sources {
				builder.Add(source)
			}

			assert.Equal(t, tt.wantCount, builder.Count())

			if tt.expectError {
				assert.NotEmpty(t, builder.Errors())
			} else {
				assert.Empty(t, builder.Errors())
			}
		})
	}
}

func TestWorkflowBuilder_AddInput(t *testing.T) {
	builder := NewWorkflowBuilder("test")

	input1 := docker.ContainerExecutionInput{
		Image:   "alpine:latest",
		Command: []string{"echo", "test1"},
	}
	input2 := docker.ContainerExecutionInput{
		Image:   "busybox:latest",
		Command: []string{"echo", "test2"},
	}

	builder.AddInput(input1).AddInput(input2)

	assert.Equal(t, 2, builder.Count())
	assert.Equal(t, "alpine:latest", builder.containers[0].Image)
	assert.Equal(t, "busybox:latest", builder.containers[1].Image)
}

func TestWorkflowBuilder_AddExitHandler(t *testing.T) {
	builder := NewWorkflowBuilder("test")

	cleanup := NewContainerSource(docker.ContainerExecutionInput{
		Image:   "alpine:latest",
		Command: []string{"cleanup.sh"},
	})

	notify := NewContainerSource(docker.ContainerExecutionInput{
		Image:   "curlimages/curl:latest",
		Command: []string{"curl", "https://webhook.site"},
	})

	builder.AddExitHandler(cleanup).AddExitHandler(notify)

	assert.Equal(t, 2, builder.ExitHandlerCount())
}

func TestWorkflowBuilder_BuildPipeline(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func() *WorkflowBuilder
		expectError bool
		validate    func(t *testing.T, input *docker.PipelineInput)
	}{
		{
			name: "valid pipeline",
			setupFunc: func() *WorkflowBuilder {
				return NewWorkflowBuilder("test").
					AddInput(docker.ContainerExecutionInput{Image: "alpine:latest"}).
					AddInput(docker.ContainerExecutionInput{Image: "busybox:latest"})
			},
			expectError: false,
			validate: func(t *testing.T, input *docker.PipelineInput) {
				assert.NotNil(t, input)
				assert.Len(t, input.Containers, 2)
				assert.True(t, input.StopOnError)
			},
		},
		{
			name: "empty pipeline",
			setupFunc: func() *WorkflowBuilder {
				return NewWorkflowBuilder("test")
			},
			expectError: true,
			validate:    nil,
		},
		{
			name: "pipeline with custom settings",
			setupFunc: func() *WorkflowBuilder {
				return NewWorkflowBuilder("test").
					AddInput(docker.ContainerExecutionInput{Image: "alpine:latest"}).
					StopOnError(false).
					Cleanup(true)
			},
			expectError: false,
			validate: func(t *testing.T, input *docker.PipelineInput) {
				assert.False(t, input.StopOnError)
				assert.True(t, input.Cleanup)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := tt.setupFunc()
			input, err := builder.BuildPipeline()

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, input)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, input)
				if tt.validate != nil {
					tt.validate(t, input)
				}
			}
		})
	}
}

func TestWorkflowBuilder_BuildParallel(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func() *WorkflowBuilder
		expectError bool
		validate    func(t *testing.T, input *docker.ParallelInput)
	}{
		{
			name: "valid parallel",
			setupFunc: func() *WorkflowBuilder {
				return NewWorkflowBuilder("test").
					Parallel(true).
					AddInput(docker.ContainerExecutionInput{Image: "alpine:latest"}).
					AddInput(docker.ContainerExecutionInput{Image: "busybox:latest"})
			},
			expectError: false,
			validate: func(t *testing.T, input *docker.ParallelInput) {
				assert.NotNil(t, input)
				assert.Len(t, input.Containers, 2)
				assert.Equal(t, "continue", input.FailureStrategy)
			},
		},
		{
			name: "parallel with fail fast",
			setupFunc: func() *WorkflowBuilder {
				return NewWorkflowBuilder("test").
					Parallel(true).
					FailFast(true).
					MaxConcurrency(5).
					AddInput(docker.ContainerExecutionInput{Image: "alpine:latest"})
			},
			expectError: false,
			validate: func(t *testing.T, input *docker.ParallelInput) {
				assert.Equal(t, "fail_fast", input.FailureStrategy)
				assert.Equal(t, 5, input.MaxConcurrency)
			},
		},
		{
			name: "empty parallel",
			setupFunc: func() *WorkflowBuilder {
				return NewWorkflowBuilder("test").Parallel(true)
			},
			expectError: true,
			validate:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := tt.setupFunc()
			input, err := builder.BuildParallel()

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, input)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, input)
				if tt.validate != nil {
					tt.validate(t, input)
				}
			}
		})
	}
}

func TestWorkflowBuilder_Build(t *testing.T) {
	t.Run("builds pipeline by default", func(t *testing.T) {
		builder := NewWorkflowBuilder("test").
			AddInput(docker.ContainerExecutionInput{Image: "alpine:latest"})

		result, err := builder.Build()
		require.NoError(t, err)

		pipelineInput, ok := result.(*docker.PipelineInput)
		assert.True(t, ok, "Expected PipelineInput")
		assert.NotNil(t, pipelineInput)
	})

	t.Run("builds parallel when enabled", func(t *testing.T) {
		builder := NewWorkflowBuilder("test").
			Parallel(true).
			AddInput(docker.ContainerExecutionInput{Image: "alpine:latest"})

		result, err := builder.Build()
		require.NoError(t, err)

		parallelInput, ok := result.(*docker.ParallelInput)
		assert.True(t, ok, "Expected ParallelInput")
		assert.NotNil(t, parallelInput)
	})
}

func TestWorkflowBuilder_BuildSingle(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func() *WorkflowBuilder
		expectError bool
	}{
		{
			name: "valid single",
			setupFunc: func() *WorkflowBuilder {
				return NewWorkflowBuilder("test").
					AddInput(docker.ContainerExecutionInput{Image: "alpine:latest"})
			},
			expectError: false,
		},
		{
			name: "empty single",
			setupFunc: func() *WorkflowBuilder {
				return NewWorkflowBuilder("test")
			},
			expectError: true,
		},
		{
			name: "multiple containers returns first",
			setupFunc: func() *WorkflowBuilder {
				return NewWorkflowBuilder("test").
					AddInput(docker.ContainerExecutionInput{Image: "alpine:latest"}).
					AddInput(docker.ContainerExecutionInput{Image: "busybox:latest"})
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := tt.setupFunc()
			input, err := builder.BuildSingle()

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, input)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, input)
				assert.Equal(t, "alpine:latest", input.Image)
			}
		})
	}
}

func TestWorkflowBuilder_WithTimeout(t *testing.T) {
	builder := NewWorkflowBuilder("test").
		AddInput(docker.ContainerExecutionInput{Image: "alpine:latest"}).
		AddInput(docker.ContainerExecutionInput{Image: "busybox:latest"}).
		WithTimeout(5 * time.Minute)

	assert.Equal(t, 5*time.Minute, builder.containers[0].RunTimeout)
	assert.Equal(t, 5*time.Minute, builder.containers[1].RunTimeout)
}

func TestWorkflowBuilder_WithAutoRemove(t *testing.T) {
	builder := NewWorkflowBuilder("test").
		AddInput(docker.ContainerExecutionInput{Image: "alpine:latest"}).
		AddInput(docker.ContainerExecutionInput{Image: "busybox:latest"}).
		WithAutoRemove(true)

	assert.True(t, builder.containers[0].AutoRemove)
	assert.True(t, builder.containers[1].AutoRemove)
}

func TestWorkflowBuilder_ChainedCalls(t *testing.T) {
	// Test fluent API with chained calls
	input, err := NewWorkflowBuilder("test").
		AddInput(docker.ContainerExecutionInput{Image: "golang:1.25", Name: "build"}).
		AddInput(docker.ContainerExecutionInput{Image: "golang:1.25", Name: "test"}).
		AddInput(docker.ContainerExecutionInput{Image: "deployer:v1", Name: "deploy"}).
		StopOnError(true).
		Cleanup(true).
		WithTimeout(10 * time.Minute).
		WithAutoRemove(true).
		BuildPipeline()

	require.NoError(t, err)
	assert.NotNil(t, input)
	assert.Len(t, input.Containers, 3)
	assert.True(t, input.StopOnError)
	assert.True(t, input.Cleanup)
	assert.Equal(t, 10*time.Minute, input.Containers[0].RunTimeout)
	assert.True(t, input.Containers[0].AutoRemove)
}

func TestContainerSource(t *testing.T) {
	input := docker.ContainerExecutionInput{
		Image:   "alpine:latest",
		Command: []string{"echo", "test"},
		Env:     map[string]string{"KEY": "value"},
	}

	source := NewContainerSource(input)
	result := source.ToInput()

	assert.Equal(t, input.Image, result.Image)
	assert.Equal(t, input.Command, result.Command)
	assert.Equal(t, input.Env, result.Env)
}

func TestWorkflowSourceFunc(t *testing.T) {
	source := WorkflowSourceFunc(func() docker.ContainerExecutionInput {
		return docker.ContainerExecutionInput{
			Image:   "alpine:latest",
			Command: []string{"echo", "test"},
		}
	})

	result := source.ToInput()
	assert.Equal(t, "alpine:latest", result.Image)
	assert.Equal(t, []string{"echo", "test"}, result.Command)
}
