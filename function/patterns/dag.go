package patterns

import (
	"github.com/jasoet/go-wf/function/builder"
	"github.com/jasoet/go-wf/function/payload"
)

// ETLWithValidation creates a 4-node DAG: validate-config and extract run in parallel,
// transform depends on both, and load depends on transform.
//
// Example:
//
//	input, err := patterns.ETLWithValidation("database", "parquet", "warehouse")
func ETLWithValidation(source, format, target string) (*payload.DAGWorkflowInput, error) {
	return builder.NewDAGBuilder("etl-with-validation").
		AddNodeWithInput("validate-config", payload.FunctionExecutionInput{
			Name: "validate-config",
			Args: map[string]string{"env": "production"},
		}).
		AddNodeWithInput("extract", payload.FunctionExecutionInput{
			Name: "extract",
			Args: map[string]string{"source": source},
		}).
		AddNodeWithInput("transform", payload.FunctionExecutionInput{
			Name: "etl-transform",
			Args: map[string]string{"format": format},
		}, "validate-config", "extract").
		AddNodeWithInput("load", payload.FunctionExecutionInput{
			Name: "load",
			Args: map[string]string{"target": target},
		}, "transform").
		FailFast(true).
		BuildDAG()
}

// CIPipeline creates a 4-node DAG representing a CI pipeline: compile (with output mapping),
// unit-test and lint depend on compile and run in parallel, publish depends on both and receives
// the artifact path via input mapping.
//
// Example:
//
//	input, err := patterns.CIPipeline()
func CIPipeline() (*payload.DAGWorkflowInput, error) {
	return builder.NewDAGBuilder("ci-pipeline").
		AddNodeWithInput("compile", payload.FunctionExecutionInput{
			Name: "compile",
		}).
		WithOutputMapping("compile", payload.OutputMapping{
			Name:      "artifact",
			ResultKey: "artifact",
		}).
		AddNodeWithInput("unit-test", payload.FunctionExecutionInput{
			Name: "run-tests",
		}, "compile").
		AddNodeWithInput("lint", payload.FunctionExecutionInput{
			Name: "validate-config",
			Args: map[string]string{"env": "ci"},
		}, "compile").
		AddNodeWithInput("publish", payload.FunctionExecutionInput{
			Name: "publish-artifact",
			Args: map[string]string{},
		}, "unit-test", "lint").
		WithInputMapping("publish", payload.FunctionInputMapping{
			Name: "artifact_path",
			From: "compile.artifact",
		}).
		FailFast(true).
		BuildDAG()
}
