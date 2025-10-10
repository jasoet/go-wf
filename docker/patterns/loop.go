package patterns

import (
	"fmt"

	"github.com/jasoet/go-wf/docker/builder"
	"github.com/jasoet/go-wf/docker/payload"
)

// ParallelLoop creates a parallel loop workflow over items.
// Each item is processed in parallel with the specified template.
//
// Example:
//
//	input, err := patterns.ParallelLoop(
//	    []string{"file1.csv", "file2.csv", "file3.csv"},
//	    "processor:v1",
//	    "process.sh {{item}}")
func ParallelLoop(items []string, image string, command string) (*payload.LoopInput, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("at least one item is required")
	}

	containerTemplate := payload.ContainerExecutionInput{
		Image:   image,
		Command: []string{"sh", "-c", command},
		Env: map[string]string{
			"ITEM":  "{{item}}",
			"INDEX": "{{index}}",
		},
	}

	input := &payload.LoopInput{
		Items:           items,
		Template:        containerTemplate,
		Parallel:        true,
		FailureStrategy: "continue",
	}

	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("loop validation failed: %w", err)
	}

	return input, nil
}

// SequentialLoop creates a sequential loop workflow over items.
// Each item is processed sequentially with the specified template.
//
// Example:
//
//	input, err := patterns.SequentialLoop(
//	    []string{"step1", "step2", "step3"},
//	    "deployer:v1",
//	    "deploy.sh {{item}}")
func SequentialLoop(items []string, image string, command string) (*payload.LoopInput, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("at least one item is required")
	}

	containerTemplate := payload.ContainerExecutionInput{
		Image:   image,
		Command: []string{"sh", "-c", command},
		Env: map[string]string{
			"ITEM":  "{{item}}",
			"INDEX": "{{index}}",
		},
	}

	input := &payload.LoopInput{
		Items:           items,
		Template:        containerTemplate,
		Parallel:        false,
		FailureStrategy: "fail_fast",
	}

	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("loop validation failed: %w", err)
	}

	return input, nil
}

// BatchProcessing creates a parallel loop workflow for batch data processing.
// Processes multiple data files in parallel with configurable concurrency.
//
// Example:
//
//	input, err := patterns.BatchProcessing(
//	    []string{"batch1.json", "batch2.json", "batch3.json"},
//	    "data-processor:v1",
//	    3)
func BatchProcessing(dataFiles []string, processorImage string, maxConcurrency int) (*payload.LoopInput, error) {
	if len(dataFiles) == 0 {
		return nil, fmt.Errorf("at least one data file is required")
	}

	containerTemplate := payload.ContainerExecutionInput{
		Image:   processorImage,
		Command: []string{"process-batch"},
		Env: map[string]string{
			"INPUT_FILE":  "{{item}}",
			"BATCH_INDEX": "{{index}}",
		},
	}

	input := &payload.LoopInput{
		Items:           dataFiles,
		Template:        containerTemplate,
		Parallel:        true,
		MaxConcurrency:  maxConcurrency,
		FailureStrategy: "continue",
	}

	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("batch processing validation failed: %w", err)
	}

	return input, nil
}

// MultiRegionDeployment creates a parameterized loop for multi-region deployment.
// Deploys to all combinations of environments and regions.
//
// Example:
//
//	input, err := patterns.MultiRegionDeployment(
//	    []string{"dev", "staging", "prod"},
//	    []string{"us-west", "us-east", "eu-central"},
//	    "deployer:v1")
func MultiRegionDeployment(environments, regions []string, deployImage string) (*payload.ParameterizedLoopInput, error) {
	if len(environments) == 0 || len(regions) == 0 {
		return nil, fmt.Errorf("at least one environment and one region are required")
	}

	containerTemplate := payload.ContainerExecutionInput{
		Image:   deployImage,
		Command: []string{"deploy", "--env={{.env}}", "--region={{.region}}"},
		Env: map[string]string{
			"ENVIRONMENT": "{{.env}}",
			"REGION":      "{{.region}}",
			"ITERATION":   "{{index}}",
		},
	}

	input := &payload.ParameterizedLoopInput{
		Parameters: map[string][]string{
			"env":    environments,
			"region": regions,
		},
		Template:        containerTemplate,
		Parallel:        true,
		FailureStrategy: "fail_fast",
	}

	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("multi-region deployment validation failed: %w", err)
	}

	return input, nil
}

// MatrixBuild creates a parameterized loop for matrix build workflows.
// Builds across multiple combinations of language versions and platforms.
//
// Example:
//
//	input, err := patterns.MatrixBuild(
//	    map[string][]string{
//	        "go_version": {"1.21", "1.22", "1.23"},
//	        "platform": {"linux", "darwin", "windows"},
//	    },
//	    "builder:v1")
func MatrixBuild(buildMatrix map[string][]string, buildImage string) (*payload.ParameterizedLoopInput, error) {
	if len(buildMatrix) == 0 {
		return nil, fmt.Errorf("build matrix cannot be empty")
	}

	// Create command with all matrix parameters
	cmdParts := []string{"build"}
	for key := range buildMatrix {
		cmdParts = append(cmdParts, fmt.Sprintf("--%s={{.%s}}", key, key))
	}

	containerTemplate := payload.ContainerExecutionInput{
		Image:   buildImage,
		Command: cmdParts,
		Env: map[string]string{
			"BUILD_INDEX": "{{index}}",
		},
	}

	// Add matrix parameters to environment
	for key := range buildMatrix {
		containerTemplate.Env[key] = fmt.Sprintf("{{.%s}}", key)
	}

	input := &payload.ParameterizedLoopInput{
		Parameters:      buildMatrix,
		Template:        containerTemplate,
		Parallel:        true,
		FailureStrategy: "fail_fast",
	}

	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("matrix build validation failed: %w", err)
	}

	return input, nil
}

// ParameterSweep creates a parameterized loop for parameter sweep workflows.
// Useful for scientific computing or ML hyperparameter tuning.
//
// Example:
//
//	input, err := patterns.ParameterSweep(
//	    map[string][]string{
//	        "learning_rate": {"0.001", "0.01", "0.1"},
//	        "batch_size": {"32", "64", "128"},
//	    },
//	    "ml-trainer:v1",
//	    5)
func ParameterSweep(parameters map[string][]string, trainerImage string, maxConcurrency int) (*payload.ParameterizedLoopInput, error) {
	if len(parameters) == 0 {
		return nil, fmt.Errorf("at least one parameter is required")
	}

	containerTemplate := payload.ContainerExecutionInput{
		Image:   trainerImage,
		Command: []string{"train", "--config={{index}}"},
		Env: map[string]string{
			"EXPERIMENT_INDEX": "{{index}}",
		},
	}

	// Add all parameters to environment
	for key := range parameters {
		containerTemplate.Env[key] = fmt.Sprintf("{{.%s}}", key)
	}

	input := &payload.ParameterizedLoopInput{
		Parameters:      parameters,
		Template:        containerTemplate,
		Parallel:        true,
		MaxConcurrency:  maxConcurrency,
		FailureStrategy: "continue", // Continue even if some experiments fail
	}

	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("parameter sweep validation failed: %w", err)
	}

	return input, nil
}

// ParallelLoopWithTemplate creates a parallel loop with a custom template source.
//
// Example:
//
//	template := template.NewContainer("process", "alpine:latest",
//	    template.WithCommand("echo", "Processing {{item}}"))
//	input, err := patterns.ParallelLoopWithTemplate(
//	    []string{"a", "b", "c"},
//	    template)
func ParallelLoopWithTemplate(items []string, templateSource builder.WorkflowSource) (*payload.LoopInput, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("at least one item is required")
	}

	if templateSource == nil {
		return nil, fmt.Errorf("template source cannot be nil")
	}

	input := &payload.LoopInput{
		Items:           items,
		Template:        templateSource.ToInput(),
		Parallel:        true,
		FailureStrategy: "continue",
	}

	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("loop validation failed: %w", err)
	}

	return input, nil
}
