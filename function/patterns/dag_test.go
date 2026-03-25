package patterns

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestETLWithValidation(t *testing.T) {
	input, err := ETLWithValidation("database", "parquet", "warehouse")
	require.NoError(t, err)
	require.NotNil(t, input)
	assert.Equal(t, 4, len(input.Nodes))
	assert.True(t, input.FailFast)

	// validate-config and extract have no dependencies
	assert.Equal(t, "validate-config", input.Nodes[0].Name)
	assert.Empty(t, input.Nodes[0].Dependencies)
	assert.Equal(t, "extract", input.Nodes[1].Name)
	assert.Empty(t, input.Nodes[1].Dependencies)

	// transform depends on validate-config and extract
	assert.Equal(t, "transform", input.Nodes[2].Name)
	assert.ElementsMatch(t, []string{"validate-config", "extract"}, input.Nodes[2].Dependencies)

	// load depends on transform
	assert.Equal(t, "load", input.Nodes[3].Name)
	assert.Equal(t, []string{"transform"}, input.Nodes[3].Dependencies)
}

func TestCIPipeline(t *testing.T) {
	input, err := CIPipeline()
	require.NoError(t, err)
	require.NotNil(t, input)
	assert.Equal(t, 4, len(input.Nodes))
	assert.True(t, input.FailFast)

	// compile has output mappings
	assert.Equal(t, "compile", input.Nodes[0].Name)
	assert.Empty(t, input.Nodes[0].Dependencies)
	require.Len(t, input.Nodes[0].Outputs, 1)
	assert.Equal(t, "artifact", input.Nodes[0].Outputs[0].Name)
	assert.Equal(t, "artifact", input.Nodes[0].Outputs[0].ResultKey)

	// unit-test and lint depend on compile
	assert.Equal(t, "unit-test", input.Nodes[1].Name)
	assert.Equal(t, []string{"compile"}, input.Nodes[1].Dependencies)
	assert.Equal(t, "lint", input.Nodes[2].Name)
	assert.Equal(t, []string{"compile"}, input.Nodes[2].Dependencies)

	// publish depends on unit-test and lint, has input mapping
	assert.Equal(t, "publish", input.Nodes[3].Name)
	assert.ElementsMatch(t, []string{"unit-test", "lint"}, input.Nodes[3].Dependencies)
	require.Len(t, input.Nodes[3].Inputs, 1)
	assert.Equal(t, "artifact_path", input.Nodes[3].Inputs[0].Name)
	assert.Equal(t, "compile.artifact", input.Nodes[3].Inputs[0].From)
}
