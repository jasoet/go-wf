package patterns

import (
	"fmt"

	"github.com/jasoet/go-wf/function/builder"
	"github.com/jasoet/go-wf/function/payload"
)

// ETLPipeline creates a 3-step ETL (Extract, Transform, Load) pipeline.
//
// Example:
//
//	input, err := patterns.ETLPipeline("s3://bucket/data", "json", "postgres://db/table")
func ETLPipeline(source, format, target string) (*payload.PipelineInput, error) {
	return builder.NewWorkflowBuilder("etl-pipeline").
		AddInput(payload.FunctionExecutionInput{
			Name: "extract",
			Args: map[string]string{"source": source},
		}).
		AddInput(payload.FunctionExecutionInput{
			Name: "etl-transform",
			Args: map[string]string{"format": format},
		}).
		AddInput(payload.FunctionExecutionInput{
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
func ValidateTransformNotify(email, name, channel string) (*payload.PipelineInput, error) {
	return builder.NewWorkflowBuilder("validate-transform-notify").
		AddInput(payload.FunctionExecutionInput{
			Name: "validate",
			Args: map[string]string{"email": email, "name": name},
		}).
		AddInput(payload.FunctionExecutionInput{
			Name: "transform",
			Args: map[string]string{"name": name, "email": email},
		}).
		AddInput(payload.FunctionExecutionInput{
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
func MultiEnvironmentDeploy(version string, environments []string) (*payload.PipelineInput, error) {
	if len(environments) == 0 {
		return nil, fmt.Errorf("at least one environment is required")
	}

	wb := builder.NewWorkflowBuilder("multi-env-deploy")

	for _, env := range environments {
		wb.AddInput(payload.FunctionExecutionInput{
			Name: "deploy-service",
			Args: map[string]string{"environment": env, "version": version},
		})
	}

	return wb.StopOnError(true).BuildPipeline()
}
