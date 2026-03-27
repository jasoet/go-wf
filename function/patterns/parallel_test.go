package patterns

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFanOutFanIn(t *testing.T) {
	names := []string{"task-a", "task-b", "task-c"}
	input, err := FanOutFanIn(names)

	require.NoError(t, err)
	assert.Len(t, input.Tasks, 3)
	assert.Equal(t, "task-a", input.Tasks[0].Name)
	assert.Equal(t, "task-b", input.Tasks[1].Name)
	assert.Equal(t, "task-c", input.Tasks[2].Name)
}

func TestFanOutFanIn_Empty(t *testing.T) {
	input, err := FanOutFanIn([]string{})

	assert.Error(t, err)
	assert.Nil(t, input)
}

func TestParallelDataFetch(t *testing.T) {
	input, err := ParallelDataFetch()

	require.NoError(t, err)
	assert.Len(t, input.Tasks, 3)

	expected := []string{"fetch-users", "fetch-orders", "fetch-inventory"}
	for i, name := range expected {
		assert.Equal(t, name, input.Tasks[i].Name)
	}
}

func TestParallelHealthCheck(t *testing.T) {
	services := []string{"api", "database", "cache"}
	input, err := ParallelHealthCheck(services, "production")

	require.NoError(t, err)
	assert.Len(t, input.Tasks, 3)
	assert.Equal(t, "fail_fast", input.FailureStrategy)

	for i, svc := range services {
		assert.Equal(t, "health-check", input.Tasks[i].Name)
		assert.Equal(t, svc, input.Tasks[i].Args["service"])
		assert.Equal(t, "production", input.Tasks[i].Args["environment"])
	}
}

func TestParallelHealthCheck_Empty(t *testing.T) {
	input, err := ParallelHealthCheck([]string{}, "production")

	assert.Error(t, err)
	assert.Nil(t, input)
}
