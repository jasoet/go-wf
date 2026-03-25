package workflow

import (
	wf "go.temporal.io/sdk/workflow"

	"github.com/jasoet/go-wf/function/payload"
)

// InstrumentedDAGWorkflow wraps DAGWorkflow with structured logging at boundaries.
func InstrumentedDAGWorkflow(ctx wf.Context, input payload.DAGWorkflowInput) (*payload.FunctionDAGWorkflowOutput, error) {
	logger := wf.GetLogger(ctx)
	logger.Info("dag.start",
		"node_count", len(input.Nodes),
		"fail_fast", input.FailFast,
		"max_parallel", input.MaxParallel,
	)

	startTime := wf.Now(ctx)
	result, err := DAGWorkflow(ctx, input)

	duration := wf.Now(ctx).Sub(startTime)

	if err != nil {
		logger.Error("dag.failed",
			"error", err,
			"duration", duration,
		)
		return result, err
	}

	logger.Info("dag.complete",
		"node_count", len(input.Nodes),
		"success_count", result.TotalSuccess,
		"failure_count", result.TotalFailed,
		"duration", duration,
	)

	return result, nil
}
