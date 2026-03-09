package builder

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jasoet/go-wf/function/payload"
)

func TestWorkflowBuilder_BuildPipeline(t *testing.T) {
	input, err := NewWorkflowBuilder("test-pipeline").
		AddInput(payload.FunctionExecutionInput{Name: "step1"}).
		AddInput(payload.FunctionExecutionInput{Name: "step2"}).
		StopOnError(true).
		BuildPipeline()

	require.NoError(t, err)
	require.NotNil(t, input)

	assert.Len(t, input.Functions, 2)
	assert.True(t, input.StopOnError)
}

func TestWorkflowBuilder_BuildParallel(t *testing.T) {
	input, err := NewWorkflowBuilder("test-parallel").
		AddInput(payload.FunctionExecutionInput{Name: "task-a"}).
		AddInput(payload.FunctionExecutionInput{Name: "task-b"}).
		Parallel(true).
		FailFast(true).
		MaxConcurrency(5).
		BuildParallel()

	require.NoError(t, err)
	require.NotNil(t, input)

	assert.Len(t, input.Functions, 2)
	assert.Equal(t, "fail_fast", input.FailureStrategy)
	assert.Equal(t, 5, input.MaxConcurrency)
}

func TestWorkflowBuilder_BuildSingle(t *testing.T) {
	input, err := NewWorkflowBuilder("single").
		AddInput(payload.FunctionExecutionInput{Name: "only-one"}).
		BuildSingle()

	require.NoError(t, err)
	require.NotNil(t, input)

	assert.Equal(t, "only-one", input.Name)
}

func TestWorkflowBuilder_Build_AutoSelectsPipeline(t *testing.T) {
	result, err := NewWorkflowBuilder("auto").
		AddInput(payload.FunctionExecutionInput{Name: "step1"}).
		Build()

	require.NoError(t, err)

	_, ok := result.(*payload.PipelineInput)
	assert.True(t, ok, "Expected PipelineInput for non-parallel mode")
}

func TestWorkflowBuilder_Build_AutoSelectsParallel(t *testing.T) {
	result, err := NewWorkflowBuilder("auto").
		AddInput(payload.FunctionExecutionInput{Name: "step1"}).
		Parallel(true).
		Build()

	require.NoError(t, err)

	_, ok := result.(*payload.ParallelInput)
	assert.True(t, ok, "Expected ParallelInput for parallel mode")
}

func TestWorkflowBuilder_EmptyError(t *testing.T) {
	_, err := NewWorkflowBuilder("empty").BuildPipeline()
	assert.Error(t, err)
}

func TestWorkflowBuilder_NilSourceError(t *testing.T) {
	_, err := NewWorkflowBuilder("nil-source").
		Add(nil).
		AddInput(payload.FunctionExecutionInput{Name: "ok"}).
		BuildPipeline()
	assert.Error(t, err)
}

func TestWorkflowBuilder_WithSource(t *testing.T) {
	source := NewFunctionSource(payload.FunctionExecutionInput{Name: "from-source"})

	input, err := NewWorkflowBuilder("with-source").
		Add(source).
		BuildSingle()

	require.NoError(t, err)
	assert.Equal(t, "from-source", input.Name)
}

func TestWorkflowBuilder_WithOptions(t *testing.T) {
	b := NewWorkflowBuilder("opts",
		WithStopOnError(false),
		WithParallelMode(true),
		WithFailFast(true),
		WithMaxConcurrency(10),
	)

	b.AddInput(payload.FunctionExecutionInput{Name: "a"})

	input, err := b.BuildParallel()
	require.NoError(t, err)

	assert.Equal(t, "fail_fast", input.FailureStrategy)
	assert.Equal(t, 10, input.MaxConcurrency)
}

func TestWorkflowBuilder_Count(t *testing.T) {
	b := NewWorkflowBuilder("count")
	assert.Equal(t, 0, b.Count())

	b.AddInput(payload.FunctionExecutionInput{Name: "a"})
	assert.Equal(t, 1, b.Count())
}
