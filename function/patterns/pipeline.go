package patterns

import (
	"fmt"

	"github.com/jasoet/go-wf/function/payload"
	"github.com/jasoet/go-wf/workflow"
)

// ETLPipeline creates a 3-step ETL (Extract, Transform, Load) pipeline.
//
// Example:
//
//	input, err := patterns.ETLPipeline("s3://bucket/data", "json", "postgres://db/table")
func ETLPipeline(source, format, target string) (*workflow.PipelineInput[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput], error) {
	input := &workflow.PipelineInput[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput]{
		Tasks: []*payload.FunctionExecutionInput{
			{Name: "extract", Args: map[string]string{"source": source}},
			{Name: "etl-transform", Args: map[string]string{"format": format}},
			{Name: "load", Args: map[string]string{"target": target}},
		},
		StopOnError: true,
	}
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("ETLPipeline validation failed: %w", err)
	}
	return input, nil
}

// ValidateTransformNotify creates a 3-step pipeline: validate, transform, notify.
//
// Example:
//
//	input, err := patterns.ValidateTransformNotify("user@example.com", "report", "#alerts")
func ValidateTransformNotify(email, name, channel string) (*workflow.PipelineInput[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput], error) {
	input := &workflow.PipelineInput[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput]{
		Tasks: []*payload.FunctionExecutionInput{
			{Name: "validate", Args: map[string]string{"email": email, "name": name}},
			{Name: "transform", Args: map[string]string{"name": name, "email": email}},
			{Name: "notify", Args: map[string]string{"name": name, "channel": channel}},
		},
		StopOnError: true,
	}
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("ValidateTransformNotify validation failed: %w", err)
	}
	return input, nil
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

	tasks := make([]*payload.FunctionExecutionInput, 0, len(environments))
	for _, env := range environments {
		tasks = append(tasks, &payload.FunctionExecutionInput{
			Name: "deploy-service",
			Args: map[string]string{"environment": env, "version": version},
		})
	}

	input := &workflow.PipelineInput[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput]{
		Tasks:       tasks,
		StopOnError: true,
	}
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("MultiEnvironmentDeploy validation failed: %w", err)
	}
	return input, nil
}
