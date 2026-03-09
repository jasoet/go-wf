package builder

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jasoet/go-wf/function/payload"
)

func TestLoopBuilder_BuildLoop(t *testing.T) {
	input, err := NewLoopBuilder([]string{"a", "b", "c"}).
		WithTemplate(payload.FunctionExecutionInput{Name: "process-{{item}}"}).
		Parallel(true).
		FailFast(true).
		MaxConcurrency(2).
		BuildLoop()

	require.NoError(t, err)
	require.NotNil(t, input)

	assert.Len(t, input.Items, 3)
	assert.Equal(t, "process-{{item}}", input.Template.Name)
	assert.True(t, input.Parallel)
	assert.Equal(t, "fail_fast", input.FailureStrategy)
}

func TestLoopBuilder_BuildParameterizedLoop(t *testing.T) {
	input, err := NewParameterizedLoopBuilder(map[string][]string{
		"env":    {"dev", "prod"},
		"region": {"us", "eu"},
	}).
		WithTemplate(payload.FunctionExecutionInput{
			Name: "deploy",
			Args: map[string]string{"target": "{{.env}}-{{.region}}"},
		}).
		BuildParameterizedLoop()

	require.NoError(t, err)
	require.NotNil(t, input)

	assert.Len(t, input.Parameters, 2)
	assert.Equal(t, "deploy", input.Template.Name)
}

func TestLoopBuilder_EmptyItemsError(t *testing.T) {
	_, err := NewLoopBuilder([]string{}).
		WithTemplate(payload.FunctionExecutionInput{Name: "test"}).
		BuildLoop()
	assert.Error(t, err)
}

func TestLoopBuilder_EmptyParametersError(t *testing.T) {
	_, err := NewParameterizedLoopBuilder(map[string][]string{}).
		WithTemplate(payload.FunctionExecutionInput{Name: "test"}).
		BuildParameterizedLoop()
	assert.Error(t, err)
}

func TestForEach(t *testing.T) {
	lb := ForEach([]string{"x", "y"}, payload.FunctionExecutionInput{Name: "fn"})
	input, err := lb.BuildLoop()
	require.NoError(t, err)
	assert.Len(t, input.Items, 2)
}

func TestForEachParam(t *testing.T) {
	lb := ForEachParam(
		map[string][]string{"v": {"1", "2"}},
		payload.FunctionExecutionInput{Name: "fn"},
	)
	input, err := lb.BuildParameterizedLoop()
	require.NoError(t, err)
	assert.Len(t, input.Parameters, 1)
}

func TestLoopBuilder_NilSourceError(t *testing.T) {
	_, err := NewLoopBuilder([]string{"a"}).
		WithSource(nil).
		BuildLoop()
	assert.Error(t, err)
}

func TestLoopBuilder_WithSource(t *testing.T) {
	source := NewFunctionSource(payload.FunctionExecutionInput{Name: "from-source"})
	input, err := NewLoopBuilder([]string{"a"}).
		WithSource(source).
		BuildLoop()

	require.NoError(t, err)
	assert.Equal(t, "from-source", input.Template.Name)
}
