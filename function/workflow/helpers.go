package workflow

import (
	"github.com/jasoet/go-wf/function/payload"
	generic "github.com/jasoet/go-wf/workflow"
)

// toTaskPtrs converts a slice of FunctionExecutionInput values to a slice of pointers.
func toTaskPtrs(functions []payload.FunctionExecutionInput) []*payload.FunctionExecutionInput {
	ptrs := make([]*payload.FunctionExecutionInput, len(functions))
	for i := range functions {
		ptrs[i] = &functions[i]
	}
	return ptrs
}

// toPipelineOutput converts a generic pipeline output to a function-specific output.
func toPipelineOutput(g *generic.PipelineOutput[payload.FunctionExecutionOutput], err error) (*payload.PipelineOutput, error) {
	if g == nil {
		return nil, err
	}
	return &payload.PipelineOutput{
		Results: g.Results, TotalSuccess: g.TotalSuccess, TotalFailed: g.TotalFailed, TotalDuration: g.TotalDuration,
	}, err
}

// toParallelOutput converts a generic parallel output to a function-specific output.
func toParallelOutput(g *generic.ParallelOutput[payload.FunctionExecutionOutput], err error) (*payload.ParallelOutput, error) {
	if g == nil {
		return nil, err
	}
	return &payload.ParallelOutput{
		Results: g.Results, TotalSuccess: g.TotalSuccess, TotalFailed: g.TotalFailed, TotalDuration: g.TotalDuration,
	}, err
}
