package builder

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jasoet/go-wf/function/payload"
)

// dummyActivity is a no-op stand-in for the activity function required by Build.
func dummyActivity() {}

func TestWorkflowBuilder_Build_Pipeline(t *testing.T) {
	def, err := NewFunctionBuilder().
		Name("test-pipeline").
		Activity(dummyActivity).
		Pipeline().
		Add(&payload.FunctionExecutionInput{Name: "step1"}).
		Add(&payload.FunctionExecutionInput{Name: "step2"}).
		StopOnError(true).
		Build()

	require.NoError(t, err)
	require.NotNil(t, def)

	assert.Equal(t, "test-pipeline", def.Name)
	assert.Equal(t, "function-test-pipeline", def.TaskQueue)
}

func TestWorkflowBuilder_Build_Parallel(t *testing.T) {
	def, err := NewFunctionBuilder().
		Name("test-parallel").
		Activity(dummyActivity).
		Parallel().
		Add(&payload.FunctionExecutionInput{Name: "task-a"}).
		Add(&payload.FunctionExecutionInput{Name: "task-b"}).
		FailFast(true).
		MaxConcurrency(5).
		Build()

	require.NoError(t, err)
	require.NotNil(t, def)

	assert.Equal(t, "test-parallel", def.Name)
	assert.Equal(t, "function-test-parallel", def.TaskQueue)
}

func TestWorkflowBuilder_Build_Single(t *testing.T) {
	def, err := NewFunctionBuilder().
		Name("single").
		Activity(dummyActivity).
		Single().
		Add(&payload.FunctionExecutionInput{Name: "only-one"}).
		Build()

	require.NoError(t, err)
	require.NotNil(t, def)

	assert.Equal(t, "single", def.Name)
	assert.Equal(t, "function-single", def.TaskQueue)
}

func TestWorkflowBuilder_Build_CustomTaskQueue(t *testing.T) {
	def, err := NewFunctionBuilder().
		Name("custom").
		TaskQueue("my-special-queue").
		Activity(dummyActivity).
		Pipeline().
		Add(&payload.FunctionExecutionInput{Name: "step1"}).
		Build()

	require.NoError(t, err)
	require.NotNil(t, def)

	assert.Equal(t, "my-special-queue", def.TaskQueue)
}

func TestWorkflowBuilder_Build_RequiresName(t *testing.T) {
	_, err := NewFunctionBuilder().
		Activity(dummyActivity).
		Pipeline().
		Add(&payload.FunctionExecutionInput{Name: "step1"}).
		Build()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Name is required")
}

func TestWorkflowBuilder_Build_RequiresMode(t *testing.T) {
	_, err := NewFunctionBuilder().
		Name("x").
		Activity(dummyActivity).
		Add(&payload.FunctionExecutionInput{Name: "step1"}).
		Build()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), ".Pipeline()")
}

func TestWorkflowBuilder_Build_RequiresActivity(t *testing.T) {
	_, err := NewFunctionBuilder().
		Name("x").
		Pipeline().
		Add(&payload.FunctionExecutionInput{Name: "step1"}).
		Build()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Activity is required")
}

func TestWorkflowBuilder_Build_RequiresInputs(t *testing.T) {
	_, err := NewFunctionBuilder().
		Name("empty").
		Activity(dummyActivity).
		Pipeline().
		Build()

	assert.Error(t, err)
}

func TestWorkflowBuilder_NewInput_Pipeline(t *testing.T) {
	def, err := NewFunctionBuilder().
		Name("ni-pipeline").
		Activity(dummyActivity).
		Pipeline().
		Add(&payload.FunctionExecutionInput{Name: "step1"}).
		Build()

	require.NoError(t, err)
	in := def.NewInput()
	require.NotNil(t, in)
}

func TestWorkflowBuilder_NewInput_Parallel(t *testing.T) {
	def, err := NewFunctionBuilder().
		Name("ni-parallel").
		Activity(dummyActivity).
		Parallel().
		Add(&payload.FunctionExecutionInput{Name: "step1"}).
		Build()

	require.NoError(t, err)
	in := def.NewInput()
	require.NotNil(t, in)
}

func TestWorkflowBuilder_WithOptions(t *testing.T) {
	def, err := NewFunctionBuilder().
		Name("opts").
		Activity(dummyActivity).
		Parallel().
		FailFast(true).
		MaxConcurrency(10).
		Add(&payload.FunctionExecutionInput{Name: "a"}).
		Build()

	require.NoError(t, err)
	assert.Equal(t, "opts", def.Name)
}

func TestWorkflowBuilder_Count(t *testing.T) {
	b := NewFunctionBuilder().Name("count")
	assert.Equal(t, 0, b.Count())

	b.Add(&payload.FunctionExecutionInput{Name: "a"})
	assert.Equal(t, 1, b.Count())
}

func TestWorkflowBuilder_WithSource(t *testing.T) {
	source := NewFunctionSource(payload.FunctionExecutionInput{Name: "from-source"})
	fnInput := source.ToInput()

	def, err := NewFunctionBuilder().
		Name("with-source").
		Activity(dummyActivity).
		Single().
		Add(&fnInput).
		Build()

	require.NoError(t, err)
	require.NotNil(t, def)
	assert.Equal(t, "with-source", def.Name)
}
