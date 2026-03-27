package workflow

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDAGInput_Validate(t *testing.T) {
	t.Run("valid simple DAG (A -> B -> C)", func(t *testing.T) {
		dag := &DAGInput[testInput, testOutput]{
			Nodes: []DAGNode[testInput, testOutput]{
				{Name: "A", Input: testInput{Value: "a"}},
				{Name: "B", Input: testInput{Value: "b"}, Dependencies: []string{"A"}},
				{Name: "C", Input: testInput{Value: "c"}, Dependencies: []string{"B"}},
			},
		}
		err := dag.Validate()
		assert.NoError(t, err)
	})

	t.Run("valid parallel DAG (A, B independent, C depends on both)", func(t *testing.T) {
		dag := &DAGInput[testInput, testOutput]{
			Nodes: []DAGNode[testInput, testOutput]{
				{Name: "A", Input: testInput{Value: "a"}},
				{Name: "B", Input: testInput{Value: "b"}},
				{Name: "C", Input: testInput{Value: "c"}, Dependencies: []string{"A", "B"}},
			},
		}
		err := dag.Validate()
		assert.NoError(t, err)
	})

	t.Run("invalid: empty nodes", func(t *testing.T) {
		dag := &DAGInput[testInput, testOutput]{
			Nodes: []DAGNode[testInput, testOutput]{},
		}
		err := dag.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least one node is required")
	})

	t.Run("invalid: duplicate node names", func(t *testing.T) {
		dag := &DAGInput[testInput, testOutput]{
			Nodes: []DAGNode[testInput, testOutput]{
				{Name: "A", Input: testInput{Value: "a"}},
				{Name: "A", Input: testInput{Value: "b"}},
			},
		}
		err := dag.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate node name: A")
	})

	t.Run("invalid: dependency references non-existent node", func(t *testing.T) {
		dag := &DAGInput[testInput, testOutput]{
			Nodes: []DAGNode[testInput, testOutput]{
				{Name: "A", Input: testInput{Value: "a"}, Dependencies: []string{"Z"}},
			},
		}
		err := dag.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "dependency node not found: Z")
	})

	t.Run("invalid: simple cycle (A -> B -> A)", func(t *testing.T) {
		dag := &DAGInput[testInput, testOutput]{
			Nodes: []DAGNode[testInput, testOutput]{
				{Name: "A", Input: testInput{Value: "a"}, Dependencies: []string{"B"}},
				{Name: "B", Input: testInput{Value: "b"}, Dependencies: []string{"A"}},
			},
		}
		err := dag.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "circular dependency")
	})

	t.Run("invalid: self-reference (A -> A)", func(t *testing.T) {
		dag := &DAGInput[testInput, testOutput]{
			Nodes: []DAGNode[testInput, testOutput]{
				{Name: "A", Input: testInput{Value: "a"}, Dependencies: []string{"A"}},
			},
		}
		err := dag.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "circular dependency")
	})

	t.Run("invalid: complex cycle (A -> B -> C -> A)", func(t *testing.T) {
		dag := &DAGInput[testInput, testOutput]{
			Nodes: []DAGNode[testInput, testOutput]{
				{Name: "A", Input: testInput{Value: "a"}, Dependencies: []string{"C"}},
				{Name: "B", Input: testInput{Value: "b"}, Dependencies: []string{"A"}},
				{Name: "C", Input: testInput{Value: "c"}, Dependencies: []string{"B"}},
			},
		}
		err := dag.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "circular dependency")
	})

	t.Run("valid: diamond dependency (A -> B, A -> C, B -> D, C -> D)", func(t *testing.T) {
		dag := &DAGInput[testInput, testOutput]{
			Nodes: []DAGNode[testInput, testOutput]{
				{Name: "A", Input: testInput{Value: "a"}},
				{Name: "B", Input: testInput{Value: "b"}, Dependencies: []string{"A"}},
				{Name: "C", Input: testInput{Value: "c"}, Dependencies: []string{"A"}},
				{Name: "D", Input: testInput{Value: "d"}, Dependencies: []string{"B", "C"}},
			},
		}
		err := dag.Validate()
		assert.NoError(t, err)
	})

	t.Run("invalid: node input validation failure", func(t *testing.T) {
		dag := &DAGInput[testInput, testOutput]{
			Nodes: []DAGNode[testInput, testOutput]{
				{Name: "A", Input: testInput{Value: ""}},
			},
		}
		err := dag.Validate()
		require.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "node A"))
	})
}
