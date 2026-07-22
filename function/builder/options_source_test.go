package builder

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jasoet/go-wf/function/payload"
	"github.com/jasoet/go-wf/workflow"
)

func fnInput(name string) *payload.FunctionExecutionInput {
	return &payload.FunctionExecutionInput{Name: name}
}

func TestBuilderOptions(t *testing.T) {
	t.Run("WithStopOnError", func(t *testing.T) {
		b := NewFunctionBuilder()
		WithStopOnError[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput](false)(b)
		assert.False(t, b.stopOnError)
	})

	t.Run("WithParallelMode is a no-op", func(t *testing.T) {
		b := NewFunctionBuilder()
		WithParallelMode[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput](true)(b)
		assert.Equal(t, modeUnset, b.mode)
	})

	t.Run("WithFailFast", func(t *testing.T) {
		b := NewFunctionBuilder()
		WithFailFast[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput](true)(b)
		assert.True(t, b.failFast)
	})

	t.Run("WithMaxConcurrency", func(t *testing.T) {
		b := NewFunctionBuilder()
		WithMaxConcurrency[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput](8)(b)
		assert.Equal(t, 8, b.maxConcurrency)
	})

	t.Run("WithExecutionOptions", func(t *testing.T) {
		opts := &workflow.ExecutionOptions{StartToCloseTimeout: time.Minute}
		b := NewFunctionBuilder()
		WithExecutionOptions[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput](opts)(b)
		assert.Equal(t, opts, b.executionOptions)
	})
}

func TestWorkflowBuilder_Errors(t *testing.T) {
	b := NewFunctionBuilder()
	assert.Empty(t, b.Errors())
}

func TestFunctionSource(t *testing.T) {
	input := payload.FunctionExecutionInput{
		Name: "echo",
		Args: map[string]string{"k": "v"},
	}

	src := NewFunctionSource(input)
	got := src.ToInput()

	assert.Equal(t, "echo", got.Name)
	assert.Equal(t, map[string]string{"k": "v"}, got.Args)
}

func TestWorkflowSourceFunc(t *testing.T) {
	src := WorkflowSourceFunc(func() payload.FunctionExecutionInput {
		return payload.FunctionExecutionInput{Name: "from-func"}
	})

	assert.Equal(t, "from-func", src.ToInput().Name)
}

func TestLoopBuilder_TaskQueue(t *testing.T) {
	def, err := NewFunctionLoopBuilder([]string{"a"}).
		Name("tq-loop").
		TaskQueue("custom-queue").
		Activity(func() {}).
		WithTemplate(fnInput("echo")).
		Build()

	require.NoError(t, err)
	assert.Equal(t, "custom-queue", def.TaskQueue)
}

func TestLoopBuilder_ModeOverrides(t *testing.T) {
	t.Run("ParameterizedLoop overrides item loop mode", func(t *testing.T) {
		lb := NewFunctionLoopBuilder([]string{"a"}).ParameterizedLoop()
		assert.Equal(t, loopModeParameterizedLoop, lb.mode)
	})

	t.Run("Loop overrides parameterized mode", func(t *testing.T) {
		lb := NewFunctionParameterizedLoopBuilder(map[string][]string{"env": {"dev"}}).Loop()
		assert.Equal(t, loopModeLoop, lb.mode)
	})

	t.Run("Build uses overridden mode", func(t *testing.T) {
		def, err := NewFunctionParameterizedLoopBuilder(map[string][]string{"env": {"dev"}}).
			Name("overridden").
			Activity(func() {}).
			WithTemplate(fnInput("echo")).
			Loop().
			WithTemplate(fnInput("echo")).
			Build()
		// Loop mode with no items must fail validation.
		_ = def
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least one item")
	})
}

func TestLoopBuilder_PublicBuildInputs(t *testing.T) {
	t.Run("BuildLoop success", func(t *testing.T) {
		input, err := NewFunctionLoopBuilder([]string{"a", "b"}).
			WithTemplate(fnInput("echo")).
			Parallel(true).
			MaxConcurrency(2).
			BuildLoop()
		require.NoError(t, err)
		require.NotNil(t, input)
		assert.Equal(t, []string{"a", "b"}, input.Items)
		assert.True(t, input.Parallel)
		assert.Equal(t, 2, input.MaxConcurrency)
		assert.Equal(t, FailureStrategyContinue, input.FailureStrategy)
	})

	t.Run("BuildLoop fail fast", func(t *testing.T) {
		input, err := NewFunctionLoopBuilder([]string{"a"}).
			WithTemplate(fnInput("echo")).
			FailFast(true).
			BuildLoop()
		require.NoError(t, err)
		assert.Equal(t, FailureStrategyFailFast, input.FailureStrategy)
	})

	t.Run("BuildParameterizedLoop success", func(t *testing.T) {
		input, err := NewFunctionParameterizedLoopBuilder(map[string][]string{"env": {"dev", "prod"}}).
			WithTemplate(fnInput("deploy")).
			BuildParameterizedLoop()
		require.NoError(t, err)
		require.NotNil(t, input)
		assert.Len(t, input.Parameters, 1)
		assert.Equal(t, FailureStrategyContinue, input.FailureStrategy)
	})

	t.Run("BuildParameterizedLoop invalid template fails validation", func(t *testing.T) {
		_, err := NewFunctionParameterizedLoopBuilder(map[string][]string{"env": {"dev"}}).
			WithTemplate(&payload.FunctionExecutionInput{}).
			BuildParameterizedLoop()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parameterized loop validation failed")
	})

	t.Run("BuildLoop invalid template fails validation", func(t *testing.T) {
		_, err := NewFunctionLoopBuilder([]string{"a"}).
			WithTemplate(&payload.FunctionExecutionInput{}).
			BuildLoop()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "loop validation failed")
	})
}
