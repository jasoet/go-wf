package builder

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/temporal"

	"github.com/jasoet/go-wf/container/payload"
	"github.com/jasoet/go-wf/workflow"
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
				containers:     []payload.ContainerExecutionInput{},
				exitHandlers:   []payload.ContainerExecutionInput{},
				stopOnError:    true,
				cleanup:        false,
				mode:           modeUnset,
				failFast:       false,
				maxConcurrency: 0,
			},
		},
		{
			name: "with options",
			opts: []BuilderOption{
				WithStopOnError(false),
				WithParallelMode(),
				WithFailFast(true),
				WithMaxConcurrency(5),
			},
			want: &WorkflowBuilder{
				containers:     []payload.ContainerExecutionInput{},
				exitHandlers:   []payload.ContainerExecutionInput{},
				stopOnError:    false,
				cleanup:        false,
				mode:           modeParallel,
				failFast:       true,
				maxConcurrency: 5,
			},
		},
		{
			name: "with cleanup",
			opts: []BuilderOption{
				WithCleanup(true),
			},
			want: &WorkflowBuilder{
				containers:   []payload.ContainerExecutionInput{},
				exitHandlers: []payload.ContainerExecutionInput{},
				stopOnError:  true,
				cleanup:      true,
				mode:         modeUnset,
				failFast:     false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewWorkflowBuilder(tt.opts...)
			assert.Equal(t, tt.want.stopOnError, got.stopOnError)
			assert.Equal(t, tt.want.cleanup, got.cleanup)
			assert.Equal(t, tt.want.mode, got.mode)
			assert.Equal(t, tt.want.failFast, got.failFast)
			assert.Equal(t, tt.want.maxConcurrency, got.maxConcurrency)
			assert.NotNil(t, got.containers)
			assert.NotNil(t, got.exitHandlers)
		})
	}
}

func TestBuilderOptions_GlobalTimeoutAndAutoRemove(t *testing.T) {
	t.Run("WithGlobalTimeout applies to existing containers", func(t *testing.T) {
		builder := NewWorkflowBuilder().
			AddInput(payload.ContainerExecutionInput{Image: "alpine:latest"}).
			AddInput(payload.ContainerExecutionInput{Image: "busybox:latest"})

		// Apply WithGlobalTimeout option
		WithGlobalTimeout(5 * time.Minute)(builder)

		assert.Equal(t, 5*time.Minute, builder.containers[0].RunTimeout)
		assert.Equal(t, 5*time.Minute, builder.containers[1].RunTimeout)
	})

	t.Run("WithGlobalAutoRemove applies to existing containers", func(t *testing.T) {
		builder := NewWorkflowBuilder().
			AddInput(payload.ContainerExecutionInput{Image: "alpine:latest"}).
			AddInput(payload.ContainerExecutionInput{Image: "busybox:latest"})

		// Apply WithGlobalAutoRemove option
		WithGlobalAutoRemove(true)(builder)

		assert.True(t, builder.containers[0].AutoRemove)
		assert.True(t, builder.containers[1].AutoRemove)
	})
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
				NewContainerSource(payload.ContainerExecutionInput{
					Image: "alpine:latest",
				}),
			},
			wantCount:   1,
			expectError: false,
		},
		{
			name: "add multiple sources",
			sources: []WorkflowSource{
				NewContainerSource(payload.ContainerExecutionInput{Image: "alpine:latest"}),
				NewContainerSource(payload.ContainerExecutionInput{Image: "busybox:latest"}),
				NewContainerSource(payload.ContainerExecutionInput{Image: "nginx:latest"}),
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
			builder := NewWorkflowBuilder()
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
	builder := NewWorkflowBuilder()

	input1 := payload.ContainerExecutionInput{
		Image:   "alpine:latest",
		Command: []string{"echo", "test1"},
	}
	input2 := payload.ContainerExecutionInput{
		Image:   "busybox:latest",
		Command: []string{"echo", "test2"},
	}

	builder.AddInput(input1).AddInput(input2)

	assert.Equal(t, 2, builder.Count())
	assert.Equal(t, "alpine:latest", builder.containers[0].Image)
	assert.Equal(t, "busybox:latest", builder.containers[1].Image)
}

func TestWorkflowBuilder_AddExitHandler(t *testing.T) {
	builder := NewWorkflowBuilder()

	cleanup := NewContainerSource(payload.ContainerExecutionInput{
		Image:   "alpine:latest",
		Command: []string{"cleanup.sh"},
	})

	notify := NewContainerSource(payload.ContainerExecutionInput{
		Image:   "curlimages/curl:latest",
		Command: []string{"curl", "https://webhook.site"},
	})

	builder.AddExitHandler(cleanup).AddExitHandler(notify)

	assert.Equal(t, 2, builder.ExitHandlerCount())
}

func TestWorkflowBuilder_AddExitHandlerInput(t *testing.T) {
	builder := NewWorkflowBuilder()

	cleanup := payload.ContainerExecutionInput{
		Image:   "alpine:latest",
		Command: []string{"cleanup.sh"},
	}

	notify := payload.ContainerExecutionInput{
		Image:   "curlimages/curl:latest",
		Command: []string{"curl", "https://webhook.site"},
	}

	builder.AddExitHandlerInput(cleanup).AddExitHandlerInput(notify)

	assert.Equal(t, 2, builder.ExitHandlerCount())
	assert.Equal(t, "alpine:latest", builder.exitHandlers[0].Image)
	assert.Equal(t, "curlimages/curl:latest", builder.exitHandlers[1].Image)
}

func TestWorkflowBuilder_buildPipelineInput(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func() *WorkflowBuilder
		expectError bool
		validate    func(t *testing.T, input *payload.PipelineInput)
	}{
		{
			name: "valid pipeline",
			setupFunc: func() *WorkflowBuilder {
				return NewWorkflowBuilder().
					AddInput(payload.ContainerExecutionInput{Image: "alpine:latest"}).
					AddInput(payload.ContainerExecutionInput{Image: "busybox:latest"})
			},
			expectError: false,
			validate: func(t *testing.T, input *payload.PipelineInput) {
				assert.NotNil(t, input)
				assert.Len(t, input.Containers, 2)
				assert.True(t, input.StopOnError)
			},
		},
		{
			name: "empty pipeline",
			setupFunc: func() *WorkflowBuilder {
				return NewWorkflowBuilder()
			},
			expectError: true,
			validate:    nil,
		},
		{
			name: "pipeline with custom settings",
			setupFunc: func() *WorkflowBuilder {
				return NewWorkflowBuilder().
					AddInput(payload.ContainerExecutionInput{Image: "alpine:latest"}).
					StopOnError(false).
					Cleanup(true)
			},
			expectError: false,
			validate: func(t *testing.T, input *payload.PipelineInput) {
				assert.False(t, input.StopOnError)
				assert.True(t, input.Cleanup)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := tt.setupFunc()
			input, err := builder.buildPipelineInput()

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

func TestWorkflowBuilder_buildParallelInput(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func() *WorkflowBuilder
		expectError bool
		validate    func(t *testing.T, input *payload.ParallelInput)
	}{
		{
			name: "valid parallel",
			setupFunc: func() *WorkflowBuilder {
				return NewWorkflowBuilder().
					AddInput(payload.ContainerExecutionInput{Image: "alpine:latest"}).
					AddInput(payload.ContainerExecutionInput{Image: "busybox:latest"})
			},
			expectError: false,
			validate: func(t *testing.T, input *payload.ParallelInput) {
				assert.NotNil(t, input)
				assert.Len(t, input.Containers, 2)
				assert.Equal(t, "continue", input.FailureStrategy)
			},
		},
		{
			name: "parallel with fail fast",
			setupFunc: func() *WorkflowBuilder {
				return NewWorkflowBuilder().
					FailFast(true).
					MaxConcurrency(5).
					AddInput(payload.ContainerExecutionInput{Image: "alpine:latest"})
			},
			expectError: false,
			validate: func(t *testing.T, input *payload.ParallelInput) {
				assert.Equal(t, "fail_fast", input.FailureStrategy)
				assert.Equal(t, 5, input.MaxConcurrency)
			},
		},
		{
			name: "empty parallel",
			setupFunc: func() *WorkflowBuilder {
				return NewWorkflowBuilder()
			},
			expectError: true,
			validate:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := tt.setupFunc()
			input, err := builder.buildParallelInput()

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
	t.Run("pipeline mode produces definition", func(t *testing.T) {
		def, err := NewWorkflowBuilder().
			Name("daily-deploy").
			Pipeline().
			AddInput(payload.ContainerExecutionInput{Image: "alpine:latest"}).
			Build()
		require.NoError(t, err)
		assert.Equal(t, "daily-deploy", def.Name)
		assert.Equal(t, "container-daily-deploy", def.TaskQueue)
	})

	t.Run("parallel mode produces definition", func(t *testing.T) {
		def, err := NewWorkflowBuilder().
			Name("parallel-job").
			Parallel().
			AddInput(payload.ContainerExecutionInput{Image: "alpine:latest"}).
			Build()
		require.NoError(t, err)
		assert.Equal(t, "parallel-job", def.Name)
		assert.Equal(t, "container-parallel-job", def.TaskQueue)
	})

	t.Run("single mode produces definition", func(t *testing.T) {
		def, err := NewWorkflowBuilder().
			Name("single-job").
			Single().
			AddInput(payload.ContainerExecutionInput{Image: "alpine:latest"}).
			Build()
		require.NoError(t, err)
		assert.Equal(t, "single-job", def.Name)
		assert.Equal(t, "container-single-job", def.TaskQueue)
	})

	t.Run("custom task queue is used", func(t *testing.T) {
		def, err := NewWorkflowBuilder().
			Name("my-job").
			TaskQueue("custom-queue").
			Pipeline().
			AddInput(payload.ContainerExecutionInput{Image: "alpine:latest"}).
			Build()
		require.NoError(t, err)
		assert.Equal(t, "custom-queue", def.TaskQueue)
	})

	t.Run("requires name", func(t *testing.T) {
		_, err := NewWorkflowBuilder().
			Pipeline().
			AddInput(payload.ContainerExecutionInput{Image: "alpine:latest"}).
			Build()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Name is required")
	})

	t.Run("requires mode", func(t *testing.T) {
		_, err := NewWorkflowBuilder().
			Name("x").
			AddInput(payload.ContainerExecutionInput{Image: "alpine:latest"}).
			Build()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Pipeline()")
	})

	t.Run("pipeline mode validates containers", func(t *testing.T) {
		_, err := NewWorkflowBuilder().
			Name("empty-pipeline").
			Pipeline().
			Build()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "at least one container")
	})

	t.Run("parallel mode validates containers", func(t *testing.T) {
		_, err := NewWorkflowBuilder().
			Name("empty-parallel").
			Parallel().
			Build()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "at least one container")
	})

	t.Run("single mode validates containers", func(t *testing.T) {
		_, err := NewWorkflowBuilder().
			Name("empty-single").
			Single().
			Build()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "at least one container")
	})
}

func TestWorkflowBuilder_buildSingleInput(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func() *WorkflowBuilder
		expectError bool
	}{
		{
			name: "valid single",
			setupFunc: func() *WorkflowBuilder {
				return NewWorkflowBuilder().
					AddInput(payload.ContainerExecutionInput{Image: "alpine:latest"})
			},
			expectError: false,
		},
		{
			name: "empty single",
			setupFunc: func() *WorkflowBuilder {
				return NewWorkflowBuilder()
			},
			expectError: true,
		},
		{
			name: "multiple containers returns first",
			setupFunc: func() *WorkflowBuilder {
				return NewWorkflowBuilder().
					AddInput(payload.ContainerExecutionInput{Image: "alpine:latest"}).
					AddInput(payload.ContainerExecutionInput{Image: "busybox:latest"})
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := tt.setupFunc()
			input, err := builder.buildSingleInput()

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
	builder := NewWorkflowBuilder().
		AddInput(payload.ContainerExecutionInput{Image: "alpine:latest"}).
		AddInput(payload.ContainerExecutionInput{Image: "busybox:latest"}).
		WithTimeout(5 * time.Minute)

	assert.Equal(t, 5*time.Minute, builder.containers[0].RunTimeout)
	assert.Equal(t, 5*time.Minute, builder.containers[1].RunTimeout)
}

func TestWorkflowBuilder_WithAutoRemove(t *testing.T) {
	builder := NewWorkflowBuilder().
		AddInput(payload.ContainerExecutionInput{Image: "alpine:latest"}).
		AddInput(payload.ContainerExecutionInput{Image: "busybox:latest"}).
		WithAutoRemove(true)

	assert.True(t, builder.containers[0].AutoRemove)
	assert.True(t, builder.containers[1].AutoRemove)
}

func TestWorkflowBuilder_ChainedCalls(t *testing.T) {
	// Test fluent API with chained calls producing a job.Definition
	def, err := NewWorkflowBuilder().
		Name("cicd-pipeline").
		Pipeline().
		AddInput(payload.ContainerExecutionInput{Image: "golang:1.25", Name: "build"}).
		AddInput(payload.ContainerExecutionInput{Image: "golang:1.25", Name: "test"}).
		AddInput(payload.ContainerExecutionInput{Image: "deployer:v1", Name: "deploy"}).
		StopOnError(true).
		Cleanup(true).
		WithTimeout(10 * time.Minute).
		WithAutoRemove(true).
		Build()

	require.NoError(t, err)
	assert.NotNil(t, def)
	assert.Equal(t, "cicd-pipeline", def.Name)
	assert.Equal(t, "container-cicd-pipeline", def.TaskQueue)
}

func TestContainerSource(t *testing.T) {
	input := payload.ContainerExecutionInput{
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
	source := WorkflowSourceFunc(func() payload.ContainerExecutionInput {
		return payload.ContainerExecutionInput{
			Image:   "alpine:latest",
			Command: []string{"echo", "test"},
		}
	})

	result := source.ToInput()
	assert.Equal(t, "alpine:latest", result.Image)
	assert.Equal(t, []string{"echo", "test"}, result.Command)
}

func TestWorkflowBuilder_ExecutionOptions(t *testing.T) {
	t.Run("single mode builds with RunTimeout without error", func(t *testing.T) {
		def, err := NewWorkflowBuilder().
			Name("derive").Single().
			AddInput(payload.ContainerExecutionInput{Image: "alpine", RunTimeout: 45 * time.Minute}).
			Build()
		require.NoError(t, err)
		in, ok := def.NewInput().(payload.ContainerExecutionInput)
		require.True(t, ok)
		_ = in // single mode: options live on the wrapper input; see pipeline case
	})

	t.Run("pipeline input carries derived options", func(t *testing.T) {
		def, err := NewWorkflowBuilder().
			Name("derive-pipe").Pipeline().
			AddInput(payload.ContainerExecutionInput{Image: "alpine", RunTimeout: 45 * time.Minute}).
			Build()
		require.NoError(t, err)
		in, ok := def.NewInput().(payload.PipelineInput)
		require.True(t, ok)
		require.NotNil(t, in.Options)
		assert.Equal(t, 47*time.Minute, in.Options.StartToCloseTimeout)
	})

	t.Run("derivation uses max RunTimeout across containers", func(t *testing.T) {
		def, err := NewWorkflowBuilder().
			Name("derive-max").Parallel().
			AddInput(payload.ContainerExecutionInput{Image: "alpine", RunTimeout: 5 * time.Minute}).
			AddInput(payload.ContainerExecutionInput{Image: "alpine", RunTimeout: 30 * time.Minute}).
			Build()
		require.NoError(t, err)
		in, ok := def.NewInput().(payload.ParallelInput)
		require.True(t, ok)
		require.NotNil(t, in.Options)
		assert.Equal(t, 32*time.Minute, in.Options.StartToCloseTimeout)
	})

	t.Run("no RunTimeout means no derived options", func(t *testing.T) {
		def, err := NewWorkflowBuilder().
			Name("no-derive").Pipeline().
			AddInput(payload.ContainerExecutionInput{Image: "alpine"}).
			Build()
		require.NoError(t, err)
		in, ok := def.NewInput().(payload.PipelineInput)
		require.True(t, ok)
		assert.Nil(t, in.Options)
	})

	t.Run("explicit StartToClose below max RunTimeout fails Build", func(t *testing.T) {
		_, err := NewWorkflowBuilder(WithExecutionOptions(&workflow.ExecutionOptions{StartToCloseTimeout: 5 * time.Minute})).
			Name("bad").Pipeline().
			AddInput(payload.ContainerExecutionInput{Image: "alpine", RunTimeout: 10 * time.Minute}).
			Build()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "start_to_close_timeout")
	})

	t.Run("explicit StartToClose above max RunTimeout passes", func(t *testing.T) {
		def, err := NewWorkflowBuilder().
			WithExecutionOptions(&workflow.ExecutionOptions{StartToCloseTimeout: 15 * time.Minute}).
			Name("good").Pipeline().
			AddInput(payload.ContainerExecutionInput{Image: "alpine", RunTimeout: 10 * time.Minute}).
			Build()
		require.NoError(t, err)
		in, ok := def.NewInput().(payload.PipelineInput)
		require.True(t, ok)
		require.NotNil(t, in.Options)
		assert.Equal(t, 15*time.Minute, in.Options.StartToCloseTimeout)
	})

	t.Run("RetryPolicy-only explicit options derive StartToClose from RunTimeout", func(t *testing.T) {
		rp := &temporal.RetryPolicy{MaximumAttempts: 5}
		explicit := &workflow.ExecutionOptions{RetryPolicy: rp}
		def, err := NewWorkflowBuilder().
			WithExecutionOptions(explicit).
			Name("retry-only").Pipeline().
			AddInput(payload.ContainerExecutionInput{Image: "alpine", RunTimeout: 45 * time.Minute}).
			Build()
		require.NoError(t, err)
		in, ok := def.NewInput().(payload.PipelineInput)
		require.True(t, ok)
		require.NotNil(t, in.Options)
		assert.Equal(t, 47*time.Minute, in.Options.StartToCloseTimeout)
		assert.Equal(t, rp, in.Options.RetryPolicy)
		// The caller's struct must not be mutated.
		assert.Zero(t, explicit.StartToCloseTimeout)
	})
}

func TestLoopBuilder_ExecutionOptions(t *testing.T) {
	t.Run("loop input carries derived options from template", func(t *testing.T) {
		def, err := NewLoopBuilder([]string{"a", "b"}).
			Name("loop-derive").
			WithTemplate(payload.ContainerExecutionInput{Image: "alpine", RunTimeout: 10 * time.Minute}).
			Build()
		require.NoError(t, err)
		in, ok := def.NewInput().(payload.LoopInput)
		require.True(t, ok)
		require.NotNil(t, in.Options)
		assert.Equal(t, 12*time.Minute, in.Options.StartToCloseTimeout)
	})

	t.Run("explicit StartToClose below template RunTimeout fails Build", func(t *testing.T) {
		_, err := NewParameterizedLoopBuilder(map[string][]string{"env": {"dev"}}).
			Name("loop-bad").
			WithTemplate(payload.ContainerExecutionInput{Image: "alpine", RunTimeout: 10 * time.Minute}).
			WithExecutionOptions(&workflow.ExecutionOptions{StartToCloseTimeout: 5 * time.Minute}).
			Build()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "start_to_close_timeout")
	})
}
