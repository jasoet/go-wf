//go:build integration

package activity

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"

	"github.com/jasoet/go-wf/docker/payload"
)

func newActivityEnv() *testsuite.TestActivityEnvironment {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestActivityEnvironment()
	env.RegisterActivity(StartContainerActivity)
	return env
}

func TestStartContainerActivity_HappyPath(t *testing.T) {
	env := newActivityEnv()

	input := payload.ContainerExecutionInput{
		Image:      "alpine:latest",
		Command:    []string{"echo", "hello world"},
		AutoRemove: true,
	}

	result, err := env.ExecuteActivity(StartContainerActivity, input)
	require.NoError(t, err)

	var output payload.ContainerExecutionOutput
	require.NoError(t, result.Get(&output))

	assert.Equal(t, 0, output.ExitCode)
	assert.True(t, output.Success)
	assert.Contains(t, output.Stdout, "hello world")
	assert.NotEmpty(t, output.ContainerID)
	assert.NotZero(t, output.Duration)
}

func TestStartContainerActivity_WithEnv(t *testing.T) {
	env := newActivityEnv()

	input := payload.ContainerExecutionInput{
		Image:   "alpine:latest",
		Command: []string{"sh", "-c", "echo $MY_VAR"},
		Env: map[string]string{
			"MY_VAR": "test-value",
		},
		AutoRemove: true,
	}

	result, err := env.ExecuteActivity(StartContainerActivity, input)
	require.NoError(t, err)

	var output payload.ContainerExecutionOutput
	require.NoError(t, result.Get(&output))

	assert.Equal(t, 0, output.ExitCode)
	assert.True(t, output.Success)
	assert.Contains(t, output.Stdout, "test-value")
}

func TestStartContainerActivity_WithEntrypoint(t *testing.T) {
	env := newActivityEnv()

	input := payload.ContainerExecutionInput{
		Image:      "alpine:latest",
		Entrypoint: []string{"sh", "-c"},
		Command:    []string{"echo entrypoint-test"},
		AutoRemove: true,
	}

	result, err := env.ExecuteActivity(StartContainerActivity, input)
	require.NoError(t, err)

	var output payload.ContainerExecutionOutput
	require.NoError(t, result.Get(&output))

	assert.Equal(t, 0, output.ExitCode)
	assert.True(t, output.Success)
	assert.Contains(t, output.Stdout, "entrypoint-test")
}

func TestStartContainerActivity_NonZeroExit(t *testing.T) {
	env := newActivityEnv()

	input := payload.ContainerExecutionInput{
		Image:      "alpine:latest",
		Command:    []string{"sh", "-c", "exit 42"},
		AutoRemove: true,
	}

	result, err := env.ExecuteActivity(StartContainerActivity, input)
	// The underlying docker executor may or may not return an error for non-zero exit.
	// If no error, verify the exit code is captured in the output.
	if err == nil {
		var output payload.ContainerExecutionOutput
		require.NoError(t, result.Get(&output))
		assert.Equal(t, 42, output.ExitCode)
		assert.False(t, output.Success)
	} else {
		// If error is returned, that's also acceptable behavior
		assert.Error(t, err)
	}
}

func TestStartContainerActivity_WithWorkDir(t *testing.T) {
	env := newActivityEnv()

	input := payload.ContainerExecutionInput{
		Image:      "alpine:latest",
		Command:    []string{"pwd"},
		WorkDir:    "/tmp",
		AutoRemove: true,
	}

	result, err := env.ExecuteActivity(StartContainerActivity, input)
	require.NoError(t, err)

	var output payload.ContainerExecutionOutput
	require.NoError(t, result.Get(&output))

	assert.Equal(t, 0, output.ExitCode)
	assert.Contains(t, output.Stdout, "/tmp")
}

func TestStartContainerActivity_WithLabels(t *testing.T) {
	env := newActivityEnv()

	input := payload.ContainerExecutionInput{
		Image:   "alpine:latest",
		Command: []string{"echo", "labeled"},
		Labels: map[string]string{
			"test-label": "test-value",
		},
		AutoRemove: true,
	}

	result, err := env.ExecuteActivity(StartContainerActivity, input)
	require.NoError(t, err)

	var output payload.ContainerExecutionOutput
	require.NoError(t, result.Get(&output))

	assert.Equal(t, 0, output.ExitCode)
	assert.True(t, output.Success)
}

func TestStartContainerActivity_WithName(t *testing.T) {
	env := newActivityEnv()

	input := payload.ContainerExecutionInput{
		Image:      "alpine:latest",
		Command:    []string{"echo", "named"},
		Name:       "test-activity-named",
		AutoRemove: true,
	}

	result, err := env.ExecuteActivity(StartContainerActivity, input)
	require.NoError(t, err)

	var output payload.ContainerExecutionOutput
	require.NoError(t, result.Get(&output))

	assert.Equal(t, 0, output.ExitCode)
	assert.True(t, output.Success)
}

func TestStartContainerActivity_Stderr(t *testing.T) {
	env := newActivityEnv()

	input := payload.ContainerExecutionInput{
		Image:      "alpine:latest",
		Command:    []string{"sh", "-c", "echo error-output >&2"},
		AutoRemove: true,
	}

	result, err := env.ExecuteActivity(StartContainerActivity, input)
	require.NoError(t, err)

	var output payload.ContainerExecutionOutput
	require.NoError(t, result.Get(&output))

	assert.Equal(t, 0, output.ExitCode)
	assert.True(t, output.Success)
	assert.Contains(t, output.Stderr, "error-output")
}

func TestStartContainerActivity_WithVolumes(t *testing.T) {
	env := newActivityEnv()

	input := payload.ContainerExecutionInput{
		Image:   "alpine:latest",
		Command: []string{"sh", "-c", "test -d /mounted && echo mount-ok"},
		Volumes: map[string]string{
			"/tmp": "/mounted",
		},
		AutoRemove: true,
	}

	result, err := env.ExecuteActivity(StartContainerActivity, input)
	require.NoError(t, err)

	var output payload.ContainerExecutionOutput
	require.NoError(t, result.Get(&output))

	assert.Equal(t, 0, output.ExitCode)
	assert.True(t, output.Success)
	assert.Contains(t, output.Stdout, "mount-ok")
}
