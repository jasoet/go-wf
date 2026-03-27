package function

import (
	"github.com/jasoet/go-wf/function/payload"
)

// Type aliases for extended (DAG) types re-exported from function/payload.
type (
	OutputMapping             = payload.OutputMapping
	FunctionInputMapping      = payload.FunctionInputMapping
	DataMapping               = payload.DataMapping
	FunctionDAGNode           = payload.FunctionDAGNode
	DAGWorkflowInput          = payload.DAGWorkflowInput
	FunctionNodeResult        = payload.FunctionNodeResult
	FunctionDAGWorkflowOutput = payload.FunctionDAGWorkflowOutput
)
