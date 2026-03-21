package workflow

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
	wf "go.temporal.io/sdk/workflow"
)

type OtelWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
}

func TestOtelWorkflowTestSuite(t *testing.T) {
	suite.Run(t, new(OtelWorkflowTestSuite))
}

// instrumentedPipelineWrapper is a non-generic workflow wrapper for testing.
func instrumentedPipelineWrapper(ctx wf.Context, input PipelineInput[testInput]) (*PipelineOutput[testOutput], error) {
	return InstrumentedPipelineWorkflow[testInput, testOutput](ctx, input)
}

// instrumentedParallelWrapper is a non-generic workflow wrapper for testing.
func instrumentedParallelWrapper(ctx wf.Context, input ParallelInput[testInput]) (*ParallelOutput[testOutput], error) {
	return InstrumentedParallelWorkflow[testInput, testOutput](ctx, input)
}

// instrumentedLoopWrapper is a non-generic workflow wrapper for testing.
func instrumentedLoopWrapper(ctx wf.Context, input LoopInput[testInput]) (*LoopOutput[testOutput], error) {
	substitutor := func(template testInput, item string, index int, params map[string]string) testInput {
		return testInput{
			Name:     fmt.Sprintf("%s-%s", template.Name, item),
			Value:    item,
			Activity: template.Activity,
		}
	}
	return InstrumentedLoopWorkflow[testInput, testOutput](ctx, input, substitutor)
}

// instrumentedParameterizedLoopWrapper is a non-generic workflow wrapper for testing.
func instrumentedParameterizedLoopWrapper(ctx wf.Context, input ParameterizedLoopInput[testInput]) (*LoopOutput[testOutput], error) {
	substitutor := func(template testInput, _ string, _ int, params map[string]string) testInput {
		value := template.Value
		for k, v := range params {
			value += fmt.Sprintf("-%s=%s", k, v)
		}
		return testInput{
			Name:     template.Name,
			Value:    value,
			Activity: template.Activity,
		}
	}
	return InstrumentedParameterizedLoopWorkflow[testInput, testOutput](ctx, input, substitutor)
}

func (s *OtelWorkflowTestSuite) TestInstrumentedPipelineWorkflow_AllSuccess() {
	env := s.NewTestWorkflowEnvironment()
	registerTestActivity(env)

	input := PipelineInput[testInput]{
		Tasks: []testInput{
			{Name: "step1", Value: "a", Activity: "TestActivity"},
			{Name: "step2", Value: "b", Activity: "TestActivity"},
		},
		StopOnError: true,
	}

	env.OnActivity("TestActivity", mock.Anything, mock.Anything).Return(
		&testOutput{Result: "ok", Success: true}, nil)

	env.ExecuteWorkflow(instrumentedPipelineWrapper, input)

	require.True(s.T(), env.IsWorkflowCompleted())
	require.NoError(s.T(), env.GetWorkflowError())

	var result PipelineOutput[testOutput]
	require.NoError(s.T(), env.GetWorkflowResult(&result))
	assert.Equal(s.T(), 2, result.TotalSuccess)
	assert.Equal(s.T(), 0, result.TotalFailed)
	assert.Len(s.T(), result.Results, 2)
}

func (s *OtelWorkflowTestSuite) TestInstrumentedPipelineWorkflow_StopOnError() {
	env := s.NewTestWorkflowEnvironment()
	registerTestActivity(env)

	input := PipelineInput[testInput]{
		Tasks: []testInput{
			{Name: "step1", Value: "a", Activity: "TestActivity"},
			{Name: "step2", Value: "b", Activity: "TestActivity"},
		},
		StopOnError: true,
	}

	env.OnActivity("TestActivity", mock.Anything, input.Tasks[0]).Return(
		&testOutput{Result: "ok", Success: true}, nil).Once()

	env.OnActivity("TestActivity", mock.Anything, input.Tasks[1]).Return(
		&testOutput{Result: "fail", Success: false}, nil).Once()

	env.ExecuteWorkflow(instrumentedPipelineWrapper, input)

	require.True(s.T(), env.IsWorkflowCompleted())
	assert.Error(s.T(), env.GetWorkflowError())
}

func (s *OtelWorkflowTestSuite) TestInstrumentedParallelWorkflow_AllSuccess() {
	env := s.NewTestWorkflowEnvironment()
	registerTestActivity(env)

	input := ParallelInput[testInput]{
		Tasks: []testInput{
			{Name: "task1", Value: "x", Activity: "TestActivity"},
			{Name: "task2", Value: "y", Activity: "TestActivity"},
		},
		FailureStrategy: "continue",
	}

	env.OnActivity("TestActivity", mock.Anything, mock.Anything).Return(
		&testOutput{Result: "ok", Success: true}, nil)

	env.ExecuteWorkflow(instrumentedParallelWrapper, input)

	require.True(s.T(), env.IsWorkflowCompleted())
	require.NoError(s.T(), env.GetWorkflowError())

	var result ParallelOutput[testOutput]
	require.NoError(s.T(), env.GetWorkflowResult(&result))
	assert.Equal(s.T(), 2, result.TotalSuccess)
	assert.Equal(s.T(), 0, result.TotalFailed)
	assert.Len(s.T(), result.Results, 2)
}

func (s *OtelWorkflowTestSuite) TestInstrumentedParallelWorkflow_FailFast() {
	env := s.NewTestWorkflowEnvironment()
	registerTestActivity(env)

	input := ParallelInput[testInput]{
		Tasks: []testInput{
			{Name: "task1", Value: "x", Activity: "TestActivity"},
			{Name: "task2", Value: "y", Activity: "TestActivity"},
		},
		FailureStrategy: "fail_fast",
	}

	env.OnActivity("TestActivity", mock.Anything, input.Tasks[0]).Return(
		&testOutput{Result: "fail", Success: false}, nil).Once()

	env.OnActivity("TestActivity", mock.Anything, input.Tasks[1]).Return(
		&testOutput{Result: "ok", Success: true}, nil).Once()

	env.ExecuteWorkflow(instrumentedParallelWrapper, input)

	require.True(s.T(), env.IsWorkflowCompleted())
	assert.Error(s.T(), env.GetWorkflowError())
}

func (s *OtelWorkflowTestSuite) TestInstrumentedLoopWorkflow_Sequential_Success() {
	env := s.NewTestWorkflowEnvironment()
	registerTestActivity(env)

	input := LoopInput[testInput]{
		Items:    []string{"a", "b", "c"},
		Template: testInput{Name: "step", Value: "template", Activity: "TestActivity"},
		Parallel: false,
	}

	env.OnActivity("TestActivity", mock.Anything, mock.Anything).Return(
		&testOutput{Result: "ok", Success: true}, nil)

	env.ExecuteWorkflow(instrumentedLoopWrapper, input)

	require.True(s.T(), env.IsWorkflowCompleted())
	require.NoError(s.T(), env.GetWorkflowError())

	var result LoopOutput[testOutput]
	require.NoError(s.T(), env.GetWorkflowResult(&result))
	assert.Equal(s.T(), 3, result.TotalSuccess)
	assert.Equal(s.T(), 0, result.TotalFailed)
	assert.Len(s.T(), result.Results, 3)
}

func (s *OtelWorkflowTestSuite) TestInstrumentedLoopWorkflow_Parallel_Success() {
	env := s.NewTestWorkflowEnvironment()
	registerTestActivity(env)

	input := LoopInput[testInput]{
		Items:    []string{"x", "y"},
		Template: testInput{Name: "task", Value: "template", Activity: "TestActivity"},
		Parallel: true,
	}

	env.OnActivity("TestActivity", mock.Anything, mock.Anything).Return(
		&testOutput{Result: "ok", Success: true}, nil)

	env.ExecuteWorkflow(instrumentedLoopWrapper, input)

	require.True(s.T(), env.IsWorkflowCompleted())
	require.NoError(s.T(), env.GetWorkflowError())

	var result LoopOutput[testOutput]
	require.NoError(s.T(), env.GetWorkflowResult(&result))
	assert.Equal(s.T(), 2, result.TotalSuccess)
	assert.Equal(s.T(), 0, result.TotalFailed)
}

func (s *OtelWorkflowTestSuite) TestInstrumentedLoopWorkflow_FailFast() {
	env := s.NewTestWorkflowEnvironment()
	registerTestActivity(env)

	input := LoopInput[testInput]{
		Items:           []string{"a", "b"},
		Template:        testInput{Name: "step", Value: "template", Activity: "TestActivity"},
		Parallel:        false,
		FailureStrategy: "fail_fast",
	}

	env.OnActivity("TestActivity", mock.Anything, mock.Anything).Return(
		&testOutput{Result: "fail", Success: false}, nil)

	env.ExecuteWorkflow(instrumentedLoopWrapper, input)

	require.True(s.T(), env.IsWorkflowCompleted())
	assert.Error(s.T(), env.GetWorkflowError())
}

func (s *OtelWorkflowTestSuite) TestInstrumentedParameterizedLoopWorkflow_Success() {
	env := s.NewTestWorkflowEnvironment()
	registerTestActivity(env)

	input := ParameterizedLoopInput[testInput]{
		Parameters: map[string][]string{
			"env":  {"dev", "prod"},
			"arch": {"amd64"},
		},
		Template: testInput{Name: "build", Value: "base", Activity: "TestActivity"},
		Parallel: false,
	}

	env.OnActivity("TestActivity", mock.Anything, mock.Anything).Return(
		&testOutput{Result: "ok", Success: true}, nil)

	env.ExecuteWorkflow(instrumentedParameterizedLoopWrapper, input)

	require.True(s.T(), env.IsWorkflowCompleted())
	require.NoError(s.T(), env.GetWorkflowError())

	var result LoopOutput[testOutput]
	require.NoError(s.T(), env.GetWorkflowResult(&result))
	assert.Equal(s.T(), 2, result.TotalSuccess) // 2 env * 1 arch = 2 combinations
	assert.Equal(s.T(), 0, result.TotalFailed)
}

func (s *OtelWorkflowTestSuite) TestInstrumentedParameterizedLoopWorkflow_FailFast() {
	env := s.NewTestWorkflowEnvironment()
	registerTestActivity(env)

	input := ParameterizedLoopInput[testInput]{
		Parameters: map[string][]string{
			"env": {"dev", "prod"},
		},
		Template:        testInput{Name: "deploy", Value: "base", Activity: "TestActivity"},
		Parallel:        false,
		FailureStrategy: "fail_fast",
	}

	env.OnActivity("TestActivity", mock.Anything, mock.Anything).Return(
		&testOutput{Result: "fail", Success: false}, nil)

	env.ExecuteWorkflow(instrumentedParameterizedLoopWrapper, input)

	require.True(s.T(), env.IsWorkflowCompleted())
	assert.Error(s.T(), env.GetWorkflowError())
}
