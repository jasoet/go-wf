package builder

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jasoet/go-wf/datasync"
)

type mockSource[T any] struct{ name string }

func (m *mockSource[T]) Name() string                         { return m.name }
func (m *mockSource[T]) Fetch(_ context.Context) ([]T, error) { return nil, nil }

type mockSink[U any] struct{ name string }

func (m *mockSink[U]) Name() string { return m.name }
func (m *mockSink[U]) Write(_ context.Context, _ []U) (datasync.WriteResult, error) {
	return datasync.WriteResult{}, nil
}

func TestSyncJobBuilder_Build_Success(t *testing.T) {
	job, err := NewSyncJobBuilder[int, int]("test-job").
		WithSource(&mockSource[int]{name: "src"}).
		WithMapper(datasync.IdentityMapper[int]()).
		WithSink(&mockSink[int]{name: "dst"}).
		WithSchedule(5 * time.Minute).
		WithMetadata(map[string]string{"key": "val"}).
		WithActivityTimeout(10 * time.Minute).
		WithMaxRetries(5).
		Build()

	require.NoError(t, err)
	assert.Equal(t, "test-job", job.Name)
	assert.Equal(t, 5*time.Minute, job.Schedule)
	assert.Equal(t, 10*time.Minute, job.ActivityTimeout)
	assert.Equal(t, int32(5), job.MaxRetries)
}

func TestSyncJobBuilder_Build_MissingName(t *testing.T) {
	_, err := NewSyncJobBuilder[int, int]("").
		WithSource(&mockSource[int]{name: "src"}).
		WithMapper(datasync.IdentityMapper[int]()).
		WithSink(&mockSink[int]{name: "dst"}).
		WithSchedule(5 * time.Minute).
		Build()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "job name")
}

func TestSyncJobBuilder_Build_MissingSource(t *testing.T) {
	_, err := NewSyncJobBuilder[int, int]("test").
		WithMapper(datasync.IdentityMapper[int]()).
		WithSink(&mockSink[int]{name: "dst"}).
		WithSchedule(5 * time.Minute).
		Build()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "source")
}

func TestSyncJobBuilder_Build_MissingMapper(t *testing.T) {
	_, err := NewSyncJobBuilder[int, int]("test").
		WithSource(&mockSource[int]{name: "src"}).
		WithSink(&mockSink[int]{name: "dst"}).
		WithSchedule(5 * time.Minute).
		Build()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mapper")
}

func TestSyncJobBuilder_Build_MissingSink(t *testing.T) {
	_, err := NewSyncJobBuilder[int, int]("test").
		WithSource(&mockSource[int]{name: "src"}).
		WithMapper(datasync.IdentityMapper[int]()).
		WithSchedule(5 * time.Minute).
		Build()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "sink")
}

func TestSyncJobBuilder_Build_InvalidSchedule(t *testing.T) {
	_, err := NewSyncJobBuilder[int, int]("test").
		WithSource(&mockSource[int]{name: "src"}).
		WithMapper(datasync.IdentityMapper[int]()).
		WithSink(&mockSink[int]{name: "dst"}).
		Build()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "schedule")
}
