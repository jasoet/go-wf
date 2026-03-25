package builder

import (
	"fmt"

	"github.com/jasoet/go-wf/function/payload"
)

// DAGBuilder provides a fluent API for constructing function DAG workflow inputs.
type DAGBuilder struct {
	name        string
	nodes       []payload.FunctionDAGNode
	nodeIndex   map[string]int // name → index in nodes slice
	failFast    bool
	maxParallel int
	errors      []error
}

// NewDAGBuilder creates a new DAG builder with the specified name.
func NewDAGBuilder(name string) *DAGBuilder {
	return &DAGBuilder{
		name:      name,
		nodes:     make([]payload.FunctionDAGNode, 0),
		nodeIndex: make(map[string]int),
	}
}

// AddNode adds a node from a WorkflowSource with optional dependencies.
func (b *DAGBuilder) AddNode(name string, source WorkflowSource, deps ...string) *DAGBuilder {
	if source == nil {
		b.errors = append(b.errors, fmt.Errorf("cannot add nil source for node %q", name))
		return b
	}

	return b.AddNodeWithInput(name, source.ToInput(), deps...)
}

// AddNodeWithInput adds a node with a FunctionExecutionInput directly.
func (b *DAGBuilder) AddNodeWithInput(name string, input payload.FunctionExecutionInput, deps ...string) *DAGBuilder {
	if _, exists := b.nodeIndex[name]; exists {
		b.errors = append(b.errors, fmt.Errorf("duplicate node name: %s", name))
		return b
	}

	node := payload.FunctionDAGNode{
		Name:         name,
		Function:     input,
		Dependencies: deps,
	}

	b.nodeIndex[name] = len(b.nodes)
	b.nodes = append(b.nodes, node)
	return b
}

// WithOutputMapping appends output mappings to the named node.
func (b *DAGBuilder) WithOutputMapping(nodeName string, mappings ...payload.OutputMapping) *DAGBuilder {
	idx, exists := b.nodeIndex[nodeName]
	if !exists {
		b.errors = append(b.errors, fmt.Errorf("unknown node for output mapping: %s", nodeName))
		return b
	}

	b.nodes[idx].Outputs = append(b.nodes[idx].Outputs, mappings...)
	return b
}

// WithInputMapping appends input mappings to the named node.
func (b *DAGBuilder) WithInputMapping(nodeName string, mappings ...payload.FunctionInputMapping) *DAGBuilder {
	idx, exists := b.nodeIndex[nodeName]
	if !exists {
		b.errors = append(b.errors, fmt.Errorf("unknown node for input mapping: %s", nodeName))
		return b
	}

	b.nodes[idx].Inputs = append(b.nodes[idx].Inputs, mappings...)
	return b
}

// WithDataMapping sets the data mapping on the named node.
func (b *DAGBuilder) WithDataMapping(nodeName, fromNode string) *DAGBuilder {
	idx, exists := b.nodeIndex[nodeName]
	if !exists {
		b.errors = append(b.errors, fmt.Errorf("unknown node for data mapping: %s", nodeName))
		return b
	}

	b.nodes[idx].DataInput = &payload.DataMapping{
		FromNode: fromNode,
	}
	return b
}

// FailFast configures fail-fast behavior for the DAG workflow.
func (b *DAGBuilder) FailFast(ff bool) *DAGBuilder {
	b.failFast = ff
	return b
}

// MaxParallel sets the maximum number of parallel node executions.
func (b *DAGBuilder) MaxParallel(max int) *DAGBuilder {
	b.maxParallel = max
	return b
}

// BuildDAG creates the DAG workflow input, returning an error if validation fails.
func (b *DAGBuilder) BuildDAG() (*payload.DAGWorkflowInput, error) {
	if len(b.errors) > 0 {
		return nil, b.errors[0]
	}

	if len(b.nodes) == 0 {
		return nil, fmt.Errorf("DAG workflow requires at least one node")
	}

	input := &payload.DAGWorkflowInput{
		Nodes:       b.nodes,
		FailFast:    b.failFast,
		MaxParallel: b.maxParallel,
	}

	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("DAG validation failed: %w", err)
	}

	return input, nil
}
