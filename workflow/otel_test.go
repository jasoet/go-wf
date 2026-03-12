package workflow

import (
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
