package workflow

import (
	"fmt"
	"sync"
	"time"

	"github.com/jasoet/go-wf/docker/activity"
	"github.com/jasoet/go-wf/docker/artifacts"
	"github.com/jasoet/go-wf/docker/payload"
	"go.temporal.io/sdk/temporal"
	wf "go.temporal.io/sdk/workflow"
)

// DAGWorkflow executes containers in a DAG (Directed Acyclic Graph) pattern.
// This allows for complex dependencies between containers where execution order
// is determined by the dependency graph rather than simple sequential or parallel execution.
//
// Example:
//
//	input := payload.DAGWorkflowInput{
//	    Nodes: []payload.DAGNode{
//	        {Name: "build", Container: buildInput},
//	        {Name: "test", Container: testInput, Dependencies: []string{"build"}},
//	        {Name: "deploy", Container: deployInput, Dependencies: []string{"test"}},
//	    },
//	}
//	output, err := docker.DAGWorkflow(ctx, input)
func DAGWorkflow(ctx wf.Context, input payload.DAGWorkflowInput) (*payload.DAGWorkflowOutput, error) {
	logger := wf.GetLogger(ctx)
	logger.Info("Starting DAG workflow", "nodes", len(input.Nodes))

	// Validate input
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid DAG input: %w", err)
	}

	startTime := wf.Now(ctx)
	output := &payload.DAGWorkflowOutput{
		Results:     make(map[string]*payload.ContainerExecutionOutput),
		NodeResults: make([]payload.NodeResult, 0, len(input.Nodes)),
	}

	// Build dependency map
	depMap := make(map[string][]string)
	for _, node := range input.Nodes {
		depMap[node.Name] = node.Dependencies
	}

	// Execute nodes based on dependencies
	executed := make(map[string]bool)
	results := make(map[string]*payload.ContainerExecutionOutput)
	stepOutputs := make(map[string]map[string]string) // Store extracted outputs by step name
	resultsMutex := sync.Mutex{}

	// Activity options
	ao := wf.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = wf.WithActivityOptions(ctx, ao)

	// Execute nodes in topological order
	var executeNode func(nodeName string) error
	executeNode = func(nodeName string) error {
		// Check if already executed
		resultsMutex.Lock()
		if executed[nodeName] {
			resultsMutex.Unlock()
			return nil
		}
		resultsMutex.Unlock()

		// Find node
		var node *payload.DAGNode
		for i := range input.Nodes {
			if input.Nodes[i].Name == nodeName {
				node = &input.Nodes[i]
				break
			}
		}
		if node == nil {
			return fmt.Errorf("node not found: %s", nodeName)
		}

		// Execute dependencies first
		for _, dep := range node.Dependencies {
			if err := executeNode(dep); err != nil {
				return err
			}

			// Check if dependency failed (if fail-fast enabled)
			if input.FailFast {
				resultsMutex.Lock()
				depResult := results[dep]
				resultsMutex.Unlock()

				if depResult != nil && !depResult.Success {
					return fmt.Errorf("dependency %s failed", dep)
				}
			}
		}

		logger.Info("Executing node", "name", nodeName)

		// Prepare container input with input substitution
		containerInput := node.Container.ContainerExecutionInput

		// Apply input mappings if defined
		if len(node.Container.Inputs) > 0 {
			resultsMutex.Lock()
			inputErr := SubstituteInputs(&containerInput, node.Container.Inputs, stepOutputs)
			resultsMutex.Unlock()

			if inputErr != nil {
				return fmt.Errorf("failed to substitute inputs for node %s: %w", nodeName, inputErr)
			}
			logger.Info("Applied input mappings", "name", nodeName, "inputs", len(node.Container.Inputs))
		}

		// Download input artifacts if artifact store is configured
		if input.ArtifactStore != nil && len(node.Container.InputArtifacts) > 0 {
			store, ok := input.ArtifactStore.(artifacts.ArtifactStore)
			if !ok {
				return fmt.Errorf("invalid artifact store type")
			}

			for _, artifact := range node.Container.InputArtifacts {
				metadata := artifacts.ArtifactMetadata{
					Name:       artifact.Name,
					Path:       artifact.Path,
					Type:       artifact.Type,
					WorkflowID: wf.GetInfo(ctx).WorkflowExecution.ID,
					RunID:      wf.GetInfo(ctx).WorkflowExecution.RunID,
					StepName:   nodeName,
				}

				downloadInput := artifacts.DownloadArtifactInput{
					Metadata: metadata,
					DestPath: artifact.Path,
				}

				err := wf.ExecuteActivity(ctx, artifacts.DownloadArtifactActivity, store, downloadInput).Get(ctx, nil)
				if err != nil && !artifact.Optional {
					return fmt.Errorf("failed to download artifact %s: %w", artifact.Name, err)
				}
				if err == nil {
					logger.Info("Downloaded artifact", "name", artifact.Name, "path", artifact.Path)
				}
			}
		}

		// Execute this node
		var result payload.ContainerExecutionOutput
		err := wf.ExecuteActivity(ctx, activity.StartContainerActivity, containerInput).Get(ctx, &result)

		// Extract outputs if defined
		if len(node.Container.Outputs) > 0 && result.Success {
			outputs, extractErr := ExtractOutputs(node.Container.Outputs, &result)
			if extractErr != nil {
				logger.Error("Failed to extract outputs", "name", nodeName, "error", extractErr)
				// Don't fail the workflow, just log the error
			} else {
				resultsMutex.Lock()
				stepOutputs[nodeName] = outputs
				resultsMutex.Unlock()
				logger.Info("Extracted outputs", "name", nodeName, "outputs", outputs)
			}
		}

		// Upload output artifacts if artifact store is configured
		if input.ArtifactStore != nil && len(node.Container.OutputArtifacts) > 0 && result.Success {
			store, ok := input.ArtifactStore.(artifacts.ArtifactStore)
			if !ok {
				return fmt.Errorf("invalid artifact store type")
			}

			for _, artifact := range node.Container.OutputArtifacts {
				metadata := artifacts.ArtifactMetadata{
					Name:       artifact.Name,
					Path:       artifact.Path,
					Type:       artifact.Type,
					WorkflowID: wf.GetInfo(ctx).WorkflowExecution.ID,
					RunID:      wf.GetInfo(ctx).WorkflowExecution.RunID,
					StepName:   nodeName,
				}

				uploadInput := artifacts.UploadArtifactInput{
					Metadata:   metadata,
					SourcePath: artifact.Path,
				}

				err := wf.ExecuteActivity(ctx, artifacts.UploadArtifactActivity, store, uploadInput).Get(ctx, nil)
				if err != nil && !artifact.Optional {
					logger.Error("Failed to upload artifact", "name", artifact.Name, "error", err)
					// Don't fail the workflow, just log the error
				} else if err == nil {
					logger.Info("Uploaded artifact", "name", artifact.Name, "path", artifact.Path)
				}
			}
		}

		resultsMutex.Lock()
		results[nodeName] = &result
		executed[nodeName] = true
		resultsMutex.Unlock()

		nodeResult := payload.NodeResult{
			NodeName:  nodeName,
			Result:    &result,
			StartTime: wf.Now(ctx),
		}

		if err != nil || !result.Success {
			nodeResult.Success = false
			nodeResult.Error = err
			output.TotalFailed++

			logger.Error("Node failed", "name", nodeName, "error", err)

			if input.FailFast {
				return err
			}
		} else {
			nodeResult.Success = true
			output.TotalSuccess++
			logger.Info("Node completed", "name", nodeName)
		}

		output.NodeResults = append(output.NodeResults, nodeResult)

		return nil
	}

	// Execute all nodes
	for _, node := range input.Nodes {
		if err := executeNode(node.Name); err != nil {
			output.TotalDuration = wf.Now(ctx).Sub(startTime)
			return output, err
		}
	}

	// Copy results to output
	output.Results = results
	output.StepOutputs = stepOutputs
	output.TotalDuration = wf.Now(ctx).Sub(startTime)

	logger.Info("DAG workflow completed",
		"success", output.TotalSuccess,
		"failed", output.TotalFailed,
		"duration", output.TotalDuration)

	return output, nil
}

// WorkflowWithParameters executes a workflow with input parameters.
//
// Parameters are substituted in environment variables and commands.
//
// Example:
//
//	input := payload.ContainerExecutionInput{
//	    Image: "alpine:latest",
//	    Command: []string{"echo", "{{.version}}"},
//	    Env: map[string]string{"VERSION": "{{.version}}"},
//	}
//	params := []payload.WorkflowParameter{
//	    {Name: "version", Value: "v1.2.3"},
//	}
//	output, err := docker.WorkflowWithParameters(ctx, input, params)
func WorkflowWithParameters(ctx wf.Context, input payload.ContainerExecutionInput, params []payload.WorkflowParameter) (*payload.ContainerExecutionOutput, error) {
	// Substitute parameters in input
	paramMap := make(map[string]string)
	for _, param := range params {
		paramMap[param.Name] = param.Value
	}

	// Substitute in environment variables
	for key, value := range input.Env {
		for paramName, paramValue := range paramMap {
			placeholder := fmt.Sprintf("{{.%s}}", paramName)
			value = replaceAll(value, placeholder, paramValue)
		}
		input.Env[key] = value
	}

	// Substitute in command
	for i, cmd := range input.Command {
		for paramName, paramValue := range paramMap {
			placeholder := fmt.Sprintf("{{.%s}}", paramName)
			cmd = replaceAll(cmd, placeholder, paramValue)
		}
		input.Command[i] = cmd
	}

	// Execute workflow
	return executeContainerInternal(ctx, input)
}

// executeContainerInternal is a helper to execute container without workflow context creation.
func executeContainerInternal(ctx wf.Context, input payload.ContainerExecutionInput) (*payload.ContainerExecutionOutput, error) {
	logger := wf.GetLogger(ctx)
	logger.Info("Executing container", "image", input.Image)

	timeout := input.RunTimeout
	if timeout == 0 {
		timeout = 10 * time.Minute
	}

	ao := wf.ActivityOptions{
		StartToCloseTimeout: timeout,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = wf.WithActivityOptions(ctx, ao)

	var output payload.ContainerExecutionOutput
	err := wf.ExecuteActivity(ctx, activity.StartContainerActivity, input).Get(ctx, &output)

	return &output, err
}

// replaceAll replaces all occurrences of old with new in s.
func replaceAll(s, old, new string) string {
	result := ""
	for {
		i := indexOf(s, old)
		if i == -1 {
			result += s
			break
		}
		result += s[:i] + new
		s = s[i+len(old):]
	}
	return result
}

// indexOf returns the index of the first occurrence of substr in s, or -1 if not present.
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
