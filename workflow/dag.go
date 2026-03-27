package workflow

import (
	"fmt"
	"time"

	"github.com/jasoet/go-wf/workflow/errors"
)

// DAGNode defines a single node in a DAG workflow.
type DAGNode[I TaskInput, O TaskOutput] struct {
	Name         string   `json:"name" validate:"required"`
	Input        I        `json:"input" validate:"required"`
	Dependencies []string `json:"dependencies,omitempty"`
}

// DAGInput defines a DAG workflow execution.
type DAGInput[I TaskInput, O TaskOutput] struct {
	Nodes       []DAGNode[I, O] `json:"nodes" validate:"required,min=1"`
	FailFast    bool            `json:"fail_fast"`
	MaxParallel int             `json:"max_parallel,omitempty"`
}

// Validate validates DAG input including cycle detection.
func (d *DAGInput[I, O]) Validate() error {
	if len(d.Nodes) == 0 {
		return errors.ErrInvalidInput.Wrap("at least one node is required")
	}

	// Build node name set and check for duplicates.
	nodeMap := make(map[string]bool, len(d.Nodes))
	for _, node := range d.Nodes {
		if node.Name == "" {
			return errors.ErrInvalidInput.Wrap("node name is required")
		}
		if nodeMap[node.Name] {
			return errors.ErrInvalidInput.Wrap(fmt.Sprintf("duplicate node name: %s", node.Name))
		}
		nodeMap[node.Name] = true
	}

	// Check that all dependencies reference existing nodes.
	for _, node := range d.Nodes {
		for _, dep := range node.Dependencies {
			if !nodeMap[dep] {
				return errors.ErrInvalidInput.Wrap(fmt.Sprintf("dependency node not found: %s", dep))
			}
		}
	}

	// DFS-based cycle detection.
	if err := dagDetectCycles(d.Nodes); err != nil {
		return err
	}

	// Validate each node's input.
	for idx := range d.Nodes {
		if err := d.Nodes[idx].Input.Validate(); err != nil {
			return errors.ErrInvalidInput.Wrap(
				fmt.Sprintf("node %s: %v", d.Nodes[idx].Name, err),
			)
		}
	}

	return nil
}

// dagDetectCycles uses DFS to find circular dependencies in the DAG.
func dagDetectCycles[I TaskInput, O TaskOutput](nodes []DAGNode[I, O]) error {
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

// NodeResult holds the result of a single DAG node execution.
type NodeResult[O TaskOutput] struct {
	Name     string        `json:"name"`
	Result   *O            `json:"result,omitempty"`
	Error    string        `json:"error,omitempty"`
	Duration time.Duration `json:"duration"`
	Success  bool          `json:"success"`
}

// DAGOutput holds the results of a DAG workflow execution.
type DAGOutput[O TaskOutput] struct {
	Results       map[string]*O   `json:"results"`
	NodeResults   []NodeResult[O] `json:"node_results"`
	TotalSuccess  int             `json:"total_success"`
	TotalFailed   int             `json:"total_failed"`
	TotalDuration time.Duration   `json:"total_duration"`
}
