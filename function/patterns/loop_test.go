package patterns

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBatchProcess(t *testing.T) {
	items := []string{"file1.csv", "file2.csv", "file3.csv"}
	input, err := BatchProcess(items, "process-file")

	require.NoError(t, err)
	require.NotNil(t, input)
	assert.Equal(t, items, input.Items)
	assert.Equal(t, "process-file", input.Template.Name)
	assert.True(t, input.Parallel)
	assert.Equal(t, "continue", input.FailureStrategy)
	assert.Equal(t, "{{item}}", input.Template.Args["file"])
}

func TestBatchProcess_Empty(t *testing.T) {
	input, err := BatchProcess([]string{}, "process-file")

	assert.Error(t, err)
	assert.Nil(t, input)
}

func TestSequentialMigration(t *testing.T) {
	migrations := []string{"001_create_users.sql", "002_add_index.sql", "003_seed_data.sql"}
	input, err := SequentialMigration(migrations)

	require.NoError(t, err)
	require.NotNil(t, input)
	assert.Equal(t, migrations, input.Items)
	assert.Equal(t, "run-migration", input.Template.Name)
	assert.False(t, input.Parallel)
	assert.Equal(t, "fail_fast", input.FailureStrategy)
	assert.Equal(t, "{{item}}", input.Template.Args["migration"])
}

func TestSequentialMigration_Empty(t *testing.T) {
	input, err := SequentialMigration([]string{})

	assert.Error(t, err)
	assert.Nil(t, input)
}

func TestMultiRegionDeploy(t *testing.T) {
	environments := []string{"dev", "staging", "prod"}
	regions := []string{"us-west", "us-east", "eu-central"}
	version := "v1.2.3"

	input, err := MultiRegionDeploy(environments, regions, version)

	require.NoError(t, err)
	require.NotNil(t, input)
	assert.Equal(t, environments, input.Parameters["environment"])
	assert.Equal(t, regions, input.Parameters["region"])
	assert.Equal(t, "deploy-service", input.Template.Name)
	assert.Equal(t, version, input.Template.Args["version"])
	assert.Equal(t, "{{.environment}}", input.Template.Args["environment"])
	assert.Equal(t, "{{.region}}", input.Template.Args["region"])
	assert.True(t, input.Parallel)
	assert.Equal(t, "fail_fast", input.FailureStrategy)
}

func TestMultiRegionDeploy_EmptyEnvironments(t *testing.T) {
	input, err := MultiRegionDeploy([]string{}, []string{"us-west"}, "v1.0.0")

	assert.Error(t, err)
	assert.Nil(t, input)
}

func TestMultiRegionDeploy_EmptyRegions(t *testing.T) {
	input, err := MultiRegionDeploy([]string{"prod"}, []string{}, "v1.0.0")

	assert.Error(t, err)
	assert.Nil(t, input)
}

func TestParameterSweep(t *testing.T) {
	params := map[string][]string{
		"learning_rate": {"0.001", "0.01", "0.1"},
		"batch_size":    {"32", "64", "128"},
	}

	input, err := ParameterSweep(params, "train-model", 5)

	require.NoError(t, err)
	require.NotNil(t, input)
	assert.Equal(t, params, input.Parameters)
	assert.Equal(t, "train-model", input.Template.Name)
	assert.True(t, input.Parallel)
	assert.Equal(t, 5, input.MaxConcurrency)
	assert.Equal(t, "continue", input.FailureStrategy)
	// Check that all parameter keys are mapped in template args
	for key := range params {
		assert.Contains(t, input.Template.Args, key)
	}
}

func TestParameterSweep_Empty(t *testing.T) {
	input, err := ParameterSweep(map[string][]string{}, "train-model", 5)

	assert.Error(t, err)
	assert.Nil(t, input)
}
