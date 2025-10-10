package docker

import (
	"fmt"
	"strings"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// ExecuteContainerWorkflow runs a single container and returns results.
func ExecuteContainerWorkflow(ctx workflow.Context, input ContainerExecutionInput) (*ContainerExecutionOutput, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting container execution workflow",
		"image", input.Image,
		"name", input.Name)

	// Validate input
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	// Set default timeout if not specified
	timeout := input.RunTimeout
	if timeout == 0 {
		timeout = 10 * time.Minute
	}

	// Activity options with retry policy
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: timeout,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Execute container activity
	var output ContainerExecutionOutput
	err := workflow.ExecuteActivity(ctx, StartContainerActivity, input).Get(ctx, &output)
	if err != nil {
		logger.Error("Container execution failed", "error", err)
		return nil, err
	}

	logger.Info("Container execution completed",
		"success", output.Success,
		"exitCode", output.ExitCode,
		"duration", output.Duration)

	return &output, nil
}

// ContainerPipelineWorkflow executes containers sequentially.
func ContainerPipelineWorkflow(ctx workflow.Context, input PipelineInput) (*PipelineOutput, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting container pipeline workflow", "steps", len(input.Containers))

	// Validate input
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	startTime := workflow.Now(ctx)
	output := &PipelineOutput{
		Results: make([]ContainerExecutionOutput, 0, len(input.Containers)),
	}

	// Default activity options
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	for i, containerInput := range input.Containers {
		stepName := containerInput.Name
		if stepName == "" {
			stepName = fmt.Sprintf("step-%d", i+1)
		}

		logger.Info("Executing pipeline step",
			"step", i+1,
			"name", stepName,
			"image", containerInput.Image)

		// Execute step
		var result ContainerExecutionOutput
		err := workflow.ExecuteActivity(ctx, StartContainerActivity, containerInput).Get(ctx, &result)

		output.Results = append(output.Results, result)

		if err != nil || !result.Success {
			output.TotalFailed++
			logger.Error("Pipeline step failed",
				"step", i+1,
				"name", stepName,
				"error", err)

			if input.StopOnError {
				output.TotalDuration = workflow.Now(ctx).Sub(startTime)
				return output, fmt.Errorf("pipeline stopped at step %d: %w", i+1, err)
			}
			continue
		}

		output.TotalSuccess++
		logger.Info("Pipeline step completed",
			"step", i+1,
			"name", stepName,
			"duration", result.Duration)
	}

	output.TotalDuration = workflow.Now(ctx).Sub(startTime)

	logger.Info("Pipeline workflow completed",
		"success", output.TotalSuccess,
		"failed", output.TotalFailed,
		"totalDuration", output.TotalDuration)

	return output, nil
}

// ParallelContainersWorkflow executes multiple containers in parallel.
func ParallelContainersWorkflow(ctx workflow.Context, input ParallelInput) (*ParallelOutput, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting parallel containers workflow",
		"containers", len(input.Containers),
		"maxConcurrency", input.MaxConcurrency)

	// Validate input
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	startTime := workflow.Now(ctx)

	// Default activity options
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Execute containers in parallel
	futures := make([]workflow.Future, len(input.Containers))

	for i, containerInput := range input.Containers {
		// Execute each container as an activity
		futures[i] = workflow.ExecuteActivity(ctx, StartContainerActivity, containerInput)
	}

	// Collect results
	output := &ParallelOutput{
		Results: make([]ContainerExecutionOutput, 0, len(input.Containers)),
	}

	for i, future := range futures {
		var result ContainerExecutionOutput
		err := future.Get(ctx, &result)

		output.Results = append(output.Results, result)

		if err != nil || !result.Success {
			output.TotalFailed++

			if input.FailureStrategy == "fail_fast" {
				output.TotalDuration = workflow.Now(ctx).Sub(startTime)
				return output, fmt.Errorf("parallel execution failed at container %d: %w", i, err)
			}
		} else {
			output.TotalSuccess++
		}
	}

	output.TotalDuration = workflow.Now(ctx).Sub(startTime)

	logger.Info("Parallel workflow completed",
		"success", output.TotalSuccess,
		"failed", output.TotalFailed,
		"totalDuration", output.TotalDuration)

	return output, nil
}

// substituteTemplate replaces template variables in a string.
// Supports: {{item}}, {{index}}, and {{.paramName}} syntax.
func substituteTemplate(template string, item string, index int, params map[string]string) string {
	result := template

	// Replace {{item}}
	result = strings.ReplaceAll(result, "{{item}}", item)

	// Replace {{index}}
	result = strings.ReplaceAll(result, "{{index}}", fmt.Sprintf("%d", index))

	// Replace {{.paramName}} with parameter values
	for key, value := range params {
		result = strings.ReplaceAll(result, fmt.Sprintf("{{.%s}}", key), value)
		result = strings.ReplaceAll(result, fmt.Sprintf("{{%s}}", key), value)
	}

	return result
}

// substituteContainerInput creates a new container input with substituted values.
func substituteContainerInput(template ContainerExecutionInput, item string, index int, params map[string]string) ContainerExecutionInput {
	result := template

	// Substitute in image
	result.Image = substituteTemplate(template.Image, item, index, params)

	// Substitute in command
	if len(template.Command) > 0 {
		result.Command = make([]string, len(template.Command))
		for i, cmd := range template.Command {
			result.Command[i] = substituteTemplate(cmd, item, index, params)
		}
	}

	// Substitute in entrypoint
	if len(template.Entrypoint) > 0 {
		result.Entrypoint = make([]string, len(template.Entrypoint))
		for i, entry := range template.Entrypoint {
			result.Entrypoint[i] = substituteTemplate(entry, item, index, params)
		}
	}

	// Substitute in environment variables
	if len(template.Env) > 0 {
		result.Env = make(map[string]string, len(template.Env))
		for key, value := range template.Env {
			newKey := substituteTemplate(key, item, index, params)
			newValue := substituteTemplate(value, item, index, params)
			result.Env[newKey] = newValue
		}
	}

	// Substitute in name
	if template.Name != "" {
		result.Name = substituteTemplate(template.Name, item, index, params)
	}

	// Substitute in work directory
	if template.WorkDir != "" {
		result.WorkDir = substituteTemplate(template.WorkDir, item, index, params)
	}

	// Substitute in volumes
	if len(template.Volumes) > 0 {
		result.Volumes = make(map[string]string, len(template.Volumes))
		for key, value := range template.Volumes {
			newKey := substituteTemplate(key, item, index, params)
			newValue := substituteTemplate(value, item, index, params)
			result.Volumes[newKey] = newValue
		}
	}

	return result
}

// LoopWorkflow executes containers in a loop over items (withItems pattern).
func LoopWorkflow(ctx workflow.Context, input LoopInput) (*LoopOutput, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting loop workflow",
		"items", len(input.Items),
		"parallel", input.Parallel,
		"maxConcurrency", input.MaxConcurrency)

	// Validate input
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	startTime := workflow.Now(ctx)

	// Default activity options
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	output := &LoopOutput{
		Results:   make([]ContainerExecutionOutput, 0, len(input.Items)),
		ItemCount: len(input.Items),
	}

	if input.Parallel {
		// Execute in parallel
		futures := make([]workflow.Future, len(input.Items))

		for i, item := range input.Items {
			// Substitute template variables
			containerInput := substituteContainerInput(input.Template, item, i, nil)

			logger.Info("Scheduling loop iteration",
				"index", i,
				"item", item,
				"image", containerInput.Image)

			// Execute container activity
			futures[i] = workflow.ExecuteActivity(ctx, StartContainerActivity, containerInput)
		}

		// Collect results
		for i, future := range futures {
			var result ContainerExecutionOutput
			err := future.Get(ctx, &result)

			output.Results = append(output.Results, result)

			if err != nil || !result.Success {
				output.TotalFailed++
				logger.Error("Loop iteration failed",
					"index", i,
					"item", input.Items[i],
					"error", err)

				if input.FailureStrategy == "fail_fast" {
					output.TotalDuration = workflow.Now(ctx).Sub(startTime)
					return output, fmt.Errorf("loop failed at iteration %d: %w", i, err)
				}
			} else {
				output.TotalSuccess++
			}
		}
	} else {
		// Execute sequentially
		for i, item := range input.Items {
			// Substitute template variables
			containerInput := substituteContainerInput(input.Template, item, i, nil)

			logger.Info("Executing loop iteration",
				"index", i,
				"item", item,
				"image", containerInput.Image)

			// Execute container activity
			var result ContainerExecutionOutput
			err := workflow.ExecuteActivity(ctx, StartContainerActivity, containerInput).Get(ctx, &result)

			output.Results = append(output.Results, result)

			if err != nil || !result.Success {
				output.TotalFailed++
				logger.Error("Loop iteration failed",
					"index", i,
					"item", item,
					"error", err)

				if input.FailureStrategy == "fail_fast" {
					output.TotalDuration = workflow.Now(ctx).Sub(startTime)
					return output, fmt.Errorf("loop failed at iteration %d: %w", i, err)
				}
			} else {
				output.TotalSuccess++
			}
		}
	}

	output.TotalDuration = workflow.Now(ctx).Sub(startTime)

	logger.Info("Loop workflow completed",
		"success", output.TotalSuccess,
		"failed", output.TotalFailed,
		"totalDuration", output.TotalDuration,
		"itemCount", output.ItemCount)

	return output, nil
}

// generateParameterCombinations generates all combinations of parameter values (cartesian product).
func generateParameterCombinations(params map[string][]string) []map[string]string {
	if len(params) == 0 {
		return nil
	}

	// Convert map to ordered slices for consistent iteration
	keys := make([]string, 0, len(params))
	values := make([][]string, 0, len(params))

	for key, vals := range params {
		keys = append(keys, key)
		values = append(values, vals)
	}

	// Generate cartesian product
	var result []map[string]string
	var generate func(int, map[string]string)

	generate = func(depth int, current map[string]string) {
		if depth == len(keys) {
			// Make a copy of current combination
			combo := make(map[string]string, len(current))
			for k, v := range current {
				combo[k] = v
			}
			result = append(result, combo)
			return
		}

		key := keys[depth]
		for _, value := range values[depth] {
			current[key] = value
			generate(depth+1, current)
		}
	}

	generate(0, make(map[string]string))
	return result
}

// ParameterizedLoopWorkflow executes containers with parameterized loops (withParam pattern).
func ParameterizedLoopWorkflow(ctx workflow.Context, input ParameterizedLoopInput) (*LoopOutput, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting parameterized loop workflow",
		"parameters", len(input.Parameters),
		"parallel", input.Parallel,
		"maxConcurrency", input.MaxConcurrency)

	// Validate input
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	// Generate all parameter combinations
	combinations := generateParameterCombinations(input.Parameters)

	logger.Info("Generated parameter combinations",
		"combinations", len(combinations))

	startTime := workflow.Now(ctx)

	// Default activity options
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	output := &LoopOutput{
		Results:   make([]ContainerExecutionOutput, 0, len(combinations)),
		ItemCount: len(combinations),
	}

	if input.Parallel {
		// Execute in parallel
		futures := make([]workflow.Future, len(combinations))

		for i, params := range combinations {
			// Substitute template variables
			containerInput := substituteContainerInput(input.Template, "", i, params)

			logger.Info("Scheduling parameterized iteration",
				"index", i,
				"params", params,
				"image", containerInput.Image)

			// Execute container activity
			futures[i] = workflow.ExecuteActivity(ctx, StartContainerActivity, containerInput)
		}

		// Collect results
		for i, future := range futures {
			var result ContainerExecutionOutput
			err := future.Get(ctx, &result)

			output.Results = append(output.Results, result)

			if err != nil || !result.Success {
				output.TotalFailed++
				logger.Error("Parameterized iteration failed",
					"index", i,
					"params", combinations[i],
					"error", err)

				if input.FailureStrategy == "fail_fast" {
					output.TotalDuration = workflow.Now(ctx).Sub(startTime)
					return output, fmt.Errorf("parameterized loop failed at iteration %d: %w", i, err)
				}
			} else {
				output.TotalSuccess++
			}
		}
	} else {
		// Execute sequentially
		for i, params := range combinations {
			// Substitute template variables
			containerInput := substituteContainerInput(input.Template, "", i, params)

			logger.Info("Executing parameterized iteration",
				"index", i,
				"params", params,
				"image", containerInput.Image)

			// Execute container activity
			var result ContainerExecutionOutput
			err := workflow.ExecuteActivity(ctx, StartContainerActivity, containerInput).Get(ctx, &result)

			output.Results = append(output.Results, result)

			if err != nil || !result.Success {
				output.TotalFailed++
				logger.Error("Parameterized iteration failed",
					"index", i,
					"params", params,
					"error", err)

				if input.FailureStrategy == "fail_fast" {
					output.TotalDuration = workflow.Now(ctx).Sub(startTime)
					return output, fmt.Errorf("parameterized loop failed at iteration %d: %w", i, err)
				}
			} else {
				output.TotalSuccess++
			}
		}
	}

	output.TotalDuration = workflow.Now(ctx).Sub(startTime)

	logger.Info("Parameterized loop workflow completed",
		"success", output.TotalSuccess,
		"failed", output.TotalFailed,
		"totalDuration", output.TotalDuration,
		"combinations", output.ItemCount)

	return output, nil
}
