package chunk

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/jasoet/go-wf/datasync"
)

type stubPartitioner struct {
	parts []Partition[int64]
}

func (s *stubPartitioner) Partitions(_ context.Context) ([]Partition[int64], error) {
	return s.parts, nil
}

func TestChunkedSync_Build_RequiresPartitioner(t *testing.T) {
	defer func() { assert.NotNil(t, recover()) }()
	_ = NewChunkedSync[string, string, int64]("job-x").
		Fetcher(func(_ context.Context, _, _ int64) ([]string, error) { return nil, nil }).
		Mapper(datasync.IdentityMapper[string]()).
		Sink(&stubSink{name: "sink"}).
		Build()
}

func TestChunkedSync_Build_RequiresFetcher(t *testing.T) {
	defer func() { assert.NotNil(t, recover()) }()
	_ = NewChunkedSync[string, string, int64]("job-x").
		Partitioner(&stubPartitioner{}).
		Mapper(datasync.IdentityMapper[string]()).
		Sink(&stubSink{name: "sink"}).
		Build()
}

func TestChunkedSync_Build_RequiresMapper(t *testing.T) {
	defer func() { assert.NotNil(t, recover()) }()
	_ = NewChunkedSync[string, string, int64]("job-x").
		Partitioner(&stubPartitioner{}).
		Fetcher(func(_ context.Context, _, _ int64) ([]string, error) { return nil, nil }).
		Sink(&stubSink{name: "sink"}).
		Build()
}

func TestChunkedSync_Build_RequiresSink(t *testing.T) {
	defer func() { assert.NotNil(t, recover()) }()
	_ = NewChunkedSync[string, string, int64]("job-x").
		Partitioner(&stubPartitioner{}).
		Fetcher(func(_ context.Context, _, _ int64) ([]string, error) { return nil, nil }).
		Mapper(datasync.IdentityMapper[string]()).
		Build()
}

func TestChunkedSync_Build_PopulatesRegistration(t *testing.T) {
	reg := NewChunkedSync[string, string, int64]("job-x").
		Partitioner(&stubPartitioner{}).
		Fetcher(func(_ context.Context, _, _ int64) ([]string, error) { return nil, nil }).
		Mapper(datasync.IdentityMapper[string]()).
		Sink(&stubSink{name: "sink"}).
		Schedule(15 * time.Minute).
		Disabled(true).
		Build()
	assert.Equal(t, "job-x", reg.Name)
	assert.Equal(t, "sync-job-x", reg.TaskQueue)
	assert.Equal(t, 15*time.Minute, reg.Schedule)
	assert.True(t, reg.Disabled)
}
