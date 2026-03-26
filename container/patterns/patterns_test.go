package patterns

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildTestDeploy(t *testing.T) {
	input, err := BuildTestDeploy("golang:1.25", "golang:1.25", "deployer:v1")
	require.NoError(t, err)
	assert.NotNil(t, input)
	assert.Equal(t, 3, len(input.Containers))
	assert.True(t, input.StopOnError)
}

func TestBuildTestDeployWithHealthCheck(t *testing.T) {
	input, err := BuildTestDeployWithHealthCheck(
		"golang:1.25",
		"deployer:v1",
		"https://myapp.com/health")
	require.NoError(t, err)
	assert.NotNil(t, input)
	assert.Equal(t, 4, len(input.Containers))
	assert.True(t, input.StopOnError)
}

func TestBuildTestDeployWithNotification(t *testing.T) {
	input, err := BuildTestDeployWithNotification(
		"golang:1.25",
		"deployer:v1",
		"https://hooks.slack.com/services/...",
		`{"text": "Deploy complete"}`)
	require.NoError(t, err)
	assert.NotNil(t, input)
	assert.True(t, input.StopOnError)
}

func TestMultiEnvironmentDeploy(t *testing.T) {
	input, err := MultiEnvironmentDeploy("deployer:v1", []string{"staging", "production"})
	require.NoError(t, err)
	assert.NotNil(t, input)
	assert.Equal(t, 2, len(input.Containers))
	assert.True(t, input.StopOnError)
}

func TestFanOutFanIn(t *testing.T) {
	tests := []struct {
		name        string
		tasks       []string
		expectError bool
	}{
		{
			name:        "valid tasks",
			tasks:       []string{"task-1", "task-2", "task-3"},
			expectError: false,
		},
		{
			name:        "empty tasks",
			tasks:       []string{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input, err := FanOutFanIn("alpine:latest", tt.tasks)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, input)
				assert.Equal(t, len(tt.tasks), len(input.Containers))
			}
		})
	}
}

func TestParallelDataProcessing(t *testing.T) {
	input, err := ParallelDataProcessing(
		"processor:v1",
		[]string{"data-1.csv", "data-2.csv", "data-3.csv"},
		"process.sh")
	require.NoError(t, err)
	assert.NotNil(t, input)
	assert.Equal(t, 3, len(input.Containers))
}

func TestParallelTestSuite(t *testing.T) {
	input, err := ParallelTestSuite(
		"golang:1.25",
		map[string]string{
			"unit":        "go test ./internal/...",
			"integration": "go test ./tests/integration/...",
		})
	require.NoError(t, err)
	assert.NotNil(t, input)
	assert.Equal(t, 2, len(input.Containers))
	assert.Equal(t, "fail_fast", input.FailureStrategy)
}

func TestParallelDeployment(t *testing.T) {
	input, err := ParallelDeployment("deployer:v1", []string{"us-west", "us-east", "eu-central"})
	require.NoError(t, err)
	assert.NotNil(t, input)
	assert.Equal(t, 3, len(input.Containers))
	assert.Equal(t, "continue", input.FailureStrategy)
}

func TestMapReduce(t *testing.T) {
	input, err := MapReduce(
		"alpine:latest",
		[]string{"file1.txt", "file2.txt"},
		"wc -w",
		"awk '{sum+=$1} END {print sum}'")
	require.NoError(t, err)
	assert.NotNil(t, input)
	// Should have map tasks + reduce task
	assert.True(t, len(input.Containers) >= 3)
}

func TestPatternEdgeCases(t *testing.T) {
	t.Run("empty data items", func(t *testing.T) {
		_, err := ParallelDataProcessing("processor:v1", []string{}, "process.sh")
		assert.Error(t, err)
	})

	t.Run("empty test suites", func(t *testing.T) {
		_, err := ParallelTestSuite("golang:1.25", map[string]string{})
		assert.Error(t, err)
	})

	t.Run("empty regions", func(t *testing.T) {
		_, err := ParallelDeployment("deployer:v1", []string{})
		assert.Error(t, err)
	})

	t.Run("empty map inputs", func(t *testing.T) {
		_, err := MapReduce("alpine:latest", []string{}, "wc -w", "sum")
		assert.Error(t, err)
	})
}
