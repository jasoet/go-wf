package builder

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jasoet/go-wf/v2/datasync"
	"github.com/jasoet/go-wf/v2/workflow/store"
)

func TestSyncJobBuilder_OptionalSetters(t *testing.T) {
	local, err := store.NewLocalStore(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = local.Close() })

	b := NewSyncJobBuilder[int, int]("opts-job").
		WithHeartbeatTimeout(15 * time.Second).
		WithRetryInitialInterval(time.Second).
		WithRetryBackoffCoefficient(3.5).
		WithRetryMaxInterval(time.Minute).
		WithStore(local)

	assert.Equal(t, 15*time.Second, b.heartbeatTimeout)
	assert.Equal(t, time.Second, b.retryInitialInterval)
	assert.Equal(t, 3.5, b.retryBackoffCoefficient)
	assert.Equal(t, time.Minute, b.retryMaxInterval)
	assert.Same(t, local, b.store)
}

func TestSyncJobBuilder_Build_WithAllOptions(t *testing.T) {
	local, err := store.NewLocalStore(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = local.Close() })

	def, err := NewSyncJobBuilder[int, int]("full-job").
		WithSource(&mockSource[int]{name: "src"}).
		WithMapper(datasync.IdentityMapper[int]()).
		WithSink(&mockSink[int]{name: "dst"}).
		WithSchedule(time.Minute).
		WithHeartbeatTimeout(5 * time.Second).
		WithRetryInitialInterval(2 * time.Second).
		WithRetryBackoffCoefficient(1.5).
		WithRetryMaxInterval(30 * time.Second).
		WithStore(local).
		Build()

	require.NoError(t, err)
	assert.Equal(t, "full-job", def.Name)
}
