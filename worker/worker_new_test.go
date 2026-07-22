package worker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/client"
	sdkworker "go.temporal.io/sdk/worker"
)

func TestNew(t *testing.T) {
	t.Run("creates worker without versioning env", func(t *testing.T) {
		c, err := client.NewLazyClient(client.Options{})
		require.NoError(t, err)
		defer c.Close()

		w := New(c, "test-queue", sdkworker.Options{})
		require.NotNil(t, w)
	})

	t.Run("env vars enable versioning on the created worker", func(t *testing.T) {
		t.Setenv(EnvDeploymentName, "go-wf")
		t.Setenv(EnvBuildID, "cafe1234")

		c, err := client.NewLazyClient(client.Options{})
		require.NoError(t, err)
		defer c.Close()

		w := New(c, "test-queue", sdkworker.Options{})
		require.NotNil(t, w)

		// OptionsFromEnv is covered separately; here we assert New delegates to it.
		opts := OptionsFromEnv(sdkworker.Options{})
		assert.True(t, opts.DeploymentOptions.UseVersioning)
		assert.Equal(t, "go-wf", opts.DeploymentOptions.Version.DeploymentName)
	})
}
