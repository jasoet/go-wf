package builder

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jasoet/go-wf/function/payload"
)

func TestDAGBuilder_SimpleTwoNodeDAG(t *testing.T) {
	dag, err := NewDAGBuilder("two-node").
		AddNodeWithInput("step1", payload.FunctionExecutionInput{Name: "func-a"}).
		AddNodeWithInput("step2", payload.FunctionExecutionInput{Name: "func-b"}, "step1").
		BuildDAG()

	require.NoError(t, err)
	require.NotNil(t, dag)

	assert.Len(t, dag.Nodes, 2)
	assert.Equal(t, "step1", dag.Nodes[0].Name)
	assert.Empty(t, dag.Nodes[0].Dependencies)
	assert.Equal(t, "step2", dag.Nodes[1].Name)
	assert.Equal(t, []string{"step1"}, dag.Nodes[1].Dependencies)
}

func TestDAGBuilder_WithWorkflowSource(t *testing.T) {
	source := NewFunctionSource(payload.FunctionExecutionInput{
		Name: "source-func",
		Args: map[string]string{"key": "value"},
	})

	dag, err := NewDAGBuilder("source-dag").
		AddNode("node1", source).
		BuildDAG()

	require.NoError(t, err)
	require.NotNil(t, dag)

	assert.Len(t, dag.Nodes, 1)
	assert.Equal(t, "source-func", dag.Nodes[0].Function.Name)
	assert.Equal(t, "value", dag.Nodes[0].Function.Args["key"])
}

func TestDAGBuilder_WithOutputAndInputMappings(t *testing.T) {
	dag, err := NewDAGBuilder("mapped-dag").
		AddNodeWithInput("producer", payload.FunctionExecutionInput{Name: "produce"}).
		WithOutputMapping("producer",
			payload.OutputMapping{Name: "result_file", ResultKey: "output_path"},
		).
		AddNodeWithInput("consumer", payload.FunctionExecutionInput{Name: "consume"}, "producer").
		WithInputMapping("consumer",
			payload.FunctionInputMapping{Name: "input_path", From: "producer.result_file", Required: true},
		).
		BuildDAG()

	require.NoError(t, err)
	require.NotNil(t, dag)

	assert.Len(t, dag.Nodes[0].Outputs, 1)
	assert.Equal(t, "result_file", dag.Nodes[0].Outputs[0].Name)
	assert.Equal(t, "output_path", dag.Nodes[0].Outputs[0].ResultKey)

	assert.Len(t, dag.Nodes[1].Inputs, 1)
	assert.Equal(t, "input_path", dag.Nodes[1].Inputs[0].Name)
	assert.Equal(t, "producer.result_file", dag.Nodes[1].Inputs[0].From)
	assert.True(t, dag.Nodes[1].Inputs[0].Required)
}

func TestDAGBuilder_WithDataMapping(t *testing.T) {
	dag, err := NewDAGBuilder("data-dag").
		AddNodeWithInput("generator", payload.FunctionExecutionInput{Name: "gen"}).
		AddNodeWithInput("processor", payload.FunctionExecutionInput{Name: "proc"}, "generator").
		WithDataMapping("processor", "generator").
		BuildDAG()

	require.NoError(t, err)
	require.NotNil(t, dag)

	require.NotNil(t, dag.Nodes[1].DataInput)
	assert.Equal(t, "generator", dag.Nodes[1].DataInput.FromNode)
}

func TestDAGBuilder_FailFastAndMaxParallel(t *testing.T) {
	dag, err := NewDAGBuilder("config-dag").
		AddNodeWithInput("node1", payload.FunctionExecutionInput{Name: "f1"}).
		AddNodeWithInput("node2", payload.FunctionExecutionInput{Name: "f2"}).
		FailFast(true).
		MaxParallel(3).
		BuildDAG()

	require.NoError(t, err)
	require.NotNil(t, dag)

	assert.True(t, dag.FailFast)
	assert.Equal(t, 3, dag.MaxParallel)
}

func TestDAGBuilder_EmptyDAGError(t *testing.T) {
	_, err := NewDAGBuilder("empty").BuildDAG()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least one node")
}

func TestDAGBuilder_NilSourceError(t *testing.T) {
	_, err := NewDAGBuilder("nil-source").
		AddNode("bad", nil).
		BuildDAG()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nil source")
}

func TestDAGBuilder_MappingToUnknownNodeError(t *testing.T) {
	_, err := NewDAGBuilder("unknown-node").
		AddNodeWithInput("node1", payload.FunctionExecutionInput{Name: "f1"}).
		WithOutputMapping("nonexistent",
			payload.OutputMapping{Name: "out", ResultKey: "key"},
		).
		BuildDAG()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown node")

	// Also test input mapping to unknown node.
	_, err = NewDAGBuilder("unknown-input").
		AddNodeWithInput("node1", payload.FunctionExecutionInput{Name: "f1"}).
		WithInputMapping("nonexistent",
			payload.FunctionInputMapping{Name: "in", From: "node1.out"},
		).
		BuildDAG()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown node")

	// Also test data mapping to unknown node.
	_, err = NewDAGBuilder("unknown-data").
		AddNodeWithInput("node1", payload.FunctionExecutionInput{Name: "f1"}).
		WithDataMapping("nonexistent", "node1").
		BuildDAG()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown node")
}
