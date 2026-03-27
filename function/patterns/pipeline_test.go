package patterns

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestETLPipeline(t *testing.T) {
	input, err := ETLPipeline("s3://bucket/data", "json", "postgres://db/table")
	require.NoError(t, err)
	assert.NotNil(t, input)
	assert.Equal(t, 3, len(input.Tasks))
	assert.True(t, input.StopOnError)
	assert.Equal(t, "extract", input.Tasks[0].Name)
	assert.Equal(t, "etl-transform", input.Tasks[1].Name)
	assert.Equal(t, "load", input.Tasks[2].Name)
}

func TestValidateTransformNotify(t *testing.T) {
	input, err := ValidateTransformNotify("user@example.com", "report", "#alerts")
	require.NoError(t, err)
	assert.NotNil(t, input)
	assert.Equal(t, 3, len(input.Tasks))
	assert.True(t, input.StopOnError)
	assert.Equal(t, "validate", input.Tasks[0].Name)
	assert.Equal(t, "transform", input.Tasks[1].Name)
	assert.Equal(t, "notify", input.Tasks[2].Name)
}

func TestMultiEnvironmentDeploy(t *testing.T) {
	environments := []string{"staging", "production", "canary"}
	input, err := MultiEnvironmentDeploy("v1.2.3", environments)
	require.NoError(t, err)
	assert.NotNil(t, input)
	assert.Equal(t, len(environments), len(input.Tasks))
	assert.True(t, input.StopOnError)
}

func TestMultiEnvironmentDeploy_Empty(t *testing.T) {
	_, err := MultiEnvironmentDeploy("v1.0.0", []string{})
	assert.Error(t, err)
}
