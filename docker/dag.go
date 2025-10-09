package docker

import (
	"fmt"
	"sync"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// DAGWorkflow executes containers in a DAG (Directed Acyclic Graph) pattern.
// This allows for complex dependencies between containers where execution order
// is determined by the dependency graph rather than simple sequential or parallel execution.
//
// Example:
//
//	input := docker.DAGWorkflowInput{
//	    Nodes: []docker.DAGNode{
//	        {Name: "build", Container: buildInput},
//	        {Name: "test", Container: testInput, Dependencies: []string{"build"}},
//	        {Name: "deploy", Container: deployInput, Dependencies: []string{"test"}},
//	    },
//	}
//	output, err := docker.DAGWorkflow(ctx, input)
func DAGWorkflow(ctx workflow.Context, input DAGWorkflowInput) (*DAGWorkflowOutput, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting DAG workflow", "nodes", len(input.Nodes))

	// Validate input
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid DAG input: %w", err)
	}

	startTime := workflow.Now(ctx)
	output := &DAGWorkflowOutput{
		Results:     make(map[string]*ContainerExecutionOutput),
		NodeResults: make([]NodeResult, 0, len(input.Nodes)),
	}

	// Build dependency map
	depMap := make(map[string][]string)
	for _, node := range input.Nodes {
		depMap[node.Name] = node.Dependencies
	}

	// Execute nodes based on dependencies
	executed := make(map[string]bool)
	results := make(map[string]*ContainerExecutionOutput)
	resultsMutex := sync.Mutex{}

	// Activity options
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
		var node *DAGNode
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

		// Execute this node
		var result ContainerExecutionOutput
		err := workflow.ExecuteActivity(ctx, StartContainerActivity, node.Container.ContainerExecutionInput).Get(ctx, &result)

		resultsMutex.Lock()
		results[nodeName] = &result
		executed[nodeName] = true
		resultsMutex.Unlock()

		nodeResult := NodeResult{
			NodeName:  nodeName,
			Result:    &result,
			StartTime: workflow.Now(ctx),
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
			output.TotalDuration = workflow.Now(ctx).Sub(startTime)
			return output, err
		}
	}

	// Copy results to output
	output.Results = results
	output.TotalDuration = workflow.Now(ctx).Sub(startTime)

	logger.Info("DAG workflow completed",
		"success", output.TotalSuccess,
		"failed", output.TotalFailed,
		"duration", output.TotalDuration)

	return output, nil
}

// DAGWorkflowOutput defines the output from DAG workflow execution.
type DAGWorkflowOutput struct {
	Results       map[string]*ContainerExecutionOutput `json:"results"`
	NodeResults   []NodeResult                         `json:"node_results"`
	TotalSuccess  int                                  `json:"total_success"`
	TotalFailed   int                                  `json:"total_failed"`
	TotalDuration time.Duration                        `json:"total_duration"`
}

// NodeResult represents the result of a single DAG node execution.
type NodeResult struct {
	NodeName  string                    `json:"node_name"`
	Result    *ContainerExecutionOutput `json:"result"`
	Success   bool                      `json:"success"`
	Error     error                     `json:"error,omitempty"`
	StartTime time.Time                 `json:"start_time"`
}

// WorkflowWithParameters executes a workflow with input parameters.
//
// Parameters are substituted in environment variables and commands.
//
// Example:
//
//	input := docker.ContainerExecutionInput{
//	    Image: "alpine:latest",
//	    Command: []string{"echo", "{{.version}}"},
//	    Env: map[string]string{"VERSION": "{{.version}}"},
//	}
//	params := []docker.WorkflowParameter{
//	    {Name: "version", Value: "v1.2.3"},
//	}
//	output, err := docker.WorkflowWithParameters(ctx, input, params)
func WorkflowWithParameters(ctx workflow.Context, input ContainerExecutionInput, params []WorkflowParameter) (*ContainerExecutionOutput, error) {
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
func executeContainerInternal(ctx workflow.Context, input ContainerExecutionInput) (*ContainerExecutionOutput, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Executing container", "image", input.Image)

	timeout := input.RunTimeout
	if timeout == 0 {
		timeout = 10 * time.Minute
	}

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

	var output ContainerExecutionOutput
	err := workflow.ExecuteActivity(ctx, StartContainerActivity, input).Get(ctx, &output)

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
