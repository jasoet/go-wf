package workflow

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"go.temporal.io/sdk/temporal"
	wf "go.temporal.io/sdk/workflow"

	"github.com/jasoet/go-wf/function/payload"
)

// dagState holds shared mutable state for function DAG execution.
type dagState struct {
	mu          sync.Mutex
	executed    map[string]bool
	results     map[string]*payload.FunctionExecutionOutput
	stepOutputs map[string]map[string]string
	stepData    map[string][]byte
}

func newDagState() *dagState {
	return &dagState{
		executed:    make(map[string]bool),
		results:     make(map[string]*payload.FunctionExecutionOutput),
		stepOutputs: make(map[string]map[string]string),
		stepData:    make(map[string][]byte),
	}
}

// DAGWorkflow executes functions in a DAG (Directed Acyclic Graph) pattern.
// Execution order is determined by the dependency graph, with support for
// input mappings (passing outputs between nodes) and data mappings (passing
// byte data between nodes).
func DAGWorkflow(ctx wf.Context, input payload.DAGWorkflowInput) (*payload.FunctionDAGWorkflowOutput, error) {
	logger := wf.GetLogger(ctx)
	logger.Info("Starting function DAG workflow", "nodes", len(input.Nodes))

	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid DAG input: %w", err)
	}

	startTime := wf.Now(ctx)
	output := &payload.FunctionDAGWorkflowOutput{
		Results:     make(map[string]*payload.FunctionExecutionOutput),
		NodeResults: make([]payload.FunctionNodeResult, 0, len(input.Nodes)),
	}

	ctx = wf.WithActivityOptions(ctx, dagActivityOptions())
	state := newDagState()
	nodeMap := buildFnNodeMap(input.Nodes)

	var executeNode func(string) error
	executeNode = func(nodeName string) error {
		return executeFnDAGNode(ctx, nodeName, nodeMap, &input, state, output, executeNode)
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

	logger.Info("Function DAG workflow completed",
		"success", output.TotalSuccess,
		"failed", output.TotalFailed,
		"duration", output.TotalDuration)

	return output, nil
}

func buildFnNodeMap(nodes []payload.FunctionDAGNode) map[string]*payload.FunctionDAGNode {
	nodeMap := make(map[string]*payload.FunctionDAGNode, len(nodes))
	for i := range nodes {
		nodeMap[nodes[i].Name] = &nodes[i]
	}
	return nodeMap
}

func executeFnDAGNode(
	ctx wf.Context,
	nodeName string,
	nodeMap map[string]*payload.FunctionDAGNode,
	input *payload.DAGWorkflowInput,
	state *dagState,
	output *payload.FunctionDAGWorkflowOutput,
	executeNode func(string) error,
) error {
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

	if err := executeFnDependencies(executeNode, node, input, state); err != nil {
		return err
	}

	logger.Info("Executing function node", "name", nodeName)

	fnInput := node.Function
	if err := applyFnInputMappings(logger, &fnInput, node, state); err != nil {
		return err
	}
	applyFnDataMapping(&fnInput, node, state)

	var result payload.FunctionExecutionOutput
	err := wf.ExecuteActivity(ctx, fnInput.ActivityName(), fnInput).Get(ctx, &result)

	extractFnOutputs(logger, node, &result, state)

	state.mu.Lock()
	state.results[nodeName] = &result
	if result.Data != nil {
		state.stepData[nodeName] = result.Data
	}
	state.executed[nodeName] = true
	state.mu.Unlock()

	recordFnNodeResult(nodeName, &result, err, ctx, input.FailFast, output, logger)

	if (err != nil || !result.Success) && input.FailFast {
		if err != nil {
			return err
		}
		return fmt.Errorf("node %s failed", nodeName)
	}
	return nil
}

func executeFnDependencies(
	executeNode func(string) error,
	node *payload.FunctionDAGNode,
	input *payload.DAGWorkflowInput,
	state *dagState,
) error {
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

func applyFnInputMappings(
	logger interface{ Info(string, ...interface{}) },
	fnInput *payload.FunctionExecutionInput,
	node *payload.FunctionDAGNode,
	state *dagState,
) error {
	if len(node.Inputs) == 0 {
		return nil
	}

	if fnInput.Args == nil {
		fnInput.Args = make(map[string]string)
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	for _, mapping := range node.Inputs {
		parts := strings.SplitN(mapping.From, ".", 2)
		if len(parts) != 2 {
			if mapping.Required {
				return fmt.Errorf("invalid input mapping format for node %s: %s (expected 'nodeName.outputName')", node.Name, mapping.From)
			}
			if mapping.Default != "" {
				fnInput.Args[mapping.Name] = mapping.Default
			}
			continue
		}

		fromNode := parts[0]
		outputName := parts[1]

		if outputs, ok := state.stepOutputs[fromNode]; ok {
			if value, ok := outputs[outputName]; ok {
				fnInput.Args[mapping.Name] = value
				continue
			}
		}

		if mapping.Default != "" {
			fnInput.Args[mapping.Name] = mapping.Default
		} else if mapping.Required {
			return fmt.Errorf("required input %s not found for node %s (from %s)", mapping.Name, node.Name, mapping.From)
		}
	}

	logger.Info("Applied input mappings", "name", node.Name, "inputs", len(node.Inputs))
	return nil
}

func applyFnDataMapping(
	fnInput *payload.FunctionExecutionInput,
	node *payload.FunctionDAGNode,
	state *dagState,
) {
	if node.DataInput == nil {
		return
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	if data, ok := state.stepData[node.DataInput.FromNode]; ok {
		fnInput.Data = data
	}
}

func extractFnOutputs(
	logger interface {
		Info(string, ...interface{})
		Error(string, ...interface{})
	},
	node *payload.FunctionDAGNode,
	result *payload.FunctionExecutionOutput,
	state *dagState,
) {
	if len(node.Outputs) == 0 || !result.Success {
		return
	}

	outputs := make(map[string]string)
	for _, om := range node.Outputs {
		if value, ok := result.Result[om.ResultKey]; ok {
			outputs[om.Name] = value
		} else if om.Default != "" {
			outputs[om.Name] = om.Default
		} else {
			logger.Error("Output key not found in result", "name", node.Name, "key", om.ResultKey)
		}
	}

	if len(outputs) > 0 {
		state.mu.Lock()
		state.stepOutputs[node.Name] = outputs
		state.mu.Unlock()
		logger.Info("Extracted outputs", "name", node.Name, "outputs", outputs)
	}
}

func recordFnNodeResult(
	nodeName string,
	result *payload.FunctionExecutionOutput,
	err error,
	ctx wf.Context,
	failFast bool,
	output *payload.FunctionDAGWorkflowOutput,
	logger interface {
		Info(string, ...interface{})
		Error(string, ...interface{})
	},
) {
	nodeResult := payload.FunctionNodeResult{
		NodeName:  nodeName,
		Result:    result,
		StartTime: wf.Now(ctx),
	}

	if err != nil || !result.Success {
		nodeResult.Success = false
		nodeResult.Error = err
		output.TotalFailed++
		logger.Error("Function node failed", "name", nodeName, "error", err)
	} else {
		nodeResult.Success = true
		output.TotalSuccess++
		logger.Info("Function node completed", "name", nodeName)
	}

	output.NodeResults = append(output.NodeResults, nodeResult)
}

func dagActivityOptions() wf.ActivityOptions {
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
