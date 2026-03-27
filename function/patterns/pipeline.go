package patterns

import (
	"fmt"

	"github.com/jasoet/go-wf/function/builder"
	"github.com/jasoet/go-wf/function/payload"
	"github.com/jasoet/go-wf/workflow"
)

// ETLPipeline creates a 3-step ETL (Extract, Transform, Load) pipeline.
//
// Example:
//
//	input, err := patterns.ETLPipeline("s3://bucket/data", "json", "postgres://db/table")
func ETLPipeline(source, format, target string) (*workflow.PipelineInput[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput], error) {
	return builder.NewFunctionBuilder("etl-pipeline").
		Add(&payload.FunctionExecutionInput{
			Name: "extract",
			Args: map[string]string{"source": source},
		}).
		Add(&payload.FunctionExecutionInput{
			Name: "etl-transform",
			Args: map[string]string{"format": format},
		}).
		Add(&payload.FunctionExecutionInput{
			Name: "load",
			Args: map[string]string{"target": target},
		}).
		StopOnError(true).
		BuildPipeline()
}

// ValidateTransformNotify creates a 3-step pipeline: validate, transform, notify.
//
// Example:
//
//	input, err := patterns.ValidateTransformNotify("user@example.com", "report", "#alerts")
func ValidateTransformNotify(email, name, channel string) (*workflow.PipelineInput[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput], error) {
	return builder.NewFunctionBuilder("validate-transform-notify").
		Add(&payload.FunctionExecutionInput{
			Name: "validate",
			Args: map[string]string{"email": email, "name": name},
		}).
		Add(&payload.FunctionExecutionInput{
			Name: "transform",
			Args: map[string]string{"name": name, "email": email},
		}).
		Add(&payload.FunctionExecutionInput{
			Name: "notify",
			Args: map[string]string{"name": name, "channel": channel},
		}).
		StopOnError(true).
		BuildPipeline()
}

// MultiEnvironmentDeploy creates a pipeline that deploys a service to multiple environments sequentially.
//
// Example:
//
//	input, err := patterns.MultiEnvironmentDeploy("v1.2.3", []string{"staging", "production"})
func MultiEnvironmentDeploy(version string, environments []string) (*workflow.PipelineInput[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput], error) {
	if len(environments) == 0 {
		return nil, fmt.Errorf("at least one environment is required")
	}

	wb := builder.NewFunctionBuilder("multi-env-deploy")

	for _, env := range environments {
		wb.Add(&payload.FunctionExecutionInput{
			Name: "deploy-service",
			Args: map[string]string{"environment": env, "version": version},
		})
	}

	return wb.StopOnError(true).BuildPipeline()
}
