package builder

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jasoet/go-wf/function/payload"
	"github.com/jasoet/go-wf/workflow"
)

func TestWorkflowBuilder_BuildPipeline(t *testing.T) {
	input, err := NewFunctionBuilder("test-pipeline").
		Add(&payload.FunctionExecutionInput{Name: "step1"}).
		Add(&payload.FunctionExecutionInput{Name: "step2"}).
		StopOnError(true).
		BuildPipeline()

	require.NoError(t, err)
	require.NotNil(t, input)

	assert.Len(t, input.Tasks, 2)
	assert.True(t, input.StopOnError)
}

func TestWorkflowBuilder_BuildParallel(t *testing.T) {
	input, err := NewFunctionBuilder("test-parallel").
		Add(&payload.FunctionExecutionInput{Name: "task-a"}).
		Add(&payload.FunctionExecutionInput{Name: "task-b"}).
		Parallel(true).
		FailFast(true).
		MaxConcurrency(5).
		BuildParallel()

	require.NoError(t, err)
	require.NotNil(t, input)

	assert.Len(t, input.Tasks, 2)
	assert.Equal(t, "fail_fast", input.FailureStrategy)
	assert.Equal(t, 5, input.MaxConcurrency)
}

func TestWorkflowBuilder_BuildSingle(t *testing.T) {
	input, err := NewFunctionBuilder("single").
		Add(&payload.FunctionExecutionInput{Name: "only-one"}).
		BuildSingle()

	require.NoError(t, err)
	require.NotNil(t, input)

	assert.Equal(t, "only-one", (*input).Name)
}

func TestWorkflowBuilder_Build_AutoSelectsPipeline(t *testing.T) {
	result, err := NewFunctionBuilder("auto").
		Add(&payload.FunctionExecutionInput{Name: "step1"}).
		Build()

	require.NoError(t, err)

	_, ok := result.(*workflow.PipelineInput[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput])
	assert.True(t, ok, "Expected PipelineInput for non-parallel mode")
}

func TestWorkflowBuilder_Build_AutoSelectsParallel(t *testing.T) {
	result, err := NewFunctionBuilder("auto").
		Add(&payload.FunctionExecutionInput{Name: "step1"}).
		Parallel(true).
		Build()

	require.NoError(t, err)

	_, ok := result.(*workflow.ParallelInput[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput])
	assert.True(t, ok, "Expected ParallelInput for parallel mode")
}

func TestWorkflowBuilder_EmptyError(t *testing.T) {
	_, err := NewFunctionBuilder("empty").BuildPipeline()
	assert.Error(t, err)
}

func TestWorkflowBuilder_WithSource(t *testing.T) {
	source := NewFunctionSource(payload.FunctionExecutionInput{Name: "from-source"})
	fnInput := source.ToInput()

	input, err := NewFunctionBuilder("with-source").
		Add(&fnInput).
		BuildSingle()

	require.NoError(t, err)
	assert.Equal(t, "from-source", (*input).Name)
}

func TestWorkflowBuilder_WithOptions(t *testing.T) {
	b := NewFunctionBuilder("opts")
	b.StopOnError(false).
		Parallel(true).
		FailFast(true).
		MaxConcurrency(10)

	b.Add(&payload.FunctionExecutionInput{Name: "a"})

	input, err := b.BuildParallel()
	require.NoError(t, err)

	assert.Equal(t, "fail_fast", input.FailureStrategy)
	assert.Equal(t, 10, input.MaxConcurrency)
}

func TestWorkflowBuilder_Count(t *testing.T) {
	b := NewFunctionBuilder("count")
	assert.Equal(t, 0, b.Count())

	b.Add(&payload.FunctionExecutionInput{Name: "a"})
	assert.Equal(t, 1, b.Count())
}
