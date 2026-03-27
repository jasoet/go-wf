package patterns

import (
	"fmt"

	"github.com/jasoet/go-wf/function/builder"
	"github.com/jasoet/go-wf/function/payload"
	"github.com/jasoet/go-wf/workflow"
)

// BatchProcess creates a parallel loop workflow for batch processing items.
// Each item is processed in parallel using a function template with the specified name.
// FailureStrategy is set to "continue" so remaining items are processed even if some fail.
//
// Example:
//
//	input, err := patterns.BatchProcess(
//	    []string{"file1.csv", "file2.csv", "file3.csv"},
//	    "process-file")
func BatchProcess(items []string, functionName string) (*workflow.LoopInput[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput], error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("at least one item is required")
	}

	template := &payload.FunctionExecutionInput{
		Name: functionName,
		Args: map[string]string{
			"file": "{{item}}",
		},
	}

	input, err := builder.NewFunctionLoopBuilder(items).
		WithTemplate(template).
		Parallel(true).
		FailFast(false).
		BuildLoop()
	if err != nil {
		return nil, fmt.Errorf("batch process validation failed: %w", err)
	}

	return input, nil
}

// SequentialMigration creates a sequential loop workflow for running database migrations.
// Each migration is executed in order with fail-fast behavior so that later migrations
// are skipped if an earlier one fails.
//
// Example:
//
//	input, err := patterns.SequentialMigration(
//	    []string{"001_create_users.sql", "002_add_index.sql"})
func SequentialMigration(migrations []string) (*workflow.LoopInput[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput], error) {
	if len(migrations) == 0 {
		return nil, fmt.Errorf("at least one migration is required")
	}

	template := &payload.FunctionExecutionInput{
		Name: "run-migration",
		Args: map[string]string{
			"migration": "{{item}}",
		},
	}

	input, err := builder.NewFunctionLoopBuilder(migrations).
		WithTemplate(template).
		Parallel(false).
		FailFast(true).
		BuildLoop()
	if err != nil {
		return nil, fmt.Errorf("sequential migration validation failed: %w", err)
	}

	return input, nil
}

// MultiRegionDeploy creates a parameterized loop for deploying a service version
// across multiple environments and regions. Runs in parallel with fail-fast behavior.
//
// Example:
//
//	input, err := patterns.MultiRegionDeploy(
//	    []string{"dev", "staging", "prod"},
//	    []string{"us-west", "us-east", "eu-central"},
//	    "v1.2.3")
func MultiRegionDeploy(environments, regions []string, version string) (*workflow.ParameterizedLoopInput[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput], error) {
	if len(environments) == 0 {
		return nil, fmt.Errorf("at least one environment is required")
	}
	if len(regions) == 0 {
		return nil, fmt.Errorf("at least one region is required")
	}

	template := &payload.FunctionExecutionInput{
		Name: "deploy-service",
		Args: map[string]string{
			"version":     version,
			"environment": "{{.environment}}",
			"region":      "{{.region}}",
		},
	}

	params := map[string][]string{
		"environment": environments,
		"region":      regions,
	}

	input, err := builder.NewFunctionParameterizedLoopBuilder(params).
		WithTemplate(template).
		Parallel(true).
		FailFast(true).
		BuildParameterizedLoop()
	if err != nil {
		return nil, fmt.Errorf("multi-region deploy validation failed: %w", err)
	}

	return input, nil
}

// ParameterSweep creates a parameterized loop for sweeping over parameter combinations.
// Useful for scientific computing or ML hyperparameter tuning. Runs in parallel with
// configurable max concurrency and "continue" failure strategy so all combinations are tried.
//
// Example:
//
//	input, err := patterns.ParameterSweep(
//	    map[string][]string{
//	        "learning_rate": {"0.001", "0.01", "0.1"},
//	        "batch_size":    {"32", "64", "128"},
//	    },
//	    "train-model",
//	    5)
func ParameterSweep(params map[string][]string, functionName string, maxConcurrency int) (*workflow.ParameterizedLoopInput[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput], error) {
	if len(params) == 0 {
		return nil, fmt.Errorf("at least one parameter is required")
	}

	args := make(map[string]string)
	for key := range params {
		args[key] = fmt.Sprintf("{{.%s}}", key)
	}

	template := &payload.FunctionExecutionInput{
		Name: functionName,
		Args: args,
	}

	input, err := builder.NewFunctionParameterizedLoopBuilder(params).
		WithTemplate(template).
		Parallel(true).
		MaxConcurrency(maxConcurrency).
		FailFast(false).
		BuildParameterizedLoop()
	if err != nil {
		return nil, fmt.Errorf("parameter sweep validation failed: %w", err)
	}

	return input, nil
}
