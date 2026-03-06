package workflow

import (
	"fmt"
	"sync"
	"time"

	"go.temporal.io/sdk/temporal"
	wf "go.temporal.io/sdk/workflow"

	"github.com/jasoet/go-wf/docker/payload"
	"github.com/jasoet/go-wf/workflow/artifacts"
)

// dagState holds shared mutable state for DAG execution.
type dagState struct {
	mu          sync.Mutex
	executed    map[string]bool
	results     map[string]*payload.ContainerExecutionOutput
	stepOutputs map[string]map[string]string
}

func newDAGState() *dagState {
	return &dagState{
		executed:    make(map[string]bool),
		results:     make(map[string]*payload.ContainerExecutionOutput),
		stepOutputs: make(map[string]map[string]string),
	}
}

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

	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid DAG input: %w", err)
	}

	startTime := wf.Now(ctx)
	output := &payload.DAGWorkflowOutput{
		Results:     make(map[string]*payload.ContainerExecutionOutput),
		NodeResults: make([]payload.NodeResult, 0, len(input.Nodes)),
	}

	ctx = wf.WithActivityOptions(ctx, defaultActivityOptions())
	state := newDAGState()
	nodeMap := buildNodeMap(input.Nodes)

	var executeNode func(string) error
	executeNode = func(nodeName string) error {
		return executeDAGNode(ctx, nodeName, nodeMap, &input, state, output, executeNode)
	}

	for _, node := range input.Nodes {
		if err := executeNode(node.Name); err != nil {
			output.TotalDuration = wf.Now(ctx).Sub(startTime)
			return output, err
		}
	}

	output.Results = state.results
	output.StepOutputs = state.stepOutputs
	output.TotalDuration = wf.Now(ctx).Sub(startTime)

	logger.Info("DAG workflow completed",
		"success", output.TotalSuccess,
		"failed", output.TotalFailed,
		"duration", output.TotalDuration)

	return output, nil
}

func buildNodeMap(nodes []payload.DAGNode) map[string]*payload.DAGNode {
	nodeMap := make(map[string]*payload.DAGNode, len(nodes))
	for i := range nodes {
		nodeMap[nodes[i].Name] = &nodes[i]
	}
	return nodeMap
}

func executeDAGNode(ctx wf.Context, nodeName string, nodeMap map[string]*payload.DAGNode, input *payload.DAGWorkflowInput, state *dagState, output *payload.DAGWorkflowOutput, executeNode func(string) error) error {
	logger := wf.GetLogger(ctx)

	state.mu.Lock()
	if state.executed[nodeName] {
		state.mu.Unlock()
		return nil
	}
	state.mu.Unlock()

	node, ok := nodeMap[nodeName]
	if !ok {
		return fmt.Errorf("node not found: %s", nodeName)
	}

	if err := executeDependencies(executeNode, node, input, state); err != nil {
		return err
	}

	logger.Info("Executing node", "name", nodeName)

	containerInput := node.Container.ContainerExecutionInput
	if err := applyInputMappings(logger, &containerInput, node, state); err != nil {
		return err
	}

	if err := downloadInputArtifacts(ctx, logger, input, node); err != nil {
		return err
	}

	var result payload.ContainerExecutionOutput
	err := wf.ExecuteActivity(ctx, containerInput.ActivityName(), containerInput).Get(ctx, &result)

	extractAndStoreOutputs(logger, node, &result, state)
	uploadOutputArtifacts(ctx, logger, input, node, &result)

	state.mu.Lock()
	state.results[nodeName] = &result
	state.executed[nodeName] = true
	state.mu.Unlock()

	recordNodeResult(nodeName, &result, err, ctx, input.FailFast, output, logger)

	if (err != nil || !result.Success) && input.FailFast {
		return err
	}
	return nil
}

func recordNodeResult(nodeName string, result *payload.ContainerExecutionOutput, err error, ctx wf.Context, failFast bool, output *payload.DAGWorkflowOutput, logger interface {
	Info(string, ...interface{})
	Error(string, ...interface{})
},
) {
	nodeResult := payload.NodeResult{
		NodeName:  nodeName,
		Result:    result,
		StartTime: wf.Now(ctx),
	}

	if err != nil || !result.Success {
		nodeResult.Success = false
		nodeResult.Error = err
		output.TotalFailed++
		logger.Error("Node failed", "name", nodeName, "error", err)
	} else {
		nodeResult.Success = true
		output.TotalSuccess++
		logger.Info("Node completed", "name", nodeName)
	}

	output.NodeResults = append(output.NodeResults, nodeResult)
}

func defaultActivityOptions() wf.ActivityOptions {
	return wf.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
}

func executeDependencies(executeNode func(string) error, node *payload.DAGNode, input *payload.DAGWorkflowInput, state *dagState) error {
	for _, dep := range node.Dependencies {
		if err := executeNode(dep); err != nil {
			return err
		}

		if input.FailFast {
			state.mu.Lock()
			depResult := state.results[dep]
			state.mu.Unlock()

			if depResult != nil && !depResult.Success {
				return fmt.Errorf("dependency %s failed", dep)
			}
		}
	}
	return nil
}

func applyInputMappings(logger interface{ Info(string, ...interface{}) }, containerInput *payload.ContainerExecutionInput, node *payload.DAGNode, state *dagState) error {
	if len(node.Container.Inputs) == 0 {
		return nil
	}

	state.mu.Lock()
	inputErr := SubstituteInputs(containerInput, node.Container.Inputs, state.stepOutputs)
	state.mu.Unlock()

	if inputErr != nil {
		return fmt.Errorf("failed to substitute inputs for node %s: %w", node.Name, inputErr)
	}
	logger.Info("Applied input mappings", "name", node.Name, "inputs", len(node.Container.Inputs))
	return nil
}

func downloadInputArtifacts(ctx wf.Context, logger interface{ Info(string, ...interface{}) }, input *payload.DAGWorkflowInput, node *payload.DAGNode) error {
	if input.ArtifactStore == nil || len(node.Container.InputArtifacts) == 0 {
		return nil
	}

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
			StepName:   node.Name,
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
	return nil
}

func extractAndStoreOutputs(logger interface {
	Info(string, ...interface{})
	Error(string, ...interface{})
}, node *payload.DAGNode, result *payload.ContainerExecutionOutput, state *dagState,
) {
	if len(node.Container.Outputs) == 0 || !result.Success {
		return
	}

	outputs, extractErr := ExtractOutputs(node.Container.Outputs, result)
	if extractErr != nil {
		logger.Error("Failed to extract outputs", "name", node.Name, "error", extractErr)
		return
	}

	state.mu.Lock()
	state.stepOutputs[node.Name] = outputs
	state.mu.Unlock()
	logger.Info("Extracted outputs", "name", node.Name, "outputs", outputs)
}

func uploadOutputArtifacts(ctx wf.Context, logger interface {
	Info(string, ...interface{})
	Error(string, ...interface{})
}, input *payload.DAGWorkflowInput, node *payload.DAGNode, result *payload.ContainerExecutionOutput,
) {
	if input.ArtifactStore == nil || len(node.Container.OutputArtifacts) == 0 || !result.Success {
		return
	}

	store, ok := input.ArtifactStore.(artifacts.ArtifactStore)
	if !ok {
		return
	}

	for _, artifact := range node.Container.OutputArtifacts {
		metadata := artifacts.ArtifactMetadata{
			Name:       artifact.Name,
			Path:       artifact.Path,
			Type:       artifact.Type,
			WorkflowID: wf.GetInfo(ctx).WorkflowExecution.ID,
			RunID:      wf.GetInfo(ctx).WorkflowExecution.RunID,
			StepName:   node.Name,
		}

		uploadInput := artifacts.UploadArtifactInput{
			Metadata:   metadata,
			SourcePath: artifact.Path,
		}

		err := wf.ExecuteActivity(ctx, artifacts.UploadArtifactActivity, store, uploadInput).Get(ctx, nil)
		if err != nil && !artifact.Optional {
			logger.Error("Failed to upload artifact", "name", artifact.Name, "error", err)
		} else if err == nil {
			logger.Info("Uploaded artifact", "name", artifact.Name, "path", artifact.Path)
		}
	}
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
	err := wf.ExecuteActivity(ctx, input.ActivityName(), input).Get(ctx, &output)

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
