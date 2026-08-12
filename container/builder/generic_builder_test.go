package builder

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jasoet/go-wf/v2/container/payload"
	"github.com/jasoet/go-wf/v2/workflow"
)

// newTestGenericBuilder returns a GenericBuilder over container payload types.
func newTestGenericBuilder() *GenericBuilder[*payload.ContainerExecutionInput, payload.ContainerExecutionOutput] {
	return NewGenericBuilder[*payload.ContainerExecutionInput, payload.ContainerExecutionOutput]()
}

func validContainerPtr(image string) *payload.ContainerExecutionInput {
	return &payload.ContainerExecutionInput{Image: image}
}

func TestGenericBuilder_NewDefaults(t *testing.T) {
	b := newTestGenericBuilder()

	assert.NotNil(t, b.inputs)
	assert.True(t, b.stopOnError)
	assert.Equal(t, 0, b.Count())
	assert.Empty(t, b.Errors())
}

func TestGenericBuilder_FluentSetters(t *testing.T) {
	b := newTestGenericBuilder().
		Add(validContainerPtr("alpine:latest")).
		Add(validContainerPtr("busybox:latest")).
		StopOnError(false).
		Cleanup(true).
		FailFast(true).
		MaxConcurrency(4)

	assert.Equal(t, 2, b.Count())
	assert.False(t, b.stopOnError)
	assert.True(t, b.cleanup)
	assert.True(t, b.failFast)
	assert.Equal(t, 4, b.maxConcurrency)
}

func TestGenericBuilder_BuildPipeline_Success(t *testing.T) {
	b := newTestGenericBuilder().
		Add(validContainerPtr("alpine:latest")).
		Add(validContainerPtr("busybox:latest"))

	input, err := b.BuildPipeline()
	require.NoError(t, err)
	require.NotNil(t, input)
	assert.Len(t, input.Tasks, 2)
	assert.True(t, input.StopOnError)
	assert.False(t, input.Cleanup)
}

func TestGenericBuilder_BuildPipeline_CustomSettings(t *testing.T) {
	b := newTestGenericBuilder().
		Add(validContainerPtr("alpine:latest")).
		StopOnError(false).
		Cleanup(true)

	input, err := b.BuildPipeline()
	require.NoError(t, err)
	assert.False(t, input.StopOnError)
	assert.True(t, input.Cleanup)
}

func TestGenericBuilder_BuildPipeline_Errors(t *testing.T) {
	t.Run("no inputs", func(t *testing.T) {
		_, err := newTestGenericBuilder().BuildPipeline()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least one input")
	})

	t.Run("invalid task fails validation", func(t *testing.T) {
		_, err := newTestGenericBuilder().
			Add(&payload.ContainerExecutionInput{}).
			BuildPipeline()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "pipeline validation failed")
	})
}

func TestGenericBuilder_BuildParallel(t *testing.T) {
	t.Run("default failure strategy is continue", func(t *testing.T) {
		input, err := newTestGenericBuilder().
			Add(validContainerPtr("alpine:latest")).
			BuildParallel()
		require.NoError(t, err)
		require.NotNil(t, input)
		assert.Len(t, input.Tasks, 1)
		assert.Equal(t, FailureStrategyContinue, input.FailureStrategy)
		assert.Equal(t, 0, input.MaxConcurrency)
	})

	t.Run("fail fast and max concurrency propagate", func(t *testing.T) {
		input, err := newTestGenericBuilder().
			Add(validContainerPtr("alpine:latest")).
			FailFast(true).
			MaxConcurrency(3).
			BuildParallel()
		require.NoError(t, err)
		assert.Equal(t, FailureStrategyFailFast, input.FailureStrategy)
		assert.Equal(t, 3, input.MaxConcurrency)
	})

	t.Run("no inputs", func(t *testing.T) {
		_, err := newTestGenericBuilder().BuildParallel()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least one input")
	})

	t.Run("invalid task fails validation", func(t *testing.T) {
		_, err := newTestGenericBuilder().
			Add(&payload.ContainerExecutionInput{}).
			BuildParallel()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parallel validation failed")
	})
}

func TestGenericBuilder_BuildSingle(t *testing.T) {
	t.Run("returns first input", func(t *testing.T) {
		input, err := newTestGenericBuilder().
			Add(validContainerPtr("alpine:latest")).
			Add(validContainerPtr("busybox:latest")).
			BuildSingle()
		require.NoError(t, err)
		require.NotNil(t, input)
		assert.Equal(t, "alpine:latest", (*input).Image)
	})

	t.Run("no inputs", func(t *testing.T) {
		_, err := newTestGenericBuilder().BuildSingle()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least one input")
	})

	t.Run("invalid input fails validation", func(t *testing.T) {
		_, err := newTestGenericBuilder().
			Add(&payload.ContainerExecutionInput{}).
			BuildSingle()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "single validation failed")
	})
}

func TestWorkflowBuilder_PublicBuildInputs(t *testing.T) {
	t.Run("BuildPipeline", func(t *testing.T) {
		input, err := NewWorkflowBuilder().
			AddInput(payload.ContainerExecutionInput{Image: "alpine:latest"}).
			BuildPipeline()
		require.NoError(t, err)
		require.NotNil(t, input)
		assert.Len(t, input.Containers, 1)
	})

	t.Run("BuildPipeline propagates accumulated error", func(t *testing.T) {
		_, err := NewWorkflowBuilder().Add(nil).BuildPipeline()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "nil source")
	})

	t.Run("BuildParallel", func(t *testing.T) {
		input, err := NewWorkflowBuilder().
			FailFast(true).
			MaxConcurrency(2).
			AddInput(payload.ContainerExecutionInput{Image: "alpine:latest"}).
			BuildParallel()
		require.NoError(t, err)
		require.NotNil(t, input)
		assert.Equal(t, FailureStrategyFailFast, input.FailureStrategy)
		assert.Equal(t, 2, input.MaxConcurrency)
	})

	t.Run("BuildParallel propagates accumulated error", func(t *testing.T) {
		_, err := NewWorkflowBuilder().Add(nil).BuildParallel()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "nil source")
	})

	t.Run("BuildSingle", func(t *testing.T) {
		input, err := NewWorkflowBuilder().
			AddInput(payload.ContainerExecutionInput{Image: "alpine:latest"}).
			BuildSingle()
		require.NoError(t, err)
		require.NotNil(t, input)
		assert.Equal(t, "alpine:latest", input.Image)
	})

	t.Run("BuildSingle propagates accumulated error", func(t *testing.T) {
		_, err := NewWorkflowBuilder().Add(nil).BuildSingle()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "nil source")
	})
}

func TestWorkflowBuilder_GenericBuildInputs(t *testing.T) {
	t.Run("BuildGenericPipeline", func(t *testing.T) {
		input, err := NewWorkflowBuilder().
			AddInput(payload.ContainerExecutionInput{Image: "alpine:latest"}).
			Cleanup(true).
			BuildGenericPipeline()
		require.NoError(t, err)
		require.NotNil(t, input)
		assert.Len(t, input.Tasks, 1)
		assert.Equal(t, "alpine:latest", input.Tasks[0].Image)
		assert.True(t, input.StopOnError)
		assert.True(t, input.Cleanup)
	})

	t.Run("BuildGenericPipeline empty fails", func(t *testing.T) {
		_, err := NewWorkflowBuilder().BuildGenericPipeline()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least one container")
	})

	t.Run("BuildGenericPipeline propagates accumulated error", func(t *testing.T) {
		_, err := NewWorkflowBuilder().Add(nil).BuildGenericPipeline()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "nil source")
	})

	t.Run("BuildGenericPipeline invalid container fails validation", func(t *testing.T) {
		_, err := NewWorkflowBuilder().
			AddInput(payload.ContainerExecutionInput{}).
			BuildGenericPipeline()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "pipeline validation failed")
	})

	t.Run("BuildGenericParallel", func(t *testing.T) {
		input, err := NewWorkflowBuilder().
			FailFast(true).
			MaxConcurrency(7).
			AddInput(payload.ContainerExecutionInput{Image: "alpine:latest"}).
			BuildGenericParallel()
		require.NoError(t, err)
		require.NotNil(t, input)
		assert.Len(t, input.Tasks, 1)
		assert.Equal(t, FailureStrategyFailFast, input.FailureStrategy)
		assert.Equal(t, 7, input.MaxConcurrency)
	})

	t.Run("BuildGenericParallel empty fails", func(t *testing.T) {
		_, err := NewWorkflowBuilder().BuildGenericParallel()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least one container")
	})

	t.Run("BuildGenericParallel propagates accumulated error", func(t *testing.T) {
		_, err := NewWorkflowBuilder().Add(nil).BuildGenericParallel()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "nil source")
	})

	t.Run("BuildGenericParallel invalid container fails validation", func(t *testing.T) {
		_, err := NewWorkflowBuilder().
			AddInput(payload.ContainerExecutionInput{}).
			BuildGenericParallel()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parallel validation failed")
	})
}

func TestLoopBuilder_ModeOverrides(t *testing.T) {
	t.Run("ParameterizedLoop on item builder switches workflow type", func(t *testing.T) {
		b := NewLoopBuilder([]string{"a"}).
			WithTemplate(payload.ContainerExecutionInput{Image: "alpine:latest"}).
			ParameterizedLoop()
		assert.Equal(t, loopModeParameterizedLoop, b.mode)
	})

	t.Run("Loop on parameterized builder switches workflow type", func(t *testing.T) {
		b := NewParameterizedLoopBuilder(map[string][]string{"env": {"dev"}}).
			WithTemplate(payload.ContainerExecutionInput{Image: "alpine:latest"}).
			Loop()
		assert.Equal(t, loopModeLoop, b.mode)
	})

	t.Run("build after override uses new mode", func(t *testing.T) {
		def, err := NewLoopBuilder([]string{"a"}).
			Name("override").
			WithTemplate(payload.ContainerExecutionInput{Image: "alpine:latest"}).
			Loop().
			Build()
		require.NoError(t, err)
		in, ok := def.NewInput().(payload.LoopInput)
		require.True(t, ok)
		assert.Equal(t, []string{"a"}, in.Items)
	})
}

func TestGenericSourceAdapters(t *testing.T) {
	t.Run("GenericSourceFunc", func(t *testing.T) {
		src := GenericSourceFunc[*payload.ContainerExecutionInput](func() *payload.ContainerExecutionInput {
			return validContainerPtr("alpine:latest")
		})
		assert.Equal(t, "alpine:latest", src.ToTaskInput().Image)
	})

	t.Run("TaskInputSource", func(t *testing.T) {
		src := NewTaskInputSource[*payload.ContainerExecutionInput](validContainerPtr("busybox:latest"))
		assert.Equal(t, "busybox:latest", src.ToTaskInput().Image)
	})
}

func TestBuilderOptions_ModeSelectors(t *testing.T) {
	t.Run("WithPipelineMode", func(t *testing.T) {
		b := NewWorkflowBuilder(WithPipelineMode())
		assert.Equal(t, modePipeline, b.mode)
	})

	t.Run("WithSingleMode", func(t *testing.T) {
		b := NewWorkflowBuilder(WithSingleMode())
		assert.Equal(t, modeSingle, b.mode)
	})

	t.Run("WithExecutionOptions", func(t *testing.T) {
		opts := &workflow.ExecutionOptions{StartToCloseTimeout: 5 * time.Minute}
		b := NewWorkflowBuilder(WithExecutionOptions(opts))
		assert.Equal(t, opts, b.executionOptions)
	})
}
