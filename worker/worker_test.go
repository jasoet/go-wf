package worker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdkworker "go.temporal.io/sdk/worker"
	temporalwf "go.temporal.io/sdk/workflow"
)

func TestVersionedOptions(t *testing.T) {
	opts := VersionedOptions(sdkworker.Options{}, "go-wf", "abc123")
	require.True(t, opts.DeploymentOptions.UseVersioning)
	assert.Equal(t, "go-wf", opts.DeploymentOptions.Version.DeploymentName)
	assert.Equal(t, "abc123", opts.DeploymentOptions.Version.BuildID)
	assert.Equal(t, temporalwf.VersioningBehaviorPinned, opts.DeploymentOptions.DefaultVersioningBehavior)
}

func TestOptionsFromEnv(t *testing.T) {
	t.Run("unset env leaves options untouched", func(t *testing.T) {
		opts := OptionsFromEnv(sdkworker.Options{})
		assert.False(t, opts.DeploymentOptions.UseVersioning)
	})
	t.Run("both vars enable versioning", func(t *testing.T) {
		t.Setenv("TEMPORAL_DEPLOYMENT_NAME", "go-wf")
		t.Setenv("TEMPORAL_BUILD_ID", "deadbeef")
		opts := OptionsFromEnv(sdkworker.Options{})
		assert.True(t, opts.DeploymentOptions.UseVersioning)
		assert.Equal(t, "deadbeef", opts.DeploymentOptions.Version.BuildID)
	})
	t.Run("only one var set leaves options untouched", func(t *testing.T) {
		t.Setenv("TEMPORAL_BUILD_ID", "deadbeef")
		opts := OptionsFromEnv(sdkworker.Options{})
		assert.False(t, opts.DeploymentOptions.UseVersioning)
	})
}
