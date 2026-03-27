package function

import (
	"github.com/jasoet/go-wf/function/payload"
)

// Type aliases re-exported from function/payload for convenience.
// External callers can use function.FunctionExecutionInput instead of
// function/payload.FunctionExecutionInput.
type (
	FunctionExecutionInput  = payload.FunctionExecutionInput
	FunctionExecutionOutput = payload.FunctionExecutionOutput
	PipelineInput           = payload.PipelineInput
	PipelineOutput          = payload.PipelineOutput
	ParallelInput           = payload.ParallelInput
	ParallelOutput          = payload.ParallelOutput
	LoopInput               = payload.LoopInput
	ParameterizedLoopInput  = payload.ParameterizedLoopInput
	LoopOutput              = payload.LoopOutput
)
