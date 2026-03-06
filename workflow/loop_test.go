package workflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
	wf "go.temporal.io/sdk/workflow"
)

// testSubstitutor is a simple substitutor that copies the template and sets the Name field.
func testSubstitutor(template testInput, item string, index int, params map[string]string) testInput {
	result := template
	if item != "" {
		result.Name = item
	}
	result.Value = SubstituteTemplate(template.Value, item, index, params)
	return result
}

// loopWrapper is a non-generic workflow wrapper for testing.
func loopWrapper(ctx wf.Context, input LoopInput[testInput]) (*LoopOutput[testOutput], error) {
	return LoopWorkflow[testInput, testOutput](ctx, input, testSubstitutor)
}

// parameterizedLoopWrapper is a non-generic workflow wrapper for testing.
func parameterizedLoopWrapper(ctx wf.Context, input ParameterizedLoopInput[testInput]) (*LoopOutput[testOutput], error) {
	return ParameterizedLoopWorkflow[testInput, testOutput](ctx, input, testSubstitutor)
}

func TestLoopWorkflow_SequentialSuccess(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerTestActivity(env)

	input := LoopInput[testInput]{
		Items: []string{"item1", "item2", "item3"},
		Template: testInput{
			Value:    "process-{{item}}",
			Activity: "TestActivity",
		},
		Parallel:        false,
		FailureStrategy: "continue",
	}

	env.OnActivity("TestActivity", mock.Anything, mock.Anything).Return(
		&testOutput{Result: "ok", Success: true}, nil)

	env.ExecuteWorkflow(loopWrapper, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result LoopOutput[testOutput]
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 3, result.ItemCount)
	assert.Equal(t, 3, result.TotalSuccess)
	assert.Equal(t, 0, result.TotalFailed)
}

func TestLoopWorkflow_ParallelSuccess(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerTestActivity(env)

	input := LoopInput[testInput]{
		Items: []string{"a", "b", "c"},
		Template: testInput{
			Value:    "process-{{item}}",
			Activity: "TestActivity",
		},
		Parallel:        true,
		FailureStrategy: "continue",
	}

	env.OnActivity("TestActivity", mock.Anything, mock.Anything).Return(
		&testOutput{Result: "ok", Success: true}, nil)

	env.ExecuteWorkflow(loopWrapper, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result LoopOutput[testOutput]
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 3, result.ItemCount)
	assert.Equal(t, 3, result.TotalSuccess)
	assert.Equal(t, 0, result.TotalFailed)
}

func TestLoopWorkflow_SequentialFailFast(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerTestActivity(env)

	input := LoopInput[testInput]{
		Items: []string{"a", "b", "c"},
		Template: testInput{
			Value:    "process-{{item}}",
			Activity: "TestActivity",
		},
		Parallel:        false,
		FailureStrategy: FailureStrategyFailFast,
	}

	// First succeeds
	env.OnActivity("TestActivity", mock.Anything, testInput{Name: "a", Value: "process-a", Activity: "TestActivity"}).Return(
		&testOutput{Result: "ok", Success: true}, nil).Once()

	// Second fails
	env.OnActivity("TestActivity", mock.Anything, testInput{Name: "b", Value: "process-b", Activity: "TestActivity"}).Return(
		&testOutput{Result: "fail", Success: false}, nil).Once()

	env.ExecuteWorkflow(loopWrapper, input)

	require.True(t, env.IsWorkflowCompleted())
	assert.Error(t, env.GetWorkflowError())
	assert.Contains(t, env.GetWorkflowError().Error(), "loop failed")
}

func TestLoopWorkflow_ContinueOnFailure(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerTestActivity(env)

	input := LoopInput[testInput]{
		Items: []string{"a", "b", "c"},
		Template: testInput{
			Value:    "process-{{item}}",
			Activity: "TestActivity",
		},
		Parallel:        false,
		FailureStrategy: FailureStrategyContinue,
	}

	// First succeeds
	env.OnActivity("TestActivity", mock.Anything, testInput{Name: "a", Value: "process-a", Activity: "TestActivity"}).Return(
		&testOutput{Result: "ok", Success: true}, nil).Once()

	// Second fails
	env.OnActivity("TestActivity", mock.Anything, testInput{Name: "b", Value: "process-b", Activity: "TestActivity"}).Return(
		&testOutput{Result: "fail", Success: false}, nil).Once()

	// Third succeeds
	env.OnActivity("TestActivity", mock.Anything, testInput{Name: "c", Value: "process-c", Activity: "TestActivity"}).Return(
		&testOutput{Result: "ok", Success: true}, nil).Once()

	env.ExecuteWorkflow(loopWrapper, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result LoopOutput[testOutput]
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 2, result.TotalSuccess)
	assert.Equal(t, 1, result.TotalFailed)
	assert.Len(t, result.Results, 3)
}

func TestLoopWorkflow_InvalidInput(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := LoopInput[testInput]{
		Items: []string{},
		Template: testInput{
			Value:    "test",
			Activity: "TestActivity",
		},
	}

	env.ExecuteWorkflow(loopWrapper, input)

	require.True(t, env.IsWorkflowCompleted())
	assert.Error(t, env.GetWorkflowError())
}

func TestParameterizedLoopWorkflow_Success(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerTestActivity(env)

	input := ParameterizedLoopInput[testInput]{
		Parameters: map[string][]string{
			"env": {"dev", "prod"},
		},
		Template: testInput{
			Value:    "deploy-{{.env}}",
			Activity: "TestActivity",
		},
		Parallel:        false,
		FailureStrategy: "continue",
	}

	env.OnActivity("TestActivity", mock.Anything, mock.Anything).Return(
		&testOutput{Result: "ok", Success: true}, nil)

	env.ExecuteWorkflow(parameterizedLoopWrapper, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result LoopOutput[testOutput]
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 2, result.ItemCount)
	assert.Equal(t, 2, result.TotalSuccess)
	assert.Equal(t, 0, result.TotalFailed)
}

func TestParameterizedLoopWorkflow_ParallelSuccess(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerTestActivity(env)

	input := ParameterizedLoopInput[testInput]{
		Parameters: map[string][]string{
			"env":    {"dev", "prod"},
			"region": {"us", "eu"},
		},
		Template: testInput{
			Value:    "deploy-{{.env}}-{{.region}}",
			Activity: "TestActivity",
		},
		Parallel:        true,
		FailureStrategy: "continue",
	}

	env.OnActivity("TestActivity", mock.Anything, mock.Anything).Return(
		&testOutput{Result: "ok", Success: true}, nil)

	env.ExecuteWorkflow(parameterizedLoopWrapper, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result LoopOutput[testOutput]
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 4, result.ItemCount)
	assert.Equal(t, 4, result.TotalSuccess)
}

func TestParameterizedLoopWorkflow_InvalidInput(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := ParameterizedLoopInput[testInput]{
		Parameters: map[string][]string{},
		Template: testInput{
			Value:    "test",
			Activity: "TestActivity",
		},
	}

	env.ExecuteWorkflow(parameterizedLoopWrapper, input)

	require.True(t, env.IsWorkflowCompleted())
	assert.Error(t, env.GetWorkflowError())
}
