package payload

import (
	"fmt"
	"time"

	"github.com/jasoet/go-wf/workflow/artifacts"
	"github.com/jasoet/go-wf/workflow/errors"
)

// OutputMapping defines how to capture output from a function execution result.
type OutputMapping struct {
	// Name is the output identifier.
	Name string `json:"name" validate:"required"`

	// ResultKey is the key to extract from the function result map.
	ResultKey string `json:"result_key" validate:"required"`

	// Default value if extraction fails.
	Default string `json:"default,omitempty"`
}

// FunctionInputMapping defines how to map outputs from previous nodes to inputs.
type FunctionInputMapping struct {
	// Name is the argument or parameter name.
	Name string `json:"name" validate:"required"`

	// From specifies the source in format "node-name.output-name".
	From string `json:"from" validate:"required"`

	// Default value if the source is not available.
	Default string `json:"default,omitempty"`

	// Required indicates if this input must be present.
	Required bool `json:"required"`
}

// DataMapping defines how to pass data output from one node to another.
type DataMapping struct {
	// FromNode is the source node whose data output will be used.
	FromNode string `json:"from_node" validate:"required"`

	// Optional indicates if missing data should be ignored.
	Optional bool `json:"optional"`
}

// FunctionDAGNode represents a node in a function DAG workflow.
type FunctionDAGNode struct {
	// Name is the node identifier.
	Name string `json:"name" validate:"required"`

	// Function is the function execution input for this node.
	Function FunctionExecutionInput `json:"function" validate:"required"`

	// Dependencies are the nodes that must complete before this node.
	Dependencies []string `json:"dependencies,omitempty"`

	// Outputs defines how to capture outputs from the function result.
	Outputs []OutputMapping `json:"outputs,omitempty"`

	// Inputs defines how to map outputs from previous nodes to function args.
	Inputs []FunctionInputMapping `json:"inputs,omitempty"`

	// DataInput defines how to pass data from a previous node.
	DataInput *DataMapping `json:"data_input,omitempty"`

	// InputArtifacts defines artifacts to download before execution.
	InputArtifacts []artifacts.ArtifactRef `json:"input_artifacts,omitempty"`

	// OutputArtifacts defines artifacts to upload after execution.
	OutputArtifacts []artifacts.ArtifactRef `json:"output_artifacts,omitempty"`
}

// DAGWorkflowInput defines a DAG (Directed Acyclic Graph) workflow for functions.
type DAGWorkflowInput struct {
	// Nodes are the workflow nodes.
	Nodes []FunctionDAGNode `json:"nodes" validate:"required,min=1"`

	// FailFast determines if the workflow should stop on first failure.
	FailFast bool `json:"fail_fast"`

	// MaxParallel limits the number of parallel executions.
	MaxParallel int `json:"max_parallel,omitempty"`

	// ArtifactStore is the artifact storage backend (optional).
	// If nil, artifact operations are skipped.
	ArtifactStore artifacts.ArtifactStore `json:"-"`
}

// Validate validates DAG workflow input including structural integrity checks.
func (i *DAGWorkflowInput) Validate() error {
	if len(i.Nodes) == 0 {
		return errors.ErrInvalidInput.Wrap("at least one node is required")
	}

	// Build node name set and check for duplicates.
	nodeMap := make(map[string]bool, len(i.Nodes))
	for _, node := range i.Nodes {
		if nodeMap[node.Name] {
			return errors.ErrInvalidInput.Wrap(fmt.Sprintf("duplicate node name: %s", node.Name))
		}
		nodeMap[node.Name] = true
	}

	// Check that all dependencies reference existing nodes.
	for _, node := range i.Nodes {
		for _, dep := range node.Dependencies {
			if !nodeMap[dep] {
				return errors.ErrInvalidInput.Wrap("dependency node not found: " + dep)
			}
		}
	}

	// DFS-based cycle detection.
	if err := detectCycles(i.Nodes); err != nil {
		return err
	}

	return nil
}

// detectCycles uses DFS to find circular dependencies in the DAG.
func detectCycles(nodes []FunctionDAGNode) error {
	// Build adjacency list: node -> dependencies.
	deps := make(map[string][]string, len(nodes))
	for _, node := range nodes {
		deps[node.Name] = node.Dependencies
	}

	const (
		unvisited = 0
		visiting  = 1
		visited   = 2
	)

	state := make(map[string]int, len(nodes))

	var dfs func(name string) error
	dfs = func(name string) error {
		state[name] = visiting
		for _, dep := range deps[name] {
			switch state[dep] {
			case visiting:
				return errors.ErrInvalidInput.Wrap(
					fmt.Sprintf("circular dependency detected involving node: %s", dep),
				)
			case unvisited:
				if err := dfs(dep); err != nil {
					return err
				}
			}
		}
		state[name] = visited
		return nil
	}

	for _, node := range nodes {
		if state[node.Name] == unvisited {
			if err := dfs(node.Name); err != nil {
				return err
			}
		}
	}

	return nil
}

// FunctionNodeResult represents the execution result of a single DAG node.
type FunctionNodeResult struct {
	// NodeName is the name of the node.
	NodeName string `json:"node_name"`

	// Result is the function execution output.
	Result *FunctionExecutionOutput `json:"result,omitempty"`

	// StartTime is when the node started executing.
	StartTime time.Time `json:"start_time"`

	// Success indicates if the node executed successfully.
	Success bool `json:"success"`

	// Error contains error information if the node failed.
	Error error `json:"error,omitempty"`
}

// FunctionDAGWorkflowOutput defines the output of a function DAG workflow execution.
type FunctionDAGWorkflowOutput struct {
	// Results is a map of node name to execution output.
	Results map[string]*FunctionExecutionOutput `json:"results"`

	// NodeResults is a list of all node results in execution order.
	NodeResults []FunctionNodeResult `json:"node_results"`

	// StepOutputs contains extracted outputs from each step.
	StepOutputs map[string]map[string]string `json:"step_outputs,omitempty"`

	// TotalSuccess is the count of successful nodes.
	TotalSuccess int `json:"total_success"`

	// TotalFailed is the count of failed nodes.
	TotalFailed int `json:"total_failed"`

	// TotalDuration is the total execution time.
	TotalDuration time.Duration `json:"total_duration"`
}
